package memory

import (
	"context"
	"encoding/json"
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
	devices       map[string]string // enrollmentID -> name
	unrestricted  map[string]bool
	activityEvents  []store.ActivityEvent
	packs         map[string]store.WhitelistPack
	packItems     map[string][]store.WhitelistPackItem            // packID -> items
	packAssign    map[string][]store.WhitelistPackAssignment      // packID -> assignments

	credits   map[string]store.DeviceCredits
	ledger    []store.CreditLedgerEntry
	ledgerKey map[string]struct{} // reason|refType|refID
	packages  map[string]store.CreditPackage
	purchases map[string]store.CreditPurchase
	clientUID map[string]string // clientUniqueID -> purchaseID
	settings    *store.CreditSettings
	mdmSettings *store.MDMSettings
	abmDevices  json.RawMessage
	abmSyncedAt *time.Time
	pushTokens  map[string]store.DevicePushToken // token -> row

	allotmentRules  map[string]store.CreditAllotmentRule
	allotmentGrants map[string]store.CreditAllotmentGrant // id -> grant

	webhooks          []store.WebhookEndpoint
	webhookDeliveries []store.WebhookDelivery
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
		members:      map[string]map[string]struct{}{},
		devices:      map[string]string{},
		unrestricted: map[string]bool{},
		packs:        map[string]store.WhitelistPack{},
		packItems:    map[string][]store.WhitelistPackItem{},
		packAssign:   map[string][]store.WhitelistPackAssignment{},
		credits:      map[string]store.DeviceCredits{},
		ledger:          nil,
		ledgerKey:       map[string]struct{}{},
		packages:        map[string]store.CreditPackage{},
		purchases:       map[string]store.CreditPurchase{},
		clientUID:       map[string]string{},
		pushTokens:      map[string]store.DevicePushToken{},
		allotmentRules:  map[string]store.CreditAllotmentRule{},
		allotmentGrants: map[string]store.CreditAllotmentGrant{},
	}
	for _, app := range policy.Essentials {
		id := uuid.NewString()
		t := policy.Target{Type: policy.TargetGlobal}
		s.allowlist[entryKey(policy.KindApp, app, t)] = policy.Entry{
			ID: id, Kind: policy.KindApp, Value: app, Target: t,
		}
	}
	seedCreditPackages(s)
	return s
}

func seedCreditPackages(s *Store) {
	for _, p := range defaultCreditPackages() {
		s.packages[p.ID] = p
	}
}

func defaultCreditPackages() []store.CreditPackage {
	return []store.CreditPackage{
		{ID: "a0000000-0000-4000-8000-000000000010", NameHe: "10 קרדיטים", Credits: 10, PriceAgorot: 1000, Active: true, SortOrder: 10},
		{ID: "a0000000-0000-4000-8000-000000000050", NameHe: "50 קרדיטים", Credits: 50, PriceAgorot: 4500, Active: true, SortOrder: 20},
		{ID: "a0000000-0000-4000-8000-000000000100", NameHe: "100 קרדיטים", Credits: 100, PriceAgorot: 8000, Active: true, SortOrder: 30},
	}
}

func (s *Store) Ping(context.Context) error { return nil }
func (s *Store) Kind() string               { return "memory" }

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

func (s *Store) ListWhitelistPacks(context.Context) ([]store.WhitelistPack, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]store.WhitelistPack, 0, len(s.packs))
	for _, p := range s.packs {
		p.ItemCount = len(s.packItems[p.ID])
		out = append(out, p)
	}
	return out, nil
}

func (s *Store) GetWhitelistPack(_ context.Context, id string) (store.WhitelistPack, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.packs[id]
	if !ok {
		return store.WhitelistPack{}, store.ErrNotFound
	}
	p.ItemCount = len(s.packItems[id])
	return p, nil
}

func (s *Store) CreateWhitelistPack(_ context.Context, p store.WhitelistPack) (store.WhitelistPack, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" {
		return store.WhitelistPack{}, fmt.Errorf("name is required")
	}
	if p.ID == "" {
		p.ID = uuid.NewString()
	}
	p.CreatedAt = time.Now().UTC()
	s.packs[p.ID] = p
	return p, nil
}

func (s *Store) UpdateWhitelistPack(_ context.Context, p store.WhitelistPack) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, ok := s.packs[p.ID]
	if !ok {
		return store.ErrNotFound
	}
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" {
		return fmt.Errorf("name is required")
	}
	cur.Name = p.Name
	cur.Description = strings.TrimSpace(p.Description)
	s.packs[p.ID] = cur
	return nil
}

