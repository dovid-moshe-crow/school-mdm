package memory

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/dwdmsh/school-mdm/internal/policy"
	"github.com/dwdmsh/school-mdm/internal/store"
)

func systemKey(kind policy.Kind, value string) string {
	v := value
	if kind == policy.KindApp {
		v = policy.AppKey(value)
	}
	return string(kind) + "|" + v
}

func (s *Store) ListSystemAllowlist(context.Context) ([]store.SystemAllowlistItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]store.SystemAllowlistItem, 0, len(s.systemAllowlist))
	for _, it := range s.systemAllowlist {
		out = append(out, it)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return strings.ToLower(out[i].Value) < strings.ToLower(out[j].Value)
	})
	return out, nil
}

func (s *Store) UpsertSystemAllowlist(_ context.Context, item store.SystemAllowlistItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	item.Value = policy.Normalize(item.Kind, item.Value)
	if item.Value == "" {
		return fmt.Errorf("value is required")
	}
	s.systemAllowlist[systemKey(item.Kind, item.Value)] = item
	return nil
}

func (s *Store) SetSystemAllowlistEnabled(_ context.Context, kind policy.Kind, value string, enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := systemKey(kind, policy.Normalize(kind, value))
	it, ok := s.systemAllowlist[k]
	if !ok {
		return store.ErrNotFound
	}
	it.Enabled = enabled
	s.systemAllowlist[k] = it
	return nil
}

func (s *Store) DeleteSystemAllowlist(_ context.Context, kind policy.Kind, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := systemKey(kind, policy.Normalize(kind, value))
	if _, ok := s.systemAllowlist[k]; !ok {
		return store.ErrNotFound
	}
	delete(s.systemAllowlist, k)
	return nil
}
