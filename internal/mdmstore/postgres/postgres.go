package postgres

import (
	"context"
	"crypto/x509"
	"database/sql"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"strings"
	"time"

	"github.com/dwdmsh/school-mdm/internal/mdmstore"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/micromdm/plist"
)

// Store implements mdmstore.Store against schema mdm.
type Store struct {
	db *sql.DB
}

// OpenDSN opens a database/sql pool with search_path=mdm on every connection.
// school migrations must already have created schema mdm.
//
// Neon pooler endpoints ignore startup search_path and break prepared statements;
// we prefer the direct host and also SET search_path after connect.
func OpenDSN(ctx context.Context, databaseURL string) (*Store, *sql.DB, error) {
	cfg, err := pgx.ParseConfig(preferDirectNeonHost(databaseURL))
	if err != nil {
		return nil, nil, fmt.Errorf("parse mdm db url: %w", err)
	}
	if cfg.RuntimeParams == nil {
		cfg.RuntimeParams = map[string]string{}
	}
	cfg.RuntimeParams["search_path"] = "mdm"
	db := stdlib.OpenDB(*cfg, stdlib.OptionAfterConnect(func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, `SET search_path TO mdm, public`)
		return err
	}))
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, nil, fmt.Errorf("ping mdm db: %w", err)
	}
	var sp string
	if err := db.QueryRowContext(ctx, `SHOW search_path`).Scan(&sp); err != nil {
		_ = db.Close()
		return nil, nil, fmt.Errorf("verify search_path: %w", err)
	}
	if !strings.Contains(sp, "mdm") {
		_ = db.Close()
		return nil, nil, fmt.Errorf("search_path=%q does not include mdm (avoid Neon -pooler URL for MDM)", sp)
	}
	return &Store{db: db}, db, nil
}

// preferDirectNeonHost rewrites ep-…-pooler.… to ep-….… so session params work.
func preferDirectNeonHost(databaseURL string) string {
	return strings.Replace(databaseURL, "-pooler.", ".", 1)
}

// DB exposes the underlying *sql.DB (search_path=mdm) for nanomdm/SCEP.
func (s *Store) DB() *sql.DB { return s.db }

