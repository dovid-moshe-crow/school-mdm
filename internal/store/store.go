package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
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

// WhitelistPack is a named set of apps/URLs (e.g. "Games") assignable to targets.
type WhitelistPack struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	ItemCount   int       `json:"item_count,omitempty"`
}

// WhitelistPackItem is one app or URL inside a pack.
type WhitelistPackItem struct {
	PackID string      `json:"pack_id"`
	Kind   policy.Kind `json:"kind"`
	Value  string      `json:"value"`
}

// WhitelistPackAssignment attaches a pack to global / device-group / device.
type WhitelistPackAssignment struct {
	PackID     string            `json:"pack_id"`
	TargetType policy.TargetType `json:"target_type"`
	TargetID   string            `json:"target_id"`
}

// CustomProfile is an uploaded Apple configuration profile (.mobileconfig).
type CustomProfile struct {
	ID                 string    `json:"id"`
	Name               string    `json:"name"`
	Description        string    `json:"description"`
	Filename           string    `json:"filename"`
	PayloadIdentifier  string    `json:"payload_identifier"`
	PayloadUUID        string    `json:"payload_uuid"`
	PayloadDisplayName string    `json:"payload_display_name"`
	PayloadType        string    `json:"payload_type"`
	SizeBytes          int       `json:"size_bytes"`
	AssignmentCount    int       `json:"assignment_count,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
	Payload            []byte    `json:"-"`
}

// CustomProfileAssignment attaches a custom profile to global / group / device.
type CustomProfileAssignment struct {
	ProfileID  string            `json:"profile_id"`
	TargetType policy.TargetType `json:"target_type"`
	TargetID   string            `json:"target_id"`
}

// SystemAllowlistItem is a built-in Apple/system app applied to every device.
type SystemAllowlistItem struct {
	Kind    policy.Kind `json:"kind"`
	Value   string      `json:"value"`
	Enabled bool        `json:"enabled"`
}

const (
	TimerActionAdd    = "add"
	TimerActionRemove = "remove"
	TimerOnce         = "once"
	TimerWeekly       = "weekly"
)

// PolicyTimer adds or removes whitelist packs and/or custom profiles on
// devices/groups on a schedule (Israel local time for weekly clocks).
type PolicyTimer struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Action     string     `json:"action"`
	PackIDs    []string   `json:"pack_ids"`
	ProfileIDs []string   `json:"profile_ids"`
	DeviceIDs  []string   `json:"device_ids"`
	GroupIDs   []string   `json:"group_ids"`
	Schedule   string     `json:"schedule"`
	RunAt      *time.Time `json:"run_at,omitempty"`
	Weekdays   []int      `json:"weekdays,omitempty"`
	TimeOfDay  string     `json:"time_of_day,omitempty"`
	Enabled    bool       `json:"enabled"`
	LastRunAt  *time.Time `json:"last_run_at,omitempty"`
	LastRunKey string     `json:"last_run_key,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

// Device is an enrollment with optional display name and policy mode.
type Device struct {
	EnrollmentID string   `json:"enrollment_id"`
	Name         string   `json:"name"`
	Unrestricted bool     `json:"unrestricted"`
	SerialNumber string   `json:"serial_number,omitempty"`
	Enabled      *bool    `json:"enabled,omitempty"`
	LastSeenAt   string   `json:"last_seen_at,omitempty"`
	GroupIDs     []string `json:"group_ids,omitempty"`
	MDM          bool     `json:"mdm,omitempty"`
}

// Activity categories / results for the Admin audit trail.
const (
	ActivityCategoryMDM      = "mdm"
	ActivityCategoryPolicy   = "policy"
	ActivityCategoryGroups   = "groups"
	ActivityCategoryRequests = "requests"
	ActivityCategoryCredits  = "credits"
	ActivityCategoryDevices  = "devices"
	ActivityCategoryABM      = "abm"
	ActivityCategorySystem   = "system"
	ActivityCategoryWebhooks = "webhooks"

	ActivityActorAdmin   = "admin"
	ActivityActorDevice  = "device"
	ActivityActorSystem  = "system"
	ActivityActorWebhook = "webhook"

	ActivityResultOK    = "ok"
	ActivityResultError = "error"
	ActivityResultInfo  = "info"
)

// ActivityEvent is one searchable audit row.
type ActivityEvent struct {
	ID           string          `json:"id"`
	At           time.Time       `json:"at"`
	Category     string          `json:"category"`
	Action       string          `json:"action"`
	ActorType    string          `json:"actor_type"`
	Actor        string          `json:"actor"`
	EnrollmentID string          `json:"enrollment_id,omitempty"`
	GroupID      string          `json:"group_id,omitempty"`
	RequestID    string          `json:"request_id,omitempty"`
	CommandUUID  string          `json:"command_uuid,omitempty"`
	Result       string          `json:"result"`
	Summary      string          `json:"summary"`
	Detail       json.RawMessage `json:"detail,omitempty"`
}

// ActivityFilter selects rows for ListActivityEvents.
type ActivityFilter struct {
	From         *time.Time
	To           *time.Time
	Category     string
	Action       string
	EnrollmentID string
	ActorType    string
	Result       string
	Q            string // summary ILIKE
	Limit        int
	Offset       int
}

// WebhookEndpoint is an outbound HTTP receiver for activity events.
type WebhookEndpoint struct {
	ID          string    `json:"id"`
	URL         string    `json:"url"`
	Secret      string    `json:"secret,omitempty"`
	Description string    `json:"description"`
	Events      []string  `json:"events"`
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
}

const (
	WebhookDeliveryPending = "pending"
	WebhookDeliverySuccess = "success"
	WebhookDeliveryFailed  = "failed"
)

// WebhookDelivery is one HTTP POST attempt to a webhook endpoint.
type WebhookDelivery struct {
	ID         string    `json:"id"`
	EndpointID string    `json:"endpoint_id"`
	EventID    string    `json:"event_id"`
	EventName  string    `json:"event_name"`
	Status     string    `json:"status"`
	Attempt    int       `json:"attempt"`
	HTTPStatus int       `json:"http_status"`
	Error      string    `json:"error,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// AccessRequest is kept as an alias for older call sites.
type AccessRequest = Request

// LedgerReason classifies a credit balance change.
type LedgerReason string

const (
	LedgerPurchase        LedgerReason = "purchase"
	LedgerSpend           LedgerReason = "spend"
	LedgerRefund          LedgerReason = "refund"
	LedgerGift            LedgerReason = "gift"
	LedgerAdjust          LedgerReason = "adjust"
	LedgerAllotment       LedgerReason = "allotment"
	LedgerAllotmentExpire LedgerReason = "allotment_expire"
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
	AllotmentEveryone   AllotmentTargetType = "everyone"
	AllotmentGroup      AllotmentTargetType = "group"
	AllotmentIndividual AllotmentTargetType = "individual"
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
	EnrollmentID     string    `json:"enrollment_id"`
	Balance          int       `json:"balance"`
	AllotmentBalance int       `json:"allotment_balance"`
	UpdatedAt        time.Time `json:"updated_at"`
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
	Applied bool
	Credits DeviceCredits
	Expired int // clawed back from prior period(s) of this rule
	Granted int
	Entry   *CreditLedgerEntry // allotment grant entry when Applied
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

// MDMSettings is the singleton ABM/DEP configuration for this school server.
type MDMSettings struct {
	DepName            string     `json:"dep_name"`
	DEPProfileUUID     string     `json:"dep_profile_uuid,omitempty"`
	CompanionBundleID  string     `json:"companion_bundle_id"`
	CompanionITunesID  int64      `json:"companion_itunes_id"`
	CompanionEnabled   bool       `json:"companion_enabled"`
	LockScreenEnabled  bool       `json:"lock_screen_enabled"`
	LockScreenFootnote string     `json:"lock_screen_footnote"`
	HasVPPToken        bool       `json:"has_vpp_token"`
	VPPTokenFilename   string     `json:"vpp_token_filename,omitempty"`
	VPPTokenUpdatedAt  *time.Time `json:"vpp_token_updated_at,omitempty"`
	VPPToken           []byte     `json:"-"` // content token bytes; never serialize to clients
	UpdatedAt          time.Time  `json:"updated_at"`
}

// ABMDeviceCache is the last synced Apple DEP device list for Admin.
type ABMDeviceCache struct {
	Devices  json.RawMessage `json:"devices"`
	SyncedAt *time.Time      `json:"synced_at,omitempty"`
}

// DevicePushToken is an Expo/APNs push token registered by the KFilter app.
type DevicePushToken struct {
	EnrollmentID string    `json:"enrollment_id"`
	Token        string    `json:"token"`
	Platform     string    `json:"platform"`
	UpdatedAt    time.Time `json:"updated_at"`
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

// CreditPurchaseFilter selects rows for ListCreditPurchases.
type CreditPurchaseFilter struct {
	EnrollmentID string
	Status       string // pending|paid|failed|expired; empty = all
	Limit        int
	Offset       int
}

// Store persists allowlists, grants, requests, groups, app metadata, and credits.
type Store interface {
	Ping(ctx context.Context) error
	Kind() string

	ListAllowlist(ctx context.Context) ([]policy.Entry, error)
	UpsertAllowlist(ctx context.Context, entry policy.Entry) error
	DeleteAllowlist(ctx context.Context, kind policy.Kind, value string, target policy.Target) error

	ListWhitelistPacks(ctx context.Context) ([]WhitelistPack, error)
	GetWhitelistPack(ctx context.Context, id string) (WhitelistPack, error)
	CreateWhitelistPack(ctx context.Context, p WhitelistPack) (WhitelistPack, error)
	UpdateWhitelistPack(ctx context.Context, p WhitelistPack) error
	DeleteWhitelistPack(ctx context.Context, id string) error
	ListWhitelistPackItems(ctx context.Context, packID string) ([]WhitelistPackItem, error)
	AddWhitelistPackItem(ctx context.Context, item WhitelistPackItem) error
	RemoveWhitelistPackItem(ctx context.Context, packID string, kind policy.Kind, value string) error
	ListWhitelistPackAssignments(ctx context.Context, packID string) ([]WhitelistPackAssignment, error)
	SetWhitelistPackAssignment(ctx context.Context, a WhitelistPackAssignment) error
	RemoveWhitelistPackAssignment(ctx context.Context, packID string, target policy.Target) error
	// ListAllowlistFromPacks returns synthetic allowlist entries from packs that apply
	// to the given device (global + its groups + device assignments).
	ListAllowlistFromPacks(ctx context.Context, enrollmentID string, groupIDs []string) ([]policy.Entry, error)

	ListCustomProfiles(ctx context.Context) ([]CustomProfile, error)
	GetCustomProfile(ctx context.Context, id string) (CustomProfile, error)
	GetCustomProfilePayload(ctx context.Context, id string) ([]byte, error)
	CreateCustomProfile(ctx context.Context, p CustomProfile) (CustomProfile, error)
	UpdateCustomProfile(ctx context.Context, p CustomProfile) (CustomProfile, error)
	ReplaceCustomProfilePayload(ctx context.Context, id string, payload []byte, filename string) (CustomProfile, error)
	DeleteCustomProfile(ctx context.Context, id string) error
	ListCustomProfileAssignments(ctx context.Context, profileID string) ([]CustomProfileAssignment, error)
	SetCustomProfileAssignment(ctx context.Context, a CustomProfileAssignment) error
	RemoveCustomProfileAssignment(ctx context.Context, profileID string, target policy.Target) error
	// ListCustomProfilesForDevice returns assigned custom profiles (with payload) for a device.
	ListCustomProfilesForDevice(ctx context.Context, enrollmentID string, groupIDs []string) ([]CustomProfile, error)

	ListSystemAllowlist(ctx context.Context) ([]SystemAllowlistItem, error)
	UpsertSystemAllowlist(ctx context.Context, item SystemAllowlistItem) error
	SetSystemAllowlistEnabled(ctx context.Context, kind policy.Kind, value string, enabled bool) error
	DeleteSystemAllowlist(ctx context.Context, kind policy.Kind, value string) error

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
	GetDevice(ctx context.Context, enrollmentID string) (Device, error)
	SetDeviceName(ctx context.Context, enrollmentID, name string) error
	SetDeviceUnrestricted(ctx context.Context, enrollmentID string, unrestricted bool) error
	EnsureDevice(ctx context.Context, enrollmentID string) error

	InsertActivityEvent(ctx context.Context, e ActivityEvent) (ActivityEvent, error)
	ListActivityEvents(ctx context.Context, f ActivityFilter) ([]ActivityEvent, error)

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

	GetMDMSettings(ctx context.Context) (MDMSettings, error)
	UpsertMDMSettings(ctx context.Context, settings MDMSettings) (MDMSettings, error)

	// Apple DEP device list cache (last successful sync from ASM/ABM).
	GetABMDeviceCache(ctx context.Context) (ABMDeviceCache, error)
	SaveABMDeviceCache(ctx context.Context, devices json.RawMessage) (ABMDeviceCache, error)

	UpsertPushToken(ctx context.Context, t DevicePushToken) error
	ListPushTokens(ctx context.Context, enrollmentID string) ([]DevicePushToken, error)
	DeletePushToken(ctx context.Context, token string) error
	HasPushToken(ctx context.Context, enrollmentID string) (bool, error)

	CreateCreditPurchase(ctx context.Context, p CreditPurchase) (CreditPurchase, error)
	GetCreditPurchase(ctx context.Context, id string) (CreditPurchase, error)
	GetCreditPurchaseByClientUnique(ctx context.Context, clientUniqueID string) (CreditPurchase, error)
	ListCreditPurchases(ctx context.Context, f CreditPurchaseFilter) ([]CreditPurchase, error)
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

	ListWebhookEndpoints(ctx context.Context) ([]WebhookEndpoint, error)
	GetWebhookEndpoint(ctx context.Context, id string) (WebhookEndpoint, error)
	CreateWebhookEndpoint(ctx context.Context, ep WebhookEndpoint) (WebhookEndpoint, error)
	UpdateWebhookEndpoint(ctx context.Context, ep WebhookEndpoint) (WebhookEndpoint, error)
	DeleteWebhookEndpoint(ctx context.Context, id string) error
	InsertWebhookDelivery(ctx context.Context, d WebhookDelivery) (WebhookDelivery, error)
	ListWebhookDeliveries(ctx context.Context, endpointID string, limit int) ([]WebhookDelivery, error)

	ListPolicyTimers(ctx context.Context) ([]PolicyTimer, error)
	GetPolicyTimer(ctx context.Context, id string) (PolicyTimer, error)
	CreatePolicyTimer(ctx context.Context, t PolicyTimer) (PolicyTimer, error)
	UpdatePolicyTimer(ctx context.Context, t PolicyTimer) (PolicyTimer, error)
	DeletePolicyTimer(ctx context.Context, id string) error
	TouchPolicyTimerRun(ctx context.Context, id string, at time.Time, runKey string, enabled bool) error
}

// NormalizePolicyTimer trims IDs, canonicalizes action/schedule/time, and validates.
func NormalizePolicyTimer(t *PolicyTimer) error {
	if t == nil {
		return fmt.Errorf("timer is required")
	}
	t.Name = strings.TrimSpace(t.Name)
	t.Action = strings.ToLower(strings.TrimSpace(t.Action))
	t.Schedule = strings.ToLower(strings.TrimSpace(t.Schedule))
	t.PackIDs = uniqueTrimmedStrings(t.PackIDs)
	t.ProfileIDs = uniqueTrimmedStrings(t.ProfileIDs)
	t.DeviceIDs = uniqueTrimmedStrings(t.DeviceIDs)
	t.GroupIDs = uniqueTrimmedStrings(t.GroupIDs)
	t.Weekdays = uniqueWeekdays(t.Weekdays)
	t.LastRunKey = strings.TrimSpace(t.LastRunKey)
	if strings.TrimSpace(t.TimeOfDay) != "" {
		h, m, err := ParseTimerTimeOfDay(t.TimeOfDay)
		if err != nil {
			return err
		}
		t.TimeOfDay = fmt.Sprintf("%02d:%02d", h, m)
	} else {
		t.TimeOfDay = ""
	}
	if t.RunAt != nil {
		u := t.RunAt.UTC()
		t.RunAt = &u
	}
	return ValidatePolicyTimer(*t)
}

// ParseTimerTimeOfDay accepts HH:MM or H:MM (optional seconds).
func ParseTimerTimeOfDay(s string) (hour, min int, err error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, 0, fmt.Errorf("time_of_day is required")
	}
	parts := strings.Split(s, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0, 0, fmt.Errorf("time_of_day must be HH:MM")
	}
	hour, err = strconv.Atoi(parts[0])
	if err != nil || hour < 0 || hour > 23 {
		return 0, 0, fmt.Errorf("time_of_day hour must be 0-23")
	}
	min, err = strconv.Atoi(parts[1])
	if err != nil || min < 0 || min > 59 {
		return 0, 0, fmt.Errorf("time_of_day minute must be 0-59")
	}
	return hour, min, nil
}

// ValidatePolicyTimer checks required fields after normalization.
func ValidatePolicyTimer(t PolicyTimer) error {
	if strings.TrimSpace(t.Name) == "" {
		return fmt.Errorf("name is required")
	}
	switch t.Action {
	case TimerActionAdd, TimerActionRemove:
	default:
		return fmt.Errorf("action must be add or remove")
	}
	if len(t.PackIDs) == 0 && len(t.ProfileIDs) == 0 {
		return fmt.Errorf("at least one pack or profile is required")
	}
	if len(t.DeviceIDs) == 0 && len(t.GroupIDs) == 0 {
		return fmt.Errorf("at least one device or group is required")
	}
	switch t.Schedule {
	case TimerOnce:
		if t.RunAt == nil || t.RunAt.IsZero() {
			return fmt.Errorf("run_at is required for a one-time timer")
		}
	case TimerWeekly:
		if len(t.Weekdays) == 0 {
			return fmt.Errorf("at least one weekday is required")
		}
		if _, _, err := ParseTimerTimeOfDay(t.TimeOfDay); err != nil {
			return err
		}
	default:
		return fmt.Errorf("schedule must be once or weekly")
	}
	return nil
}

func uniqueTrimmedStrings(in []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func uniqueWeekdays(in []int) []int {
	seen := map[int]struct{}{}
	out := make([]int, 0, len(in))
	for _, d := range in {
		if d < 0 || d > 6 {
			continue
		}
		if _, ok := seen[d]; ok {
			continue
		}
		seen[d] = struct{}{}
		out = append(out, d)
	}
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j] < out[i] {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}
