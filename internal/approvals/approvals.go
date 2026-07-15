package approvals

import (
	"context"
	"fmt"
	"time"

	"github.com/dwdmsh/school-mdm/internal/mdm"
	"github.com/dwdmsh/school-mdm/internal/policy"
	"github.com/dwdmsh/school-mdm/internal/profiles"
	"github.com/dwdmsh/school-mdm/internal/store"
)

// Service handles access requests and allowlist decisions.
type Service struct {
	Store    store.Store
	Enqueue  mdm.CommandEnqueuer
	PortalURL string
}

// CreateRequestInput is student-submitted.
type CreateRequestInput struct {
	Kind         policy.Kind
	Value        string
	EnrollmentID string
	Reason       string
}

// DecideInput is admin Approve/Deny.
type DecideInput struct {
	RequestID string
	Approve   bool
	Duration  string // "15m", "1h", "24h", "permanent", or empty => permanent
}

// CreateRequest stores a pending request.
func (s *Service) CreateRequest(ctx context.Context, in CreateRequestInput) (store.AccessRequest, error) {
	if in.Kind != policy.KindApp && in.Kind != policy.KindURL {
		return store.AccessRequest{}, fmt.Errorf("kind must be app or url")
	}
	if policy.Normalize(in.Kind, in.Value) == "" {
		return store.AccessRequest{}, fmt.Errorf("value is required")
	}
	return s.Store.CreateRequest(ctx, store.AccessRequest{
		Kind:         in.Kind,
		Value:        in.Value,
		EnrollmentID: in.EnrollmentID,
		Reason:       in.Reason,
		Status:       store.StatusPending,
	})
}

// Decide approves or denies a pending request and, on approve, updates policy + enqueues a profile.
func (s *Service) Decide(ctx context.Context, in DecideInput) (store.AccessRequest, error) {
	req, err := s.Store.GetRequest(ctx, in.RequestID)
	if err != nil {
		return store.AccessRequest{}, err
	}
	if req.Status != store.StatusPending {
		return store.AccessRequest{}, fmt.Errorf("request %s is already %s", req.ID, req.Status)
	}

	now := time.Now().UTC()
	req.DecidedAt = &now
	req.Duration = in.Duration

	if !in.Approve {
		req.Status = store.StatusDenied
		if err := s.Store.UpdateRequest(ctx, req); err != nil {
			return store.AccessRequest{}, err
		}
		return req, nil
	}

	req.Status = store.StatusApproved
	expires, permanent, err := parseDuration(in.Duration, now)
	if err != nil {
		return store.AccessRequest{}, err
	}

	if permanent {
		if err := s.Store.UpsertAllowlist(ctx, policy.Entry{
			Kind:  req.Kind,
			Value: req.Value,
			Scope: "global",
		}); err != nil {
			return store.AccessRequest{}, err
		}
	} else {
		if err := s.Store.AddGrant(ctx, policy.Grant{
			Kind:         req.Kind,
			Value:        req.Value,
			EnrollmentID: req.EnrollmentID,
			ExpiresAt:    expires,
		}); err != nil {
			return store.AccessRequest{}, err
		}
	}

	if err := s.Store.UpdateRequest(ctx, req); err != nil {
		return store.AccessRequest{}, err
	}

	if err := s.pushAllowlistProfile(ctx, req.EnrollmentID); err != nil {
		return store.AccessRequest{}, fmt.Errorf("approved but enqueue failed: %w", err)
	}
	return req, nil
}

// EffectiveAllowlist returns merged apps and URLs.
func (s *Service) EffectiveAllowlist(ctx context.Context, enrollmentID string) (apps, urls []string, err error) {
	base, err := s.Store.ListAllowlist(ctx)
	if err != nil {
		return nil, nil, err
	}
	grants, err := s.Store.ListGrants(ctx)
	if err != nil {
		return nil, nil, err
	}
	apps, urls = policy.Effective(base, grants, enrollmentID, time.Now().UTC())
	return apps, urls, nil
}

func (s *Service) pushAllowlistProfile(ctx context.Context, enrollmentID string) error {
	apps, urls, err := s.EffectiveAllowlist(ctx, enrollmentID)
	if err != nil {
		return err
	}
	profile, err := profiles.BuildAllowlistProfile("School Allowlists", apps, urls)
	if err != nil {
		return err
	}
	if s.Enqueue == nil {
		return fmt.Errorf("command enqueuer is not configured")
	}
	if enrollmentID == "" {
		enrollmentID = "unassigned"
	}
	return s.Enqueue.InstallProfile(ctx, enrollmentID, profile)
}

// EnsureWebClip enqueues the Request Access Web Clip profile (stub-friendly).
func (s *Service) EnsureWebClip(ctx context.Context, enrollmentID string) error {
	raw, err := profiles.BuildRequestWebClipProfile(s.PortalURL)
	if err != nil {
		return err
	}
	if enrollmentID == "" {
		enrollmentID = "unassigned"
	}
	return s.Enqueue.InstallProfile(ctx, enrollmentID, raw)
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
			// end of UTC day
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
