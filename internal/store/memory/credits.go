package memory

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/dwdmsh/school-mdm/internal/store"
)

func ledgerIdempotencyKey(reason store.LedgerReason, refType, refID string) string {
	return string(reason) + "|" + refType + "|" + refID
}

func (s *Store) EnsureCreditBalance(_ context.Context, enrollmentID string) (store.DeviceCredits, error) {
	enrollmentID = strings.TrimSpace(enrollmentID)
	if enrollmentID == "" {
		return store.DeviceCredits{}, fmt.Errorf("enrollment_id is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if bal, ok := s.credits[enrollmentID]; ok {
		return bal, nil
	}
	bal := store.DeviceCredits{EnrollmentID: enrollmentID, Balance: 0, AllotmentBalance: 0, UpdatedAt: time.Now().UTC()}
	s.credits[enrollmentID] = bal
	return bal, nil
}

func (s *Store) GetCreditBalance(_ context.Context, enrollmentID string) (store.DeviceCredits, error) {
	enrollmentID = strings.TrimSpace(enrollmentID)
	s.mu.Lock()
	defer s.mu.Unlock()
	if bal, ok := s.credits[enrollmentID]; ok {
		return bal, nil
	}
	return store.DeviceCredits{EnrollmentID: enrollmentID, Balance: 0, AllotmentBalance: 0, UpdatedAt: time.Now().UTC()}, nil
}

func (s *Store) AdjustCredits(_ context.Context, in store.AdjustCreditsInput) (store.AdjustCreditsResult, error) {
	enrollmentID := strings.TrimSpace(in.EnrollmentID)
	if enrollmentID == "" {
		return store.AdjustCreditsResult{}, fmt.Errorf("enrollment_id is required")
	}
	if in.Delta == 0 {
		return store.AdjustCreditsResult{}, fmt.Errorf("delta must not be zero")
	}
	if in.Reason == "" {
		return store.AdjustCreditsResult{}, fmt.Errorf("reason is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	refType := strings.TrimSpace(in.RefType)
	refID := strings.TrimSpace(in.RefID)
	if refType != "" && refID != "" {
		key := ledgerIdempotencyKey(in.Reason, refType, refID)
		if _, exists := s.ledgerKey[key]; exists {
			bal := s.credits[enrollmentID]
			return store.AdjustCreditsResult{
				Balance:          bal.Balance,
				AllotmentBalance: bal.AllotmentBalance,
				Applied:          false,
			}, nil
		}
	}

	bal, ok := s.credits[enrollmentID]
	if !ok {
		bal = store.DeviceCredits{EnrollmentID: enrollmentID}
	}

	now := time.Now().UTC()
	perm, allot := bal.Balance, bal.AllotmentBalance

	switch {
	case in.Reason == store.LedgerSpend && in.Delta < 0:
		// Spend: allotment bucket first, then permanent.
		need := -in.Delta
		if allot+perm < need {
			return store.AdjustCreditsResult{}, store.ErrInsufficientCredits
		}
		fromAllot := need
		if fromAllot > allot {
			fromAllot = allot
		}
		fromPerm := need - fromAllot
		allot -= fromAllot
		perm -= fromPerm
		if fromAllot > 0 {
			s.consumeAllotmentGrantsLocked(enrollmentID, fromAllot)
		}
	case in.Delta < 0:
		// Admin adjust / other deductions touch permanent only.
		if perm+in.Delta < 0 {
			return store.AdjustCreditsResult{}, store.ErrInsufficientCredits
		}
		perm += in.Delta
	default:
		// Purchases, gifts, refunds credit the permanent bucket.
		perm += in.Delta
	}

	bal.Balance = perm
	bal.AllotmentBalance = allot
	bal.UpdatedAt = now
	s.credits[enrollmentID] = bal

	entry := store.CreditLedgerEntry{
		ID:           uuid.NewString(),
		EnrollmentID: enrollmentID,
		Delta:        in.Delta,
		BalanceAfter: bal.Available(),
		Reason:       in.Reason,
		RefType:      refType,
		RefID:        refID,
		Note:         strings.TrimSpace(in.Note),
		CreatedAt:    now,
	}
	s.ledger = append(s.ledger, entry)
	if refType != "" && refID != "" {
		s.ledgerKey[ledgerIdempotencyKey(in.Reason, refType, refID)] = struct{}{}
	}
	return store.AdjustCreditsResult{
		Balance:          perm,
		AllotmentBalance: allot,
		Entry:            entry,
		Applied:          true,
	}, nil
}

// consumeAllotmentGrantsLocked reduces remaining on oldest grants first (caller holds lock).
func (s *Store) consumeAllotmentGrantsLocked(enrollmentID string, amount int) {
	if amount <= 0 {
		return
	}
	grants := make([]store.CreditAllotmentGrant, 0)
	for _, g := range s.allotmentGrants {
		if g.EnrollmentID == enrollmentID && g.Remaining > 0 {
			grants = append(grants, g)
		}
	}
	sort.Slice(grants, func(i, j int) bool {
		if grants[i].GrantedAt.Equal(grants[j].GrantedAt) {
			return grants[i].ID < grants[j].ID
		}
		return grants[i].GrantedAt.Before(grants[j].GrantedAt)
	})
	left := amount
	for i := range grants {
		if left <= 0 {
			break
		}
		take := grants[i].Remaining
		if take > left {
			take = left
		}
		grants[i].Remaining -= take
		left -= take
		s.allotmentGrants[grants[i].ID] = grants[i]
	}
}

func (s *Store) ListCreditLedger(_ context.Context, enrollmentID string, limit int) ([]store.CreditLedgerEntry, error) {
	enrollmentID = strings.TrimSpace(enrollmentID)
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]store.CreditLedgerEntry, 0)
	for i := len(s.ledger) - 1; i >= 0; i-- {
		e := s.ledger[i]
		if e.EnrollmentID != enrollmentID {
			continue
		}
		out = append(out, e)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (s *Store) ListCreditBalances(context.Context) ([]store.DeviceCredits, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]store.DeviceCredits, 0, len(s.credits))
	for _, bal := range s.credits {
		out = append(out, bal)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].EnrollmentID < out[j].EnrollmentID
	})
	return out, nil
}