func (s *Store) DeleteWhitelistPack(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.packs[id]; !ok {
		return store.ErrNotFound
	}
	delete(s.packs, id)
	delete(s.packItems, id)
	delete(s.packAssign, id)
	return nil
}

func (s *Store) ListWhitelistPackItems(_ context.Context, packID string) ([]store.WhitelistPackItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := s.packItems[packID]
	out := make([]store.WhitelistPackItem, len(items))
	copy(out, items)
	return out, nil
}

func (s *Store) AddWhitelistPackItem(_ context.Context, item store.WhitelistPackItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.packs[item.PackID]; !ok {
		return store.ErrNotFound
	}
	item.Value = policy.Normalize(item.Kind, item.Value)
	for i, existing := range s.packItems[item.PackID] {
		same := existing.Kind == item.Kind && existing.Value == item.Value
		if item.Kind == policy.KindApp {
			same = existing.Kind == item.Kind && policy.AppKey(existing.Value) == policy.AppKey(item.Value)
		}
		if same {
			s.packItems[item.PackID][i] = item
			return nil
		}
	}
	s.packItems[item.PackID] = append(s.packItems[item.PackID], item)
	return nil
}

func (s *Store) RemoveWhitelistPackItem(_ context.Context, packID string, kind policy.Kind, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	value = policy.Normalize(kind, value)
	items := s.packItems[packID]
	next := items[:0]
	for _, it := range items {
		same := it.Kind == kind && it.Value == value
		if kind == policy.KindApp {
			same = it.Kind == kind && policy.AppKey(it.Value) == policy.AppKey(value)
		}
		if !same {
			next = append(next, it)
		}
	}
	s.packItems[packID] = next
	return nil
}

func (s *Store) ListWhitelistPackAssignments(_ context.Context, packID string) ([]store.WhitelistPackAssignment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	as := s.packAssign[packID]
	out := make([]store.WhitelistPackAssignment, len(as))
	copy(out, as)
	return out, nil
}

func (s *Store) SetWhitelistPackAssignment(_ context.Context, a store.WhitelistPackAssignment) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.packs[a.PackID]; !ok {
		return store.ErrNotFound
	}
	t := policy.Target{Type: a.TargetType, ID: a.TargetID}
	normalizeTarget(&t)
	a.TargetType, a.TargetID = t.Type, t.ID
	for _, existing := range s.packAssign[a.PackID] {
		if existing.TargetType == a.TargetType && existing.TargetID == a.TargetID {
			return nil
		}
	}
	s.packAssign[a.PackID] = append(s.packAssign[a.PackID], a)
	return nil
}

func (s *Store) RemoveWhitelistPackAssignment(_ context.Context, packID string, target policy.Target) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	normalizeTarget(&target)
	as := s.packAssign[packID]
	next := as[:0]
	for _, a := range as {
		if !(a.TargetType == target.Type && a.TargetID == target.ID) {
			next = append(next, a)
		}
	}
	s.packAssign[packID] = next
	return nil
}

func (s *Store) ListAllowlistFromPacks(_ context.Context, enrollmentID string, groupIDs []string) ([]policy.Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	groupSet := map[string]struct{}{}
	for _, g := range groupIDs {
		groupSet[g] = struct{}{}
	}
	seen := map[string]struct{}{}
	var out []policy.Entry
	for packID, assigns := range s.packAssign {
		applies := false
		for _, a := range assigns {
			switch a.TargetType {
			case policy.TargetGlobal:
				applies = true
			case policy.TargetDevice:
				if a.TargetID == enrollmentID {
					applies = true
				}
			case policy.TargetGroup:
				if _, ok := groupSet[a.TargetID]; ok {
					applies = true
				}
			}
			if applies {
				break
			}
		}
		if !applies {
			continue
		}
		for _, it := range s.packItems[packID] {
			k := string(it.Kind) + "|" + it.Value
			if _, ok := seen[k]; ok {
				continue
			}
			seen[k] = struct{}{}
			out = append(out, policy.Entry{
				Kind:   it.Kind,
				Value:  it.Value,
				Target: policy.Target{Type: policy.TargetGlobal},
			})
		}
	}
	return out, nil
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

func (s *Store) DeleteRequest(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.requests[id]; !ok {
		return fmt.Errorf("request %s: %w", id, store.ErrNotFound)
	}
	delete(s.requests, id)
	delete(s.messages, id)
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
	meta, ok := s.apps[policy.AppKey(bundleID)]
	if !ok {
		return store.AppMeta{}, fmt.Errorf("app %s: %w", bundleID, store.ErrNotFound)
	}
	return meta, nil
}

func (s *Store) UpsertAppMeta(_ context.Context, meta store.AppMeta) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	meta.BundleID = strings.TrimSpace(meta.BundleID)
	if meta.BundleID == "" {
		return fmt.Errorf("bundle_id is required")
	}
	if meta.UpdatedAt.IsZero() {
		meta.UpdatedAt = time.Now().UTC()
	}
	s.apps[policy.AppKey(meta.BundleID)] = meta
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
		out = append(out, store.Device{
			EnrollmentID: id,
			Name:         s.devices[id],
			Unrestricted: s.unrestricted[id],
		})
	}
	return out, nil
}

