package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/dwdmsh/school-mdm/internal/store"
)

func (s *Store) ListWebhookEndpoints(ctx context.Context) ([]store.WebhookEndpoint, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, url, secret, description, COALESCE(events, '{}'), enabled, created_at
		FROM webhook_endpoints
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []store.WebhookEndpoint
	for rows.Next() {
		ep, err := scanWebhook(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, ep)
	}
	if out == nil {
		out = []store.WebhookEndpoint{}
	}
	return out, rows.Err()
}

func (s *Store) GetWebhookEndpoint(ctx context.Context, id string) (store.WebhookEndpoint, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, url, secret, description, COALESCE(events, '{}'), enabled, created_at
		FROM webhook_endpoints WHERE id=$1
	`, id)
	ep, err := scanWebhook(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return store.WebhookEndpoint{}, fmt.Errorf("webhook %s: %w", id, store.ErrNotFound)
		}
		return store.WebhookEndpoint{}, err
	}
	return ep, nil
}

func (s *Store) CreateWebhookEndpoint(ctx context.Context, ep store.WebhookEndpoint) (store.WebhookEndpoint, error) {
	if ep.ID == "" {
		ep.ID = uuid.NewString()
	}
	if ep.Events == nil {
		ep.Events = []string{}
	}
	err := s.pool.QueryRow(ctx, `
		INSERT INTO webhook_endpoints (id, url, secret, description, events, enabled)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING created_at
	`, ep.ID, ep.URL, ep.Secret, ep.Description, ep.Events, ep.Enabled).Scan(&ep.CreatedAt)
	if err != nil {
		return store.WebhookEndpoint{}, err
	}
	return ep, nil
}

func (s *Store) UpdateWebhookEndpoint(ctx context.Context, ep store.WebhookEndpoint) (store.WebhookEndpoint, error) {
	if ep.Events == nil {
		ep.Events = []string{}
	}
	err := s.pool.QueryRow(ctx, `
		UPDATE webhook_endpoints
		SET url=$2, secret=$3, description=$4, events=$5, enabled=$6
		WHERE id=$1
		RETURNING created_at
	`, ep.ID, ep.URL, ep.Secret, ep.Description, ep.Events, ep.Enabled).Scan(&ep.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return store.WebhookEndpoint{}, fmt.Errorf("webhook %s: %w", ep.ID, store.ErrNotFound)
		}
		return store.WebhookEndpoint{}, err
	}
	return ep, nil
}

func (s *Store) DeleteWebhookEndpoint(ctx context.Context, id string) error {
	ct, err := s.pool.Exec(ctx, `DELETE FROM webhook_endpoints WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("webhook %s: %w", id, store.ErrNotFound)
	}
	return nil
}

func (s *Store) InsertWebhookDelivery(ctx context.Context, d store.WebhookDelivery) (store.WebhookDelivery, error) {
	if d.ID == "" {
		d.ID = uuid.NewString()
	}
	if d.Attempt <= 0 {
		d.Attempt = 1
	}
	err := s.pool.QueryRow(ctx, `
		INSERT INTO webhook_deliveries (
			id, endpoint_id, event_id, event_name, status, attempt, http_status, error
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING created_at
	`, d.ID, d.EndpointID, d.EventID, d.EventName, d.Status, d.Attempt, d.HTTPStatus, d.Error).Scan(&d.CreatedAt)
	if err != nil {
		return store.WebhookDelivery{}, err
	}
	return d, nil
}

func (s *Store) ListWebhookDeliveries(ctx context.Context, endpointID string, limit int) ([]store.WebhookDelivery, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, endpoint_id, event_id, event_name, status, attempt, http_status, error, created_at
		FROM webhook_deliveries
		WHERE endpoint_id=$1
		ORDER BY created_at DESC
		LIMIT $2
	`, strings.TrimSpace(endpointID), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []store.WebhookDelivery
	for rows.Next() {
		var d store.WebhookDelivery
		if err := rows.Scan(
			&d.ID, &d.EndpointID, &d.EventID, &d.EventName, &d.Status,
			&d.Attempt, &d.HTTPStatus, &d.Error, &d.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	if out == nil {
		out = []store.WebhookDelivery{}
	}
	return out, rows.Err()
}

type webhookRow interface {
	Scan(dest ...any) error
}

func scanWebhook(row webhookRow) (store.WebhookEndpoint, error) {
	var ep store.WebhookEndpoint
	if err := row.Scan(&ep.ID, &ep.URL, &ep.Secret, &ep.Description, &ep.Events, &ep.Enabled, &ep.CreatedAt); err != nil {
		return store.WebhookEndpoint{}, err
	}
	if ep.Events == nil {
		ep.Events = []string{}
	}
	return ep, nil
}
