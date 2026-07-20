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

func (s *Store) DeleteRequest(ctx context.Context, id string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `DELETE FROM request_messages WHERE request_id=$1`, id); err != nil {
		return err
	}
	ct, err := tx.Exec(ctx, `DELETE FROM requests WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("request %s: %w", id, store.ErrNotFound)
	}
	return tx.Commit(ctx)
}

func (s *Store) EnsureCreditBalance(ctx context.Context, enrollmentID string) (store.DeviceCredits, error) {
	enrollmentID = strings.TrimSpace(enrollmentID)
	if enrollmentID == "" {
		return store.DeviceCredits{}, fmt.Errorf("enrollment_id is required")
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO device_credits (enrollment_id, balance, allotment_balance, updated_at)
		VALUES ($1, 0, 0, now())
		ON CONFLICT (enrollment_id) DO NOTHING
	`, enrollmentID)
	if err != nil {
		return store.DeviceCredits{}, err
	}
	return s.GetCreditBalance(ctx, enrollmentID)
}

func (s *Store) GetCreditBalance(ctx context.Context, enrollmentID string) (store.DeviceCredits, error) {
	enrollmentID = strings.TrimSpace(enrollmentID)
	var bal store.DeviceCredits
	err := s.pool.QueryRow(ctx, `
		SELECT enrollment_id, balance, allotment_balance, updated_at
		FROM device_credits WHERE enrollment_id=$1
	`, enrollmentID).Scan(&bal.EnrollmentID, &bal.Balance, &bal.AllotmentBalance, &bal.UpdatedAt)
	if err == pgx.ErrNoRows {
		return store.DeviceCredits{EnrollmentID: enrollmentID, UpdatedAt: time.Now().UTC()}, nil
	}
	if err != nil {
		return store.DeviceCredits{}, err
	}
	return bal, nil
}

func (s *Store) AdjustCredits(ctx context.Context, in store.AdjustCreditsInput) (store.AdjustCreditsResult, error) {
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
	refType := strings.TrimSpace(in.RefType)
	refID := strings.TrimSpace(in.RefID)

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return store.AdjustCreditsResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if refType != "" && refID != "" {
		var existingBal int
		err := tx.QueryRow(ctx, `
			SELECT balance_after FROM credit_ledger
			WHERE reason=$1 AND ref_type=$2 AND ref_id=$3
			LIMIT 1
		`, string(in.Reason), refType, refID).Scan(&existingBal)
		if err == nil {
			bal, _ := s.getCreditsTx(ctx, tx, enrollmentID)
			return store.AdjustCreditsResult{
				Balance:          bal.Balance,
				AllotmentBalance: bal.AllotmentBalance,
				Applied:          false,
			}, nil
		}
		if err != pgx.ErrNoRows {
			return store.AdjustCreditsResult{}, err
		}
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO device_credits (enrollment_id, balance, allotment_balance, updated_at)
		VALUES ($1, 0, 0, now())
		ON CONFLICT (enrollment_id) DO NOTHING
	`, enrollmentID); err != nil {
		return store.AdjustCreditsResult{}, err
	}

	var perm, allot int
	err = tx.QueryRow(ctx, `
		SELECT balance, allotment_balance FROM device_credits WHERE enrollment_id=$1 FOR UPDATE
	`, enrollmentID).Scan(&perm, &allot)
	if err != nil {
		return store.AdjustCreditsResult{}, err
	}

	switch {
	case in.Reason == store.LedgerSpend && in.Delta < 0:
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
			if err := consumeAllotmentGrantsTx(ctx, tx, enrollmentID, fromAllot); err != nil {
				return store.AdjustCreditsResult{}, err
			}
		}
	case in.Delta < 0:
		if perm+in.Delta < 0 {
			return store.AdjustCreditsResult{}, store.ErrInsufficientCredits
		}
		perm += in.Delta
	default:
		perm += in.Delta
	}

	if _, err := tx.Exec(ctx, `
		UPDATE device_credits SET balance=$2, allotment_balance=$3, updated_at=now()
		WHERE enrollment_id=$1
	`, enrollmentID, perm, allot); err != nil {
		return store.AdjustCreditsResult{}, err
	}

	available := perm + allot
	entry := store.CreditLedgerEntry{
		ID:           uuid.NewString(),
		EnrollmentID: enrollmentID,
		Delta:        in.Delta,
		BalanceAfter: available,
		Reason:       in.Reason,
		RefType:      refType,
		RefID:        refID,
		Note:         strings.TrimSpace(in.Note),
		CreatedAt:    time.Now().UTC(),
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO credit_ledger (id, enrollment_id, delta, balance_after, reason, ref_type, ref_id, note, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
	`, entry.ID, entry.EnrollmentID, entry.Delta, entry.BalanceAfter, string(entry.Reason), entry.RefType, entry.RefID, entry.Note, entry.CreatedAt); err != nil {
		return store.AdjustCreditsResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return store.AdjustCreditsResult{}, err
	}
	return store.AdjustCreditsResult{
		Balance:          perm,
		AllotmentBalance: allot,
		Entry:            entry,
		Applied:          true,
	}, nil
}

