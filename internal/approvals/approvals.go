package approvals

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dwdmsh/school-mdm/internal/devicepush"
	"github.com/dwdmsh/school-mdm/internal/mdm"
	"github.com/dwdmsh/school-mdm/internal/notify"
	"github.com/dwdmsh/school-mdm/internal/policy"
	"github.com/dwdmsh/school-mdm/internal/store"
)

// Service handles student requests and allowlist decisions.
type Service struct {
	Store     store.Store
	Enqueue   mdm.CommandEnqueuer
	Push      *devicepush.Service // preferred reconcile path; falls back to Enqueue
	Notify    *notify.Service    // optional Expo push for students
	PortalURL string
	Credits   CreditSpender // optional; when set, access requests cost credits
	// AsyncPush reconciles devices after approve without blocking the HTTP caller.
	AsyncPush bool
}

// CreditSpender is satisfied by *credits.Service (kept narrow to avoid import cycles).
type CreditSpender interface {
	SpendForAccessRequest(ctx context.Context, enrollmentID, requestID string) error
	RefundForDeniedRequest(ctx context.Context, enrollmentID, requestID string) error
}

// CreateRequestInput is student-submitted.
type CreateRequestInput struct {
	Type         store.RequestType // access | general | bug (optional if Kind is app/url)
	Kind         policy.Kind       // app | url for access
	Value        string
	EnrollmentID string
	Reason       string
}

// DecideInput is admin Approve/Deny/Resolve.
type DecideInput struct {
	RequestID string
	Approve   bool
	Duration  string // access only: "15m", "1h", "24h", "permanent"
	Scope     string // access only: device | group | global (default device)
	GroupID   string // required when Scope=group
}

// CreateRequest stores a pending request and seeds the conversation with the reason.
// Access requests spend credits after create; on spend failure the request is deleted.
func (s *Service) CreateRequest(ctx context.Context, in CreateRequestInput) (store.Request, error) {
	typ, target, value, err := normalizeCreate(in)
	if err != nil {
		return store.Request{}, err
	}
	reason := strings.TrimSpace(in.Reason)
	req, err := s.Store.CreateRequest(ctx, store.Request{
		Type:         typ,
		TargetKind:   target,
		Value:        value,
		EnrollmentID: strings.TrimSpace(in.EnrollmentID),
		Reason:       reason,
		Status:       store.StatusPending,
	})
	if err != nil {
		return store.Request{}, err
	}

	if typ == store.TypeAccess && s.Credits != nil {
		if err := s.Credits.SpendForAccessRequest(ctx, req.EnrollmentID, req.ID); err != nil {
			_ = s.Store.DeleteRequest(ctx, req.ID)
			return store.Request{}, err
		}
	}

	// Seed the thread only with the student's written reason — never the
	// target value (bundle id / URL), which is already shown on the ticket.
	if reason != "" {
		if _, err := s.Store.AddRequestMessage(ctx, store.RequestMessage{
			RequestID:  req.ID,
			AuthorRole: store.AuthorStudent,
			Body:       reason,
		}); err != nil {
			return store.Request{}, err
		}
	}
	return req, nil
}

func normalizeCreate(in CreateRequestInput) (store.RequestType, policy.Kind, string, error) {
	typ := store.RequestType(strings.TrimSpace(string(in.Type)))
	kind := policy.Kind(strings.TrimSpace(string(in.Kind)))

	// Back-compat: kind=app|url implies access.
	if typ == "" && (kind == policy.KindApp || kind == policy.KindURL) {
		typ = store.TypeAccess
	}
	if typ == "" {
		return "", "", "", fmt.Errorf("type is required (access, general, or bug)")
	}

	value := strings.TrimSpace(in.Value)
	if value == "" {
		return "", "", "", fmt.Errorf("value/subject is required")
	}

	switch typ {
	case store.TypeAccess:
		if kind != policy.KindApp && kind != policy.KindURL {
			return "", "", "", fmt.Errorf("access requests need kind app or url")
		}
		value = policy.Normalize(kind, value)
		if value == "" {
			return "", "", "", fmt.Errorf("value is required")
		}
		return typ, kind, value, nil
	case store.TypeGeneral, store.TypeBug:
		return typ, "", value, nil
	default:
		return "", "", "", fmt.Errorf("type must be access, general, or bug")
	}
}

