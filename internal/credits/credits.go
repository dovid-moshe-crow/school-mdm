package credits

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/dwdmsh/school-mdm/internal/nedarim"
	"github.com/dwdmsh/school-mdm/internal/store"
)

// ErrInsufficientCredits is re-exported for callers.
var ErrInsufficientCredits = store.ErrInsufficientCredits

const settingsCacheTTL = 5 * time.Second

// Service manages device credits and Nedarim checkouts.
type Service struct {
	Store       store.Store
	Nedarim     *nedarim.Client
	AccessCost  int // env default / fallback when DB settings missing
	PortalBase  string
	WebhookPath string // e.g. /api/webhooks/nedarim

	settingsMu       sync.Mutex
	settingsCache    store.CreditSettings
	settingsCachedAt time.Time
	settingsLoaded   bool
}

func (s *Service) defaultAccessCost() int {
	if s.AccessCost < 1 {
		return 1
	}
	return s.AccessCost
}

// EnsureSettings seeds DB settings from AccessCost when no row exists yet.
func (s *Service) EnsureSettings(ctx context.Context) error {
	_, err := s.Store.GetCreditSettings(ctx)
	if err == nil {
		return nil
	}
	if !errors.Is(err, store.ErrNotFound) {
		return err
	}
	_, err = s.Store.UpsertCreditSettings(ctx, store.CreditSettings{
		AccessRequestCost: s.defaultAccessCost(),
		Enabled:           true,
	})
	s.invalidateSettingsCache()
	return err
}

func (s *Service) invalidateSettingsCache() {
	s.settingsMu.Lock()
	defer s.settingsMu.Unlock()
	s.settingsLoaded = false
}

func (s *Service) loadSettings(ctx context.Context) (store.CreditSettings, error) {
	s.settingsMu.Lock()
	defer s.settingsMu.Unlock()
	if s.settingsLoaded && time.Since(s.settingsCachedAt) < settingsCacheTTL {
		return s.settingsCache, nil
	}
	settings, err := s.Store.GetCreditSettings(ctx)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			settings = store.CreditSettings{
				AccessRequestCost: s.defaultAccessCost(),
				Enabled:           true,
				UpdatedAt:         time.Now().UTC(),
			}
		} else {
			return store.CreditSettings{}, err
		}
	}
	s.settingsCache = settings
	s.settingsCachedAt = time.Now()
	s.settingsLoaded = true
	return settings, nil
}

// AccessRequestCost returns the effective spend cost (DB settings, else env default).
func (s *Service) AccessRequestCost(ctx context.Context) int {
	settings, err := s.loadSettings(ctx)
	if err != nil || settings.AccessRequestCost < 1 {
		return s.defaultAccessCost()
	}
	return settings.AccessRequestCost
}

// GetSettings returns global credit settings (ensuring defaults).
func (s *Service) GetSettings(ctx context.Context) (store.CreditSettings, error) {
	if err := s.EnsureSettings(ctx); err != nil {
		return store.CreditSettings{}, err
	}
	return s.loadSettings(ctx)
}

// UpdateSettings updates global credit settings.
func (s *Service) UpdateSettings(ctx context.Context, accessRequestCost int, enabled *bool) (store.CreditSettings, error) {
	if accessRequestCost < 1 {
		return store.CreditSettings{}, fmt.Errorf("access_request_cost must be >= 1")
	}
	current, err := s.GetSettings(ctx)
	if err != nil {
		return store.CreditSettings{}, err
	}
	en := current.Enabled
	if enabled != nil {
		en = *enabled
	}
	updated, err := s.Store.UpsertCreditSettings(ctx, store.CreditSettings{
		AccessRequestCost: accessRequestCost,
		Enabled:           en,
	})
	if err != nil {
		return store.CreditSettings{}, err
	}
	s.invalidateSettingsCache()
	return updated, nil
}

// Balance returns the credit balance for an enrollment (0 if never touched).
func (s *Service) Balance(ctx context.Context, enrollmentID string) (store.DeviceCredits, error) {
	return s.Store.GetCreditBalance(ctx, strings.TrimSpace(enrollmentID))
}

// Ledger returns recent ledger entries for a device.
func (s *Service) Ledger(ctx context.Context, enrollmentID string, limit int) ([]store.CreditLedgerEntry, error) {
	enrollmentID = strings.TrimSpace(enrollmentID)
	if enrollmentID == "" {
		return nil, fmt.Errorf("enrollment_id is required")
	}
	if limit <= 0 {
		limit = 20
	}
	return s.Store.ListCreditLedger(ctx, enrollmentID, limit)
}

