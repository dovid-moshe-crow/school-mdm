package memory

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/dwdmsh/school-mdm/internal/store"
)

func (s *Store) ListWebhookEndpoints(_ context.Context) ([]store.WebhookEndpoint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]store.WebhookEndpoint, 0, len(s.webhooks))
	for i := len(s.webhooks) - 1; i >= 0; i-- {
		out = append(out, cloneWebhook(s.webhooks[i]))
	}
	return out, nil
}

func (s *Store) GetWebhookEndpoint(_ context.Context, id string) (store.WebhookEndpoint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, ep := range s.webhooks {
		if ep.ID == id {
			return cloneWebhook(ep), nil
		}
	}
	return store.WebhookEndpoint{}, fmt.Errorf("webhook %s: %w", id, store.ErrNotFound)
}

func (s *Store) CreateWebhookEndpoint(_ context.Context, ep store.WebhookEndpoint) (store.WebhookEndpoint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ep.ID == "" {
		ep.ID = uuid.NewString()
	}
	if ep.CreatedAt.IsZero() {
		ep.CreatedAt = time.Now().UTC()
	}
	if ep.Events == nil {
		ep.Events = []string{}
	}
	s.webhooks = append(s.webhooks, cloneWebhook(ep))
	return cloneWebhook(ep), nil
}

func (s *Store) UpdateWebhookEndpoint(_ context.Context, ep store.WebhookEndpoint) (store.WebhookEndpoint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, existing := range s.webhooks {
		if existing.ID != ep.ID {
			continue
		}
		ep.CreatedAt = existing.CreatedAt
		if ep.Events == nil {
			ep.Events = []string{}
		}
		s.webhooks[i] = cloneWebhook(ep)
		return cloneWebhook(ep), nil
	}
	return store.WebhookEndpoint{}, fmt.Errorf("webhook %s: %w", ep.ID, store.ErrNotFound)
}

func (s *Store) DeleteWebhookEndpoint(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	found := false
	kept := s.webhooks[:0]
	for _, ep := range s.webhooks {
		if ep.ID == id {
			found = true
			continue
		}
		kept = append(kept, ep)
	}
	if !found {
		return fmt.Errorf("webhook %s: %w", id, store.ErrNotFound)
	}
	s.webhooks = kept
	dkept := s.webhookDeliveries[:0]
	for _, d := range s.webhookDeliveries {
		if d.EndpointID == id {
			continue
		}
		dkept = append(dkept, d)
	}
	s.webhookDeliveries = dkept
	return nil
}

func (s *Store) InsertWebhookDelivery(_ context.Context, d store.WebhookDelivery) (store.WebhookDelivery, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if d.ID == "" {
		d.ID = uuid.NewString()
	}
	if d.CreatedAt.IsZero() {
		d.CreatedAt = time.Now().UTC()
	}
	if d.Attempt <= 0 {
		d.Attempt = 1
	}
	s.webhookDeliveries = append([]store.WebhookDelivery{d}, s.webhookDeliveries...)
	return d, nil
}

func (s *Store) ListWebhookDeliveries(_ context.Context, endpointID string, limit int) ([]store.WebhookDelivery, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	endpointID = strings.TrimSpace(endpointID)
	out := make([]store.WebhookDelivery, 0, limit)
	for _, d := range s.webhookDeliveries {
		if d.EndpointID != endpointID {
			continue
		}
		out = append(out, d)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func cloneWebhook(ep store.WebhookEndpoint) store.WebhookEndpoint {
	cp := ep
	if ep.Events != nil {
		cp.Events = append([]string(nil), ep.Events...)
	}
	return cp
}
