package memory

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/dwdmsh/school-mdm/internal/store"
)

func (s *Store) ListAPITokens(_ context.Context) ([]store.APIToken, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]store.APIToken, 0, len(s.apiTokens))
	for i := len(s.apiTokens) - 1; i >= 0; i-- {
		out = append(out, cloneAPIToken(s.apiTokens[i]))
	}
	return out, nil
}

func (s *Store) CreateAPIToken(_ context.Context, tok store.APIToken) (store.APIToken, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if tok.ID == "" {
		tok.ID = uuid.NewString()
	}
	if tok.CreatedAt.IsZero() {
		tok.CreatedAt = time.Now().UTC()
	}
	s.apiTokens = append(s.apiTokens, cloneAPIToken(tok))
	return cloneAPIToken(tok), nil
}

func (s *Store) DeleteAPIToken(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	kept := s.apiTokens[:0]
	found := false
	for _, tok := range s.apiTokens {
		if tok.ID == id {
			found = true
			continue
		}
		kept = append(kept, tok)
	}
	if !found {
		return fmt.Errorf("api token %s: %w", id, store.ErrNotFound)
	}
	s.apiTokens = kept
	return nil
}

func (s *Store) TouchAPIToken(_ context.Context, rawToken string) (store.APIToken, error) {
	hash := store.HashAPIToken(rawToken)
	if hash == "" || strings.TrimSpace(rawToken) == "" {
		return store.APIToken{}, fmt.Errorf("api token: %w", store.ErrNotFound)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	for i, tok := range s.apiTokens {
		if tok.TokenHash != hash {
			continue
		}
		s.apiTokens[i].LastUsedAt = &now
		return cloneAPIToken(s.apiTokens[i]), nil
	}
	return store.APIToken{}, fmt.Errorf("api token: %w", store.ErrNotFound)
}

func cloneAPIToken(t store.APIToken) store.APIToken {
	out := t
	if t.LastUsedAt != nil {
		v := *t.LastUsedAt
		out.LastUsedAt = &v
	}
	return out
}
