// Package mdmstore is the school-facing view of MDM protocol storage (schema mdm).
// Protocol engines (nanomdm / SCEP) use database/sql with search_path=mdm;
// this package provides list/import helpers without growing school store.Store.
package mdmstore

import (
	"context"
	"time"
)

// Enrollment is a managed device enrollment row.
type Enrollment struct {
	ID               string    `json:"id"`
	DeviceID         string    `json:"device_id"`
	SerialNumber     string    `json:"serial_number,omitempty"`
	Type             string    `json:"type"`
	Topic            string    `json:"topic"`
	PushMagic        string    `json:"push_magic,omitempty"`
	Enabled          bool      `json:"enabled"`
	TokenUpdateTally int       `json:"token_update_tally"`
	LastSeenAt       time.Time `json:"last_seen_at"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// PushCertInfo is metadata about an uploaded APNs push certificate.
type PushCertInfo struct {
	Topic    string     `json:"topic"`
	NotAfter *time.Time `json:"not_after,omitempty"`
}

// CommandResult is a device response to an MDM command (when available).
type CommandResult struct {
	EnrollmentID   string         `json:"enrollment_id"`
	CommandUUID    string         `json:"command_uuid"`
	RequestType    string         `json:"request_type"`
	Status         string         `json:"status"`
	Result         string         `json:"result,omitempty"`
	QueryResponses map[string]any `json:"query_responses,omitempty"`
	// Parsed is the command result plist decoded to JSON-friendly maps/slices
	// so the admin UI can render lists (apps, profiles, etc.) instead of raw XML.
	Parsed    map[string]any `json:"parsed,omitempty"`
	UpdatedAt time.Time      `json:"updated_at"`
	Pending   bool           `json:"pending"`
}

// Store reads/writes MDM continuity data in schema mdm.
type Store interface {
	ListEnrollments(ctx context.Context) ([]Enrollment, error)
	GetEnrollment(ctx context.Context, id string) (*Enrollment, error)
	DeleteEnrollment(ctx context.Context, id string) error
	GetPushCertInfo(ctx context.Context, topic string) (*PushCertInfo, error)
	UpsertPushCert(ctx context.Context, topic string, certPEM, keyPEM []byte) error
	GetCommandResult(ctx context.Context, enrollmentID, commandUUID string) (*CommandResult, error)

	// Import helpers (same writers used by cmd/mdmimport).
	ImportDevice(ctx context.Context, d ImportDevice) error
	ImportEnrollment(ctx context.Context, e ImportEnrollment) error
	ImportCertAuth(ctx context.Context, enrollmentID, sha256Hex string) error
	ImportSCEPCA(ctx context.Context, certPEM, encryptedKeyPEM []byte) error
	ImportSCEPCert(ctx context.Context, c ImportSCEPCert) error
}

// ImportDevice is a raw MDM devices row for transform import.
type ImportDevice struct {
	ID                string
	IdentityCert      *string
	SerialNumber      *string
	UnlockToken       []byte
	UnlockTokenAt     *time.Time
	Authenticate      string
	AuthenticateAt    time.Time
	TokenUpdate       *string
	TokenUpdateAt     *time.Time
	BootstrapTokenB64 *string
	BootstrapTokenAt  *time.Time
}

// ImportEnrollment is a raw enrollments row for transform import.
type ImportEnrollment struct {
	ID               string
	DeviceID         string
	UserID           *string
	Type             string
	Topic            string
	PushMagic        string
	TokenHex         string
	Enabled          bool
	TokenUpdateTally int
	LastSeenAt       time.Time
}

// ImportSCEPCert is a non-CA SCEP certificate row.
type ImportSCEPCert struct {
	Serial         int64
	Name           string
	NotValidBefore time.Time
	NotValidAfter  time.Time
	CertificatePEM []byte
}