// Decide approves/resolves or denies a pending request.
// Access approvals update allowlists and enqueue a profile; general/bug do not.
func (s *Service) Decide(ctx context.Context, in DecideInput) (store.Request, error) {
	req, err := s.Store.GetRequest(ctx, in.RequestID)
	if err != nil {
		return store.Request{}, err
	}
	if req.Status != store.StatusPending {
		return store.Request{}, fmt.Errorf("request %s is already %s", req.ID, req.Status)
	}

	now := time.Now().UTC()
	req.DecidedAt = &now
	req.Duration = in.Duration

	if !in.Approve {
		req.Status = store.StatusDenied
		if err := s.Store.UpdateRequest(ctx, req); err != nil {
			return store.Request{}, err
		}
		if req.Type == store.TypeAccess && s.Credits != nil {
			if err := s.Credits.RefundForDeniedRequest(ctx, req.EnrollmentID, req.ID); err != nil {
				return store.Request{}, fmt.Errorf("denied but refund failed: %w", err)
			}
		}
		s.notifyDecided(ctx, req)
		return req, nil
	}

	switch req.Type {
	case store.TypeAccess:
		req.Status = store.StatusApproved
		expires, permanent, err := parseDuration(in.Duration, now)
		if err != nil {
			return store.Request{}, err
		}
		target, err := resolveApproveTarget(in, req)
		if err != nil {
			return store.Request{}, err
		}
		if permanent {
			if err := s.Store.UpsertAllowlist(ctx, policy.Entry{
				Kind:   req.TargetKind,
				Value:  req.Value,
				Target: target,
			}); err != nil {
				return store.Request{}, err
			}
		} else {
			if err := s.Store.AddGrant(ctx, policy.Grant{
				Kind:      req.TargetKind,
				Value:     req.Value,
				Target:    target,
				ExpiresAt: expires,
			}); err != nil {
				return store.Request{}, err
			}
		}
		if err := s.Store.UpdateRequest(ctx, req); err != nil {
			return store.Request{}, err
		}
		devices, err := s.devicesAffectedBy(ctx, target, req.EnrollmentID)
		if err != nil {
			return store.Request{}, err
		}
		if err := s.pushDevices(ctx, devices); err != nil {
			return store.Request{}, err
		}
		s.notifyDecided(ctx, req)
		return req, nil

	case store.TypeBug, store.TypeGeneral:
		// General and bug tickets are "handled", not allowlist-approved.
		req.Status = store.StatusResolved
		if err := s.Store.UpdateRequest(ctx, req); err != nil {
			return store.Request{}, err
		}
		s.notifyDecided(ctx, req)
		return req, nil

	default:
		return store.Request{}, fmt.Errorf("unknown request type %q", req.Type)
	}
}

// PostMessageInput is a reply on a request thread.
type PostMessageInput struct {
	RequestID    string
	AuthorRole   store.MessageAuthor
	Body         string
	EnrollmentID string // required when AuthorRole=student; must match request
}

// PostMessage appends a message. Student replies reopen closed tickets to pending.
func (s *Service) PostMessage(ctx context.Context, in PostMessageInput) (store.RequestMessage, error) {
	body := strings.TrimSpace(in.Body)
	if body == "" {
		return store.RequestMessage{}, fmt.Errorf("message body is required")
	}
	if in.AuthorRole != store.AuthorStudent && in.AuthorRole != store.AuthorAdmin {
		return store.RequestMessage{}, fmt.Errorf("author_role must be student or admin")
	}

	req, err := s.Store.GetRequest(ctx, in.RequestID)
	if err != nil {
		return store.RequestMessage{}, err
	}

	if in.AuthorRole == store.AuthorStudent {
		want := strings.TrimSpace(req.EnrollmentID)
		got := strings.TrimSpace(in.EnrollmentID)
		if want == "" || got == "" || want != got {
			return store.RequestMessage{}, fmt.Errorf("device does not own this request")
		}
	}

	msg, err := s.Store.AddRequestMessage(ctx, store.RequestMessage{
		RequestID:  req.ID,
		AuthorRole: in.AuthorRole,
		Body:       body,
	})
	if err != nil {
		return store.RequestMessage{}, err
	}

	// Student reply reopens a closed conversation so admin sees it again.
	if in.AuthorRole == store.AuthorStudent && req.Status != store.StatusPending {
		req.Status = store.StatusPending
		req.DecidedAt = nil
		req.Duration = ""
		if err := s.Store.UpdateRequest(ctx, req); err != nil {
			return store.RequestMessage{}, err
		}
	}
	if in.AuthorRole == store.AuthorAdmin && s.Notify != nil {
		s.Notify.AdminMessage(ctx, req.EnrollmentID, body)
	}
	return msg, nil
}

func (s *Service) notifyDecided(ctx context.Context, req store.Request) {
	if s.Notify == nil {
		return
	}
	s.Notify.RequestDecided(ctx, req.EnrollmentID, string(req.Status), req.Value)
}

