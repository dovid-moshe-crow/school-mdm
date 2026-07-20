package memory

import (
	"context"
	"fmt"
	"strings"
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
	requests  map[string]store.Request
	messages  map[string][]store.RequestMessage // requestID -> chronological
	apps      map[string]store.AppMeta
	groups    map[string]store.Group
	members   map[string]map[string]struct{} // groupID -> enrollmentID set
	devices   map[string]string              // enrollmentID -> name
}

// New creates an empty memory store seeded with essentials as durable entries.
func New() *Store {
	s := &Store{
		allowlist: map[string]policy.Entry{},
		grants:    map[string]policy.Grant{},
		requests:  map[string]store.Request{},
		messages:  map[string][]store.RequestMessage{},
		apps:      map[string]store.AppMeta{},
		groups:    map[string]store.Group{},
		members:   map[string]map[string]struct{}{},
		devices:   map[string]string{},
	}
	for _, app := range policy.Essentials {
		id := uuid.NewString()
		t := policy.Target{Type: policy.TargetGlobal}
		s.allowlist[entryKey(policy.KindApp, app, t)] = policy.Entry{
			ID: id, Kind: policy.KindApp, Value: app, Target: t,
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
	normalizeTarget(&entry.Target)
	k := entryKey(entry.Kind, entry.Value, entry.Target)
	if existing, ok := s.allowlist[k]; ok {
		entry.ID = existing.ID
	} else if entry.ID == "" {
		entry.ID = uuid.NewString()
	}
	s.allowlist[k] = entry
	return nil
}

func (s *Store) DeleteAllowlist(_ context.Context, kind policy.Kind, value string, target policy.Target) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	value = policy.Normalize(kind, value)
	normalizeTarget(&target)
	delete(s.allowlist, entryKey(kind, value, target))
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
	normalizeTarget(&grant.Target)
	s.grants[grant.ID] = grant
	return nil
}

func (s *Store) DeleteGrants(_ context.Context, kind policy.Kind, value string, target policy.Target) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	value = policy.Normalize(kind, value)
	normalizeTarget(&target)
	for id, g := range s.grants {
		if g.Kind == kind && g.Value == value && g.Target.Type == target.Type && g.Target.ID == target.ID {
			delete(s.grants, id)
		}
	}
	return nil
}

func (s *Store) CreateRequest(_ context.Context, req store.Request) (store.Request, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if req.ID == "" {
		req.ID = uuid.NewString()
	}
	if req.Status == "" {
		req.Status = store.StatusPending
	}
	if req.CreatedAt.IsZero() {
		req.CreatedAt = time.Now().UTC()
	}
	s.requests[req.ID] = req
	return req, nil
}

func (s *Store) GetRequest(_ context.Context, id string) (store.Request, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	req, ok := s.requests[id]
	if !ok {
		return store.Request{}, fmt.Errorf("request %s: %w", id, store.ErrNotFound)
	}
	return req, nil
}

func (s *Store) ListRequests(_ context.Context, status *store.RequestStatus) ([]store.Request, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]store.Request, 0, len(s.requests))
	for _, r := range s.requests {
		if status != nil && r.Status != *status {
			continue
		}
		out = append(out, r)
	}
	return out, nil
}

func (s *Store) ListRequestsByEnrollment(_ context.Context, enrollmentID string) ([]store.Request, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	enrollmentID = strings.TrimSpace(enrollmentID)
	out := make([]store.Request, 0)
	for _, r := range s.requests {
		if r.EnrollmentID == enrollmentID {
			out = append(out, r)
		}
	}
	return out, nil
}

func (s *Store) UpdateRequest(_ context.Context, req store.Request) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.requests[req.ID]; !ok {
		return fmt.Errorf("request %s: %w", req.ID, store.ErrNotFound)
	}
	s.requests[req.ID] = req
	return nil
}

func (s *Store) ListRequestMessages(_ context.Context, requestID string) ([]store.RequestMessage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.requests[requestID]; !ok {
		return nil, fmt.Errorf("request %s: %w", requestID, store.ErrNotFound)
	}
	src := s.messages[requestID]
	out := make([]store.RequestMessage, len(src))
	copy(out, src)
	return out, nil
}

