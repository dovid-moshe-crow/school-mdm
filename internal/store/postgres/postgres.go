package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/dwdmsh/school-mdm/internal/policy"
	"github.com/dwdmsh/school-mdm/internal/store"
	"github.com/dwdmsh/school-mdm/migrations"
)

// Store is a Neon/Postgres-backed store.
type Store struct {
	pool *pgxpool.Pool
}

// Open connects, applies migrations, and seeds essentials.
func Open(ctx context.Context, databaseURL string) (*Store, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("connect postgres: %w", err)
	}
	s := &Store{pool: pool}
	if err := s.migrate(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	if err := s.seedEssentials(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() { s.pool.Close() }

func (s *Store) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }
func (s *Store) Kind() string                    { return "postgres" }

func (s *Store) migrate(ctx context.Context) error {
	// Ensure tracking table exists before reading filenames (bootstrapped by 001 too).
	if _, err := s.pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			filename TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`); err != nil {
		return fmt.Errorf("ensure schema_migrations: %w", err)
	}

	entries, err := fs.ReadDir(migrations.SQL, ".")
	if err != nil {
		return fmt.Errorf("list migrations: %w", err)
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)

	for _, name := range names {
		var exists bool
		if err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE filename=$1)`, name).Scan(&exists); err != nil {
			return err
		}
		if exists {
			continue
		}
		sqlBytes, err := migrations.SQL.ReadFile(name)
		if err != nil {
			return fmt.Errorf("read %s: %w", name, err)
		}
		tx, err := s.pool.Begin(ctx)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, string(sqlBytes)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("apply %s: %w", name, err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (filename) VALUES ($1)`, name); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("record %s: %w", name, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) seedEssentials(ctx context.Context) error {
	for _, app := range policy.Essentials {
		if err := s.UpsertAllowlist(ctx, policy.Entry{
			Kind:   policy.KindApp,
			Value:  app,
			Target: policy.Target{Type: policy.TargetGlobal},
		}); err != nil {
			return err
		}
	}
	return nil
}

func normalizeTarget(t *policy.Target) {
	if t.Type == "" {
		t.Type = policy.TargetGlobal
	}
	if t.Type == policy.TargetGlobal {
		t.ID = ""
	}
}

func (s *Store) ListAllowlist(ctx context.Context) ([]policy.Entry, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, kind, value, target_type, target_id FROM allowlist_entries`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []policy.Entry
	for rows.Next() {
		var e policy.Entry
		var kind, tt string
		if err := rows.Scan(&e.ID, &kind, &e.Value, &tt, &e.Target.ID); err != nil {
			return nil, err
		}
		e.Kind = policy.Kind(kind)
		e.Target.Type = policy.TargetType(tt)
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Store) UpsertAllowlist(ctx context.Context, entry policy.Entry) error {
	entry.Value = policy.Normalize(entry.Kind, entry.Value)
	normalizeTarget(&entry.Target)
	if entry.ID == "" {
		entry.ID = uuid.NewString()
	}
	if entry.Kind == policy.KindApp {
		// Case-insensitive replace so ph.telegra.telegraph → ph.telegra.Telegraph.
		_, err := s.pool.Exec(ctx, `
			DELETE FROM allowlist_entries
			WHERE kind=$1 AND lower(value)=lower($2) AND target_type=$3 AND target_id=$4
		`, string(entry.Kind), entry.Value, string(entry.Target.Type), entry.Target.ID)
		if err != nil {
			return err
		}
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO allowlist_entries (id, kind, value, target_type, target_id)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (kind, value, target_type, target_id) DO UPDATE SET value = EXCLUDED.value
	`, entry.ID, string(entry.Kind), entry.Value, string(entry.Target.Type), entry.Target.ID)
	return err
}

func (s *Store) DeleteAllowlist(ctx context.Context, kind policy.Kind, value string, target policy.Target) error {
	value = policy.Normalize(kind, value)
	normalizeTarget(&target)
	if kind == policy.KindApp {
		_, err := s.pool.Exec(ctx, `
			DELETE FROM allowlist_entries
			WHERE kind=$1 AND lower(value)=lower($2) AND target_type=$3 AND target_id=$4
		`, string(kind), value, string(target.Type), target.ID)
		return err
	}
	_, err := s.pool.Exec(ctx, `
		DELETE FROM allowlist_entries
		WHERE kind=$1 AND value=$2 AND target_type=$3 AND target_id=$4
	`, string(kind), value, string(target.Type), target.ID)
	return err
}

func (s *Store) ListWhitelistPacks(ctx context.Context) ([]store.WhitelistPack, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT p.id, p.name, p.description, p.created_at,
		       (SELECT COUNT(*) FROM whitelist_pack_items i WHERE i.pack_id = p.id)
		FROM whitelist_packs p
		ORDER BY p.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []store.WhitelistPack
	for rows.Next() {
		var p store.WhitelistPack
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.CreatedAt, &p.ItemCount); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) GetWhitelistPack(ctx context.Context, id string) (store.WhitelistPack, error) {
	var p store.WhitelistPack
	err := s.pool.QueryRow(ctx, `
		SELECT id, name, description, created_at,
		       (SELECT COUNT(*) FROM whitelist_pack_items i WHERE i.pack_id = whitelist_packs.id)
		FROM whitelist_packs WHERE id=$1`, strings.TrimSpace(id)).
		Scan(&p.ID, &p.Name, &p.Description, &p.CreatedAt, &p.ItemCount)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return store.WhitelistPack{}, store.ErrNotFound
		}
		return store.WhitelistPack{}, err
	}
	return p, nil
}

func (s *Store) CreateWhitelistPack(ctx context.Context, p store.WhitelistPack) (store.WhitelistPack, error) {
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" {
		return store.WhitelistPack{}, fmt.Errorf("name is required")
	}
	if p.ID == "" {
		p.ID = uuid.NewString()
	}
	err := s.pool.QueryRow(ctx, `
		INSERT INTO whitelist_packs (id, name, description)
		VALUES ($1, $2, $3)
		RETURNING created_at`, p.ID, p.Name, strings.TrimSpace(p.Description)).Scan(&p.CreatedAt)
	if err != nil {
		return store.WhitelistPack{}, err
	}
	return p, nil
}

func (s *Store) UpdateWhitelistPack(ctx context.Context, p store.WhitelistPack) error {
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" {
		return fmt.Errorf("name is required")
	}
	tag, err := s.pool.Exec(ctx, `
		UPDATE whitelist_packs SET name=$2, description=$3 WHERE id=$1`,
		p.ID, p.Name, strings.TrimSpace(p.Description))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return store.ErrNotFound
	}
	return nil
}

func (s *Store) DeleteWhitelistPack(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM whitelist_packs WHERE id=$1`, strings.TrimSpace(id))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return store.ErrNotFound
	}
	return nil
}

func (s *Store) ListWhitelistPackItems(ctx context.Context, packID string) ([]store.WhitelistPackItem, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT pack_id, kind, value FROM whitelist_pack_items
		WHERE pack_id=$1 ORDER BY kind, value`, strings.TrimSpace(packID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []store.WhitelistPackItem
	for rows.Next() {
		var it store.WhitelistPackItem
		var kind string
		if err := rows.Scan(&it.PackID, &kind, &it.Value); err != nil {
			return nil, err
		}
		it.Kind = policy.Kind(kind)
		out = append(out, it)
	}
	return out, rows.Err()
}

func (s *Store) AddWhitelistPackItem(ctx context.Context, item store.WhitelistPackItem) error {
	item.Value = policy.Normalize(item.Kind, item.Value)
	if item.PackID == "" || item.Value == "" {
		return fmt.Errorf("pack_id and value required")
	}
	if item.Kind == policy.KindApp {
		_, err := s.pool.Exec(ctx, `
			DELETE FROM whitelist_pack_items
			WHERE pack_id=$1 AND kind=$2 AND lower(value)=lower($3)`,
			item.PackID, string(item.Kind), item.Value)
		if err != nil {
			return err
		}
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO whitelist_pack_items (pack_id, kind, value)
		VALUES ($1, $2, $3)
		ON CONFLICT DO NOTHING`, item.PackID, string(item.Kind), item.Value)
	return err
}