func resolveApproveTarget(in DecideInput, req store.Request) (policy.Target, error) {
	scope := strings.ToLower(strings.TrimSpace(in.Scope))
	if scope == "" {
		scope = "device"
	}
	switch scope {
	case "global":
		return policy.Target{Type: policy.TargetGlobal}, nil
	case "group":
		gid := strings.TrimSpace(in.GroupID)
		if gid == "" {
			return policy.Target{}, fmt.Errorf("group_id is required when scope=group")
		}
		return policy.Target{Type: policy.TargetGroup, ID: gid}, nil
	case "device":
		id := strings.TrimSpace(req.EnrollmentID)
		if id == "" {
			return policy.Target{}, fmt.Errorf("request has no enrollment_id for device scope")
		}
		return policy.Target{Type: policy.TargetDevice, ID: id}, nil
	default:
		return policy.Target{}, fmt.Errorf("scope must be device, group, or global")
	}
}

func (s *Service) devicesAffectedBy(ctx context.Context, target policy.Target, requestDevice string) ([]string, error) {
	switch target.Type {
	case policy.TargetDevice:
		id := target.ID
		if id == "" {
			id = requestDevice
		}
		if id == "" {
			return []string{"unassigned"}, nil
		}
		return []string{id}, nil
	case policy.TargetGroup:
		members, err := s.Store.ListEnrollmentIDsForGroup(ctx, target.ID)
		if err != nil {
			return nil, err
		}
		if requestDevice != "" {
			found := false
			for _, m := range members {
				if m == requestDevice {
					found = true
					break
				}
			}
			if !found {
				members = append(members, requestDevice)
			}
		}
		if len(members) == 0 {
			if requestDevice != "" {
				return []string{requestDevice}, nil
			}
			return []string{"unassigned"}, nil
		}
		return members, nil
	case policy.TargetGlobal:
		ids, err := s.Store.ListAllEnrollmentIDs(ctx)
		if err != nil {
			return nil, err
		}
		set := map[string]struct{}{}
		for _, id := range ids {
			if id != "" {
				set[id] = struct{}{}
			}
		}
		if requestDevice != "" {
			set[requestDevice] = struct{}{}
		}
		out := make([]string, 0, len(set))
		for id := range set {
			out = append(out, id)
		}
		if len(out) == 0 {
			return []string{"unassigned"}, nil
		}
		return out, nil
	default:
		return []string{requestDevice}, nil
	}
}

// EffectiveAllowlist returns merged apps and URLs for a device.
func (s *Service) EffectiveAllowlist(ctx context.Context, enrollmentID string) (apps, urls []string, err error) {
	if s.Push != nil {
		return s.Push.EffectiveAllowlist(ctx, enrollmentID)
	}
	return (&devicepush.Service{Store: s.Store}).EffectiveAllowlist(ctx, enrollmentID)
}

func (s *Service) pushDevices(ctx context.Context, devices []string) error {
	ids := append([]string(nil), devices...)
	run := func(ctx context.Context) error {
		for _, enrollmentID := range ids {
			if err := s.pushAllowlistProfile(ctx, enrollmentID); err != nil {
				return fmt.Errorf("approved but enqueue failed for %s: %w", enrollmentID, err)
			}
		}
		return nil
	}
	if !s.AsyncPush {
		return run(ctx)
	}
	go func() {
		bg, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		_ = run(bg)
	}()
	return nil
}

func (s *Service) pushAllowlistProfile(ctx context.Context, enrollmentID string) error {
	if s.Push != nil {
		return s.Push.Reconcile(ctx, enrollmentID)
	}
	push := &devicepush.Service{Store: s.Store, Enqueue: s.Enqueue, PortalURL: s.PortalURL}
	return push.Reconcile(ctx, enrollmentID)
}

// EnsureWebClip enqueues the Request Access Web Clip for a device-scoped portal URL.
func (s *Service) EnsureWebClip(ctx context.Context, enrollmentID string) error {
	if s.Push != nil {
		return s.Push.EnsureWebClip(ctx, enrollmentID)
	}
	push := &devicepush.Service{Store: s.Store, Enqueue: s.Enqueue, PortalURL: s.PortalURL}
	return push.EnsureWebClip(ctx, enrollmentID)
}

func parseDuration(d string, now time.Time) (expires *time.Time, permanent bool, err error) {
	switch d {
	case "", "permanent":
		return nil, true, nil
	case "15m", "1h", "24h", "today":
		var dur time.Duration
		switch d {
		case "15m":
			dur = 15 * time.Minute
		case "1h":
			dur = time.Hour
		case "24h":
			dur = 24 * time.Hour
		case "today":
			y, m, day := now.Date()
			end := time.Date(y, m, day, 23, 59, 59, 0, time.UTC)
			return &end, false, nil
		}
		t := now.Add(dur)
		return &t, false, nil
	default:
		return nil, false, fmt.Errorf("unsupported duration %q (use 15m, 1h, 24h, today, permanent)", d)
	}
}