func (s *Store) AddRequestMessage(_ context.Context, msg store.RequestMessage) (store.RequestMessage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.requests[msg.RequestID]; !ok {
		return store.RequestMessage{}, fmt.Errorf("request %s: %w", msg.RequestID, store.ErrNotFound)
	}
	if msg.ID == "" {
		msg.ID = uuid.NewString()
	}
	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = time.Now().UTC()
	}
	s.messages[msg.RequestID] = append(s.messages[msg.RequestID], msg)
	return msg, nil
}

func (s *Store) CountRequestMessages(_ context.Context, requestID string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.requests[requestID]; !ok {
		return 0, fmt.Errorf("request %s: %w", requestID, store.ErrNotFound)
	}
	return len(s.messages[requestID]), nil
}

func (s *Store) LastRequestMessage(_ context.Context, requestID string) (store.RequestMessage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.requests[requestID]; !ok {
		return store.RequestMessage{}, fmt.Errorf("request %s: %w", requestID, store.ErrNotFound)
	}
	msgs := s.messages[requestID]
	if len(msgs) == 0 {
		return store.RequestMessage{}, store.ErrNotFound
	}
	return msgs[len(msgs)-1], nil
}

func (s *Store) GetAppMeta(_ context.Context, bundleID string) (store.AppMeta, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	meta, ok := s.apps[strings.ToLower(strings.TrimSpace(bundleID))]
	if !ok {
		return store.AppMeta{}, fmt.Errorf("app %s: %w", bundleID, store.ErrNotFound)
	}
	return meta, nil
}

func (s *Store) UpsertAppMeta(_ context.Context, meta store.AppMeta) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	meta.BundleID = strings.ToLower(strings.TrimSpace(meta.BundleID))
	if meta.BundleID == "" {
		return fmt.Errorf("bundle_id is required")
	}
	if meta.UpdatedAt.IsZero() {
		meta.UpdatedAt = time.Now().UTC()
	}
	s.apps[meta.BundleID] = meta
	return nil
}

func (s *Store) SearchAppMeta(_ context.Context, query string, limit int) ([]store.AppMeta, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	q := strings.ToLower(strings.TrimSpace(query))
	if limit <= 0 {
		limit = 12
	}
	out := make([]store.AppMeta, 0)
	for _, m := range s.apps {
		if strings.Contains(strings.ToLower(m.Name), q) ||
			strings.Contains(strings.ToLower(m.Artist), q) ||
			strings.Contains(m.BundleID, q) {
			out = append(out, m)
			if len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

func (s *Store) ListGroups(context.Context) ([]store.Group, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]store.Group, 0, len(s.groups))
	for _, g := range s.groups {
		out = append(out, g)
	}
	return out, nil
}

func (s *Store) GetGroup(_ context.Context, id string) (store.Group, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	g, ok := s.groups[id]
	if !ok {
		return store.Group{}, fmt.Errorf("group %s: %w", id, store.ErrNotFound)
	}
	return g, nil
}

func (s *Store) CreateGroup(_ context.Context, g store.Group) (store.Group, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	g.Name = strings.TrimSpace(g.Name)
	if g.Name == "" {
		return store.Group{}, fmt.Errorf("group name is required")
	}
	for _, existing := range s.groups {
		if strings.EqualFold(existing.Name, g.Name) {
			return store.Group{}, fmt.Errorf("group name %q already exists", g.Name)
		}
	}
	if g.ID == "" {
		g.ID = uuid.NewString()
	}
	if g.CreatedAt.IsZero() {
		g.CreatedAt = time.Now().UTC()
	}
	s.groups[g.ID] = g
	s.members[g.ID] = map[string]struct{}{}
	return g, nil
}

func (s *Store) UpdateGroup(_ context.Context, g store.Group) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	existing, ok := s.groups[g.ID]
	if !ok {
		return fmt.Errorf("group %s: %w", g.ID, store.ErrNotFound)
	}
	g.Name = strings.TrimSpace(g.Name)
	if g.Name == "" {
		return fmt.Errorf("group name is required")
	}
	for id, other := range s.groups {
		if id != g.ID && strings.EqualFold(other.Name, g.Name) {
			return fmt.Errorf("group name %q already exists", g.Name)
		}
	}
	g.CreatedAt = existing.CreatedAt
	s.groups[g.ID] = g
	return nil
}

