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

func (s *Store) ListAllotmentRules(context.Context) ([]store.CreditAllotmentRule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]store.CreditAllotmentRule, 0, len(s.allotmentRules))
	for _, r := range s.allotmentRules {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].ID < out[j].ID
		}
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out, nil
}

func (s *Store) GetAllotmentRule(_ context.Context, id string) (store.CreditAllotmentRule, error) {
	id = strings.TrimSpace(id)
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.allotmentRules[id]
	if !ok {
		return store.CreditAllotmentRule{}, fmt.Errorf("allotment rule %s: %w", id, store.ErrNotFound)
	}
	return r, nil
}

func (s *Store) CreateAllotmentRule(_ context.Context, rule store.CreditAllotmentRule) (store.CreditAllotmentRule, error) {
	if err := validateAllotmentRule(rule); err != nil {
		return store.CreditAllotmentRule{}, err
	}
	now := time.Now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	if rule.ID == "" {
		rule.ID = uuid.NewString()
	}
	if _, exists := s.allotmentRules[rule.ID]; exists {
		return store.CreditAllotmentRule{}, fmt.Errorf("allotment rule %s already exists", rule.ID)
	}
	rule.CreatedAt = now
	rule.UpdatedAt = now
	s.allotmentRules[rule.ID] = rule
	return rule, nil
}

