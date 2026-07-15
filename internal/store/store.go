package store

import (
	"context"
	"time"

	"github.com/dwdmsh/school-mdm/internal/policy"
)

// RequestType classifies what the student/admin ticket is about.
type RequestType string

const (
	TypeAccess  RequestType = "access"  // app or URL allowlist ask
	TypeGeneral RequestType = "general" // general help / ask
	TypeBug     RequestType = "bug"     // bug report
)

// RequestStatus is the lifecycle of a request.
type RequestStatus string

const (
	StatusPending  RequestStatus = "pending"
	StatusApproved RequestStatus = "approved" // access granted / general accepted
	StatusDenied   RequestStatus = "denied"
	StatusResolved RequestStatus = "resolved" // bug closed / fixed
)

// Request is a student ticket (access, general, or bug).
type Request struct {
	ID           string        `json:"id"`
	Type         RequestType   `json:"type"`
	TargetKind   policy.Kind   `json:"kind"` // app/url for access; empty otherwise
	Value        string        `json:"value"`
	EnrollmentID string        `json:"enrollment_id"`
	Reason       string        `json:"reason"`
	Status       RequestStatus `json:"status"`
	Duration     string        `json:"duration"`
	CreatedAt    time.Time     `json:"created_at"`
	DecidedAt    *time.Time    `json:"decided_at,omitempty"`
}

// AppMeta is cached App Store metadata for a bundle ID.
type AppMeta struct {
	BundleID   string    `json:"bundle_id"`
	TrackID    int64     `json:"track_id"`
	Name       string    `json:"app_name"` // App Store title (not the developer)
	Artist     string    `json:"developer"`
	ArtworkURL string    `json:"artwork_url"`
	StoreURL   string    `json:"store_url"`
	UpdatedAt  time.Time `json:"updated_at"`
	Source     string    `json:"source,omitempty"` // "cache" | "itunes"
}

// Group is a named set of device enrollments for scoped allowlists.
type Group struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

// AccessRequest is kept as an alias for older call sites.
type AccessRequest = Request

// Store persists allowlists, grants, requests, groups, and app metadata.
type Store interface {
	Ping(ctx context.Context) error
	Kind() string

	ListAllowlist(ctx context.Context) ([]policy.Entry, error)
	UpsertAllowlist(ctx context.Context, entry policy.Entry) error

	ListGrants(ctx context.Context) ([]policy.Grant, error)
	AddGrant(ctx context.Context, grant policy.Grant) error

	CreateRequest(ctx context.Context, req Request) (Request, error)
	GetRequest(ctx context.Context, id string) (Request, error)
	ListRequests(ctx context.Context, status *RequestStatus) ([]Request, error)
	UpdateRequest(ctx context.Context, req Request) error

	GetAppMeta(ctx context.Context, bundleID string) (AppMeta, error)
	UpsertAppMeta(ctx context.Context, meta AppMeta) error
	SearchAppMeta(ctx context.Context, query string, limit int) ([]AppMeta, error)

	ListGroups(ctx context.Context) ([]Group, error)
	GetGroup(ctx context.Context, id string) (Group, error)
	CreateGroup(ctx context.Context, g Group) (Group, error)
	UpdateGroup(ctx context.Context, g Group) error
	DeleteGroup(ctx context.Context, id string) error

	ListGroupMembers(ctx context.Context, groupID string) ([]string, error)
	SetGroupMembers(ctx context.Context, groupID string, enrollmentIDs []string) error
	AddGroupMember(ctx context.Context, groupID, enrollmentID string) error
	RemoveGroupMember(ctx context.Context, groupID, enrollmentID string) error
	ListGroupsForDevice(ctx context.Context, enrollmentID string) ([]string, error)
	ListEnrollmentIDsForGroup(ctx context.Context, groupID string) ([]string, error)
	ListAllEnrollmentIDs(ctx context.Context) ([]string, error)
}