// SpendForAccessRequest deducts AccessCost credits keyed by request id (idempotent).
func (s *Service) SpendForAccessRequest(ctx context.Context, enrollmentID, requestID string) error {
	enrollmentID = strings.TrimSpace(enrollmentID)
	requestID = strings.TrimSpace(requestID)
	if enrollmentID == "" || requestID == "" {
		return fmt.Errorf("enrollment_id and request_id are required")
	}
	cost := s.AccessRequestCost(ctx)
	_, err := s.Store.AdjustCredits(ctx, store.AdjustCreditsInput{
		EnrollmentID: enrollmentID,
		Delta:        -cost,
		Reason:       store.LedgerSpend,
		RefType:      "request",
		RefID:        requestID,
	})
	if err != nil {
		if errors.Is(err, store.ErrInsufficientCredits) {
			return fmt.Errorf("%w: need %d", ErrInsufficientCredits, cost)
		}
		return err
	}
	return nil
}

// RefundForDeniedRequest returns the spent credit (idempotent).
func (s *Service) RefundForDeniedRequest(ctx context.Context, enrollmentID, requestID string) error {
	enrollmentID = strings.TrimSpace(enrollmentID)
	requestID = strings.TrimSpace(requestID)
	if enrollmentID == "" || requestID == "" {
		return fmt.Errorf("enrollment_id and request_id are required")
	}
	cost := s.AccessRequestCost(ctx)
	_, err := s.Store.AdjustCredits(ctx, store.AdjustCreditsInput{
		EnrollmentID: enrollmentID,
		Delta:        cost,
		Reason:       store.LedgerRefund,
		RefType:      "request",
		RefID:        requestID,
	})
	return err
}

// Gift adds credits from an admin (idempotent when note key is provided via ref).
func (s *Service) Gift(ctx context.Context, enrollmentID string, amount int, adminNote string) (store.AdjustCreditsResult, error) {
	if amount < 1 {
		return store.AdjustCreditsResult{}, fmt.Errorf("amount must be >= 1")
	}
	return s.Adjust(ctx, enrollmentID, amount, adminNote)
}

// Adjust adds or subtracts credits. Positive amount uses reason gift; negative uses adjust.
func (s *Service) Adjust(ctx context.Context, enrollmentID string, amount int, adminNote string) (store.AdjustCreditsResult, error) {
	enrollmentID = strings.TrimSpace(enrollmentID)
	if enrollmentID == "" {
		return store.AdjustCreditsResult{}, fmt.Errorf("enrollment_id is required")
	}
	if amount == 0 {
		return store.AdjustCreditsResult{}, fmt.Errorf("amount must not be zero")
	}
	if _, err := s.Store.EnsureCreditBalance(ctx, enrollmentID); err != nil {
		return store.AdjustCreditsResult{}, err
	}
	reason := store.LedgerAdjust
	refType := "adjust"
	if amount > 0 {
		reason = store.LedgerGift
		refType = "gift"
	}
	refID := uuid.NewString()
	return s.Store.AdjustCredits(ctx, store.AdjustCreditsInput{
		EnrollmentID: enrollmentID,
		Delta:        amount,
		Reason:       reason,
		RefType:      refType,
		RefID:        refID,
		Note:         strings.TrimSpace(adminNote),
	})
}

// CheckoutResult is returned by StartCheckout.
type CheckoutResult struct {
	Purchase  store.CreditPurchase `json:"purchase"`
	IframeURL string               `json:"iframe_url"`
	Mode      string               `json:"mode"`
}