// Close closes the pool.
func (s *Store) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) ListEnrollments(ctx context.Context) ([]mdmstore.Enrollment, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT e.id, e.device_id, COALESCE(d.serial_number, ''), e.type, e.topic, e.push_magic,
       e.enabled, e.token_update_tally, e.last_seen_at, e.created_at, e.updated_at
FROM enrollments e
LEFT JOIN devices d ON d.id = e.device_id
ORDER BY e.last_seen_at DESC NULLS LAST, e.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []mdmstore.Enrollment
	for rows.Next() {
		var e mdmstore.Enrollment
		if err := rows.Scan(
			&e.ID, &e.DeviceID, &e.SerialNumber, &e.Type, &e.Topic, &e.PushMagic,
			&e.Enabled, &e.TokenUpdateTally, &e.LastSeenAt, &e.CreatedAt, &e.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Store) GetEnrollment(ctx context.Context, id string) (*mdmstore.Enrollment, error) {
	e := &mdmstore.Enrollment{}
	err := s.db.QueryRowContext(ctx, `
SELECT e.id, e.device_id, COALESCE(d.serial_number, ''), e.type, e.topic, e.push_magic,
       e.enabled, e.token_update_tally, e.last_seen_at, e.created_at, e.updated_at
FROM enrollments e
LEFT JOIN devices d ON d.id = e.device_id
WHERE e.id = $1`, id).Scan(
		&e.ID, &e.DeviceID, &e.SerialNumber, &e.Type, &e.Topic, &e.PushMagic,
		&e.Enabled, &e.TokenUpdateTally, &e.LastSeenAt, &e.CreatedAt, &e.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return e, nil
}

func (s *Store) DeleteEnrollment(ctx context.Context, id string) error {
	// Deleting the device cascades enrollments for classic device enrollments.
	res, err := s.db.ExecContext(ctx, `DELETE FROM devices WHERE id = $1`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		_, err = s.db.ExecContext(ctx, `DELETE FROM enrollments WHERE id = $1`, id)
		return err
	}
	return nil
}

func (s *Store) GetCommandResult(ctx context.Context, enrollmentID, commandUUID string) (*mdmstore.CommandResult, error) {
	out := &mdmstore.CommandResult{
		EnrollmentID: enrollmentID,
		CommandUUID:  commandUUID,
	}
	var (
		reqType string
		status  sql.NullString
		result  sql.NullString
		updated sql.NullTime
		queued  sql.NullTime
	)
	err := s.db.QueryRowContext(ctx, `
SELECT c.request_type, r.status, r.result, r.updated_at, q.created_at
FROM commands c
LEFT JOIN enrollment_queue q ON q.command_uuid = c.command_uuid AND q.id = $1
LEFT JOIN command_results r ON r.command_uuid = c.command_uuid AND r.id = $1
WHERE c.command_uuid = $2`, enrollmentID, commandUUID).Scan(&reqType, &status, &result, &updated, &queued)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	out.RequestType = reqType
	if status.Valid {
		out.Status = status.String
		out.Result = result.String
		if updated.Valid {
			out.UpdatedAt = updated.Time
		}
		out.Parsed, out.QueryResponses = parseCommandResultPlist(result.String)
	} else {
		out.Pending = true
		if queued.Valid {
			out.UpdatedAt = queued.Time
		} else {
			out.UpdatedAt = time.Now().UTC()
		}
	}
	return out, nil
}

func parseCommandResultPlist(resultPlist string) (parsed map[string]any, queryResponses map[string]any) {
	if strings.TrimSpace(resultPlist) == "" {
		return nil, nil
	}
	var top map[string]any
	if err := plist.Unmarshal([]byte(resultPlist), &top); err != nil {
		return nil, nil
	}
	sanitized, ok := sanitizePlistValue(top).(map[string]any)
	if !ok {
		return nil, nil
	}
	parsed = sanitized
	if raw, ok := sanitized["QueryResponses"]; ok {
		switch v := raw.(type) {
		case map[string]any:
			queryResponses = v
		default:
			queryResponses = map[string]any{"_raw": v}
		}
	}
	return parsed, queryResponses
}

func sanitizePlistValue(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, vv := range x {
			out[k] = sanitizePlistValue(vv)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, vv := range x {
			out[i] = sanitizePlistValue(vv)
		}
		return out
	case []byte:
		return base64.StdEncoding.EncodeToString(x)
	default:
		return v
	}
}

func (s *Store) GetPushCertInfo(ctx context.Context, topic string) (*mdmstore.PushCertInfo, error) {
	var certPEM string
	err := s.db.QueryRowContext(ctx, `SELECT cert_pem FROM push_certs WHERE topic = $1`, topic).Scan(&certPEM)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	info := &mdmstore.PushCertInfo{Topic: topic}
	block, _ := pem.Decode([]byte(certPEM))
	if block != nil {
		if cert, err := x509.ParseCertificate(block.Bytes); err == nil {
			t := cert.NotAfter.UTC()
			info.NotAfter = &t
		}
	}
	return info, nil
}

func (s *Store) UpsertPushCert(ctx context.Context, topic string, certPEM, keyPEM []byte) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO push_certs (topic, cert_pem, key_pem, stale_token)
VALUES ($1, $2, $3, 0)
ON CONFLICT (topic) DO UPDATE SET
  cert_pem = EXCLUDED.cert_pem,
  key_pem = EXCLUDED.key_pem,
  stale_token = push_certs.stale_token + 1,
  updated_at = CURRENT_TIMESTAMP`,
		topic, string(certPEM), string(keyPEM))
	return err
}

func (s *Store) ImportDevice(ctx context.Context, d mdmstore.ImportDevice) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO devices (
  id, identity_cert, serial_number, unlock_token, unlock_token_at,
  authenticate, authenticate_at, token_update, token_update_at,
  bootstrap_token_b64, bootstrap_token_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
ON CONFLICT (id) DO UPDATE SET
  identity_cert = EXCLUDED.identity_cert,
  serial_number = EXCLUDED.serial_number,
  unlock_token = EXCLUDED.unlock_token,
  unlock_token_at = EXCLUDED.unlock_token_at,
  authenticate = EXCLUDED.authenticate,
  authenticate_at = EXCLUDED.authenticate_at,
  token_update = EXCLUDED.token_update,
  token_update_at = EXCLUDED.token_update_at,
  bootstrap_token_b64 = EXCLUDED.bootstrap_token_b64,
  bootstrap_token_at = EXCLUDED.bootstrap_token_at,
  updated_at = CURRENT_TIMESTAMP`,
		d.ID, d.IdentityCert, d.SerialNumber, nullBytes(d.UnlockToken), d.UnlockTokenAt,
		d.Authenticate, d.AuthenticateAt, d.TokenUpdate, d.TokenUpdateAt,
		d.BootstrapTokenB64, d.BootstrapTokenAt,
	)
	return err
}

func (s *Store) ImportEnrollment(ctx context.Context, e mdmstore.ImportEnrollment) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO enrollments (
  id, device_id, user_id, type, topic, push_magic, token_hex,
  enabled, token_update_tally, last_seen_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
ON CONFLICT (id) DO UPDATE SET
  device_id = EXCLUDED.device_id,
  user_id = EXCLUDED.user_id,
  type = EXCLUDED.type,
  topic = EXCLUDED.topic,
  push_magic = EXCLUDED.push_magic,
  token_hex = EXCLUDED.token_hex,
  enabled = EXCLUDED.enabled,
  token_update_tally = EXCLUDED.token_update_tally,
  last_seen_at = EXCLUDED.last_seen_at,
  updated_at = CURRENT_TIMESTAMP`,
		e.ID, e.DeviceID, e.UserID, e.Type, e.Topic, e.PushMagic, e.TokenHex,
		e.Enabled, e.TokenUpdateTally, e.LastSeenAt,
	)
	return err
}

func (s *Store) ImportCertAuth(ctx context.Context, enrollmentID, sha256Hex string) error {
	sha := strings.ToLower(strings.TrimSpace(sha256Hex))
	_, err := s.db.ExecContext(ctx, `
INSERT INTO cert_auth_associations (id, sha256)
VALUES ($1, $2)
ON CONFLICT (id, sha256) DO UPDATE SET updated_at = CURRENT_TIMESTAMP`,
		enrollmentID, sha)
	return err
}

func (s *Store) ImportSCEPCA(ctx context.Context, certPEM, encryptedKeyPEM []byte) error {
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return fmt.Errorf("invalid CA cert PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return err
	}
	serial := cert.SerialNumber.Int64()
	if _, err := s.db.ExecContext(ctx, `INSERT INTO scep_serials (serial) VALUES ($1) ON CONFLICT DO NOTHING`, serial); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `
INSERT INTO scep_certificates (serial, name, not_valid_before, not_valid_after, certificate_pem)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (serial) DO UPDATE SET
  name = EXCLUDED.name,
  not_valid_before = EXCLUDED.not_valid_before,
  not_valid_after = EXCLUDED.not_valid_after,
  certificate_pem = EXCLUDED.certificate_pem`,
		serial, cert.Subject.CommonName, cert.NotBefore, cert.NotAfter, certPEM); err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO scep_ca_keys (serial, key_pem) VALUES ($1, $2)
ON CONFLICT (serial) DO UPDATE SET key_pem = EXCLUDED.key_pem`,
		serial, encryptedKeyPEM)
	return err
}

func (s *Store) ImportSCEPCert(ctx context.Context, c mdmstore.ImportSCEPCert) error {
	if _, err := s.db.ExecContext(ctx, `INSERT INTO scep_serials (serial) VALUES ($1) ON CONFLICT DO NOTHING`, c.Serial); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO scep_certificates (serial, name, not_valid_before, not_valid_after, certificate_pem)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (serial) DO UPDATE SET
  name = EXCLUDED.name,
  not_valid_before = EXCLUDED.not_valid_before,
  not_valid_after = EXCLUDED.not_valid_after,
  certificate_pem = EXCLUDED.certificate_pem`,
		c.Serial, c.Name, c.NotValidBefore, c.NotValidAfter, c.CertificatePEM)
	return err
}

func nullBytes(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return b
}

// Ensure compile-time interface satisfaction.
var _ mdmstore.Store = (*Store)(nil)
