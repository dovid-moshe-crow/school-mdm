package mdm

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// CommandEnqueuer pushes MDM commands to devices (or records them in stub mode).
type CommandEnqueuer interface {
	InstallProfile(ctx context.Context, enrollmentID string, profile []byte) error
	RemoveProfile(ctx context.Context, enrollmentID, identifier string) error
	InstallApplication(ctx context.Context, enrollmentID string, iTunesStoreID int64, configurations map[string]string) error
	ManageExistingApplication(ctx context.Context, enrollmentID, bundleID string, iTunesStoreID int64, configurations map[string]string, useStoreID bool) error
	SetApplicationConfiguration(ctx context.Context, enrollmentID, bundleID string, configuration map[string]string) error
}

// QueuedCommand is a recorded stub action.
type QueuedCommand struct {
	EnrollmentID string
	Profile      []byte
	Identifier   string // set for RemoveProfile
	At           time.Time
}

// StubEnqueuer records InstallProfile calls instead of contacting devices.
type StubEnqueuer struct {
	mu       sync.Mutex
	Commands []QueuedCommand
}

// InstallProfile implements CommandEnqueuer.
func (s *StubEnqueuer) InstallProfile(_ context.Context, enrollmentID string, profile []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := make([]byte, len(profile))
	copy(cp, profile)
	s.Commands = append(s.Commands, QueuedCommand{
		EnrollmentID: enrollmentID,
		Profile:      cp,
		At:           time.Now().UTC(),
	})
	return nil
}

// RemoveProfile implements CommandEnqueuer.
func (s *StubEnqueuer) RemoveProfile(_ context.Context, enrollmentID, identifier string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Commands = append(s.Commands, QueuedCommand{
		EnrollmentID: enrollmentID,
		Identifier:   identifier,
		At:           time.Now().UTC(),
	})
	return nil
}

// InstallApplication implements CommandEnqueuer (recorded in stub mode).
func (s *StubEnqueuer) InstallApplication(_ context.Context, enrollmentID string, iTunesStoreID int64, _ map[string]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Commands = append(s.Commands, QueuedCommand{
		EnrollmentID: enrollmentID,
		Identifier:   fmt.Sprintf("InstallApplication:%d", iTunesStoreID),
		At:           time.Now().UTC(),
	})
	return nil
}

// ManageExistingApplication implements CommandEnqueuer (recorded in stub mode).
func (s *StubEnqueuer) ManageExistingApplication(_ context.Context, enrollmentID, bundleID string, iTunesStoreID int64, _ map[string]string, useStoreID bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := bundleID
	if useStoreID || id == "" {
		id = fmt.Sprintf("%d", iTunesStoreID)
	}
	s.Commands = append(s.Commands, QueuedCommand{
		EnrollmentID: enrollmentID,
		Identifier:   "ManageExistingApplication:" + id,
		At:           time.Now().UTC(),
	})
	return nil
}

// SetApplicationConfiguration implements CommandEnqueuer (recorded in stub mode).
func (s *StubEnqueuer) SetApplicationConfiguration(_ context.Context, enrollmentID, bundleID string, _ map[string]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Commands = append(s.Commands, QueuedCommand{
		EnrollmentID: enrollmentID,
		Identifier:   "ApplicationConfiguration:" + bundleID,
		At:           time.Now().UTC(),
	})
	return nil
}

// Snapshot returns a copy of recorded commands.
func (s *StubEnqueuer) Snapshot() []QueuedCommand {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]QueuedCommand, len(s.Commands))
	copy(out, s.Commands)
	return out
}
