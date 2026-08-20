package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/dwdmsh/school-mdm/internal/store"
)

func (s *Store) ListAPITokens(ctx context.Context) ([]store.APIToken, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, name, prefix, token_hash, created_by, created_at, last_used_at
		FROM api_tokens
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []store.APIToken
	for rows.Next() {
		tok, err := scanAPIToken(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, tok)
	}
	if out == nil {
		out = []store.APIToken{}
	}
	return out, rows.Err()
}

func (s *Store) CreateAPIToken(ctx context.Context, tok store.APIToken) (store.APIToken, error) {
	if tok.ID == "" {
		tok.ID = uuid.NewString()
	}
	err := s.pool.QueryRow(ctx, `
		INSERT INTO api_tokens (id, name, prefix, token_hash, created_by)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING created_at
	`, tok.ID, tok.Name, tok.Prefix, tok.TokenHash, tok.CreatedBy).Scan(&tok.CreatedAt)
	if err != nil {
		return store.APIToken{}, err
	}
	return tok, nil
}

func (s *Store) DeleteAPIToken(ctx context.Context, id string) error {
	ct, err := s.pool.Exec(ctx, `DELETE FROM api_tokens WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("api token %s: %w", id, store.ErrNotFound)
	}
	return nil
}

func (s *Store) TouchAPIToken(ctx context.Context, rawToken string) (store.APIToken, error) {
	hash := store.HashAPIToken(rawToken)
	if hash == "" || strings.TrimSpace(rawToken) == "" {
		return store.APIToken{}, fmt.Errorf("api token: %w", store.ErrNotFound)
	}
	now := time.Now().UTC()
	row := s.pool.QueryRow(ctx, `
		UPDATE api_tokens SET last_used_at=$2
		WHERE token_hash=$1
		RETURNING id, name, prefix, token_hash, created_by, created_at, last_used_at
	`, hash, now)
	tok, err := scanAPIToken(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return store.APIToken{}, fmt.Errorf("api token: %w", store.ErrNotFound)
		}
		return store.APIToken{}, err
	}
	return tok, nil
}

type apiTokenRow interface {
	Scan(dest ...any) error
}

func scanAPIToken(row apiTokenRow) (store.APIToken, error) {
	var tok store.APIToken
	if err := row.Scan(&tok.ID, &tok.Name, &tok.Prefix, &tok.TokenHash, &tok.CreatedBy, &tok.CreatedAt, &tok.LastUsedAt); err != nil {
		return store.APIToken{}, err
	}
	tok.TokenHash = ""
	return tok, nil
}