func (s *Store) RemoveWhitelistPackItem(ctx context.Context, packID string, kind policy.Kind, value string) error {
	value = policy.Normalize(kind, value)
	if kind == policy.KindApp {
		_, err := s.pool.Exec(ctx, `
			DELETE FROM whitelist_pack_items WHERE pack_id=$1 AND kind=$2 AND lower(value)=lower($3)`,
			strings.TrimSpace(packID), string(kind), value)
		return err
	}
	_, err := s.pool.Exec(ctx, `
		DELETE FROM whitelist_pack_items WHERE pack_id=$1 AND kind=$2 AND value=$3`,
		strings.TrimSpace(packID), string(kind), value)
	return err
}

func (s *Store) ListWhitelistPackAssignments(ctx context.Context, packID string) ([]store.WhitelistPackAssignment, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT pack_id, target_type, target_id FROM whitelist_pack_assignments
		WHERE pack_id=$1 ORDER BY target_type, target_id`, strings.TrimSpace(packID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []store.WhitelistPackAssignment
	for rows.Next() {
		var a store.WhitelistPackAssignment
		var tt string
		if err := rows.Scan(&a.PackID, &tt, &a.TargetID); err != nil {
			return nil, err
		}
		a.TargetType = policy.TargetType(tt)
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) SetWhitelistPackAssignment(ctx context.Context, a store.WhitelistPackAssignment) error {
	t := policy.Target{Type: a.TargetType, ID: a.TargetID}
	normalizeTarget(&t)
	_, err := s.pool.Exec(ctx, `
		INSERT INTO whitelist_pack_assignments (pack_id, target_type, target_id)
		VALUES ($1, $2, $3)
		ON CONFLICT DO NOTHING`, a.PackID, string(t.Type), t.ID)
	return err
}

func (s *Store) RemoveWhitelistPackAssignment(ctx context.Context, packID string, target policy.Target) error {
	normalizeTarget(&target)
	_, err := s.pool.Exec(ctx, `
		DELETE FROM whitelist_pack_assignments
		WHERE pack_id=$1 AND target_type=$2 AND target_id=$3`,
		strings.TrimSpace(packID), string(target.Type), target.ID)
	return err
}

func (s *Store) ListAllowlistFromPacks(ctx context.Context, enrollmentID string, groupIDs []string) ([]policy.Entry, error) {
	// Packs assigned globally, to any of the device's groups, or to the device.
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT i.kind, i.value
		FROM whitelist_pack_assignments a
		INNER JOIN whitelist_pack_items i ON i.pack_id = a.pack_id
		WHERE a.target_type = 'global'
		   OR (a.target_type = 'device' AND a.target_id = $1)
		   OR (a.target_type = 'group' AND a.target_id = ANY($2::text[]))
	`, strings.TrimSpace(enrollmentID), groupIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []policy.Entry
	for rows.Next() {
		var kind, value string
		if err := rows.Scan(&kind, &value); err != nil {
			return nil, err
		}
		out = append(out, policy.Entry{
			Kind:   policy.Kind(kind),
			Value:  value,
			Target: policy.Target{Type: policy.TargetGlobal}, // already filtered for this device
		})
	}
	return out, rows.Err()
}

