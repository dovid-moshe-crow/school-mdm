// Package devicepush owns the single allowlist → profile → enqueue path (DRY).
package devicepush

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dwdmsh/school-mdm/internal/mdm"
	"github.com/dwdmsh/school-mdm/internal/mdmstore"
	"github.com/dwdmsh/school-mdm/internal/policy"
	"github.com/dwdmsh/school-mdm/internal/profiles"
	"github.com/dwdmsh/school-mdm/internal/store"
	"github.com/dwdmsh/school-mdm/internal/vpp"
)

// Service reconciles effective allowlists onto devices.
type Service struct {
	Store     store.Store
	MDMStore  mdmstore.Store // optional; used for serial → VPP associate
	Enqueue   mdm.CommandEnqueuer
	PortalURL string
	Log       *slog.Logger
}

// Reconcile computes the effective allowlist for enrollmentID and installs the profile.
// Always installs/refreshes the Request Access web clip.
// If the device is unrestricted (Allow all), the school allowlist profile is removed instead.
func (s *Service) Reconcile(ctx context.Context, enrollmentID string) error {
	if s == nil || s.Store == nil {
		return fmt.Errorf("devicepush not configured")
	}
	if s.Enqueue == nil {
		return fmt.Errorf("command enqueuer is not configured")
	}
	id := strings.TrimSpace(enrollmentID)
	if id == "" {
		id = "unassigned"
	}
	if d, err := s.Store.GetDevice(ctx, id); err == nil && d.Unrestricted {
		if err := s.ClearAllowlist(ctx, id); err != nil {
			return err
		}
		if err := s.EnsureWebClip(ctx, id); err != nil {
			return err
		}
		if err := s.EnsureLockScreenMessage(ctx, id); err != nil {
			return err
		}
		return s.EnsureCompanionApp(ctx, id)
	}
	apps, urls, err := s.EffectiveAllowlist(ctx, id)
	if err != nil {
		return err
	}
	portalRoot := profiles.DevicePortalRoot(s.PortalURL, id)
	profile, err := profiles.BuildAllowlistProfile("School Allowlists", apps, urls, portalRoot)
	if err != nil {
		return err
	}
	if err := s.Enqueue.InstallProfile(ctx, id, profile); err != nil {
		return err
	}
	if err := s.EnsureWebClip(ctx, id); err != nil {
		return err
	}
	if err := s.EnsureLockScreenMessage(ctx, id); err != nil {
		return err
	}
	return s.EnsureCompanionApp(ctx, id)
}

// legacyLockScreenProfileIDs are old NanoHUB / prior MDM lock-screen profiles.
// Apple allows only ONE com.apple.shareddeviceconfiguration payload; leftovers block ours.
var legacyLockScreenProfileIDs = []string{
	"com.nanohub.lockscreen",
}

// EnsureLockScreenMessage installs the supervised lock-screen asset tag + footnote.
// Asset tag = device display name (fallback: enrollment id). Footnote from MDM settings.
func (s *Service) EnsureLockScreenMessage(ctx context.Context, enrollmentID string) error {
	if s == nil || s.Store == nil || s.Enqueue == nil {
		return fmt.Errorf("devicepush not configured")
	}
	id := strings.TrimSpace(enrollmentID)
	if id == "" {
		return nil
	}
	// Always clear legacy NanoHUB lock-screen profile so our payload can apply.
	for _, legacyID := range legacyLockScreenProfileIDs {
		if err := s.Enqueue.RemoveProfile(ctx, id, legacyID); err != nil && s.Log != nil {
			s.Log.Warn("remove legacy lock screen profile", "enrollment_id", id, "profile", legacyID, "err", err)
		}
	}
	settings, err := s.Store.GetMDMSettings(ctx)
	if err != nil {
		return nil // optional during early boot
	}
	if !settings.LockScreenEnabled {
		return s.Enqueue.RemoveProfile(ctx, id, profiles.LockScreenPayloadIdentifier)
	}
	assetTag := id
	if d, err := s.Store.GetDevice(ctx, id); err == nil {
		if name := strings.TrimSpace(d.Name); name != "" {
			assetTag = name
		}
	}
	footnote := strings.TrimSpace(settings.LockScreenFootnote)
	if footnote == "" {
		footnote = "מכשיר בית ספר · KFilter"
	}
	profile, err := profiles.BuildLockScreenMessageProfile(assetTag, footnote)
	if err != nil {
		return err
	}
	return s.Enqueue.InstallProfile(ctx, id, profile)
}

