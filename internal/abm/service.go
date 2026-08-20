// Package abm wraps Apple Business Manager / DEP API calls (NanoDEP godep client).
package abm

import (
	"context"
	"fmt"
	"strings"

	"github.com/micromdm/nanodep/godep"
	"github.com/micromdm/nanodep/storage"
)

// Store combines NanoDEP storage capabilities needed for ABM operations.
type Store interface {
	godep.ClientStorage
	storage.DEPNamesQuery
}

// NameSource resolves the active DEP slot name (from DB settings).
type NameSource interface {
	DEPName(ctx context.Context) (string, error)
}

// Service wraps Apple Business Manager / DEP API calls.
type Service struct {
	store  Store
	client *godep.Client
	names  NameSource
	// fallback used only if names is nil (tests).
	fallback string
}

// NewService creates an ABM service. names supplies the active DEP name from settings.
func NewService(store Store, names NameSource) *Service {
	return &Service{
		store:    store,
		client:   godep.NewClient(store),
		names:    names,
		fallback: "nanok",
	}
}

func (s *Service) resolveName(ctx context.Context) (string, error) {
	if s.names != nil {
		name, err := s.names.DEPName(ctx)
		if err != nil {
			return "", err
		}
		name = strings.TrimSpace(name)
		if name == "" {
			return "", fmt.Errorf("dep_name empty")
		}
		return name, nil
	}
	if s.fallback != "" {
		return s.fallback, nil
	}
	return "nanok", nil
}

// DEPName returns the configured DEP server name from settings.
func (s *Service) DEPName(ctx context.Context) (string, error) {
	return s.resolveName(ctx)
}

// ListDEPNames returns configured DEP names from NanoDEP storage (rows with data).
func (s *Service) ListDEPNames(ctx context.Context) ([]string, error) {
	res, err := s.store.QueryDEPNames(ctx, &storage.DEPNamesQueryRequest{})
	if err != nil {
		return nil, fmt.Errorf("query dep names: %w", err)
	}
	return res.DEPNames, nil
}

// Account returns ABM/ASM account details from Apple.
func (s *Service) Account(ctx context.Context) (*godep.AccountDetailJson, error) {
	name, err := s.resolveName(ctx)
	if err != nil {
		return nil, err
	}
	detail, err := s.client.AccountDetail(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("abm account: %w", err)
	}
	return detail, nil
}

// SyncDevices fetches the full device list from Apple DEP (paginated).
// Uses the "Get a List of Devices" API so a one-shot admin sync returns all
// assigned devices, not only cursor deltas from Sync.
func (s *Service) SyncDevices(ctx context.Context) (*godep.FetchDeviceResponseJson, error) {
	name, err := s.resolveName(ctx)
	if err != nil {
		return nil, err
	}
	var all []godep.DeviceJson
	cursor := ""
	var last *godep.FetchDeviceResponseJson
	for {
		opts := []godep.DeviceRequestOption{godep.WithLimit(1000)}
		if cursor != "" {
			opts = append(opts, godep.WithCursor(cursor))
		}
		resp, err := s.client.FetchDevices(ctx, name, opts...)
		if err != nil {
			return nil, fmt.Errorf("abm fetch devices: %w", err)
		}
		last = resp
		all = append(all, resp.Devices...)
		if !resp.MoreToFollow {
			break
		}
		cursor = resp.Cursor
	}
	if last == nil {
		return &godep.FetchDeviceResponseJson{Devices: all}, nil
	}
	last.Devices = all
	last.MoreToFollow = false
	return last, nil
}

// SerialsNeedingProfile returns serials that do not yet have this enrollment
// profile assigned or pushed (empty, removed, or a different profile).
func SerialsNeedingProfile(devices []godep.DeviceJson, profileUUID string) []string {
	profileUUID = strings.TrimSpace(profileUUID)
	if profileUUID == "" {
		return nil
	}
	seen := map[string]struct{}{}
	var out []string
	for _, d := range devices {
		serial := strings.TrimSpace(d.SerialNumber)
		if serial == "" {
			continue
		}
		if _, ok := seen[serial]; ok {
			continue
		}
		if hasEnrollmentProfile(d, profileUUID) {
			continue
		}
		seen[serial] = struct{}{}
		out = append(out, serial)
	}
	return out
}

func hasEnrollmentProfile(d godep.DeviceJson, profileUUID string) bool {
	current := ""
	if d.ProfileUuid != nil {
		current = strings.TrimSpace(*d.ProfileUuid)
	}
	if !strings.EqualFold(current, profileUUID) {
		return false
	}
	if d.ProfileStatus == nil {
		return false
	}
	switch *d.ProfileStatus {
	case godep.DeviceJsonProfileStatusAssigned, godep.DeviceJsonProfileStatusPushed:
		return true
	default:
		return false
	}
}

// GetProfile fetches a DEP profile by UUID.
func (s *Service) GetProfile(ctx context.Context, profileUUID string) (*godep.ProfileJson, error) {
	name, err := s.resolveName(ctx)
	if err != nil {
		return nil, err
	}
	profile, err := s.client.GetProfile(ctx, name, profileUUID)
	if err != nil {
		return nil, fmt.Errorf("abm get profile: %w", err)
	}
	return profile, nil
}

// DefineProfile creates or updates a DEP enrollment profile at Apple.
func (s *Service) DefineProfile(ctx context.Context, profile *godep.ProfileJson) (*godep.DefineProfileResponseJson, error) {
	name, err := s.resolveName(ctx)
	if err != nil {
		return nil, err
	}
	resp, err := s.client.DefineProfile(ctx, name, profile)
	if err != nil {
		return nil, fmt.Errorf("abm define profile: %w", err)
	}
	return resp, nil
}

// AssignProfile assigns a DEP profile UUID to device serial numbers.
func (s *Service) AssignProfile(ctx context.Context, profileUUID string, serials []string) (*godep.AssignProfileResponseJson, error) {
	name, err := s.resolveName(ctx)
	if err != nil {
		return nil, err
	}
	resp, err := s.client.AssignProfile(ctx, name, profileUUID, serials...)
	if err != nil {
		return nil, fmt.Errorf("abm assign profile: %w", err)
	}
	return resp, nil
}
