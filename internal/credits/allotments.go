package credits

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/dwdmsh/school-mdm/internal/store"
)

// AllotmentRunResult summarizes one RunAllotments pass.
type AllotmentRunResult struct {
	RulesProcessed int `json:"rules_processed"`
	GrantsApplied  int `json:"grants_applied"`
	GrantsSkipped  int `json:"grants_skipped"`
	Errors         int `json:"errors"`
}

// PeriodKey returns the idempotency key for interval at now (UTC).
// daily: 2006-01-02; weekly: 2006-W02 (ISO); monthly: 2006-01.
func PeriodKey(interval store.AllotmentInterval, now time.Time) string {
	now = now.UTC()
	switch interval {
	case store.IntervalWeekly:
		y, w := now.ISOWeek()
		return fmt.Sprintf("%04d-W%02d", y, w)
	case store.IntervalMonthly:
		return now.Format("2006-01")
	default:
		return now.Format("2006-01-02")
	}
}

// NextPeriodStart returns when the next period begins (UTC).
func NextPeriodStart(interval store.AllotmentInterval, now time.Time) time.Time {
	now = now.UTC()
	switch interval {
	case store.IntervalWeekly:
		// ISO weeks start Monday; next period is next Monday 00:00 UTC.
		daysSinceMonday := (int(now.Weekday()) + 6) % 7
		thisMonday := time.Date(now.Year(), now.Month(), now.Day()-daysSinceMonday, 0, 0, 0, 0, time.UTC)
		return thisMonday.AddDate(0, 0, 7)
	case store.IntervalMonthly:
		return time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, time.UTC)
	default:
		return time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC)
	}
}

// ListAllotmentRules returns all allotment rules.
func (s *Service) ListAllotmentRules(ctx context.Context) ([]store.CreditAllotmentRule, error) {
	return s.Store.ListAllotmentRules(ctx)
}

// GetAllotmentRule returns one rule by id.
func (s *Service) GetAllotmentRule(ctx context.Context, id string) (store.CreditAllotmentRule, error) {
	return s.Store.GetAllotmentRule(ctx, strings.TrimSpace(id))
}

// CreateAllotmentRule creates a recurring allotment rule.
func (s *Service) CreateAllotmentRule(ctx context.Context, rule store.CreditAllotmentRule) (store.CreditAllotmentRule, error) {
	rule.Name = strings.TrimSpace(rule.Name)
	rule.Note = strings.TrimSpace(rule.Note)
	rule.TargetID = strings.TrimSpace(rule.TargetID)
	return s.Store.CreateAllotmentRule(ctx, rule)
}

// UpdateAllotmentRule updates an existing rule.
func (s *Service) UpdateAllotmentRule(ctx context.Context, rule store.CreditAllotmentRule) (store.CreditAllotmentRule, error) {
	rule.ID = strings.TrimSpace(rule.ID)
	rule.Name = strings.TrimSpace(rule.Name)
	rule.Note = strings.TrimSpace(rule.Note)
	rule.TargetID = strings.TrimSpace(rule.TargetID)
	return s.Store.UpdateAllotmentRule(ctx, rule)
}

// DeleteAllotmentRule removes a rule and claws back its unused allotment.
func (s *Service) DeleteAllotmentRule(ctx context.Context, id string) error {
	return s.Store.DeleteAllotmentRule(ctx, strings.TrimSpace(id))
}

// ResolveAllotmentTargets returns enrollment IDs in scope for a rule.
func (s *Service) ResolveAllotmentTargets(ctx context.Context, rule store.CreditAllotmentRule) ([]string, error) {
	switch rule.TargetType {
	case store.AllotmentIndividual:
		id := strings.TrimSpace(rule.TargetID)
		if id == "" {
			return nil, fmt.Errorf("target_id is required")
		}
		return []string{id}, nil
	case store.AllotmentGroup:
		return s.Store.ListEnrollmentIDsForGroup(ctx, strings.TrimSpace(rule.TargetID))
	case store.AllotmentEveryone:
		ids, err := s.Store.ListAllEnrollmentIDs(ctx)
		if err != nil {
			return nil, err
		}
		// Also include devices that only appear in credit balances.
		bals, err := s.Store.ListCreditBalances(ctx)
		if err != nil {
			return ids, nil
		}
		seen := map[string]struct{}{}
		for _, id := range ids {
			seen[id] = struct{}{}
		}
		for _, b := range bals {
			if _, ok := seen[b.EnrollmentID]; !ok {
				ids = append(ids, b.EnrollmentID)
			}
		}
		return ids, nil
	default:
		return nil, fmt.Errorf("unknown target_type %s", rule.TargetType)
	}
}

// RunAllotments applies all enabled rules for the current period (idempotent).
func (s *Service) RunAllotments(ctx context.Context, now time.Time) (AllotmentRunResult, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	rules, err := s.Store.ListAllotmentRules(ctx)
	if err != nil {
		return AllotmentRunResult{}, err
	}
	var result AllotmentRunResult
	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		result.RulesProcessed++
		period := PeriodKey(rule.Interval, now)
		targets, err := s.ResolveAllotmentTargets(ctx, rule)
		if err != nil {
			result.Errors++
			continue
		}
		for _, enrollmentID := range targets {
			res, err := s.Store.ApplyAllotmentPeriod(ctx, store.ApplyAllotmentInput{
				RuleID:       rule.ID,
				EnrollmentID: enrollmentID,
				PeriodKey:    period,
				Amount:       rule.Amount,
				Note:         rule.Name,
			})
			if err != nil {
				result.Errors++
				continue
			}
			if res.Applied {
				result.GrantsApplied++
			} else {
				result.GrantsSkipped++
			}
		}
		_ = s.Store.TouchAllotmentRuleRun(ctx, rule.ID, now)
	}
	return result, nil
}

// AllotmentRuleView adds next-period hint for admin UI.
type AllotmentRuleView struct {
	store.CreditAllotmentRule
	PeriodKey      string     `json:"period_key"`
	NextPeriodAt   time.Time  `json:"next_period_at"`
}

// ListAllotmentRulesView returns rules with period hints for now.
func (s *Service) ListAllotmentRulesView(ctx context.Context, now time.Time) ([]AllotmentRuleView, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	rules, err := s.ListAllotmentRules(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]AllotmentRuleView, 0, len(rules))
	for _, r := range rules {
		out = append(out, AllotmentRuleView{
			CreditAllotmentRule: r,
			PeriodKey:           PeriodKey(r.Interval, now),
			NextPeriodAt:        NextPeriodStart(r.Interval, now),
		})
	}
	return out, nil
}

// StartAllotmentTicker runs RunAllotments on an interval until ctx is cancelled.
func (s *Service) StartAllotmentTicker(ctx context.Context, every time.Duration, log *slog.Logger) {
	if every <= 0 {
		every = 2 * time.Minute
	}
	if log == nil {
		log = slog.Default()
	}
	run := func() {
		res, err := s.RunAllotments(ctx, time.Now().UTC())
		if err != nil {
			log.Warn("allotments run failed", "err", err)
			return
		}
		if res.GrantsApplied > 0 || res.Errors > 0 {
			log.Info("allotments run",
				"rules", res.RulesProcessed,
				"applied", res.GrantsApplied,
				"skipped", res.GrantsSkipped,
				"errors", res.Errors,
			)
		}
	}
	run()
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}