// EnsureWebClip installs home-screen shortcuts: help/request portal + app store.
func (s *Service) EnsureWebClip(ctx context.Context, enrollmentID string) error {
	if s == nil || s.Enqueue == nil {
		return fmt.Errorf("devicepush not configured")
	}
	id := strings.TrimSpace(enrollmentID)
	if id == "" {
		id = "unassigned"
	}
	help, err := profiles.BuildRequestWebClipProfile(profiles.DevicePortalURL(s.PortalURL, id))
	if err != nil {
		return err
	}
	if err := s.Enqueue.InstallProfile(ctx, id, help); err != nil {
		return err
	}
	storeProfile, err := profiles.BuildStoreWebClipProfile(profiles.DeviceStoreURL(s.PortalURL, id))
	if err != nil {
		return err
	}
	return s.Enqueue.InstallProfile(ctx, id, storeProfile)
}

// PushCompanionConfig converts a user-installed KFilter to managed (ChangeManagementState),
// then pushes Managed App Config. Works on supervised devices without Adam ID / ASM listing.
func (s *Service) PushCompanionConfig(ctx context.Context, enrollmentID string) error {
	if s == nil || s.Store == nil || s.Enqueue == nil {
		return fmt.Errorf("devicepush not configured")
	}
	id := strings.TrimSpace(enrollmentID)
	if id == "" {
		return fmt.Errorf("enrollment id required")
	}
	settings, err := s.Store.GetMDMSettings(ctx)
	if err != nil {
		return err
	}
	bundle := strings.TrimSpace(settings.CompanionBundleID)
	if bundle == "" {
		bundle = "com.kfilter.portal"
	}
	portalBase := strings.TrimRight(strings.TrimSpace(s.PortalURL), "/")
	cfg := map[string]string{"enrollment_id": id}
	if portalBase != "" {
		cfg["portal_base_url"] = portalBase
	}
	// Take over user install (TestFlight/App Store) via Identifier + ChangeManagementState.
	// Do not send iTunesStoreID here — Apple returns errors for unlisted/Custom apps;
	// bundle Identifier works on supervised devices (State=Managed).
	if err := s.Enqueue.ManageExistingApplication(ctx, id, bundle, 0, cfg, false); err != nil {
		return fmt.Errorf("manage companion: %w", err)
	}
	if err := s.Enqueue.SetApplicationConfiguration(ctx, id, bundle, cfg); err != nil {
		return fmt.Errorf("companion config: %w", err)
	}
	return nil
}

// EnsureCompanionApp installs KFilter via InstallApplication (when Adam ID is set) and
// always pushes Managed App Config. Config-only works before the Custom App is in ASM.
func (s *Service) EnsureCompanionApp(ctx context.Context, enrollmentID string) error {
	if s == nil || s.Store == nil || s.Enqueue == nil {
		return fmt.Errorf("devicepush not configured")
	}
	id := strings.TrimSpace(enrollmentID)
	if id == "" {
		return nil
	}
	settings, err := s.Store.GetMDMSettings(ctx)
	if err != nil {
		return nil // settings optional during early boot
	}
	if !settings.CompanionEnabled {
		return nil
	}
	bundle := strings.TrimSpace(settings.CompanionBundleID)
	if bundle == "" {
		bundle = "com.kfilter.portal"
	}
	portalBase := strings.TrimRight(strings.TrimSpace(s.PortalURL), "/")
	cfg := map[string]string{
		"enrollment_id": id,
	}
	if portalBase != "" {
		cfg["portal_base_url"] = portalBase
	}

	if settings.CompanionITunesID > 0 {
		// Best-effort VPP license associate before InstallApplication.
		if err := s.associateCompanionLicense(ctx, settings, id); err != nil {
			if s.Log != nil {
				s.Log.Warn("vpp associate companion", "enrollment_id", id, "err", err)
			}
		}
		if err := s.Enqueue.InstallApplication(ctx, id, settings.CompanionITunesID, cfg); err != nil {
			return fmt.Errorf("install companion: %w", err)
		}
	}

	if err := s.Enqueue.SetApplicationConfiguration(ctx, id, bundle, cfg); err != nil {
		return fmt.Errorf("companion config: %w", err)
	}
	return nil
}

func (s *Service) associateCompanionLicense(ctx context.Context, settings store.MDMSettings, enrollmentID string) error {
	token := vpp.TokenString(settings.VPPToken)
	if token == "" || s.MDMStore == nil {
		return nil
	}
	enr, err := s.MDMStore.GetEnrollment(ctx, enrollmentID)
	if err != nil || enr == nil {
		return err
	}
	serial := strings.TrimSpace(enr.SerialNumber)
	if serial == "" {
		return fmt.Errorf("device serial unknown")
	}
	client := &vpp.Client{Token: token}
	return client.AssociateLicense(ctx, strconv.FormatInt(settings.CompanionITunesID, 10), serial)
}