func (s *Store) getCreditsTx(ctx context.Context, tx pgx.Tx, enrollmentID string) (store.DeviceCredits, error) {
	var bal store.DeviceCredits
	err := tx.QueryRow(ctx, `
		SELECT enrollment_id, balance, allotment_balance, updated_at
		FROM device_credits WHERE enrollment_id=$1
	`, enrollmentID).Scan(&bal.EnrollmentID, &bal.Balance, &bal.AllotmentBalance, &bal.UpdatedAt)
	if err == pgx.ErrNoRows {
		return store.DeviceCredits{EnrollmentID: enrollmentID}, nil
	}
	return bal, err
}

func consumeAllotmentGrantsTx(ctx context.Context, tx pgx.Tx, enrollmentID string, amount int) error {
	if amount <= 0 {
		return nil
	}
	rows, err := tx.Query(ctx, `
		SELECT id, remaining FROM credit_allotment_grants
		WHERE enrollment_id=$1 AND remaining > 0
		ORDER BY granted_at ASC, id ASC
		FOR UPDATE
	`, enrollmentID)
	if err != nil {
		return err
	}
	defer rows.Close()
	type row struct {
		id        string
		remaining int
	}
	var list []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.remaining); err != nil {
			return err
		}
		list = append(list, r)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	left := amount
	for _, r := range list {
		if left <= 0 {
			break
		}
		take := r.remaining
		if take > left {
			take = left
		}
		if _, err := tx.Exec(ctx, `
			UPDATE credit_allotment_grants SET remaining=remaining-$2 WHERE id=$1
		`, r.id, take); err != nil {
			return err
		}
		left -= take
	}
	return nil
}

func (s *Store) ListCreditLedger(ctx context.Context, enrollmentID string, limit int) ([]store.CreditLedgerEntry, error) {
	enrollmentID = strings.TrimSpace(enrollmentID)
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, enrollment_id, delta, balance_after, reason, ref_type, ref_id, COALESCE(note, ''), created_at
		FROM credit_ledger
		WHERE enrollment_id=$1
		ORDER BY created_at DESC, id DESC
		LIMIT $2
	`, enrollmentID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]store.CreditLedgerEntry, 0)
	for rows.Next() {
		var e store.CreditLedgerEntry
		var reason string
		if err := rows.Scan(&e.ID, &e.EnrollmentID, &e.Delta, &e.BalanceAfter, &reason, &e.RefType, &e.RefID, &e.Note, &e.CreatedAt); err != nil {
			return nil, err
		}
		e.Reason = store.LedgerReason(reason)
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Store) ListCreditBalances(ctx context.Context) ([]store.DeviceCredits, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT enrollment_id, balance, allotment_balance, updated_at
		FROM device_credits ORDER BY enrollment_id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]store.DeviceCredits, 0)
	for rows.Next() {
		var bal store.DeviceCredits
		if err := rows.Scan(&bal.EnrollmentID, &bal.Balance, &bal.AllotmentBalance, &bal.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, bal)
	}
	return out, rows.Err()
}

func (s *Store) ListCreditPackages(ctx context.Context, activeOnly bool) ([]store.CreditPackage, error) {
	q := `
		SELECT id, name_he, credits, price_agorot, active, sort_order
		FROM credit_packages
	`
	if activeOnly {
		q += ` WHERE active = TRUE`
	}
	q += ` ORDER BY sort_order ASC, credits ASC`
	rows, err := s.pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]store.CreditPackage, 0)
	for rows.Next() {
		var p store.CreditPackage
		if err := rows.Scan(&p.ID, &p.NameHe, &p.Credits, &p.PriceAgorot, &p.Active, &p.SortOrder); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) GetCreditPackage(ctx context.Context, id string) (store.CreditPackage, error) {
	var p store.CreditPackage
	err := s.pool.QueryRow(ctx, `
		SELECT id, name_he, credits, price_agorot, active, sort_order
		FROM credit_packages WHERE id=$1
	`, id).Scan(&p.ID, &p.NameHe, &p.Credits, &p.PriceAgorot, &p.Active, &p.SortOrder)
	if err == pgx.ErrNoRows {
		return store.CreditPackage{}, fmt.Errorf("credit package %s: %w", id, store.ErrNotFound)
	}
	if err != nil {
		return store.CreditPackage{}, err
	}
	return p, nil
}

func (s *Store) CreateCreditPackage(ctx context.Context, p store.CreditPackage) (store.CreditPackage, error) {
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
	if p.ID == "" {
		p.ID = uuid.NewString()
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO credit_packages (id, name_he, credits, price_agorot, active, sort_order)
		VALUES ($1,$2,$3,$4,$5,$6)
	`, p.ID, p.NameHe, p.Credits, p.PriceAgorot, p.Active, p.SortOrder)
	if err != nil {
		return store.CreditPackage{}, err
	}
	return p, nil
}