func (s *Store) GetDevice(_ context.Context, enrollmentID string) (store.Device, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	enrollmentID = strings.TrimSpace(enrollmentID)
	name, ok := s.devices[enrollmentID]
	if !ok && !s.unrestricted[enrollmentID] {
		return store.Device{}, fmt.Errorf("device not found")
	}
	return store.Device{
		EnrollmentID: enrollmentID,
		Name:         name,
		Unrestricted: s.unrestricted[enrollmentID],
	}, nil
}

func (s *Store) EnsureDevice(_ context.Context, enrollmentID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	enrollmentID = strings.TrimSpace(enrollmentID)
	if enrollmentID == "" {
		return fmt.Errorf("enrollment_id is required")
	}
	if _, ok := s.devices[enrollmentID]; !ok {
		s.devices[enrollmentID] = ""
	}
	return nil
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

func (s *Store) SetDeviceUnrestricted(_ context.Context, enrollmentID string, unrestricted bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	enrollmentID = strings.TrimSpace(enrollmentID)
	if enrollmentID == "" {
		return fmt.Errorf("enrollment_id is required")
	}
	if _, ok := s.devices[enrollmentID]; !ok {
		s.devices[enrollmentID] = ""
	}
	s.unrestricted[enrollmentID] = unrestricted
	return nil
}

func (s *Store) InsertActivityEvent(_ context.Context, e store.ActivityEvent) (store.ActivityEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e.ID == "" {
		e.ID = uuid.NewString()
	}
	if e.At.IsZero() {
		e.At = time.Now().UTC()
	} else {
		e.At = e.At.UTC()
	}
	if e.Result == "" {
		e.Result = store.ActivityResultOK
	}
	if e.ActorType == "" {
		e.ActorType = store.ActivityActorSystem
	}
	if len(e.Detail) == 0 {
		e.Detail = json.RawMessage(`{}`)
	}
	s.activityEvents = append([]store.ActivityEvent{e}, s.activityEvents...)
	return e, nil
}

func (s *Store) ListActivityEvents(_ context.Context, f store.ActivityFilter) ([]store.ActivityEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	limit := f.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	offset := f.Offset
	if offset < 0 {
		offset = 0
	}
	q := strings.ToLower(strings.TrimSpace(f.Q))
	var matched []store.ActivityEvent
	for _, e := range s.activityEvents {
		if f.From != nil && e.At.Before(f.From.UTC()) {
			continue
		}
		if f.To != nil && e.At.After(f.To.UTC()) {
			continue
		}
		if c := strings.TrimSpace(f.Category); c != "" && e.Category != c {
			continue
		}
		if a := strings.TrimSpace(f.Action); a != "" && e.Action != a {
			continue
		}
		if id := strings.TrimSpace(f.EnrollmentID); id != "" && e.EnrollmentID != id {
			continue
		}
		if at := strings.TrimSpace(f.ActorType); at != "" && e.ActorType != at {
			continue
		}
		if r := strings.TrimSpace(f.Result); r != "" && e.Result != r {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(e.Summary), q) {
			continue
		}
		matched = append(matched, e)
	}
	if offset >= len(matched) {
		return []store.ActivityEvent{}, nil
	}
	end := offset + limit
	if end > len(matched) {
		end = len(matched)
	}
	out := make([]store.ActivityEvent, end-offset)
	copy(out, matched[offset:end])
	return out, nil
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
	v := value
	if kind == policy.KindApp {
		v = policy.AppKey(value)
	}
	return string(kind) + "|" + v + "|" + string(t.Type) + "|" + t.ID
}