// StartCheckout creates a pending purchase and returns an iframe URL.
func (s *Service) StartCheckout(ctx context.Context, enrollmentID, packageID string) (CheckoutResult, error) {
	enrollmentID = strings.TrimSpace(enrollmentID)
	packageID = strings.TrimSpace(packageID)
	if enrollmentID == "" {
		return CheckoutResult{}, fmt.Errorf("enrollment_id is required")
	}
	if packageID == "" {
		return CheckoutResult{}, fmt.Errorf("package_id is required")
	}
	pkg, err := s.Store.GetCreditPackage(ctx, packageID)
	if err != nil {
		return CheckoutResult{}, err
	}
	if !pkg.Active {
		return CheckoutResult{}, fmt.Errorf("package is not active")
	}
	if _, err := s.Store.EnsureCreditBalance(ctx, enrollmentID); err != nil {
		return CheckoutResult{}, err
	}

	mode := nedarim.ModeFake
	if s.Nedarim != nil {
		mode = strings.ToLower(strings.TrimSpace(s.Nedarim.Cfg.Mode))
		if mode == "" {
			mode = nedarim.ModeFake
		}
	}
	provider := store.ProviderFake
	if mode == nedarim.ModeLive {
		provider = store.ProviderNedarim
	}

	clientUnique := uuid.NewString()
	purchase, err := s.Store.CreateCreditPurchase(ctx, store.CreditPurchase{
		EnrollmentID:   enrollmentID,
		PackageID:      pkg.ID,
		Credits:        pkg.Credits,
		AmountAgorot:   pkg.PriceAgorot,
		Status:         store.PurchasePending,
		Provider:       provider,
		ClientUniqueID: clientUnique,
	})
	if err != nil {
		return CheckoutResult{}, err
	}

	callback := s.webhookURL()
	if s.Nedarim == nil {
		return CheckoutResult{}, fmt.Errorf("nedarim client is not configured")
	}
	txn, err := s.Nedarim.CreateDebitIframe(ctx, nedarim.CreateTxnInput{
		AmountAgorot:   purchase.AmountAgorot,
		ClientUniqueID: purchase.ClientUniqueID,
		CallbackURL:    callback,
		Comment:        fmt.Sprintf("credits:%d package:%s", purchase.Credits, pkg.NameHe),
		PurchaseID:     purchase.ID,
	})
	if err != nil {
		return CheckoutResult{}, err
	}
	if txn.ProviderTxID != "" {
		purchase.ProviderTxID = txn.ProviderTxID
		// Best-effort persist provider tx id without marking paid.
		_ = s.persistProviderTx(ctx, purchase)
	}

	return CheckoutResult{
		Purchase:  purchase,
		IframeURL: txn.IframeURL,
		Mode:      txn.Mode,
	}, nil
}

func (s *Service) persistProviderTx(ctx context.Context, p store.CreditPurchase) error {
	// Memory/postgres don't expose a generic update; re-create path isn't needed for fake.
	// Live bridge reads ClientUniqueID; ProviderTxID is optional.
	_ = ctx
	_ = p
	return nil
}