func (s *Store) ListGrants(ctx context.Context) ([]policy.Grant, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, kind, value, target_type, target_id, expires_at FROM grants`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []policy.Grant
	for rows.Next() {
		var g policy.Grant
		var kind, tt string
		var exp *time.Time
		if err := rows.Scan(&g.ID, &kind, &g.Value, &tt, &g.Target.ID, &exp); err != nil {
			return nil, err
		}
		g.Kind = policy.Kind(kind)
		g.Target.Type = policy.TargetType(tt)
		g.ExpiresAt = exp
		out = append(out, g)
	}
	return out, rows.Err()
}

func (s *Store) AddGrant(ctx context.Context, grant policy.Grant) error {
	if grant.ID == "" {
		grant.ID = uuid.NewString()
	}
	grant.Value = policy.Normalize(grant.Kind, grant.Value)
	normalizeTarget(&grant.Target)
	_, err := s.pool.Exec(ctx, `
		INSERT INTO grants (id, kind, value, target_type, target_id, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, grant.ID, string(grant.Kind), grant.Value, string(grant.Target.Type), grant.Target.ID, grant.ExpiresAt)
	return err
}

func (s *Store) DeleteGrants(ctx context.Context, kind policy.Kind, value string, target policy.Target) error {
	value = policy.Normalize(kind, value)
	normalizeTarget(&target)
	if kind == policy.KindApp {
		_, err := s.pool.Exec(ctx, `
			DELETE FROM grants
			WHERE kind=$1 AND lower(value)=lower($2) AND target_type=$3 AND target_id=$4
		`, string(kind), value, string(target.Type), target.ID)
		return err
	}
	_, err := s.pool.Exec(ctx, `
		DELETE FROM grants
		WHERE kind=$1 AND value=$2 AND target_type=$3 AND target_id=$4
	`, string(kind), value, string(target.Type), target.ID)
	return err
}

