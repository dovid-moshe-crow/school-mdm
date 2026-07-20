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
	StatusPending  RequestStatus = "pending"  // waiting for admin / open conversation
	StatusApproved RequestStatus = "approved" // access: allowlist granted
	StatusDenied   RequestStatus = "denied"   // access denied / request rejected
	StatusResolved RequestStatus = "resolved" // general/bug closed / handled
)

// MessageAuthor is who wrote a request message.
type MessageAuthor string

const (
	AuthorStudent MessageAuthor = "student"
	AuthorAdmin   MessageAuthor = "admin"
)

// RequestMessage is one turn in a student↔admin conversation.
type RequestMessage struct {
	ID         string        `json:"id"`
	RequestID  string        `json:"request_id"`
	AuthorRole MessageAuthor `json:"author_role"`
	Body       string        `json:"body"`
	CreatedAt  time.Time     `json:"created_at"`
}

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

	// Optional fields from iTunes Lookup (shown in details).
	Description    string   `json:"description,omitempty"`
	Genre          string   `json:"genre,omitempty"`
	Version        string   `json:"version,omitempty"`
	AverageRating  float64  `json:"average_rating,omitempty"`
	RatingCount    int      `json:"rating_count,omitempty"`
	ContentRating  string   `json:"content_rating,omitempty"`
	ReleaseDate    string   `json:"release_date,omitempty"`
	FormattedPrice string   `json:"formatted_price,omitempty"`
	FileSizeBytes  int64    `json:"file_size_bytes,omitempty"`
	SellerName     string   `json:"seller_name,omitempty"`
	Screenshots    []string `json:"screenshots,omitempty"`
}

// Group is a named set of device enrollments for scoped allowlists.
type Group struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	MemberCount int       `json:"member_count,omitempty"`
}

// Device is an enrollment with an optional display name.
type Device struct {
	EnrollmentID string `json:"enrollment_id"`
	Name         string `json:"name"`
}

// AccessRequest is kept as an alias for older call sites.
type AccessRequest = Request

// LedgerReason classifies a credit balance change.
type LedgerReason string

const (
	LedgerPurchase         LedgerReason = "purchase"
	LedgerSpend            LedgerReason = "spend"
	LedgerRefund           LedgerReason = "refund"
	LedgerGift             LedgerReason = "gift"
	LedgerAdjust           LedgerReason = "adjust"
	LedgerAllotment        LedgerReason = "allotment"
	LedgerAllotmentExpire  LedgerReason = "allotment_expire"
)

// AllotmentInterval is how often a rule refreshes period credits.
type AllotmentInterval string

const (
	IntervalDaily   AllotmentInterval = "daily"
	IntervalWeekly  AllotmentInterval = "weekly"
	IntervalMonthly AllotmentInterval = "monthly"
)

// AllotmentTargetType scopes who receives a rule's allotment.
type AllotmentTargetType string

const (
	AllotmentEveryone    AllotmentTargetType = "everyone"
	AllotmentGroup       AllotmentTargetType = "group"
	AllotmentIndividual  AllotmentTargetType = "individual"
)

// PurchaseStatus is the lifecycle of a credit purchase.
type PurchaseStatus string

const (
	PurchasePending PurchaseStatus = "pending"
	PurchasePaid    PurchaseStatus = "paid"
	PurchaseFailed  PurchaseStatus = "failed"
	PurchaseExpired PurchaseStatus = "expired"
)

// PaymentProvider identifies who collected the payment.
type PaymentProvider string

const (
	ProviderNedarim PaymentProvider = "nedarim"
	ProviderFake    PaymentProvider = "fake"
)

