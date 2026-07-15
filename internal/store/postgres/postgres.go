package postgres

import (
	"context"
	"fmt"
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
	sqlBytes, err := migrations.SQL.ReadFile("001_init.sql")
	if err != nil {
		return fmt.Errorf("read migration: %w", err)
	}
	if _, err := s.pool.Exec(ctx, string(sqlBytes)); err != nil {
		return fmt.Errorf("apply migration: %w", err)
	}
	return nil
}

func (s *Store) seedEssentials(ctx context.Context) error {
	for _, app := range policy.Essentials {
		if err := s.UpsertAllowlist(ctx, policy.Entry{
			Kind:  policy.KindApp,
			Value: app,
			Scope: "global",
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ListAllowlist(ctx context.Context) ([]policy.Entry, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, kind, value, scope FROM allowlist_entries`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []policy.Entry
	for rows.Next() {
		var e policy.Entry
		var kind string
		if err := rows.Scan(&e.ID, &kind, &e.Value, &e.Scope); err != nil {
			return nil, err
		}
		e.Kind = policy.Kind(kind)
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Store) UpsertAllowlist(ctx context.Context, entry policy.Entry) error {
	entry.Value = policy.Normalize(entry.Kind, entry.Value)
	if entry.Scope == "" {
		entry.Scope = "global"
	}
	if entry.ID == "" {
		entry.ID = uuid.NewString()
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO allowlist_entries (id, kind, value, scope)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (kind, value, scope) DO UPDATE SET value = EXCLUDED.value
	`, entry.ID, string(entry.Kind), entry.Value, entry.Scope)
	return err
}

func (s *Store) ListGrants(ctx context.Context) ([]policy.Grant, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, kind, value, enrollment_id, expires_at FROM grants`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []policy.Grant
	for rows.Next() {
		var g policy.Grant
		var kind string
		var exp *time.Time
		if err := rows.Scan(&g.ID, &kind, &g.Value, &g.EnrollmentID, &exp); err != nil {
			return nil, err
		}
		g.Kind = policy.Kind(kind)
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
	_, err := s.pool.Exec(ctx, `
		INSERT INTO grants (id, kind, value, enrollment_id, expires_at)
		VALUES ($1, $2, $3, $4, $5)
	`, grant.ID, string(grant.Kind), grant.Value, grant.EnrollmentID, grant.ExpiresAt)
	return err
}

func (s *Store) CreateRequest(ctx context.Context, req store.AccessRequest) (store.AccessRequest, error) {
	if req.ID == "" {
		req.ID = uuid.NewString()
	}
	req.Value = policy.Normalize(req.Kind, req.Value)
	if req.Status == "" {
		req.Status = store.StatusPending
	}
	if req.CreatedAt.IsZero() {
		req.CreatedAt = time.Now().UTC()
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO access_requests (id, kind, value, enrollment_id, reason, status, duration, created_at, decided_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
	`, req.ID, string(req.Kind), req.Value, req.EnrollmentID, req.Reason, string(req.Status), req.Duration, req.CreatedAt, req.DecidedAt)
	return req, err
}

func (s *Store) GetRequest(ctx context.Context, id string) (store.AccessRequest, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, kind, value, enrollment_id, reason, status, duration, created_at, decided_at
		FROM access_requests WHERE id=$1
	`, id)
	req, err := scanRequest(row)
	if err == pgx.ErrNoRows {
		return store.AccessRequest{}, fmt.Errorf("request %s: %w", id, store.ErrNotFound)
	}
	return req, err
}

func (s *Store) ListRequests(ctx context.Context, status *store.RequestStatus) ([]store.AccessRequest, error) {
	var rows pgx.Rows
	var err error
	if status == nil {
		rows, err = s.pool.Query(ctx, `
			SELECT id, kind, value, enrollment_id, reason, status, duration, created_at, decided_at
			FROM access_requests ORDER BY created_at DESC`)
	} else {
		rows, err = s.pool.Query(ctx, `
			SELECT id, kind, value, enrollment_id, reason, status, duration, created_at, decided_at
			FROM access_requests WHERE status=$1 ORDER BY created_at DESC`, string(*status))
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []store.AccessRequest
	for rows.Next() {
		req, err := scanRequest(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, req)
	}
	return out, rows.Err()
}

func (s *Store) UpdateRequest(ctx context.Context, req store.AccessRequest) error {
	ct, err := s.pool.Exec(ctx, `
		UPDATE access_requests
		SET kind=$2, value=$3, enrollment_id=$4, reason=$5, status=$6, duration=$7, decided_at=$8
		WHERE id=$1
	`, req.ID, string(req.Kind), req.Value, req.EnrollmentID, req.Reason, string(req.Status), req.Duration, req.DecidedAt)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("request %s: %w", req.ID, store.ErrNotFound)
	}
	return nil
}

type scannable interface {
	Scan(dest ...any) error
}

func scanRequest(row scannable) (store.AccessRequest, error) {
	var req store.AccessRequest
	var kind, status string
	if err := row.Scan(&req.ID, &kind, &req.Value, &req.EnrollmentID, &req.Reason, &status, &req.Duration, &req.CreatedAt, &req.DecidedAt); err != nil {
		return store.AccessRequest{}, err
	}
	req.Kind = policy.Kind(kind)
	req.Status = store.RequestStatus(status)
	return req, nil
}
