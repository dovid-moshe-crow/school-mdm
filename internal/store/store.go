package store

import (
	"context"
	"time"

	"github.com/dwdmsh/school-mdm/internal/policy"
)

// RequestStatus is the lifecycle of an access request.
type RequestStatus string

const (
	StatusPending  RequestStatus = "pending"
	StatusApproved RequestStatus = "approved"
	StatusDenied   RequestStatus = "denied"
)

// AccessRequest is a student ask for an app or URL.
type AccessRequest struct {
	ID           string
	Kind         policy.Kind
	Value        string
	EnrollmentID string
	Reason       string
	Status       RequestStatus
	Duration     string // e.g. "1h", "permanent"
	CreatedAt    time.Time
	DecidedAt    *time.Time
}

// Store persists allowlists, grants, and access requests.
type Store interface {
	Ping(ctx context.Context) error
	Kind() string

	ListAllowlist(ctx context.Context) ([]policy.Entry, error)
	UpsertAllowlist(ctx context.Context, entry policy.Entry) error

	ListGrants(ctx context.Context) ([]policy.Grant, error)
	AddGrant(ctx context.Context, grant policy.Grant) error

	CreateRequest(ctx context.Context, req AccessRequest) (AccessRequest, error)
	GetRequest(ctx context.Context, id string) (AccessRequest, error)
	ListRequests(ctx context.Context, status *RequestStatus) ([]AccessRequest, error)
	UpdateRequest(ctx context.Context, req AccessRequest) error
}