func (s *Store) DeleteGroup(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.groups[id]; !ok {
		return fmt.Errorf("group %s: %w", id, store.ErrNotFound)
	}
	delete(s.groups, id)
	delete(s.members, id)
	return nil
}

func (s *Store) ListGroupMembers(_ context.Context, groupID string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.groups[groupID]; !ok {
		return nil, fmt.Errorf("group %s: %w", groupID, store.ErrNotFound)
	}
	m := s.members[groupID]
	out := make([]string, 0, len(m))
	for id := range m {
		out = append(out, id)
	}
	return out, nil
}

func (s *Store) SetGroupMembers(_ context.Context, groupID string, enrollmentIDs []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.groups[groupID]; !ok {
		return fmt.Errorf("group %s: %w", groupID, store.ErrNotFound)
	}
	set := map[string]struct{}{}
	for _, id := range enrollmentIDs {
		id = strings.TrimSpace(id)
		if id != "" {
			set[id] = struct{}{}
		}
	}
	s.members[groupID] = set
	return nil
}

func (s *Store) AddGroupMember(_ context.Context, groupID, enrollmentID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.groups[groupID]; !ok {
		return fmt.Errorf("group %s: %w", groupID, store.ErrNotFound)
	}
	enrollmentID = strings.TrimSpace(enrollmentID)
	if enrollmentID == "" {
		return fmt.Errorf("enrollment_id is required")
	}
	if s.members[groupID] == nil {
		s.members[groupID] = map[string]struct{}{}
	}
	s.members[groupID][enrollmentID] = struct{}{}
	return nil
}

func (s *Store) RemoveGroupMember(_ context.Context, groupID, enrollmentID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.groups[groupID]; !ok {
		return fmt.Errorf("group %s: %w", groupID, store.ErrNotFound)
	}
	delete(s.members[groupID], strings.TrimSpace(enrollmentID))
	return nil
}

func (s *Store) ListGroupsForDevice(_ context.Context, enrollmentID string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	enrollmentID = strings.TrimSpace(enrollmentID)
	var out []string
	for gid, members := range s.members {
		if _, ok := members[enrollmentID]; ok {
			out = append(out, gid)
		}
	}
	return out, nil
}

func (s *Store) ListEnrollmentIDsForGroup(_ context.Context, groupID string) ([]string, error) {
	return s.ListGroupMembers(context.Background(), groupID)
}

func (s *Store) ListAllEnrollmentIDs(context.Context) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	set := map[string]struct{}{}
	for _, req := range s.requests {
		if id := strings.TrimSpace(req.EnrollmentID); id != "" {
			set[id] = struct{}{}
		}
	}
	for _, g := range s.grants {
		if g.Target.Type == policy.TargetDevice {
			if id := strings.TrimSpace(g.Target.ID); id != "" {
				set[id] = struct{}{}
			}
		}
	}
	for _, members := range s.members {
		for id := range members {
			set[id] = struct{}{}
		}
	}
	for _, e := range s.allowlist {
		if e.Target.Type == policy.TargetDevice {
			if id := strings.TrimSpace(e.Target.ID); id != "" {
				set[id] = struct{}{}
			}
		}
	}
	for id := range s.devices {
		set[id] = struct{}{}
	}
	out := make([]string, 0, len(set))
	for id := range set {
		out = append(out, id)
	}
	return out, nil
}

func (s *Store) ListDevices(ctx context.Context) ([]store.Device, error) {
	ids, err := s.ListAllEnrollmentIDs(ctx)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]store.Device, 0, len(ids))
	for _, id := range ids {
		out = append(out, store.Device{EnrollmentID: id, Name: s.devices[id]})
	}
	return out, nil
}

func (s *Store) SetDeviceName(_ context.Context, enrollmentID, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	enrollmentID = strings.TrimSpace(enrollmentID)
	if enrollmentID == "" {
		return fmt.Errorf("enrollment_id is required")
	}
	s.devices[enrollmentID] = strings.TrimSpace(name)
	return nil
}

func normalizeTarget(t *policy.Target) {
	if t.Type == "" {
		t.Type = policy.TargetGlobal
	}
	if t.Type == policy.TargetGlobal {
		t.ID = ""
	}
}

func entryKey(kind policy.Kind, value string, t policy.Target) string {
	return string(kind) + "|" + value + "|" + string(t.Type) + "|" + t.ID
}
