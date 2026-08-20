package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/dwdmsh/school-mdm/internal/policy"
	"github.com/dwdmsh/school-mdm/internal/store"
)

func (s *Store) seedSystemAllowlist(ctx context.Context) error {
	for _, app := range policy.SystemDefaults {
		app = strings.TrimSpace(app)
		if app == "" {
			continue
		}
		if _, err := s.pool.Exec(ctx, `
			INSERT INTO system_allowlist (kind, value)
			VALUES ('app', $1)
			ON CONFLICT DO NOTHING`, app); err != nil {
			return fmt.Errorf("seed system allowlist %s: %w", app, err)
		}
	}
	return nil
}

func (s *Store) ListSystemAllowlist(ctx context.Context) ([]store.SystemAllowlistItem, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT kind, value, enabled
		FROM system_allowlist
		ORDER BY kind, lower(value)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []store.SystemAllowlistItem
	for rows.Next() {
		var it store.SystemAllowlistItem
		var kind string
		if err := rows.Scan(&kind, &it.Value, &it.Enabled); err != nil {
			return nil, err
		}
		it.Kind = policy.Kind(kind)
		out = append(out, it)
	}
	return out, rows.Err()
}

func (s *Store) UpsertSystemAllowlist(ctx context.Context, item store.SystemAllowlistItem) error {
	item.Value = policy.Normalize(item.Kind, item.Value)
	if item.Value == "" {
		return fmt.Errorf("value is required")
	}
	if item.Kind != policy.KindApp && item.Kind != policy.KindURL {
		return fmt.Errorf("kind must be app or url")
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO system_allowlist (kind, value, enabled)
		VALUES ($1, $2, $3)
		ON CONFLICT (kind, value) DO UPDATE SET enabled = EXCLUDED.enabled`,
		string(item.Kind), item.Value, item.Enabled)
	return err
}

func (s *Store) SetSystemAllowlistEnabled(ctx context.Context, kind policy.Kind, value string, enabled bool) error {
	value = policy.Normalize(kind, value)
	tag, err := s.pool.Exec(ctx, `
		UPDATE system_allowlist SET enabled=$3
		WHERE kind=$1 AND lower(value)=lower($2)`, string(kind), value, enabled)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return store.ErrNotFound
	}
	return nil
}

func (s *Store) DeleteSystemAllowlist(ctx context.Context, kind policy.Kind, value string) error {
	value = policy.Normalize(kind, value)
	tag, err := s.pool.Exec(ctx, `
		DELETE FROM system_allowlist WHERE kind=$1 AND lower(value)=lower($2)`,
		string(kind), value)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return store.ErrNotFound
	}
	return nil
}
