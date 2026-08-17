package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/dwdmsh/school-mdm/internal/store"
)

func (s *Store) UpsertPushToken(ctx context.Context, t store.DevicePushToken) error {
	enrollment := strings.TrimSpace(t.EnrollmentID)
	token := strings.TrimSpace(t.Token)
	platform := strings.TrimSpace(t.Platform)
	if enrollment == "" || token == "" {
		return fmt.Errorf("enrollment_id and token required")
	}
	if platform == "" {
		platform = "ios"
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO device_push_tokens (enrollment_id, token, platform, updated_at)
		VALUES ($1, $2, $3, now())
		ON CONFLICT (token) DO UPDATE SET
			enrollment_id = EXCLUDED.enrollment_id,
			platform = EXCLUDED.platform,
			updated_at = now()
	`, enrollment, token, platform)
	return err
}

func (s *Store) ListPushTokens(ctx context.Context, enrollmentID string) ([]store.DevicePushToken, error) {
	enrollment := strings.TrimSpace(enrollmentID)
	if enrollment == "" {
		return nil, nil
	}
	rows, err := s.pool.Query(ctx, `
		SELECT enrollment_id, token, platform, updated_at
		FROM device_push_tokens
		WHERE enrollment_id = $1
		ORDER BY updated_at DESC
	`, enrollment)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []store.DevicePushToken
	for rows.Next() {
		var t store.DevicePushToken
		if err := rows.Scan(&t.EnrollmentID, &t.Token, &t.Platform, &t.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) DeletePushToken(ctx context.Context, token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil
	}
	_, err := s.pool.Exec(ctx, `DELETE FROM device_push_tokens WHERE token = $1`, token)
	return err
}

func (s *Store) HasPushToken(ctx context.Context, enrollmentID string) (bool, error) {
	enrollment := strings.TrimSpace(enrollmentID)
	if enrollment == "" {
		return false, nil
	}
	var n int
	err := s.pool.QueryRow(ctx, `
		SELECT COUNT(*)::int FROM device_push_tokens WHERE enrollment_id = $1
	`, enrollment).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}