// DeviceCredits is the credit balance for one enrollment.
// Two-bucket model: Balance is permanent (purchase/gift); AllotmentBalance is
// the current period allotment (resets each period, never stacks). Available()
// is what the portal shows and what spend checks against.
type DeviceCredits struct {
	EnrollmentID      string    `json:"enrollment_id"`
	Balance           int       `json:"balance"`
	AllotmentBalance  int       `json:"allotment_balance"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// Available is permanent + period allotment.
func (d DeviceCredits) Available() int {
	return d.Balance + d.AllotmentBalance
}

// CreditAllotmentRule is an admin-configured recurring credit gift.
type CreditAllotmentRule struct {
	ID         string              `json:"id"`
	Name       string              `json:"name"`
	Note       string              `json:"note,omitempty"`
	Amount     int                 `json:"amount"`
	Interval   AllotmentInterval   `json:"interval"`
	TargetType AllotmentTargetType `json:"target_type"`
	TargetID   string              `json:"target_id"` // group id or enrollment_id
	Enabled    bool                `json:"enabled"`
	LastRunAt  *time.Time          `json:"last_run_at,omitempty"`
	CreatedAt  time.Time           `json:"created_at"`
	UpdatedAt  time.Time           `json:"updated_at"`
}

// CreditAllotmentGrant tracks one period's allotment for a rule+enrollment.
// Remaining is unused allotment still sitting in allotment_balance for clawback.
type CreditAllotmentGrant struct {
	ID           string    `json:"id"`
	RuleID       string    `json:"rule_id"`
	EnrollmentID string    `json:"enrollment_id"`
	PeriodKey    string    `json:"period_key"`
	Amount       int       `json:"amount"`
	Remaining    int       `json:"remaining"`
	GrantedAt    time.Time `json:"granted_at"`
}

// ApplyAllotmentInput grants one period's allotment (idempotent on rule+enrollment+period).
type ApplyAllotmentInput struct {
	RuleID       string
	EnrollmentID string
	PeriodKey    string
	Amount       int
	Note         string
}

// ApplyAllotmentResult is returned after attempting a period grant.
type ApplyAllotmentResult struct {
	Applied  bool
	Credits  DeviceCredits
	Expired  int // clawed back from prior period(s) of this rule
	Granted  int
	Entry    *CreditLedgerEntry // allotment grant entry when Applied
}

// CreditLedgerEntry is one immutable balance change.
type CreditLedgerEntry struct {
	ID           string       `json:"id"`
	EnrollmentID string       `json:"enrollment_id"`
	Delta        int          `json:"delta"`
	BalanceAfter int          `json:"balance_after"`
	Reason       LedgerReason `json:"reason"`
	RefType      string       `json:"ref_type"`
	RefID        string       `json:"ref_id"`
	Note         string       `json:"note,omitempty"`
	CreatedAt    time.Time    `json:"created_at"`
}

// CreditPackage is a purchasable credit bundle.
type CreditPackage struct {
	ID          string `json:"id"`
	NameHe      string `json:"name_he"`
	Credits     int    `json:"credits"`
	PriceAgorot int    `json:"price_agorot"` // ILS agorot (₪1.00 = 100)
	Active      bool   `json:"active"`
	SortOrder   int    `json:"sort_order"`
}

// CreditSettings is the singleton global credits configuration.
type CreditSettings struct {
	AccessRequestCost int       `json:"access_request_cost"`
	Enabled           bool      `json:"enabled"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// CreditPurchase is a checkout intent / paid purchase.
type CreditPurchase struct {
	ID             string          `json:"id"`
	EnrollmentID   string          `json:"enrollment_id"`
	PackageID      string          `json:"package_id"`
	Credits        int             `json:"credits"`
	AmountAgorot   int             `json:"amount_agorot"`
	Status         PurchaseStatus  `json:"status"`
	Provider       PaymentProvider `json:"provider"`
	ProviderTxID   string          `json:"provider_tx_id"`
	ClientUniqueID string          `json:"client_unique_id"`
	CreatedAt      time.Time       `json:"created_at"`
	PaidAt         *time.Time      `json:"paid_at,omitempty"`
}

// AdjustCreditsInput is a single atomic balance change with ledger row.
type AdjustCreditsInput struct {
	EnrollmentID string
	Delta        int
	Reason       LedgerReason
	RefType      string
	RefID        string
	Note         string
}

// AdjustCreditsResult is returned after an atomic credit adjustment.
type AdjustCreditsResult struct {
	Balance          int // permanent bucket after change
	AllotmentBalance int // period bucket after change
	Entry            CreditLedgerEntry
	Applied          bool // false when idempotent key already existed
}

// MarkPurchasePaidInput marks a purchase paid and credits the device.
type MarkPurchasePaidInput struct {
	PurchaseID   string
	ProviderTxID string
}

// Store persists allowlists, grants, requests, groups, app metadata, and credits.
type Store interface {
	Ping(ctx context.Context) error
	Kind() string

	ListAllowlist(ctx context.Context) ([]policy.Entry, error)
	UpsertAllowlist(ctx context.Context, entry policy.Entry) error
	DeleteAllowlist(ctx context.Context, kind policy.Kind, value string, target policy.Target) error

	ListGrants(ctx context.Context) ([]policy.Grant, error)
	AddGrant(ctx context.Context, grant policy.Grant) error
	DeleteGrants(ctx context.Context, kind policy.Kind, value string, target policy.Target) error

	CreateRequest(ctx context.Context, req Request) (Request, error)
	GetRequest(ctx context.Context, id string) (Request, error)
	ListRequests(ctx context.Context, status *RequestStatus) ([]Request, error)
	ListRequestsByEnrollment(ctx context.Context, enrollmentID string) ([]Request, error)
	UpdateRequest(ctx context.Context, req Request) error
	DeleteRequest(ctx context.Context, id string) error

	ListRequestMessages(ctx context.Context, requestID string) ([]RequestMessage, error)
	AddRequestMessage(ctx context.Context, msg RequestMessage) (RequestMessage, error)
	CountRequestMessages(ctx context.Context, requestID string) (int, error)
	LastRequestMessage(ctx context.Context, requestID string) (RequestMessage, error)

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

	ListDevices(ctx context.Context) ([]Device, error)
	SetDeviceName(ctx context.Context, enrollmentID, name string) error

	EnsureCreditBalance(ctx context.Context, enrollmentID string) (DeviceCredits, error)
	GetCreditBalance(ctx context.Context, enrollmentID string) (DeviceCredits, error)
	AdjustCredits(ctx context.Context, in AdjustCreditsInput) (AdjustCreditsResult, error)
	ListCreditLedger(ctx context.Context, enrollmentID string, limit int) ([]CreditLedgerEntry, error)
	ListCreditBalances(ctx context.Context) ([]DeviceCredits, error)

	ListCreditPackages(ctx context.Context, activeOnly bool) ([]CreditPackage, error)
	GetCreditPackage(ctx context.Context, id string) (CreditPackage, error)
	CreateCreditPackage(ctx context.Context, p CreditPackage) (CreditPackage, error)
	UpdateCreditPackage(ctx context.Context, p CreditPackage) (CreditPackage, error)

	GetCreditSettings(ctx context.Context) (CreditSettings, error)
	UpsertCreditSettings(ctx context.Context, settings CreditSettings) (CreditSettings, error)

	CreateCreditPurchase(ctx context.Context, p CreditPurchase) (CreditPurchase, error)
	GetCreditPurchase(ctx context.Context, id string) (CreditPurchase, error)
	GetCreditPurchaseByClientUnique(ctx context.Context, clientUniqueID string) (CreditPurchase, error)
	MarkPurchasePaid(ctx context.Context, in MarkPurchasePaidInput) (CreditPurchase, bool, error)

	ListAllotmentRules(ctx context.Context) ([]CreditAllotmentRule, error)
	GetAllotmentRule(ctx context.Context, id string) (CreditAllotmentRule, error)
	CreateAllotmentRule(ctx context.Context, rule CreditAllotmentRule) (CreditAllotmentRule, error)
	UpdateAllotmentRule(ctx context.Context, rule CreditAllotmentRule) (CreditAllotmentRule, error)
	DeleteAllotmentRule(ctx context.Context, id string) error
	TouchAllotmentRuleRun(ctx context.Context, id string, at time.Time) error

	// ApplyAllotmentPeriod expires unused prior-period allotment for this rule,
	// then grants Amount into allotment_balance. Idempotent on rule+enrollment+period.
	ApplyAllotmentPeriod(ctx context.Context, in ApplyAllotmentInput) (ApplyAllotmentResult, error)
}
