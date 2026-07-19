package postgres

import (
	"context"
	"encoding/json"
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
	_, err := s.pool.Exec(ctx, `
		DELETE FROM allowlist_entries
		WHERE kind=$1 AND value=$2 AND target_type=$3 AND target_id=$4
	`, string(kind), value, string(target.Type), target.ID)
	return err
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
	row := s.pool.QueryRow(ctx, `
		SELECT bundle_id, track_id, name, artist, artwork_url, store_url, updated_at, details
		FROM app_metadata WHERE bundle_id=$1
	`, strings.ToLower(strings.TrimSpace(bundleID)))
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
	meta.BundleID = strings.ToLower(strings.TrimSpace(meta.BundleID))
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
		SELECT ids.id, COALESCE(d.name, '') FROM (
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
		if err := rows.Scan(&d.EnrollmentID, &d.Name); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *Store) SetDeviceName(ctx context.Context, enrollmentID, name string) error {
	enrollmentID = strings.TrimSpace(enrollmentID)
	if enrollmentID == "" {
		return fmt.Errorf("enrollment_id is required")
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO devices (enrollment_id, name, updated_at)
		VALUES ($1, $2, now())
		ON CONFLICT (enrollment_id) DO UPDATE SET name = EXCLUDED.name, updated_at = now()
	`, enrollmentID, strings.TrimSpace(name))
	return err
}