func (s *Store) UpdateCreditPackage(ctx context.Context, p store.CreditPackage) (store.CreditPackage, error) {
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
	ct, err := s.pool.Exec(ctx, `
		UPDATE credit_packages
		SET name_he=$2, credits=$3, price_agorot=$4, active=$5, sort_order=$6
		WHERE id=$1
	`, p.ID, p.NameHe, p.Credits, p.PriceAgorot, p.Active, p.SortOrder)
	if err != nil {
		return store.CreditPackage{}, err
	}
	if ct.RowsAffected() == 0 {
		return store.CreditPackage{}, fmt.Errorf("credit package %s: %w", p.ID, store.ErrNotFound)
	}
	return p, nil
}

func (s *Store) GetCreditSettings(ctx context.Context) (store.CreditSettings, error) {
	var settings store.CreditSettings
	err := s.pool.QueryRow(ctx, `
		SELECT access_request_cost, enabled, updated_at
		FROM credit_settings WHERE id=1
	`).Scan(&settings.AccessRequestCost, &settings.Enabled, &settings.UpdatedAt)
	if err == pgx.ErrNoRows {
		return store.CreditSettings{}, fmt.Errorf("credit settings: %w", store.ErrNotFound)
	}
	if err != nil {
		return store.CreditSettings{}, err
	}
	return settings, nil
}

func (s *Store) UpsertCreditSettings(ctx context.Context, settings store.CreditSettings) (store.CreditSettings, error) {
	if settings.AccessRequestCost < 1 {
		return store.CreditSettings{}, fmt.Errorf("access_request_cost must be >= 1")
	}
	err := s.pool.QueryRow(ctx, `
		INSERT INTO credit_settings (id, access_request_cost, enabled, updated_at)
		VALUES (1, $1, $2, now())
		ON CONFLICT (id) DO UPDATE SET
			access_request_cost = EXCLUDED.access_request_cost,
			enabled = EXCLUDED.enabled,
			updated_at = now()
		RETURNING access_request_cost, enabled, updated_at
	`, settings.AccessRequestCost, settings.Enabled).Scan(
		&settings.AccessRequestCost, &settings.Enabled, &settings.UpdatedAt,
	)
	if err != nil {
		return store.CreditSettings{}, err
	}
	return settings, nil
}

