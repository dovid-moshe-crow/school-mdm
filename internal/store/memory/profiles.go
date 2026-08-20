package memory

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/dwdmsh/school-mdm/internal/policy"
	"github.com/dwdmsh/school-mdm/internal/profiles"
	"github.com/dwdmsh/school-mdm/internal/store"
)

func cloneCustomProfile(p store.CustomProfile, includePayload bool) store.CustomProfile {
	out := p
	out.SizeBytes = len(p.Payload)
	if includePayload {
		out.Payload = append([]byte{}, p.Payload...)
	} else {
		out.Payload = nil
	}
	return out
}

func (s *Store) identifierOwner(identifier, exceptID string) string {
	for _, p := range s.customProfiles {
		if p.PayloadIdentifier == identifier && p.ID != exceptID {
			return p.ID
		}
	}
	return ""
}

func (s *Store) ListCustomProfiles(context.Context) ([]store.CustomProfile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]store.CustomProfile, 0, len(s.customProfiles))
	for _, p := range s.customProfiles {
		row := cloneCustomProfile(p, false)
		row.AssignmentCount = len(s.profileAssign[p.ID])
		out = append(out, row)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].Name < out[j].Name
		}
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out, nil
}

func (s *Store) GetCustomProfile(_ context.Context, id string) (store.CustomProfile, error) {
	id = strings.TrimSpace(id)
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.customProfiles[id]
	if !ok {
		return store.CustomProfile{}, store.ErrNotFound
	}
	row := cloneCustomProfile(p, false)
	row.AssignmentCount = len(s.profileAssign[id])
	return row, nil
}

func (s *Store) GetCustomProfilePayload(_ context.Context, id string) ([]byte, error) {
	id = strings.TrimSpace(id)
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.customProfiles[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	return append([]byte{}, p.Payload...), nil
}

func (s *Store) CreateCustomProfile(_ context.Context, p store.CustomProfile) (store.CustomProfile, error) {
	parsed, err := profiles.ParseMobileconfig(p.Payload)
	if err != nil {
		return store.CustomProfile{}, err
	}
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" {
		p.Name = parsed.DisplayName
	}
	if p.Name == "" {
		p.Name = parsed.Identifier
	}
	if p.Name == "" {
		return store.CustomProfile{}, fmt.Errorf("name is required")
	}
	p.Description = strings.TrimSpace(p.Description)
	p.Filename = strings.TrimSpace(p.Filename)
	p.PayloadIdentifier = parsed.Identifier
	p.PayloadUUID = parsed.UUID
	p.PayloadDisplayName = parsed.DisplayName
	p.PayloadType = parsed.PayloadType
	if p.PayloadType == "" {
		p.PayloadType = "Configuration"
	}
	now := time.Now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	if owner := s.identifierOwner(p.PayloadIdentifier, ""); owner != "" {
		return store.CustomProfile{}, fmt.Errorf("a profile with identifier %s already exists", p.PayloadIdentifier)
	}
	if p.ID == "" {
		p.ID = uuid.NewString()
	}
	p.CreatedAt = now
	p.UpdatedAt = now
	p.SizeBytes = len(p.Payload)
	s.customProfiles[p.ID] = cloneCustomProfile(p, true)
	out := cloneCustomProfile(p, false)
	out.AssignmentCount = 0
	return out, nil
}

