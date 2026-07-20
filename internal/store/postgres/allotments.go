package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/dwdmsh/school-mdm/internal/store"
)

func (s *Store) ListAllotmentRules(ctx context.Context) ([]store.CreditAllotmentRule, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, name, note, amount, interval, target_type, target_id, enabled,
		       last_run_at, created_at, updated_at
		FROM credit_allotment_rules
		ORDER BY created_at ASC, id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]store.CreditAllotmentRule, 0)
	for rows.Next() {
		r, err := scanAllotmentRule(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) GetAllotmentRule(ctx context.Context, id string) (store.CreditAllotmentRule, error) {
	id = strings.TrimSpace(id)
	row := s.pool.QueryRow(ctx, `
		SELECT id, name, note, amount, interval, target_type, target_id, enabled,
		       last_run_at, created_at, updated_at
		FROM credit_allotment_rules WHERE id=$1
	`, id)
	r, err := scanAllotmentRule(row)
	if err == pgx.ErrNoRows {
		return store.CreditAllotmentRule{}, fmt.Errorf("allotment rule %s: %w", id, store.ErrNotFound)
	}
	return r, err
}

func scanAllotmentRule(row scannable) (store.CreditAllotmentRule, error) {
	var r store.CreditAllotmentRule
	var interval, targetType string
	err := row.Scan(
		&r.ID, &r.Name, &r.Note, &r.Amount, &interval, &targetType, &r.TargetID, &r.Enabled,
		&r.LastRunAt, &r.CreatedAt, &r.UpdatedAt,
	)
	if err != nil {
		return store.CreditAllotmentRule{}, err
	}
	r.Interval = store.AllotmentInterval(interval)
	r.TargetType = store.AllotmentTargetType(targetType)
	return r, nil
}

func (s *Store) CreateAllotmentRule(ctx context.Context, rule store.CreditAllotmentRule) (store.CreditAllotmentRule, error) {
	if err := validateAllotmentRule(rule); err != nil {
		return store.CreditAllotmentRule{}, err
	}
	if rule.ID == "" {
		rule.ID = uuid.NewString()
	}
	rule.Name = strings.TrimSpace(rule.Name)
	rule.Note = strings.TrimSpace(rule.Note)
	rule.TargetID = strings.TrimSpace(rule.TargetID)
	err := s.pool.QueryRow(ctx, `
		INSERT INTO credit_allotment_rules (
			id, name, note, amount, interval, target_type, target_id, enabled, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,now(),now())
		RETURNING created_at, updated_at
	`, rule.ID, rule.Name, rule.Note, rule.Amount, string(rule.Interval), string(rule.TargetType), rule.TargetID, rule.Enabled,
	).Scan(&rule.CreatedAt, &rule.UpdatedAt)
	if err != nil {
		return store.CreditAllotmentRule{}, err
	}
	return rule, nil
}

func (s *Store) UpdateAllotmentRule(ctx context.Context, rule store.CreditAllotmentRule) (store.CreditAllotmentRule, error) {
	rule.ID = strings.TrimSpace(rule.ID)
	if rule.ID == "" {
		return store.CreditAllotmentRule{}, fmt.Errorf("id is required")
	}
	if err := validateAllotmentRule(rule); err != nil {
		return store.CreditAllotmentRule{}, err
	}
	rule.Name = strings.TrimSpace(rule.Name)
	rule.Note = strings.TrimSpace(rule.Note)
	rule.TargetID = strings.TrimSpace(rule.TargetID)
	err := s.pool.QueryRow(ctx, `
		UPDATE credit_allotment_rules
		SET name=$2, note=$3, amount=$4, interval=$5, target_type=$6, target_id=$7, enabled=$8, updated_at=now()
		WHERE id=$1
		RETURNING last_run_at, created_at, updated_at
	`, rule.ID, rule.Name, rule.Note, rule.Amount, string(rule.Interval), string(rule.TargetType), rule.TargetID, rule.Enabled,
	).Scan(&rule.LastRunAt, &rule.CreatedAt, &rule.UpdatedAt)
	if err == pgx.ErrNoRows {
		return store.CreditAllotmentRule{}, fmt.Errorf("allotment rule %s: %w", rule.ID, store.ErrNotFound)
	}
	if err != nil {
		return store.CreditAllotmentRule{}, err
	}
	return rule, nil
}

func (s *Store) DeleteAllotmentRule(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var exists int
	err = tx.QueryRow(ctx, `SELECT 1 FROM credit_allotment_rules WHERE id=$1 FOR UPDATE`, id).Scan(&exists)
	if err == pgx.ErrNoRows {
		return fmt.Errorf("allotment rule %s: %w", id, store.ErrNotFound)
	}
	if err != nil {
		return err
	}

	rows, err := tx.Query(ctx, `
		SELECT id, enrollment_id, remaining FROM credit_allotment_grants
		WHERE rule_id=$1 AND remaining > 0
		FOR UPDATE
	`, id)
	if err != nil {
		return err
	}
	type claw struct {
		grantID, enrollmentID string
		remaining             int
	}
	var claws []claw
	for rows.Next() {
		var c claw
		if err := rows.Scan(&c.grantID, &c.enrollmentID, &c.remaining); err != nil {
			rows.Close()
			return err
		}
		claws = append(claws, c)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	now := time.Now().UTC()
	for _, c := range claws {
		var perm, allot int
		if err := tx.QueryRow(ctx, `
			SELECT balance, allotment_balance FROM device_credits WHERE enrollment_id=$1 FOR UPDATE
		`, c.enrollmentID).Scan(&perm, &allot); err != nil && err != pgx.ErrNoRows {
			return err
		}
		clawAmt := c.remaining
		if clawAmt > allot {
			clawAmt = allot
		}
		if clawAmt > 0 {
			allot -= clawAmt
			if _, err := tx.Exec(ctx, `
				UPDATE device_credits SET allotment_balance=$2, updated_at=now() WHERE enrollment_id=$1
			`, c.enrollmentID, allot); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO credit_ledger (id, enrollment_id, delta, balance_after, reason, ref_type, ref_id, note, created_at)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
			`, uuid.NewString(), c.enrollmentID, -clawAmt, perm+allot, string(store.LedgerAllotmentExpire),
				"allotment_rule", id, "rule deleted", now); err != nil {
				return err
			}
		}
	}

	if _, err := tx.Exec(ctx, `DELETE FROM credit_allotment_rules WHERE id=$1`, id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) TouchAllotmentRuleRun(ctx context.Context, id string, at time.Time) error {
	id = strings.TrimSpace(id)
	if at.IsZero() {
		at = time.Now().UTC()
	}
	ct, err := s.pool.Exec(ctx, `
		UPDATE credit_allotment_rules SET last_run_at=$2, updated_at=$2 WHERE id=$1
	`, id, at)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("allotment rule %s: %w", id, store.ErrNotFound)
	}
	return nil
}

func (s *Store) ApplyAllotmentPeriod(ctx context.Context, in store.ApplyAllotmentInput) (store.ApplyAllotmentResult, error) {
	ruleID := strings.TrimSpace(in.RuleID)
	enrollmentID := strings.TrimSpace(in.EnrollmentID)
	periodKey := strings.TrimSpace(in.PeriodKey)
	if ruleID == "" || enrollmentID == "" || periodKey == "" {
		return store.ApplyAllotmentResult{}, fmt.Errorf("rule_id, enrollment_id, and period_key are required")
	}
	if in.Amount < 1 {
		return store.ApplyAllotmentResult{}, fmt.Errorf("amount must be >= 1")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return store.ApplyAllotmentResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var exists int
	err = tx.QueryRow(ctx, `SELECT 1 FROM credit_allotment_rules WHERE id=$1`, ruleID).Scan(&exists)
	if err == pgx.ErrNoRows {
		return store.ApplyAllotmentResult{}, fmt.Errorf("allotment rule %s: %w", ruleID, store.ErrNotFound)
	}
	if err != nil {
		return store.ApplyAllotmentResult{}, err
	}

	// Idempotent check.
	var existingID string
	err = tx.QueryRow(ctx, `
		SELECT id FROM credit_allotment_grants
		WHERE rule_id=$1 AND enrollment_id=$2 AND period_key=$3
	`, ruleID, enrollmentID, periodKey).Scan(&existingID)
	if err == nil {
		bal, _ := s.getCreditsTx(ctx, tx, enrollmentID)
		return store.ApplyAllotmentResult{Applied: false, Credits: bal}, nil
	}
	if err != pgx.ErrNoRows {
		return store.ApplyAllotmentResult{}, err
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO device_credits (enrollment_id, balance, allotment_balance, updated_at)
		VALUES ($1, 0, 0, now())
		ON CONFLICT (enrollment_id) DO NOTHING
	`, enrollmentID); err != nil {
		return store.ApplyAllotmentResult{}, err
	}

	var perm, allot int
	if err := tx.QueryRow(ctx, `
		SELECT balance, allotment_balance FROM device_credits WHERE enrollment_id=$1 FOR UPDATE
	`, enrollmentID).Scan(&perm, &allot); err != nil {
		return store.ApplyAllotmentResult{}, err
	}

	now := time.Now().UTC()
	note := strings.TrimSpace(in.Note)
	expired := 0

	rows, err := tx.Query(ctx, `
		SELECT id, period_key, remaining FROM credit_allotment_grants
		WHERE rule_id=$1 AND enrollment_id=$2 AND remaining > 0
		FOR UPDATE
	`, ruleID, enrollmentID)
	if err != nil {
		return store.ApplyAllotmentResult{}, err
	}
	type prior struct {
		id, periodKey string
		remaining     int
	}
	var priors []prior
	for rows.Next() {
		var p prior
		if err := rows.Scan(&p.id, &p.periodKey, &p.remaining); err != nil {
			rows.Close()
			return store.ApplyAllotmentResult{}, err
		}
		priors = append(priors, p)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return store.ApplyAllotmentResult{}, err
	}

	for _, p := range priors {
		claw := p.remaining
		if claw > allot {
			claw = allot
		}
		if claw > 0 {
			allot -= claw
			expired += claw
			if _, err := tx.Exec(ctx, `
				INSERT INTO credit_ledger (id, enrollment_id, delta, balance_after, reason, ref_type, ref_id, note, created_at)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
			`, uuid.NewString(), enrollmentID, -claw, perm+allot, string(store.LedgerAllotmentExpire),
				"allotment_period", ruleID+"|"+p.periodKey, note, now); err != nil {
				return store.ApplyAllotmentResult{}, err
			}
		}
		if _, err := tx.Exec(ctx, `UPDATE credit_allotment_grants SET remaining=0 WHERE id=$1`, p.id); err != nil {
			return store.ApplyAllotmentResult{}, err
		}
	}

	allot += in.Amount
	if _, err := tx.Exec(ctx, `
		UPDATE device_credits SET allotment_balance=$2, updated_at=now() WHERE enrollment_id=$1
	`, enrollmentID, allot); err != nil {
		return store.ApplyAllotmentResult{}, err
	}

	grantID := uuid.NewString()
	if _, err := tx.Exec(ctx, `
		INSERT INTO credit_allotment_grants (id, rule_id, enrollment_id, period_key, amount, remaining, granted_at)
		VALUES ($1,$2,$3,$4,$5,$5,$6)
	`, grantID, ruleID, enrollmentID, periodKey, in.Amount, now); err != nil {
		return store.ApplyAllotmentResult{}, err
	}

	entry := store.CreditLedgerEntry{
		ID:           uuid.NewString(),
		EnrollmentID: enrollmentID,
		Delta:        in.Amount,
		BalanceAfter: perm + allot,
		Reason:       store.LedgerAllotment,
		RefType:      "allotment_period",
		RefID:        ruleID + "|" + periodKey + "|" + enrollmentID,
		Note:         note,
		CreatedAt:    now,
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO credit_ledger (id, enrollment_id, delta, balance_after, reason, ref_type, ref_id, note, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
	`, entry.ID, entry.EnrollmentID, entry.Delta, entry.BalanceAfter, string(entry.Reason), entry.RefType, entry.RefID, entry.Note, entry.CreatedAt); err != nil {
		return store.ApplyAllotmentResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return store.ApplyAllotmentResult{}, err
	}
	return store.ApplyAllotmentResult{
		Applied: true,
		Credits: store.DeviceCredits{
			EnrollmentID:     enrollmentID,
			Balance:          perm,
			AllotmentBalance: allot,
			UpdatedAt:        now,
		},
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
	case store.AllotmentGroup, store.AllotmentIndividual:
		if strings.TrimSpace(rule.TargetID) == "" {
			return fmt.Errorf("target_id is required for %s", rule.TargetType)
		}
	default:
		return fmt.Errorf("target_type must be everyone, group, or individual")
	}
	return nil
}
