package mdm

import (
	"context"
	"sync"
	"time"
)

// CommandEnqueuer pushes MDM commands to devices (or records them in stub mode).
type CommandEnqueuer interface {
	InstallProfile(ctx context.Context, enrollmentID string, profile []byte) error
}

// QueuedCommand is a recorded stub action.
type QueuedCommand struct {
	EnrollmentID string
	Profile      []byte
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

// Snapshot returns a copy of recorded commands.
func (s *StubEnqueuer) Snapshot() []QueuedCommand {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]QueuedCommand, len(s.Commands))
	copy(out, s.Commands)
	return out
}

// NanoEnqueuer is a placeholder for a future NanoHUB-backed adapter.
// It intentionally does not contact APNs or devices in this phase.
type NanoEnqueuer struct {
	// HubURL would be the NanoHUB API base when wired.
	HubURL string
	// APIKey would authenticate enqueue calls when wired.
	APIKey string
}

// InstallProfile returns an error explaining that live enqueue is deferred.
func (n *NanoEnqueuer) InstallProfile(context.Context, string, []byte) error {
	return ErrNanoNotWired
}

// ErrNanoNotWired is returned by NanoEnqueuer until device enrollment is enabled.
var ErrNanoNotWired = errNano("nanohub enqueue is not wired yet; use StubEnqueuer")

type errNano string

func (e errNano) Error() string { return string(e) }