func (s *Store) CreateCreditPurchase(ctx context.Context, p store.CreditPurchase) (store.CreditPurchase, error) {
	if p.ID == "" {
		p.ID = uuid.NewString()
	}
	if p.ClientUniqueID == "" {
		p.ClientUniqueID = uuid.NewString()
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = time.Now().UTC()
	}
	if p.Status == "" {
		p.Status = store.PurchasePending
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO credit_purchases (
			id, enrollment_id, package_id, credits, amount_agorot, status, provider, provider_tx_id, client_unique_id, created_at, paid_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
	`, p.ID, p.EnrollmentID, p.PackageID, p.Credits, p.AmountAgorot, string(p.Status), string(p.Provider), p.ProviderTxID, p.ClientUniqueID, p.CreatedAt, p.PaidAt)
	if err != nil {
		return store.CreditPurchase{}, err
	}
	return p, nil
}

func (s *Store) GetCreditPurchase(ctx context.Context, id string) (store.CreditPurchase, error) {
	return s.scanPurchase(ctx, `
		SELECT id, enrollment_id, package_id, credits, amount_agorot, status, provider, provider_tx_id, client_unique_id, created_at, paid_at
		FROM credit_purchases WHERE id=$1
	`, id)
}

func (s *Store) GetCreditPurchaseByClientUnique(ctx context.Context, clientUniqueID string) (store.CreditPurchase, error) {
	return s.scanPurchase(ctx, `
		SELECT id, enrollment_id, package_id, credits, amount_agorot, status, provider, provider_tx_id, client_unique_id, created_at, paid_at
		FROM credit_purchases WHERE client_unique_id=$1
	`, clientUniqueID)
}

func (s *Store) scanPurchase(ctx context.Context, q string, arg string) (store.CreditPurchase, error) {
	var p store.CreditPurchase
	var status, provider string
	err := s.pool.QueryRow(ctx, q, arg).Scan(
		&p.ID, &p.EnrollmentID, &p.PackageID, &p.Credits, &p.AmountAgorot,
		&status, &provider, &p.ProviderTxID, &p.ClientUniqueID, &p.CreatedAt, &p.PaidAt,
	)
	if err == pgx.ErrNoRows {
		return store.CreditPurchase{}, fmt.Errorf("credit purchase %s: %w", arg, store.ErrNotFound)
	}
	if err != nil {
		return store.CreditPurchase{}, err
	}
	p.Status = store.PurchaseStatus(status)
	p.Provider = store.PaymentProvider(provider)
	return p, nil
}

func (s *Store) MarkPurchasePaid(ctx context.Context, in store.MarkPurchasePaidInput) (store.CreditPurchase, bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return store.CreditPurchase{}, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var p store.CreditPurchase
	var status, provider string
	err = tx.QueryRow(ctx, `
		SELECT id, enrollment_id, package_id, credits, amount_agorot, status, provider, provider_tx_id, client_unique_id, created_at, paid_at
		FROM credit_purchases WHERE id=$1 FOR UPDATE
	`, in.PurchaseID).Scan(
		&p.ID, &p.EnrollmentID, &p.PackageID, &p.Credits, &p.AmountAgorot,
		&status, &provider, &p.ProviderTxID, &p.ClientUniqueID, &p.CreatedAt, &p.PaidAt,
	)
	if err == pgx.ErrNoRows {
		return store.CreditPurchase{}, false, fmt.Errorf("credit purchase %s: %w", in.PurchaseID, store.ErrNotFound)
	}
	if err != nil {
		return store.CreditPurchase{}, false, err
	}
	p.Status = store.PurchaseStatus(status)
	p.Provider = store.PaymentProvider(provider)

	if p.Status == store.PurchasePaid {
		return p, false, nil
	}
	if p.Status != store.PurchasePending {
		return store.CreditPurchase{}, false, fmt.Errorf("purchase %s is %s", p.ID, p.Status)
	}

	// Idempotent ledger: skip credit if already applied.
	var existing int
	err = tx.QueryRow(ctx, `
		SELECT 1 FROM credit_ledger WHERE reason=$1 AND ref_type=$2 AND ref_id=$3 LIMIT 1
	`, string(store.LedgerPurchase), "purchase", p.ID).Scan(&existing)
	alreadyCredited := err == nil
	if err != nil && err != pgx.ErrNoRows {
		return store.CreditPurchase{}, false, err
	}

	now := time.Now().UTC()
	if !alreadyCredited {
		if _, err := tx.Exec(ctx, `
			INSERT INTO device_credits (enrollment_id, balance, allotment_balance, updated_at)
			VALUES ($1, 0, 0, now())
			ON CONFLICT (enrollment_id) DO NOTHING
		`, p.EnrollmentID); err != nil {
			return store.CreditPurchase{}, false, err
		}
		var perm, allot int
		if err := tx.QueryRow(ctx, `
			SELECT balance, allotment_balance FROM device_credits WHERE enrollment_id=$1 FOR UPDATE
		`, p.EnrollmentID).Scan(&perm, &allot); err != nil {
			return store.CreditPurchase{}, false, err
		}
		perm += p.Credits
		if _, err := tx.Exec(ctx, `
			UPDATE device_credits SET balance=$2, updated_at=now() WHERE enrollment_id=$1
		`, p.EnrollmentID, perm); err != nil {
			return store.CreditPurchase{}, false, err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO credit_ledger (id, enrollment_id, delta, balance_after, reason, ref_type, ref_id, created_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		`, uuid.NewString(), p.EnrollmentID, p.Credits, perm+allot, string(store.LedgerPurchase), "purchase", p.ID, now); err != nil {
			return store.CreditPurchase{}, false, err
		}
	}

	txID := in.ProviderTxID
	if txID == "" {
		txID = p.ProviderTxID
	}
	if _, err := tx.Exec(ctx, `
		UPDATE credit_purchases SET status=$2, provider_tx_id=$3, paid_at=$4 WHERE id=$1
	`, p.ID, string(store.PurchasePaid), txID, now); err != nil {
		return store.CreditPurchase{}, false, err
	}

	if err := tx.Commit(ctx); err != nil {
		return store.CreditPurchase{}, false, err
	}
	p.Status = store.PurchasePaid
	p.ProviderTxID = txID
	p.PaidAt = &now
	return p, !alreadyCredited, nil
}