func (s *Store) ListCreditPackages(_ context.Context, activeOnly bool) ([]store.CreditPackage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]store.CreditPackage, 0, len(s.packages))
	for _, p := range s.packages {
		if activeOnly && !p.Active {
			continue
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].SortOrder == out[j].SortOrder {
			return out[i].Credits < out[j].Credits
		}
		return out[i].SortOrder < out[j].SortOrder
	})
	return out, nil
}

func (s *Store) GetCreditPackage(_ context.Context, id string) (store.CreditPackage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.packages[id]
	if !ok {
		return store.CreditPackage{}, fmt.Errorf("credit package %s: %w", id, store.ErrNotFound)
	}
	return p, nil
}

func (s *Store) CreateCreditPackage(_ context.Context, p store.CreditPackage) (store.CreditPackage, error) {
	p.NameHe = strings.TrimSpace(p.NameHe)
	if p.NameHe == "" {
		return store.CreditPackage{}, fmt.Errorf("name_he is required")
	}
	if p.Credits < 1 {
		return store.CreditPackage{}, fmt.Errorf("credits must be >= 1")
	}
	if p.PriceAgorot < 1 {
		return store.CreditPackage{}, fmt.Errorf("price_agorot must be >= 1")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if p.ID == "" {
		p.ID = uuid.NewString()
	}
	if _, exists := s.packages[p.ID]; exists {
		return store.CreditPackage{}, fmt.Errorf("credit package %s already exists", p.ID)
	}
	s.packages[p.ID] = p
	return p, nil
}

func (s *Store) UpdateCreditPackage(_ context.Context, p store.CreditPackage) (store.CreditPackage, error) {
	p.ID = strings.TrimSpace(p.ID)
	p.NameHe = strings.TrimSpace(p.NameHe)
	if p.ID == "" {
		return store.CreditPackage{}, fmt.Errorf("id is required")
	}
	if p.NameHe == "" {
		return store.CreditPackage{}, fmt.Errorf("name_he is required")
	}
	if p.Credits < 1 {
		return store.CreditPackage{}, fmt.Errorf("credits must be >= 1")
	}
	if p.PriceAgorot < 1 {
		return store.CreditPackage{}, fmt.Errorf("price_agorot must be >= 1")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.packages[p.ID]; !ok {
		return store.CreditPackage{}, fmt.Errorf("credit package %s: %w", p.ID, store.ErrNotFound)
	}
	s.packages[p.ID] = p
	return p, nil
}

func (s *Store) GetCreditSettings(context.Context) (store.CreditSettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.settings == nil {
		return store.CreditSettings{}, fmt.Errorf("credit settings: %w", store.ErrNotFound)
	}
	return *s.settings, nil
}

func (s *Store) UpsertCreditSettings(_ context.Context, settings store.CreditSettings) (store.CreditSettings, error) {
	if settings.AccessRequestCost < 1 {
		return store.CreditSettings{}, fmt.Errorf("access_request_cost must be >= 1")
	}
	now := time.Now().UTC()
	settings.UpdatedAt = now
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := settings
	s.settings = &cp
	return settings, nil
}

func (s *Store) CreateCreditPurchase(_ context.Context, p store.CreditPurchase) (store.CreditPurchase, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if p.ID == "" {
		p.ID = uuid.NewString()
	}
	if p.ClientUniqueID == "" {
		p.ClientUniqueID = uuid.NewString()
	}
	if _, exists := s.clientUID[p.ClientUniqueID]; exists {
		return store.CreditPurchase{}, fmt.Errorf("client_unique_id already exists")
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = time.Now().UTC()
	}
	if p.Status == "" {
		p.Status = store.PurchasePending
	}
	s.purchases[p.ID] = p
	s.clientUID[p.ClientUniqueID] = p.ID
	return p, nil
}

func (s *Store) GetCreditPurchase(_ context.Context, id string) (store.CreditPurchase, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.purchases[id]
	if !ok {
		return store.CreditPurchase{}, fmt.Errorf("credit purchase %s: %w", id, store.ErrNotFound)
	}
	return p, nil
}

func (s *Store) GetCreditPurchaseByClientUnique(_ context.Context, clientUniqueID string) (store.CreditPurchase, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id, ok := s.clientUID[clientUniqueID]
	if !ok {
		return store.CreditPurchase{}, fmt.Errorf("credit purchase %s: %w", clientUniqueID, store.ErrNotFound)
	}
	return s.purchases[id], nil
}

func (s *Store) MarkPurchasePaid(_ context.Context, in store.MarkPurchasePaidInput) (store.CreditPurchase, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, ok := s.purchases[in.PurchaseID]
	if !ok {
		return store.CreditPurchase{}, false, fmt.Errorf("credit purchase %s: %w", in.PurchaseID, store.ErrNotFound)
	}
	if p.Status == store.PurchasePaid {
		return p, false, nil
	}
	if p.Status != store.PurchasePending {
		return store.CreditPurchase{}, false, fmt.Errorf("purchase %s is %s", p.ID, p.Status)
	}

	refType := "purchase"
	refID := p.ID
	key := ledgerIdempotencyKey(store.LedgerPurchase, refType, refID)
	if _, exists := s.ledgerKey[key]; exists {
		now := time.Now().UTC()
		p.Status = store.PurchasePaid
		p.PaidAt = &now
		if in.ProviderTxID != "" {
			p.ProviderTxID = in.ProviderTxID
		}
		s.purchases[p.ID] = p
		return p, false, nil
	}

	bal, ok := s.credits[p.EnrollmentID]
	if !ok {
		bal = store.DeviceCredits{EnrollmentID: p.EnrollmentID}
	}
	now := time.Now().UTC()
	bal.Balance += p.Credits
	bal.UpdatedAt = now
	s.credits[p.EnrollmentID] = bal

	entry := store.CreditLedgerEntry{
		ID:           uuid.NewString(),
		EnrollmentID: p.EnrollmentID,
		Delta:        p.Credits,
		BalanceAfter: bal.Available(),
		Reason:       store.LedgerPurchase,
		RefType:      refType,
		RefID:        refID,
		CreatedAt:    now,
	}
	s.ledger = append(s.ledger, entry)
	s.ledgerKey[key] = struct{}{}

	p.Status = store.PurchasePaid
	p.PaidAt = &now
	if in.ProviderTxID != "" {
		p.ProviderTxID = in.ProviderTxID
	}
	s.purchases[p.ID] = p
	return p, true, nil
}
