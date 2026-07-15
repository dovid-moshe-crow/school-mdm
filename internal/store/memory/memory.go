package memory

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/dwdmsh/school-mdm/internal/policy"
	"github.com/dwdmsh/school-mdm/internal/store"
)

// Store is an in-memory Store for local development without Neon.
type Store struct {
	mu        sync.Mutex
	allowlist map[string]policy.Entry
	grants    map[string]policy.Grant
	requests  map[string]store.AccessRequest
}

// New creates an empty memory store seeded with essentials as durable entries.
func New() *Store {
	s := &Store{
		allowlist: map[string]policy.Entry{},
		grants:    map[string]policy.Grant{},
		requests:  map[string]store.AccessRequest{},
	}
	for _, app := range policy.Essentials {
		id := uuid.NewString()
		s.allowlist[key(policy.KindApp, app, "global")] = policy.Entry{
			ID: id, Kind: policy.KindApp, Value: app, Scope: "global",
		}
	}
	return s
}

func (s *Store) Ping(context.Context) error { return nil }
func (s *Store) Kind() string                { return "memory" }

func (s *Store) ListAllowlist(context.Context) ([]policy.Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]policy.Entry, 0, len(s.allowlist))
	for _, e := range s.allowlist {
		out = append(out, e)
	}
	return out, nil
}

func (s *Store) UpsertAllowlist(_ context.Context, entry policy.Entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry.Value = policy.Normalize(entry.Kind, entry.Value)
	if entry.Scope == "" {
		entry.Scope = "global"
	}
	k := key(entry.Kind, entry.Value, entry.Scope)
	if existing, ok := s.allowlist[k]; ok {
		entry.ID = existing.ID
	} else if entry.ID == "" {
		entry.ID = uuid.NewString()
	}
	s.allowlist[k] = entry
	return nil
}

func (s *Store) ListGrants(context.Context) ([]policy.Grant, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]policy.Grant, 0, len(s.grants))
	for _, g := range s.grants {
		out = append(out, g)
	}
	return out, nil
}

func (s *Store) AddGrant(_ context.Context, grant policy.Grant) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if grant.ID == "" {
		grant.ID = uuid.NewString()
	}
	grant.Value = policy.Normalize(grant.Kind, grant.Value)
	s.grants[grant.ID] = grant
	return nil
}

func (s *Store) CreateRequest(_ context.Context, req store.AccessRequest) (store.AccessRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
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
	s.requests[req.ID] = req
	return req, nil
}

func (s *Store) GetRequest(_ context.Context, id string) (store.AccessRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	req, ok := s.requests[id]
	if !ok {
		return store.AccessRequest{}, fmt.Errorf("request %s: %w", id, store.ErrNotFound)
	}
	return req, nil
}

func (s *Store) ListRequests(_ context.Context, status *store.RequestStatus) ([]store.AccessRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]store.AccessRequest, 0, len(s.requests))
	for _, r := range s.requests {
		if status != nil && r.Status != *status {
			continue
		}
		out = append(out, r)
	}
	return out, nil
}

func (s *Store) UpdateRequest(_ context.Context, req store.AccessRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.requests[req.ID]; !ok {
		return fmt.Errorf("request %s: %w", req.ID, store.ErrNotFound)
	}
	s.requests[req.ID] = req
	return nil
}

func key(kind policy.Kind, value, scope string) string {
	return string(kind) + "|" + value + "|" + scope
}