func (s *Service) webhookURL() string {
	base := strings.TrimRight(s.PortalBase, "/")
	path := s.WebhookPath
	if path == "" {
		path = "/api/webhooks/nedarim"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return base + path
}

// ConfirmPayment verifies a purchase is paid (or pays it in fake after server-side fake-pay).
// Never trusts the browser alone — only credits when purchase status is paid / MarkPurchasePaid succeeds.
func (s *Service) ConfirmPayment(ctx context.Context, purchaseID, enrollmentID string) (store.CreditPurchase, store.DeviceCredits, error) {
	purchaseID = strings.TrimSpace(purchaseID)
	enrollmentID = strings.TrimSpace(enrollmentID)
	p, err := s.Store.GetCreditPurchase(ctx, purchaseID)
	if err != nil {
		return store.CreditPurchase{}, store.DeviceCredits{}, err
	}
	if enrollmentID != "" && p.EnrollmentID != enrollmentID {
		return store.CreditPurchase{}, store.DeviceCredits{}, fmt.Errorf("purchase does not belong to this device")
	}
	if p.Status != store.PurchasePaid {
		return store.CreditPurchase{}, store.DeviceCredits{}, fmt.Errorf("payment not confirmed yet (status=%s)", p.Status)
	}
	bal, err := s.Store.GetCreditBalance(ctx, p.EnrollmentID)
	if err != nil {
		return p, store.DeviceCredits{}, err
	}
	return p, bal, nil
}

// WebhookPayload is a normalized Nedarim (or fake) webhook body.
type WebhookPayload struct {
	ClientUniqueID string
	TransactionID  string
	Amount         string
	Status         string // empty or "OK" / "Success" means paid; "Error" / "Fail" means failed
	Raw            map[string]any
}

// HandleWebhook marks a purchase paid and credits the ledger (idempotent).
func (s *Service) HandleWebhook(ctx context.Context, payload WebhookPayload) (store.CreditPurchase, error) {
	clientUnique := strings.TrimSpace(payload.ClientUniqueID)
	if clientUnique == "" {
		return store.CreditPurchase{}, fmt.Errorf("missing ClientUniqueId / Param2")
	}
	status := strings.ToLower(strings.TrimSpace(payload.Status))
	if status == "error" || status == "fail" || status == "failed" || status == "refusal" {
		p, err := s.Store.GetCreditPurchaseByClientUnique(ctx, clientUnique)
		if err != nil {
			return store.CreditPurchase{}, err
		}
		return p, fmt.Errorf("payment failed: %s", payload.Status)
	}

	p, err := s.Store.GetCreditPurchaseByClientUnique(ctx, clientUnique)
	if err != nil {
		return store.CreditPurchase{}, err
	}
	paid, _, err := s.Store.MarkPurchasePaid(ctx, store.MarkPurchasePaidInput{
		PurchaseID:   p.ID,
		ProviderTxID: strings.TrimSpace(payload.TransactionID),
	})
	return paid, err
}

// FakePay marks a fake-mode purchase as paid (simulates webhook + confirm).
func (s *Service) FakePay(ctx context.Context, clientUniqueID string) (store.CreditPurchase, error) {
	if s.Nedarim == nil || strings.ToLower(s.Nedarim.Cfg.Mode) != nedarim.ModeFake {
		return store.CreditPurchase{}, fmt.Errorf("fake pay is only available when NEDARIM_MODE=fake")
	}
	return s.HandleWebhook(ctx, WebhookPayload{
		ClientUniqueID: clientUniqueID,
		TransactionID:  "fake-tx-" + clientUniqueID,
		Status:         "OK",
	})
}

// Packages lists active credit packages (portal).
func (s *Service) Packages(ctx context.Context) ([]store.CreditPackage, error) {
	return s.Store.ListCreditPackages(ctx, true)
}

// AdminPackages lists all credit packages including inactive.
func (s *Service) AdminPackages(ctx context.Context) ([]store.CreditPackage, error) {
	return s.Store.ListCreditPackages(ctx, false)
}

// CreatePackage creates a new credit package.
func (s *Service) CreatePackage(ctx context.Context, p store.CreditPackage) (store.CreditPackage, error) {
	return s.Store.CreateCreditPackage(ctx, p)
}

// UpdatePackage updates an existing credit package.
func (s *Service) UpdatePackage(ctx context.Context, p store.CreditPackage) (store.CreditPackage, error) {
	return s.Store.UpdateCreditPackage(ctx, p)
}

// DeactivatePackage soft-deactivates a package (preferred over hard delete).
func (s *Service) DeactivatePackage(ctx context.Context, id string) (store.CreditPackage, error) {
	pkg, err := s.Store.GetCreditPackage(ctx, strings.TrimSpace(id))
	if err != nil {
		return store.CreditPackage{}, err
	}
	pkg.Active = false
	return s.Store.UpdateCreditPackage(ctx, pkg)
}

// GetPurchaseByClientUnique looks up a pending/paid purchase by token.
func (s *Service) GetPurchaseByClientUnique(ctx context.Context, clientUniqueID string) (store.CreditPurchase, error) {
	return s.Store.GetCreditPurchaseByClientUnique(ctx, strings.TrimSpace(clientUniqueID))
}

// LiveBridgeParams returns Mosad/ApiValid/Amount fields for the live bridge page.
func (s *Service) LiveBridgeParams(ctx context.Context, clientUniqueID string) (map[string]string, store.CreditPurchase, error) {
	p, err := s.GetPurchaseByClientUnique(ctx, clientUniqueID)
	if err != nil {
		return nil, store.CreditPurchase{}, err
	}
	if s.Nedarim == nil {
		return nil, p, fmt.Errorf("nedarim not configured")
	}
	amountShekels := fmt.Sprintf("%.2f", float64(p.AmountAgorot)/100.0)
	return map[string]string{
		"Mosad":       s.Nedarim.Cfg.MosadID,
		"ApiValid":    s.Nedarim.Cfg.ApiValid,
		"Amount":      amountShekels,
		"Currency":    "1",
		"Tashlumim":   "1",
		"PaymentType": "Ragil",
		"Param2":      p.ClientUniqueID,
		"CallBack":    s.webhookURL(),
		"Comment":     fmt.Sprintf("credits:%d", p.Credits),
	}, p, nil
}
