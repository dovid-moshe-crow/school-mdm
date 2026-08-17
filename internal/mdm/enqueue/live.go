package enqueue

import (
	"context"
	"fmt"

	"github.com/dwdmsh/school-mdm/internal/mdm"
	"github.com/dwdmsh/school-mdm/internal/mdm/commands"
	nanoapi "github.com/micromdm/nanomdm/api"
)

// RawPushEnqueuer is satisfied by nanomdm's PushEnqueuer.
type RawPushEnqueuer interface {
	RawCommandEnqueueWithPush(ctx context.Context, rawCommand []byte, ids []string, noPush bool) (*nanoapi.APIResult, int, error)
}

// LiveEnqueuer implements mdm.CommandEnqueuer against NanoMDM storage + APNs.
type LiveEnqueuer struct {
	CE     RawPushEnqueuer
	NoPush bool // enqueue without APNs (tests / drain)
}

// InstallProfile queues an InstallProfile command and pushes the device.
func (e *LiveEnqueuer) InstallProfile(ctx context.Context, enrollmentID string, profile []byte) error {
	if e == nil || e.CE == nil {
		return fmt.Errorf("live enqueuer not configured")
	}
	if enrollmentID == "" {
		return fmt.Errorf("enrollment id required")
	}
	cmd, err := commands.InstallProfile(profile)
	if err != nil {
		return err
	}
	return e.enqueue(ctx, enrollmentID, cmd)
}

// RemoveProfile queues a RemoveProfile command.
func (e *LiveEnqueuer) RemoveProfile(ctx context.Context, enrollmentID, identifier string) error {
	cmd, err := commands.RemoveProfile(identifier)
	if err != nil {
		return err
	}
	return e.enqueue(ctx, enrollmentID, cmd)
}

// InstallApplication queues InstallApplication (VPP purchase method).
func (e *LiveEnqueuer) InstallApplication(ctx context.Context, enrollmentID string, iTunesStoreID int64, configurations map[string]string) error {
	cmd, err := commands.InstallApplication(iTunesStoreID, configurations, 1)
	if err != nil {
		return err
	}
	return e.enqueue(ctx, enrollmentID, cmd)
}

// ManageExistingApplication converts a user-installed app to managed + optional config.
func (e *LiveEnqueuer) ManageExistingApplication(ctx context.Context, enrollmentID, bundleID string, iTunesStoreID int64, configurations map[string]string, useStoreID bool) error {
	cmd, err := commands.ManageExistingApplication(bundleID, iTunesStoreID, configurations, useStoreID)
	if err != nil {
		return err
	}
	return e.enqueue(ctx, enrollmentID, cmd)
}

// SetApplicationConfiguration queues a Settings/ApplicationConfiguration command.
func (e *LiveEnqueuer) SetApplicationConfiguration(ctx context.Context, enrollmentID, bundleID string, configuration map[string]string) error {
	cmd, err := commands.SetApplicationConfiguration(bundleID, configuration)
	if err != nil {
		return err
	}
	return e.enqueue(ctx, enrollmentID, cmd)
}

// EnqueueRaw queues an already-built command plist (or nil for push-only).
func (e *LiveEnqueuer) EnqueueRaw(ctx context.Context, enrollmentID string, cmd []byte) error {
	return e.enqueue(ctx, enrollmentID, cmd)
}

// Push wakes the device without a new command.
func (e *LiveEnqueuer) Push(ctx context.Context, enrollmentID string) error {
	return e.enqueue(ctx, enrollmentID, nil)
}

func (e *LiveEnqueuer) enqueue(ctx context.Context, enrollmentID string, cmd []byte) error {
	r, _, err := e.CE.RawCommandEnqueueWithPush(ctx, cmd, []string{enrollmentID}, e.NoPush)
	if err != nil {
		return fmt.Errorf("enqueue: %w", err)
	}
	if r != nil {
		if err := r.Error(); err != nil {
			return err
		}
	}
	return nil
}

var _ mdm.CommandEnqueuer = (*LiveEnqueuer)(nil)