func (s *Store) CreateRequest(ctx context.Context, req store.Request) (store.Request, error) {
	if req.ID == "" {
		req.ID = uuid.NewString()
	}
	if req.Status == "" {
		req.Status = store.StatusPending
	}
	if req.CreatedAt.IsZero() {
		req.CreatedAt = time.Now().UTC()
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO requests (id, type, target_kind, value, enrollment_id, reason, status, duration, created_at, decided_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
	`, req.ID, string(req.Type), string(req.TargetKind), req.Value, req.EnrollmentID, req.Reason, string(req.Status), req.Duration, req.CreatedAt, req.DecidedAt)
	return req, err
}

func (s *Store) GetRequest(ctx context.Context, id string) (store.Request, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, type, target_kind, value, enrollment_id, reason, status, duration, created_at, decided_at
		FROM requests WHERE id=$1
	`, id)
	req, err := scanRequest(row)
	if err == pgx.ErrNoRows {
		return store.Request{}, fmt.Errorf("request %s: %w", id, store.ErrNotFound)
	}
	return req, err
}

func (s *Store) ListRequests(ctx context.Context, status *store.RequestStatus) ([]store.Request, error) {
	var rows pgx.Rows
	var err error
	if status == nil {
		rows, err = s.pool.Query(ctx, `
			SELECT id, type, target_kind, value, enrollment_id, reason, status, duration, created_at, decided_at
			FROM requests ORDER BY created_at DESC`)
	} else {
		rows, err = s.pool.Query(ctx, `
			SELECT id, type, target_kind, value, enrollment_id, reason, status, duration, created_at, decided_at
			FROM requests WHERE status=$1 ORDER BY created_at DESC`, string(*status))
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []store.Request
	for rows.Next() {
		req, err := scanRequest(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, req)
	}
	return out, rows.Err()
}

func (s *Store) ListRequestsByEnrollment(ctx context.Context, enrollmentID string) ([]store.Request, error) {
	enrollmentID = strings.TrimSpace(enrollmentID)
	if enrollmentID == "" {
		return nil, fmt.Errorf("enrollment_id is required")
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, type, target_kind, value, enrollment_id, reason, status, duration, created_at, decided_at
		FROM requests WHERE enrollment_id=$1 ORDER BY created_at DESC`, enrollmentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []store.Request
	for rows.Next() {
		req, err := scanRequest(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, req)
	}
	if out == nil {
		out = []store.Request{}
	}
	return out, rows.Err()
}

func (s *Store) UpdateRequest(ctx context.Context, req store.Request) error {
	ct, err := s.pool.Exec(ctx, `
		UPDATE requests
		SET type=$2, target_kind=$3, value=$4, enrollment_id=$5, reason=$6, status=$7, duration=$8, decided_at=$9
		WHERE id=$1
	`, req.ID, string(req.Type), string(req.TargetKind), req.Value, req.EnrollmentID, req.Reason, string(req.Status), req.Duration, req.DecidedAt)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("request %s: %w", req.ID, store.ErrNotFound)
	}
	return nil
}

func (s *Store) ListRequestMessages(ctx context.Context, requestID string) ([]store.RequestMessage, error) {
	if _, err := s.GetRequest(ctx, requestID); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, request_id, author_role, body, created_at
		FROM request_messages
		WHERE request_id=$1
		ORDER BY created_at ASC, id ASC
	`, requestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []store.RequestMessage
	for rows.Next() {
		var m store.RequestMessage
		var role string
		if err := rows.Scan(&m.ID, &m.RequestID, &role, &m.Body, &m.CreatedAt); err != nil {
			return nil, err
		}
		m.AuthorRole = store.MessageAuthor(role)
		out = append(out, m)
	}
	if out == nil {
		out = []store.RequestMessage{}
	}
	return out, rows.Err()
}

func (s *Store) AddRequestMessage(ctx context.Context, msg store.RequestMessage) (store.RequestMessage, error) {
	if _, err := s.GetRequest(ctx, msg.RequestID); err != nil {
		return store.RequestMessage{}, err
	}
	if msg.ID == "" {
		msg.ID = uuid.NewString()
	}
	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = time.Now().UTC()
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO request_messages (id, request_id, author_role, body, created_at)
		VALUES ($1,$2,$3,$4,$5)
	`, msg.ID, msg.RequestID, string(msg.AuthorRole), msg.Body, msg.CreatedAt)
	return msg, err
}

func (s *Store) CountRequestMessages(ctx context.Context, requestID string) (int, error) {
	if _, err := s.GetRequest(ctx, requestID); err != nil {
		return 0, err
	}
	var n int
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM request_messages WHERE request_id=$1`, requestID).Scan(&n)
	return n, err
}

func (s *Store) LastRequestMessage(ctx context.Context, requestID string) (store.RequestMessage, error) {
	if _, err := s.GetRequest(ctx, requestID); err != nil {
		return store.RequestMessage{}, err
	}
	row := s.pool.QueryRow(ctx, `
		SELECT id, request_id, author_role, body, created_at
		FROM request_messages
		WHERE request_id=$1
		ORDER BY created_at DESC, id DESC
		LIMIT 1
	`, requestID)
	var m store.RequestMessage
	var role string
	if err := row.Scan(&m.ID, &m.RequestID, &role, &m.Body, &m.CreatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return store.RequestMessage{}, store.ErrNotFound
		}
		return store.RequestMessage{}, err
	}
	m.AuthorRole = store.MessageAuthor(role)
	return m, nil
}

type scannable interface {
	Scan(dest ...any) error
}

func scanRequest(row scannable) (store.Request, error) {
	var req store.Request
	var typ, target, status string
	if err := row.Scan(&req.ID, &typ, &target, &req.Value, &req.EnrollmentID, &req.Reason, &status, &req.Duration, &req.CreatedAt, &req.DecidedAt); err != nil {
		return store.Request{}, err
	}
	req.Type = store.RequestType(typ)
	req.TargetKind = policy.Kind(target)
	req.Status = store.RequestStatus(status)
	return req, nil
}

func (s *Store) GetAppMeta(ctx context.Context, bundleID string) (store.AppMeta, error) {
	bundleID = strings.TrimSpace(bundleID)
	row := s.pool.QueryRow(ctx, `
		SELECT bundle_id, track_id, name, artist, artwork_url, store_url, updated_at, details
		FROM app_metadata WHERE lower(bundle_id)=lower($1)
		LIMIT 1
	`, bundleID)
	var m store.AppMeta
	var details []byte
	if err := row.Scan(&m.BundleID, &m.TrackID, &m.Name, &m.Artist, &m.ArtworkURL, &m.StoreURL, &m.UpdatedAt, &details); err != nil {
		if err == pgx.ErrNoRows {
			return store.AppMeta{}, fmt.Errorf("app %s: %w", bundleID, store.ErrNotFound)
		}
		return store.AppMeta{}, err
	}
	applyAppDetails(&m, details)
	return m, nil
}

func (s *Store) UpsertAppMeta(ctx context.Context, meta store.AppMeta) error {
	meta.BundleID = strings.TrimSpace(meta.BundleID)
	if meta.BundleID == "" {
		return fmt.Errorf("bundle_id is required")
	}
	if meta.UpdatedAt.IsZero() {
		meta.UpdatedAt = time.Now().UTC()
	}
	details, err := marshalAppDetails(meta)
	if err != nil {
		return err
	}
	// Drop other casings of the same bundle so the PK can hold Apple's form.
	if _, err := s.pool.Exec(ctx, `
		DELETE FROM app_metadata WHERE lower(bundle_id)=lower($1) AND bundle_id <> $1
	`, meta.BundleID); err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO app_metadata (bundle_id, track_id, name, artist, artwork_url, store_url, updated_at, details)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (bundle_id) DO UPDATE SET
			track_id = EXCLUDED.track_id,
			name = EXCLUDED.name,
			artist = EXCLUDED.artist,
			artwork_url = EXCLUDED.artwork_url,
			store_url = EXCLUDED.store_url,
			updated_at = EXCLUDED.updated_at,
			details = CASE
				WHEN EXCLUDED.details = '{}'::jsonb THEN app_metadata.details
				ELSE EXCLUDED.details
			END
	`, meta.BundleID, meta.TrackID, meta.Name, meta.Artist, meta.ArtworkURL, meta.StoreURL, meta.UpdatedAt, details)
	return err
}

func (s *Store) SearchAppMeta(ctx context.Context, query string, limit int) ([]store.AppMeta, error) {
	if limit <= 0 {
		limit = 12
	}
	q := "%" + strings.ToLower(strings.TrimSpace(query)) + "%"
	rows, err := s.pool.Query(ctx, `
		SELECT bundle_id, track_id, name, artist, artwork_url, store_url, updated_at, details
		FROM app_metadata
		WHERE lower(name) LIKE $1 OR lower(artist) LIKE $1 OR bundle_id LIKE $1
		ORDER BY name ASC
		LIMIT $2
	`, q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []store.AppMeta
	for rows.Next() {
		var m store.AppMeta
		var details []byte
		if err := rows.Scan(&m.BundleID, &m.TrackID, &m.Name, &m.Artist, &m.ArtworkURL, &m.StoreURL, &m.UpdatedAt, &details); err != nil {
			return nil, err
		}
		applyAppDetails(&m, details)
		out = append(out, m)
	}
	return out, rows.Err()
}

type appDetailsJSON struct {
	Description    string   `json:"description,omitempty"`
	Genre          string   `json:"genre,omitempty"`
	Version        string   `json:"version,omitempty"`
	AverageRating  float64  `json:"average_rating,omitempty"`
	RatingCount    int      `json:"rating_count,omitempty"`
	ContentRating  string   `json:"content_rating,omitempty"`
	ReleaseDate    string   `json:"release_date,omitempty"`
	FormattedPrice string   `json:"formatted_price,omitempty"`
	FileSizeBytes  int64    `json:"file_size_bytes,omitempty"`
	SellerName     string   `json:"seller_name,omitempty"`
	Screenshots    []string `json:"screenshots,omitempty"`
}

func marshalAppDetails(m store.AppMeta) ([]byte, error) {
	d := appDetailsJSON{
		Description:    m.Description,
		Genre:          m.Genre,
		Version:        m.Version,
		AverageRating:  m.AverageRating,
		RatingCount:    m.RatingCount,
		ContentRating:  m.ContentRating,
		ReleaseDate:    m.ReleaseDate,
		FormattedPrice: m.FormattedPrice,
		FileSizeBytes:  m.FileSizeBytes,
		SellerName:     m.SellerName,
		Screenshots:    m.Screenshots,
	}
	if d.Description == "" && d.Genre == "" && d.Version == "" && d.AverageRating == 0 &&
		d.RatingCount == 0 && d.ContentRating == "" && d.ReleaseDate == "" &&
		d.FormattedPrice == "" && d.FileSizeBytes == 0 && d.SellerName == "" && len(d.Screenshots) == 0 {
		return []byte("{}"), nil
	}
	return json.Marshal(d)
}

func applyAppDetails(m *store.AppMeta, raw []byte) {
	if len(raw) == 0 || string(raw) == "{}" || string(raw) == "null" {
		return
	}
	var d appDetailsJSON
	if err := json.Unmarshal(raw, &d); err != nil {
		return
	}
	m.Description = d.Description
	m.Genre = d.Genre
	m.Version = d.Version
	m.AverageRating = d.AverageRating
	m.RatingCount = d.RatingCount
	m.ContentRating = d.ContentRating
	m.ReleaseDate = d.ReleaseDate
	m.FormattedPrice = d.FormattedPrice
	m.FileSizeBytes = d.FileSizeBytes
	m.SellerName = d.SellerName
	m.Screenshots = d.Screenshots
}

func (s *Store) ListGroups(ctx context.Context) ([]store.Group, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, name, description, created_at FROM groups ORDER BY name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []store.Group
	for rows.Next() {
		var g store.Group
		if err := rows.Scan(&g.ID, &g.Name, &g.Description, &g.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func (s *Store) GetGroup(ctx context.Context, id string) (store.Group, error) {
	var g store.Group
	err := s.pool.QueryRow(ctx, `SELECT id, name, description, created_at FROM groups WHERE id=$1`, id).
		Scan(&g.ID, &g.Name, &g.Description, &g.CreatedAt)
	if err == pgx.ErrNoRows {
		return store.Group{}, fmt.Errorf("group %s: %w", id, store.ErrNotFound)
	}
	return g, err
}

func (s *Store) CreateGroup(ctx context.Context, g store.Group) (store.Group, error) {
	g.Name = strings.TrimSpace(g.Name)
	if g.Name == "" {
		return store.Group{}, fmt.Errorf("group name is required")
	}
	if g.ID == "" {
		g.ID = uuid.NewString()
	}
	if g.CreatedAt.IsZero() {
		g.CreatedAt = time.Now().UTC()
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO groups (id, name, description, created_at)
		VALUES ($1, $2, $3, $4)
	`, g.ID, g.Name, g.Description, g.CreatedAt)
	return g, err
}