// ReconcileMany reconciles each enrollment id; returns the first error.
func (s *Service) ReconcileMany(ctx context.Context, enrollmentIDs []string) error {
	for _, id := range enrollmentIDs {
		if err := s.Reconcile(ctx, id); err != nil {
			return fmt.Errorf("%s: %w", id, err)
		}
	}
	return nil
}

// ReconcileAllDevices pushes allowlist + web clip to every known device.
func (s *Service) ReconcileAllDevices(ctx context.Context) error {
	if s == nil || s.Store == nil {
		return fmt.Errorf("devicepush not configured")
	}
	devices, err := s.Store.ListDevices(ctx)
	if err != nil {
		return err
	}
	ids := make([]string, 0, len(devices))
	for _, d := range devices {
		if id := strings.TrimSpace(d.EnrollmentID); id != "" {
			ids = append(ids, id)
		}
	}
	return s.ReconcileMany(ctx, ids)
}

// ClearAllowlist removes the school allowlist profile from the device.
func (s *Service) ClearAllowlist(ctx context.Context, enrollmentID string) error {
	if s == nil || s.Enqueue == nil {
		return fmt.Errorf("devicepush not configured")
	}
	id := strings.TrimSpace(enrollmentID)
	if id == "" {
		return fmt.Errorf("enrollment id required")
	}
	return s.Enqueue.RemoveProfile(ctx, id, profiles.AllowlistPayloadIdentifier)
}

// EffectiveAllowlist returns merged apps and URLs for a device.
// Includes essentials, one-off allowlist rows, grants, items from assigned whitelist packs,
// and the school request portal (always reachable when restricted).
func (s *Service) EffectiveAllowlist(ctx context.Context, enrollmentID string) (apps, urls []string, err error) {
	var (
		base   []policy.Entry
		grants []policy.Grant
		groups []string
		err1   error
		err2   error
		err3   error
		wg     sync.WaitGroup
	)
	wg.Add(3)
	go func() {
		defer wg.Done()
		base, err1 = s.Store.ListAllowlist(ctx)
	}()
	go func() {
		defer wg.Done()
		grants, err2 = s.Store.ListGrants(ctx)
	}()
	go func() {
		defer wg.Done()
		groups, err3 = s.Store.ListGroupsForDevice(ctx, enrollmentID)
	}()
	wg.Wait()
	if err1 != nil {
		return nil, nil, err1
	}
	if err2 != nil {
		return nil, nil, err2
	}
	if err3 != nil {
		return nil, nil, err3
	}
	fromPacks, err := s.Store.ListAllowlistFromPacks(ctx, enrollmentID, groups)
	if err != nil {
		return nil, nil, err
	}
	if len(fromPacks) > 0 {
		base = append(append([]policy.Entry{}, base...), fromPacks...)
	}
	apps, urls = policy.Effective(base, grants, groups, enrollmentID, time.Now().UTC())
	urls = appendPortalURLs(urls, s.PortalURL, enrollmentID)
	return apps, urls, nil
}

func appendPortalURLs(urls []string, portalBase, enrollmentID string) []string {
	extras := make([]string, 0, 3)
	if p := profiles.DevicePortalURL(portalBase, enrollmentID); p != "" {
		extras = append(extras, p)
	}
	if p := profiles.DeviceStoreURL(portalBase, enrollmentID); p != "" {
		extras = append(extras, p)
	}
	if u, err := url.Parse(strings.TrimSpace(portalBase)); err == nil && u.Host != "" {
		scheme := u.Scheme
		if scheme == "" {
			scheme = "https"
		}
		extras = append(extras, scheme+"://"+u.Host)
	}
	// Website allow-list only (not com.apple.AppStore) so Safari can open Install links.
	extras = append(extras,
		"https://apps.apple.com",
		"https://itunes.apple.com",
		"https://matara.pro",
		"https://www.matara.pro",
		"https://www.gstatic.com",
	)
	seen := map[string]struct{}{}
	for _, u := range urls {
		seen[strings.ToLower(strings.TrimSpace(u))] = struct{}{}
	}
	out := append([]string{}, urls...)
	for _, e := range extras {
		key := strings.ToLower(strings.TrimSpace(e))
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, e)
	}
	return out
}
