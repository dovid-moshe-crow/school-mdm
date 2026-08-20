package memory

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/dwdmsh/school-mdm/internal/store"
)

func (s *Store) ListPolicyTimers(context.Context) ([]store.PolicyTimer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]store.PolicyTimer, 0, len(s.policyTimers))
	for _, t := range s.policyTimers {
		out = append(out, clonePolicyTimer(t))
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].ID > out[j].ID
		}
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out, nil
}

func (s *Store) GetPolicyTimer(_ context.Context, id string) (store.PolicyTimer, error) {
	id = strings.TrimSpace(id)
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.policyTimers[id]
	if !ok {
		return store.PolicyTimer{}, fmt.Errorf("timer %s: %w", id, store.ErrNotFound)
	}
	return clonePolicyTimer(t), nil
}

func (s *Store) CreatePolicyTimer(_ context.Context, t store.PolicyTimer) (store.PolicyTimer, error) {
	if err := store.NormalizePolicyTimer(&t); err != nil {
		return store.PolicyTimer{}, err
	}
	now := time.Now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	if t.ID == "" {
		t.ID = uuid.NewString()
	}
	if _, exists := s.policyTimers[t.ID]; exists {
		return store.PolicyTimer{}, fmt.Errorf("timer %s already exists", t.ID)
	}
	t.CreatedAt = now
	t.UpdatedAt = now
	s.policyTimers[t.ID] = clonePolicyTimer(t)
	return clonePolicyTimer(t), nil
}

func (s *Store) UpdatePolicyTimer(_ context.Context, t store.PolicyTimer) (store.PolicyTimer, error) {
	t.ID = strings.TrimSpace(t.ID)
	if t.ID == "" {
		return store.PolicyTimer{}, fmt.Errorf("id is required")
	}
	if err := store.NormalizePolicyTimer(&t); err != nil {
		return store.PolicyTimer{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	existing, ok := s.policyTimers[t.ID]
	if !ok {
		return store.PolicyTimer{}, fmt.Errorf("timer %s: %w", t.ID, store.ErrNotFound)
	}
	t.CreatedAt = existing.CreatedAt
	t.UpdatedAt = time.Now().UTC()
	s.policyTimers[t.ID] = clonePolicyTimer(t)
	return clonePolicyTimer(t), nil
}

func (s *Store) DeletePolicyTimer(_ context.Context, id string) error {
	id = strings.TrimSpace(id)
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.policyTimers[id]; !ok {
		return fmt.Errorf("timer %s: %w", id, store.ErrNotFound)
	}
	delete(s.policyTimers, id)
	return nil
}

func (s *Store) TouchPolicyTimerRun(_ context.Context, id string, at time.Time, runKey string, enabled bool) error {
	id = strings.TrimSpace(id)
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.policyTimers[id]
	if !ok {
		return fmt.Errorf("timer %s: %w", id, store.ErrNotFound)
	}
	if at.IsZero() {
		at = time.Now().UTC()
	}
	t.LastRunAt = &at
	t.LastRunKey = strings.TrimSpace(runKey)
	t.Enabled = enabled
	t.UpdatedAt = at
	s.policyTimers[id] = t
	return nil
}

func clonePolicyTimer(t store.PolicyTimer) store.PolicyTimer {
	t.PackIDs = append([]string{}, t.PackIDs...)
	t.ProfileIDs = append([]string{}, t.ProfileIDs...)
	t.DeviceIDs = append([]string{}, t.DeviceIDs...)
	t.GroupIDs = append([]string{}, t.GroupIDs...)
	t.Weekdays = append([]int{}, t.Weekdays...)
	if t.RunAt != nil {
		v := *t.RunAt
		t.RunAt = &v
	}
	if t.LastRunAt != nil {
		v := *t.LastRunAt
		t.LastRunAt = &v
	}
	return t
}