func (s *Store) UpdateGroup(ctx context.Context, g store.Group) error {
	g.Name = strings.TrimSpace(g.Name)
	if g.Name == "" {
		return fmt.Errorf("group name is required")
	}
	ct, err := s.pool.Exec(ctx, `
		UPDATE groups SET name=$2, description=$3 WHERE id=$1
	`, g.ID, g.Name, g.Description)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("group %s: %w", g.ID, store.ErrNotFound)
	}
	return nil
}

func (s *Store) DeleteGroup(ctx context.Context, id string) error {
	ct, err := s.pool.Exec(ctx, `DELETE FROM groups WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("group %s: %w", id, store.ErrNotFound)
	}
	return nil
}

func (s *Store) ListGroupMembers(ctx context.Context, groupID string) ([]string, error) {
	if _, err := s.GetGroup(ctx, groupID); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `SELECT enrollment_id FROM group_members WHERE group_id=$1 ORDER BY enrollment_id`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func (s *Store) SetGroupMembers(ctx context.Context, groupID string, enrollmentIDs []string) error {
	if _, err := s.GetGroup(ctx, groupID); err != nil {
		return err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `DELETE FROM group_members WHERE group_id=$1`, groupID); err != nil {
		return err
	}
	for _, id := range enrollmentIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO group_members (group_id, enrollment_id) VALUES ($1, $2)
			ON CONFLICT DO NOTHING
		`, groupID, id); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *Store) AddGroupMember(ctx context.Context, groupID, enrollmentID string) error {
	if _, err := s.GetGroup(ctx, groupID); err != nil {
		return err
	}
	enrollmentID = strings.TrimSpace(enrollmentID)
	if enrollmentID == "" {
		return fmt.Errorf("enrollment_id is required")
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO group_members (group_id, enrollment_id) VALUES ($1, $2)
		ON CONFLICT DO NOTHING
	`, groupID, enrollmentID)
	return err
}

func (s *Store) RemoveGroupMember(ctx context.Context, groupID, enrollmentID string) error {
	_, err := s.pool.Exec(ctx, `
		DELETE FROM group_members WHERE group_id=$1 AND enrollment_id=$2
	`, groupID, strings.TrimSpace(enrollmentID))
	return err
}

func (s *Store) ListGroupsForDevice(ctx context.Context, enrollmentID string) ([]string, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT group_id FROM group_members WHERE enrollment_id=$1
	`, strings.TrimSpace(enrollmentID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func (s *Store) ListEnrollmentIDsForGroup(ctx context.Context, groupID string) ([]string, error) {
	return s.ListGroupMembers(ctx, groupID)
}

func (s *Store) ListAllEnrollmentIDs(ctx context.Context) ([]string, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT enrollment_id FROM (
			SELECT enrollment_id FROM requests WHERE enrollment_id <> ''
			UNION
			SELECT target_id FROM grants WHERE target_type='device' AND target_id <> ''
			UNION
			SELECT target_id FROM allowlist_entries WHERE target_type='device' AND target_id <> ''
			UNION
			SELECT enrollment_id FROM group_members
			UNION
			SELECT enrollment_id FROM devices
		) t
		ORDER BY enrollment_id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func (s *Store) ListDevices(ctx context.Context) ([]store.Device, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT ids.id, COALESCE(d.name, ''), COALESCE(d.unrestricted, false) FROM (
			SELECT enrollment_id AS id FROM requests WHERE enrollment_id <> ''
			UNION
			SELECT target_id FROM grants WHERE target_type='device' AND target_id <> ''
			UNION
			SELECT target_id FROM allowlist_entries WHERE target_type='device' AND target_id <> ''
			UNION
			SELECT enrollment_id FROM group_members
			UNION
			SELECT enrollment_id FROM devices
		) ids
		LEFT JOIN devices d ON d.enrollment_id = ids.id
		ORDER BY COALESCE(NULLIF(d.name, ''), ids.id)
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []store.Device
	for rows.Next() {
		var d store.Device
		if err := rows.Scan(&d.EnrollmentID, &d.Name, &d.Unrestricted); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *Store) GetDevice(ctx context.Context, enrollmentID string) (store.Device, error) {
	enrollmentID = strings.TrimSpace(enrollmentID)
	var d store.Device
	err := s.pool.QueryRow(ctx, `
		SELECT enrollment_id, COALESCE(name, ''), COALESCE(unrestricted, false)
		FROM devices WHERE enrollment_id = $1
	`, enrollmentID).Scan(&d.EnrollmentID, &d.Name, &d.Unrestricted)
	if err != nil {
		return store.Device{}, err
	}
	return d, nil
}

func (s *Store) EnsureDevice(ctx context.Context, enrollmentID string) error {
	enrollmentID = strings.TrimSpace(enrollmentID)
	if enrollmentID == "" {
		return fmt.Errorf("enrollment_id is required")
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO devices (enrollment_id, name, unrestricted, updated_at)
		VALUES ($1, '', false, now())
		ON CONFLICT (enrollment_id) DO NOTHING
	`, enrollmentID)
	return err
}

func (s *Store) SetDeviceName(ctx context.Context, enrollmentID, name string) error {
	enrollmentID = strings.TrimSpace(enrollmentID)
	if enrollmentID == "" {
		return fmt.Errorf("enrollment_id is required")
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO devices (enrollment_id, name, unrestricted, updated_at)
		VALUES ($1, $2, false, now())
		ON CONFLICT (enrollment_id) DO UPDATE SET name = EXCLUDED.name, updated_at = now()
	`, enrollmentID, strings.TrimSpace(name))
	return err
}

func (s *Store) SetDeviceUnrestricted(ctx context.Context, enrollmentID string, unrestricted bool) error {
	enrollmentID = strings.TrimSpace(enrollmentID)
	if enrollmentID == "" {
		return fmt.Errorf("enrollment_id is required")
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO devices (enrollment_id, name, unrestricted, updated_at)
		VALUES ($1, '', $2, now())
		ON CONFLICT (enrollment_id) DO UPDATE SET unrestricted = EXCLUDED.unrestricted, updated_at = now()
	`, enrollmentID, unrestricted)
	return err
}

func (s *Store) InsertActivityEvent(ctx context.Context, e store.ActivityEvent) (store.ActivityEvent, error) {
	if e.ID == "" {
		e.ID = uuid.NewString()
	}
	if e.Result == "" {
		e.Result = store.ActivityResultOK
	}
	if e.ActorType == "" {
		e.ActorType = store.ActivityActorSystem
	}
	detail := e.Detail
	if len(detail) == 0 {
		detail = json.RawMessage(`{}`)
	}
	err := s.pool.QueryRow(ctx, `
		INSERT INTO activity_events (
			id, at, category, action, actor_type, actor,
			enrollment_id, group_id, request_id, command_uuid,
			result, summary, detail
		) VALUES (
			$1, COALESCE($2, now()), $3, $4, $5, $6,
			NULLIF($7, ''), NULLIF($8, ''), NULLIF($9, ''), NULLIF($10, ''),
			$11, $12, $13::jsonb
		)
		RETURNING at
	`,
		e.ID, nullTime(e.At), e.Category, e.Action, e.ActorType, e.Actor,
		e.EnrollmentID, e.GroupID, e.RequestID, e.CommandUUID,
		e.Result, e.Summary, string(detail),
	).Scan(&e.At)
	if err != nil {
		return store.ActivityEvent{}, err
	}
	e.Detail = detail
	return e, nil
}

func nullTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t.UTC()
}

func (s *Store) ListActivityEvents(ctx context.Context, f store.ActivityFilter) ([]store.ActivityEvent, error) {
	limit := f.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	offset := f.Offset
	if offset < 0 {
		offset = 0
	}

	var b strings.Builder
	args := make([]any, 0, 12)
	arg := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}

	b.WriteString(`
		SELECT id, at, category, action, actor_type, actor,
			COALESCE(enrollment_id, ''), COALESCE(group_id, ''),
			COALESCE(request_id, ''), COALESCE(command_uuid, ''),
			result, summary, detail
		FROM activity_events
		WHERE 1=1
	`)
	if f.From != nil {
		b.WriteString(" AND at >= " + arg(f.From.UTC()))
	}
	if f.To != nil {
		b.WriteString(" AND at <= " + arg(f.To.UTC()))
	}
	if c := strings.TrimSpace(f.Category); c != "" {
		b.WriteString(" AND category = " + arg(c))
	}
	if a := strings.TrimSpace(f.Action); a != "" {
		b.WriteString(" AND action = " + arg(a))
	}
	if e := strings.TrimSpace(f.EnrollmentID); e != "" {
		b.WriteString(" AND enrollment_id = " + arg(e))
	}
	if at := strings.TrimSpace(f.ActorType); at != "" {
		b.WriteString(" AND actor_type = " + arg(at))
	}
	if r := strings.TrimSpace(f.Result); r != "" {
		b.WriteString(" AND result = " + arg(r))
	}
	if q := strings.TrimSpace(f.Q); q != "" {
		b.WriteString(" AND summary ILIKE " + arg("%"+q+"%"))
	}
	b.WriteString(" ORDER BY at DESC LIMIT " + arg(limit) + " OFFSET " + arg(offset))

	rows, err := s.pool.Query(ctx, b.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []store.ActivityEvent
	for rows.Next() {
		var e store.ActivityEvent
		if err := rows.Scan(
			&e.ID, &e.At, &e.Category, &e.Action, &e.ActorType, &e.Actor,
			&e.EnrollmentID, &e.GroupID, &e.RequestID, &e.CommandUUID,
			&e.Result, &e.Summary, &e.Detail,
		); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