func (s *Store) UpdateAllotmentRule(_ context.Context, rule store.CreditAllotmentRule) (store.CreditAllotmentRule, error) {
	rule.ID = strings.TrimSpace(rule.ID)
	if rule.ID == "" {
		return store.CreditAllotmentRule{}, fmt.Errorf("id is required")
	}
	if err := validateAllotmentRule(rule); err != nil {
		return store.CreditAllotmentRule{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	existing, ok := s.allotmentRules[rule.ID]
	if !ok {
		return store.CreditAllotmentRule{}, fmt.Errorf("allotment rule %s: %w", rule.ID, store.ErrNotFound)
	}
	rule.CreatedAt = existing.CreatedAt
	rule.LastRunAt = existing.LastRunAt
	rule.UpdatedAt = time.Now().UTC()
	s.allotmentRules[rule.ID] = rule
	return rule, nil
}

func (s *Store) DeleteAllotmentRule(_ context.Context, id string) error {
	id = strings.TrimSpace(id)
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.allotmentRules[id]; !ok {
		return fmt.Errorf("allotment rule %s: %w", id, store.ErrNotFound)
	}
	// Expire remaining allotment attributed to this rule before deleting.
	for _, g := range s.allotmentGrants {
		if g.RuleID != id {
			continue
		}
		if g.Remaining > 0 {
			bal := s.credits[g.EnrollmentID]
			claw := g.Remaining
			if claw > bal.AllotmentBalance {
				claw = bal.AllotmentBalance
			}
			if claw > 0 {
				bal.AllotmentBalance -= claw
				bal.UpdatedAt = time.Now().UTC()
				s.credits[g.EnrollmentID] = bal
				s.ledger = append(s.ledger, store.CreditLedgerEntry{
					ID:           uuid.NewString(),
					EnrollmentID: g.EnrollmentID,
					Delta:        -claw,
					BalanceAfter: bal.Available(),
					Reason:       store.LedgerAllotmentExpire,
					RefType:      "allotment_rule",
					RefID:        id,
					Note:         "rule deleted",
					CreatedAt:    time.Now().UTC(),
				})
			}
		}
		delete(s.allotmentGrants, g.ID)
	}
	delete(s.allotmentRules, id)
	return nil
}

func (s *Store) TouchAllotmentRuleRun(_ context.Context, id string, at time.Time) error {
	id = strings.TrimSpace(id)
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.allotmentRules[id]
	if !ok {
		return fmt.Errorf("allotment rule %s: %w", id, store.ErrNotFound)
	}
	if at.IsZero() {
		at = time.Now().UTC()
	}
	r.LastRunAt = &at
	r.UpdatedAt = at
	s.allotmentRules[id] = r
	return nil
}

func (s *Store) ApplyAllotmentPeriod(_ context.Context, in store.ApplyAllotmentInput) (store.ApplyAllotmentResult, error) {
	ruleID := strings.TrimSpace(in.RuleID)
	enrollmentID := strings.TrimSpace(in.EnrollmentID)
	periodKey := strings.TrimSpace(in.PeriodKey)
	if ruleID == "" || enrollmentID == "" || periodKey == "" {
		return store.ApplyAllotmentResult{}, fmt.Errorf("rule_id, enrollment_id, and period_key are required")
	}
	if in.Amount < 1 {
		return store.ApplyAllotmentResult{}, fmt.Errorf("amount must be >= 1")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.allotmentRules[ruleID]; !ok {
		return store.ApplyAllotmentResult{}, fmt.Errorf("allotment rule %s: %w", ruleID, store.ErrNotFound)
	}

	// Idempotent: already granted this period.
	for _, g := range s.allotmentGrants {
		if g.RuleID == ruleID && g.EnrollmentID == enrollmentID && g.PeriodKey == periodKey {
			bal := s.credits[enrollmentID]
			return store.ApplyAllotmentResult{Applied: false, Credits: bal}, nil
		}
	}

	now := time.Now().UTC()
	bal, ok := s.credits[enrollmentID]
	if !ok {
		bal = store.DeviceCredits{EnrollmentID: enrollmentID}
	}

	// Claw back unused allotment from prior periods of this rule (non-stacking).
	expired := 0
	for _, g := range s.allotmentGrants {
		if g.RuleID != ruleID || g.EnrollmentID != enrollmentID || g.Remaining <= 0 {
			continue
		}
		claw := g.Remaining
		if claw > bal.AllotmentBalance {
			claw = bal.AllotmentBalance
		}
		if claw > 0 {
			bal.AllotmentBalance -= claw
			expired += claw
			s.ledger = append(s.ledger, store.CreditLedgerEntry{
				ID:           uuid.NewString(),
				EnrollmentID: enrollmentID,
				Delta:        -claw,
				BalanceAfter: bal.Available(),
				Reason:       store.LedgerAllotmentExpire,
				RefType:      "allotment_period",
				RefID:        ruleID + "|" + g.PeriodKey,
				Note:         strings.TrimSpace(in.Note),
				CreatedAt:    now,
			})
		}
		g.Remaining = 0
		s.allotmentGrants[g.ID] = g
	}

	bal.AllotmentBalance += in.Amount
	bal.UpdatedAt = now
	s.credits[enrollmentID] = bal

	grant := store.CreditAllotmentGrant{
		ID:           uuid.NewString(),
		RuleID:       ruleID,
		EnrollmentID: enrollmentID,
		PeriodKey:    periodKey,
		Amount:       in.Amount,
		Remaining:    in.Amount,
		GrantedAt:    now,
	}
	s.allotmentGrants[grant.ID] = grant

	entry := store.CreditLedgerEntry{
		ID:           uuid.NewString(),
		EnrollmentID: enrollmentID,
		Delta:        in.Amount,
		BalanceAfter: bal.Available(),
		Reason:       store.LedgerAllotment,
		RefType:      "allotment_period",
		RefID:        ruleID + "|" + periodKey + "|" + enrollmentID,
		Note:         strings.TrimSpace(in.Note),
		CreatedAt:    now,
	}
	s.ledger = append(s.ledger, entry)
	s.ledgerKey[ledgerIdempotencyKey(store.LedgerAllotment, entry.RefType, entry.RefID)] = struct{}{}

	return store.ApplyAllotmentResult{
		Applied: true,
		Credits: bal,
		Expired: expired,
		Granted: in.Amount,
		Entry:   &entry,
	}, nil
}

func validateAllotmentRule(rule store.CreditAllotmentRule) error {
	if rule.Amount < 1 {
		return fmt.Errorf("amount must be >= 1")
	}
	switch rule.Interval {
	case store.IntervalDaily, store.IntervalWeekly, store.IntervalMonthly:
	default:
		return fmt.Errorf("interval must be daily, weekly, or monthly")
	}
	switch rule.TargetType {
	case store.AllotmentEveryone:
		// target_id unused
	case store.AllotmentGroup, store.AllotmentIndividual:
		if strings.TrimSpace(rule.TargetID) == "" {
			return fmt.Errorf("target_id is required for %s", rule.TargetType)
		}
	default:
		return fmt.Errorf("target_type must be everyone, group, or individual")
	}
	return nil
}