func (s *Store) UpdateCustomProfile(_ context.Context, p store.CustomProfile) (store.CustomProfile, error) {
	p.ID = strings.TrimSpace(p.ID)
	p.Name = strings.TrimSpace(p.Name)
	if p.ID == "" {
		return store.CustomProfile{}, fmt.Errorf("id is required")
	}
	if p.Name == "" {
		return store.CustomProfile{}, fmt.Errorf("name is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, ok := s.customProfiles[p.ID]
	if !ok {
		return store.CustomProfile{}, store.ErrNotFound
	}
	cur.Name = p.Name
	cur.Description = strings.TrimSpace(p.Description)
	cur.UpdatedAt = time.Now().UTC()
	s.customProfiles[p.ID] = cur
	row := cloneCustomProfile(cur, false)
	row.AssignmentCount = len(s.profileAssign[p.ID])
	return row, nil
}

func (s *Store) ReplaceCustomProfilePayload(_ context.Context, id string, payload []byte, filename string) (store.CustomProfile, error) {
	id = strings.TrimSpace(id)
	parsed, err := profiles.ParseMobileconfig(payload)
	if err != nil {
		return store.CustomProfile{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, ok := s.customProfiles[id]
	if !ok {
		return store.CustomProfile{}, store.ErrNotFound
	}
	if owner := s.identifierOwner(parsed.Identifier, id); owner != "" {
		return store.CustomProfile{}, fmt.Errorf("a profile with identifier %s already exists", parsed.Identifier)
	}
	cur.Payload = append([]byte{}, payload...)
	cur.Filename = strings.TrimSpace(filename)
	cur.PayloadIdentifier = parsed.Identifier
	cur.PayloadUUID = parsed.UUID
	cur.PayloadDisplayName = parsed.DisplayName
	cur.PayloadType = parsed.PayloadType
	if cur.PayloadType == "" {
		cur.PayloadType = "Configuration"
	}
	if strings.TrimSpace(cur.Name) == "" {
		cur.Name = parsed.DisplayName
	}
	cur.SizeBytes = len(cur.Payload)
	cur.UpdatedAt = time.Now().UTC()
	s.customProfiles[id] = cur
	row := cloneCustomProfile(cur, false)
	row.AssignmentCount = len(s.profileAssign[id])
	return row, nil
}

func (s *Store) DeleteCustomProfile(_ context.Context, id string) error {
	id = strings.TrimSpace(id)
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.customProfiles[id]; !ok {
		return store.ErrNotFound
	}
	delete(s.customProfiles, id)
	delete(s.profileAssign, id)
	return nil
}

func (s *Store) ListCustomProfileAssignments(_ context.Context, profileID string) ([]store.CustomProfileAssignment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	as := s.profileAssign[strings.TrimSpace(profileID)]
	out := make([]store.CustomProfileAssignment, len(as))
	copy(out, as)
	return out, nil
}

func (s *Store) SetCustomProfileAssignment(_ context.Context, a store.CustomProfileAssignment) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.customProfiles[a.ProfileID]; !ok {
		return store.ErrNotFound
	}
	t := policy.Target{Type: a.TargetType, ID: a.TargetID}
	normalizeTarget(&t)
	a.TargetType, a.TargetID = t.Type, t.ID
	for _, existing := range s.profileAssign[a.ProfileID] {
		if existing.TargetType == a.TargetType && existing.TargetID == a.TargetID {
			return nil
		}
	}
	s.profileAssign[a.ProfileID] = append(s.profileAssign[a.ProfileID], a)
	return nil
}

func (s *Store) RemoveCustomProfileAssignment(_ context.Context, profileID string, target policy.Target) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	normalizeTarget(&target)
	as := s.profileAssign[profileID]
	next := as[:0]
	for _, a := range as {
		if a.TargetType == target.Type && a.TargetID == target.ID {
			continue
		}
		next = append(next, a)
	}
	s.profileAssign[profileID] = next
	return nil
}

func (s *Store) ListCustomProfilesForDevice(_ context.Context, enrollmentID string, groupIDs []string) ([]store.CustomProfile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	groupSet := map[string]struct{}{}
	for _, g := range groupIDs {
		groupSet[g] = struct{}{}
	}
	var out []store.CustomProfile
	for id, p := range s.customProfiles {
		applies := false
		for _, a := range s.profileAssign[id] {
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
		row := cloneCustomProfile(p, true)
		row.AssignmentCount = len(s.profileAssign[id])
		out = append(out, row)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}
