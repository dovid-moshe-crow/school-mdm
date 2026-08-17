package credits

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/dwdmsh/school-mdm/internal/nedarim"
	"github.com/dwdmsh/school-mdm/internal/store"
	"github.com/dwdmsh/school-mdm/internal/store/memory"
)

func testService(t *testing.T) (*Service, *memory.Store) {
	t.Helper()
	mem := memory.New()
	svc := &Service{
		Store:      mem,
		Nedarim:    &nedarim.Client{Cfg: nedarim.Config{Mode: nedarim.ModeFake, PortalBase: "http://localhost:8080"}},
		AccessCost: 1,
		PortalBase: "http://localhost:8080",
	}
	if err := svc.EnsureSettings(context.Background()); err != nil {
		t.Fatal(err)
	}
	return svc, mem
}

func TestSpendRefundIdempotent(t *testing.T) {
	ctx := context.Background()
	svc, _ := testService(t)

	if _, err := svc.Gift(ctx, "dev-1", 5, "welcome"); err != nil {
		t.Fatal(err)
	}
	bal, _ := svc.Balance(ctx, "dev-1")
	if bal.Balance != 5 {
		t.Fatalf("balance=%d", bal.Balance)
	}

	if err := svc.SpendForAccessRequest(ctx, "dev-1", "req-1"); err != nil {
		t.Fatal(err)
	}
	if err := svc.SpendForAccessRequest(ctx, "dev-1", "req-1"); err != nil {
		t.Fatal(err) // idempotent
	}
	bal, _ = svc.Balance(ctx, "dev-1")
	if bal.Balance != 4 {
		t.Fatalf("after spend balance=%d want 4", bal.Balance)
	}

	if err := svc.RefundForDeniedRequest(ctx, "dev-1", "req-1"); err != nil {
		t.Fatal(err)
	}
	if err := svc.RefundForDeniedRequest(ctx, "dev-1", "req-1"); err != nil {
		t.Fatal(err)
	}
	bal, _ = svc.Balance(ctx, "dev-1")
	if bal.Balance != 5 {
		t.Fatalf("after refund balance=%d want 5", bal.Balance)
	}
}

func TestSpendInsufficient(t *testing.T) {
	ctx := context.Background()
	svc, _ := testService(t)
	err := svc.SpendForAccessRequest(ctx, "dev-empty", "req-x")
	if !errors.Is(err, ErrInsufficientCredits) {
		t.Fatalf("err=%v", err)
	}
}

func TestFakeCheckoutPayConfirm(t *testing.T) {
	ctx := context.Background()
	svc, mem := testService(t)

	pkgs, err := svc.Packages(ctx)
	if err != nil || len(pkgs) == 0 {
		t.Fatalf("packages: %v %d", err, len(pkgs))
	}
	co, err := svc.StartCheckout(ctx, "dev-pay", pkgs[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if co.Mode != nedarim.ModeFake {
		t.Fatalf("mode=%s", co.Mode)
	}
	if co.IframeURL == "" {
		t.Fatal("expected iframe url")
	}

	paid, err := svc.FakePay(ctx, co.Purchase.ClientUniqueID)
	if err != nil {
		t.Fatal(err)
	}
	if paid.Status != store.PurchasePaid {
		t.Fatalf("status=%s", paid.Status)
	}
	// Idempotent mark paid
	_, again, err := mem.MarkPurchasePaid(ctx, store.MarkPurchasePaidInput{PurchaseID: paid.ID})
	if err != nil {
		t.Fatal(err)
	}
	if again {
		t.Fatal("second mark should not re-apply credits")
	}

	_, bal, err := svc.ConfirmPayment(ctx, paid.ID, "dev-pay")
	if err != nil {
		t.Fatal(err)
	}
	if bal.Balance != pkgs[0].Credits {
		t.Fatalf("balance=%d want %d", bal.Balance, pkgs[0].Credits)
	}

	// Confirm before pay should fail
	co2, err := svc.StartCheckout(ctx, "dev-pay", pkgs[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.ConfirmPayment(ctx, co2.Purchase.ID, "dev-pay"); err == nil {
		t.Fatal("expected confirm to fail for pending purchase")
	}
}

func TestMarkPurchasePaidIdempotent(t *testing.T) {
	ctx := context.Background()
	svc, mem := testService(t)
	pkgs, _ := svc.Packages(ctx)
	co, err := svc.StartCheckout(ctx, "dev-2", pkgs[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	p1, applied1, err := mem.MarkPurchasePaid(ctx, store.MarkPurchasePaidInput{PurchaseID: co.Purchase.ID, ProviderTxID: "tx-1"})
	if err != nil || !applied1 {
		t.Fatalf("first: applied=%v err=%v", applied1, err)
	}
	p2, applied2, err := mem.MarkPurchasePaid(ctx, store.MarkPurchasePaidInput{PurchaseID: co.Purchase.ID, ProviderTxID: "tx-1"})
	if err != nil {
		t.Fatal(err)
	}
	if applied2 {
		t.Fatal("second apply should be false")
	}
	if p1.Status != store.PurchasePaid || p2.Status != store.PurchasePaid {
		t.Fatal("both should be paid")
	}
	bal, _ := svc.Balance(ctx, "dev-2")
	if bal.Balance != pkgs[0].Credits {
		t.Fatalf("balance=%d", bal.Balance)
	}
}

func TestPackageCRUDAndActiveOnly(t *testing.T) {
	ctx := context.Background()
	svc, _ := testService(t)

	created, err := svc.CreatePackage(ctx, store.CreditPackage{
		NameHe:      "בדיקה",
		Credits:     7,
		PriceAgorot: 700,
		Active:      true,
		SortOrder:   5,
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.ID == "" {
		t.Fatal("expected id")
	}

	created.NameHe = "בדיקה מעודכנת"
	created.Credits = 8
	created.PriceAgorot = 750
	created.SortOrder = 6
	updated, err := svc.UpdatePackage(ctx, created)
	if err != nil {
		t.Fatal(err)
	}
	if updated.NameHe != "בדיקה מעודכנת" || updated.Credits != 8 || updated.PriceAgorot != 750 {
		t.Fatalf("update failed: %+v", updated)
	}

	all, err := svc.AdminPackages(ctx)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, p := range all {
		if p.ID == created.ID {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("package missing from admin list")
	}

	if _, err := svc.DeactivatePackage(ctx, created.ID); err != nil {
		t.Fatal(err)
	}
	active, err := svc.Packages(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range active {
		if p.ID == created.ID {
			t.Fatal("inactive package should not appear in portal packages")
		}
	}
}

func TestSettingsUpdateAffectsSpend(t *testing.T) {
	ctx := context.Background()
	svc, _ := testService(t)

	if _, err := svc.UpdateSettings(ctx, 3, nil); err != nil {
		t.Fatal(err)
	}
	if got := svc.AccessRequestCost(ctx); got != 3 {
		t.Fatalf("access cost=%d want 3", got)
	}
	if _, err := svc.Gift(ctx, "dev-cost", 5, ""); err != nil {
		t.Fatal(err)
	}
	if err := svc.SpendForAccessRequest(ctx, "dev-cost", "req-cost"); err != nil {
		t.Fatal(err)
	}
	bal, _ := svc.Balance(ctx, "dev-cost")
	if bal.Balance != 2 {
		t.Fatalf("balance=%d want 2 after spending 3", bal.Balance)
	}
}

func TestAdjustGiftAndSubtract(t *testing.T) {
	ctx := context.Background()
	svc, _ := testService(t)

	res, err := svc.Adjust(ctx, "dev-adj", 10, "welcome gift")
	if err != nil {
		t.Fatal(err)
	}
	if res.Balance != 10 || res.Entry.Reason != store.LedgerGift {
		t.Fatalf("gift result=%+v", res)
	}
	if res.Entry.Note != "welcome gift" {
		t.Fatalf("note=%q", res.Entry.Note)
	}

	res, err = svc.Adjust(ctx, "dev-adj", -3, "correction")
	if err != nil {
		t.Fatal(err)
	}
	if res.Balance != 7 || res.Entry.Reason != store.LedgerAdjust {
		t.Fatalf("adjust result=%+v", res)
	}

	ledger, err := svc.Ledger(ctx, "dev-adj", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(ledger) < 2 {
		t.Fatalf("ledger len=%d", len(ledger))
	}
}

func TestPortalPackagesActiveOnly(t *testing.T) {
	ctx := context.Background()
	svc, mem := testService(t)
	pkgs, err := mem.ListCreditPackages(ctx, false)
	if err != nil || len(pkgs) == 0 {
		t.Fatalf("seed packages: %v %d", err, len(pkgs))
	}
	pkgs[0].Active = false
	if _, err := mem.UpdateCreditPackage(ctx, pkgs[0]); err != nil {
		t.Fatal(err)
	}
	active, err := svc.Packages(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range active {
		if p.ID == pkgs[0].ID {
			t.Fatal("deactivated package returned by Packages()")
		}
		if !p.Active {
			t.Fatal("Packages() returned inactive package")
		}
	}
}

func TestAllotmentIndividualIdempotentAndReset(t *testing.T) {
	ctx := context.Background()
	svc, mem := testService(t)
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)

	if _, err := svc.Gift(ctx, "dev-a", 7, "bought"); err != nil {
		t.Fatal(err)
	}

	rule, err := svc.CreateAllotmentRule(ctx, store.CreditAllotmentRule{
		Name:       "daily 5",
		Amount:     5,
		Interval:   store.IntervalDaily,
		TargetType: store.AllotmentIndividual,
		TargetID:   "dev-a",
		Enabled:    true,
	})
	if err != nil {
		t.Fatal(err)
	}

	res, err := svc.RunAllotments(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	if res.GrantsApplied != 1 {
		t.Fatalf("applied=%d want 1", res.GrantsApplied)
	}
	bal, _ := svc.Balance(ctx, "dev-a")
	if bal.AllotmentBalance != 5 || bal.Balance != 7 {
		t.Fatalf("after grant: allot=%d perm=%d", bal.AllotmentBalance, bal.Balance)
	}

	// Same period: idempotent, no double grant.
	res, err = svc.RunAllotments(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	if res.GrantsApplied != 0 || res.GrantsSkipped != 1 {
		t.Fatalf("second run applied=%d skipped=%d", res.GrantsApplied, res.GrantsSkipped)
	}
	bal, _ = svc.Balance(ctx, "dev-a")
	if bal.AllotmentBalance != 5 || bal.Balance != 7 {
		t.Fatalf("after idempotent: allot=%d perm=%d", bal.AllotmentBalance, bal.Balance)
	}

	// Spend 2 from allotment.
	if err := svc.SpendForAccessRequest(ctx, "dev-a", "req-allot-1"); err != nil {
		t.Fatal(err)
	}
	// Temporarily set cost... AccessCost is 1 by default.
	bal, _ = svc.Balance(ctx, "dev-a")
	if bal.AllotmentBalance != 4 || bal.Balance != 7 {
		t.Fatalf("after spend: allot=%d perm=%d want allot 4", bal.AllotmentBalance, bal.Balance)
	}

	// New day: expire unused (4), grant fresh 5 → allotment 5, permanent unchanged.
	nextDay := now.AddDate(0, 0, 1)
	res, err = svc.RunAllotments(ctx, nextDay)
	if err != nil {
		t.Fatal(err)
	}
	if res.GrantsApplied != 1 {
		t.Fatalf("new period applied=%d", res.GrantsApplied)
	}
	bal, _ = svc.Balance(ctx, "dev-a")
	if bal.AllotmentBalance != 5 {
		t.Fatalf("new period allot=%d want 5 (non-stacking)", bal.AllotmentBalance)
	}
	if bal.Balance != 7 {
		t.Fatalf("permanent should be untouched, got %d", bal.Balance)
	}

	_ = mem
	_ = rule
}

func TestAllotmentSpendPrefersAllotmentThenPermanent(t *testing.T) {
	ctx := context.Background()
	svc, _ := testService(t)
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)

	if _, err := svc.Gift(ctx, "dev-spend", 3, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateAllotmentRule(ctx, store.CreditAllotmentRule{
		Amount: 2, Interval: store.IntervalDaily,
		TargetType: store.AllotmentIndividual, TargetID: "dev-spend", Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RunAllotments(ctx, now); err != nil {
		t.Fatal(err)
	}

	// Cost 1: take from allotment.
	if err := svc.SpendForAccessRequest(ctx, "dev-spend", "r1"); err != nil {
		t.Fatal(err)
	}
	bal, _ := svc.Balance(ctx, "dev-spend")
	if bal.AllotmentBalance != 1 || bal.Balance != 3 {
		t.Fatalf("after first spend allot=%d perm=%d", bal.AllotmentBalance, bal.Balance)
	}

	// Raise cost to 3: 1 allotment + 2 permanent.
	if _, err := svc.UpdateSettings(ctx, 3, nil); err != nil {
		t.Fatal(err)
	}
	if err := svc.SpendForAccessRequest(ctx, "dev-spend", "r2"); err != nil {
		t.Fatal(err)
	}
	bal, _ = svc.Balance(ctx, "dev-spend")
	if bal.AllotmentBalance != 0 || bal.Balance != 1 {
		t.Fatalf("after split spend allot=%d perm=%d want 0/1", bal.AllotmentBalance, bal.Balance)
	}
}

func TestAllotmentGroupAndEveryone(t *testing.T) {
	ctx := context.Background()
	svc, mem := testService(t)
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)

	g, err := mem.CreateGroup(ctx, store.Group{Name: "class-a"})
	if err != nil {
		t.Fatal(err)
	}
	if err := mem.SetGroupMembers(ctx, g.ID, []string{"g1", "g2"}); err != nil {
		t.Fatal(err)
	}
	_ = mem.SetDeviceName(ctx, "solo", "Solo")

	if _, err := svc.CreateAllotmentRule(ctx, store.CreditAllotmentRule{
		Amount: 4, Interval: store.IntervalWeekly,
		TargetType: store.AllotmentGroup, TargetID: g.ID, Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateAllotmentRule(ctx, store.CreditAllotmentRule{
		Amount: 1, Interval: store.IntervalDaily,
		TargetType: store.AllotmentEveryone, Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}

	res, err := svc.RunAllotments(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	if res.GrantsApplied < 3 {
		t.Fatalf("expected grants for g1,g2,solo at least, applied=%d", res.GrantsApplied)
	}

	b1, _ := svc.Balance(ctx, "g1")
	b2, _ := svc.Balance(ctx, "g2")
	bs, _ := svc.Balance(ctx, "solo")
	// g1/g2: group 4 + everyone 1 = 5 allotment
	if b1.AllotmentBalance != 5 || b2.AllotmentBalance != 5 {
		t.Fatalf("group+everyone: g1=%d g2=%d want 5", b1.AllotmentBalance, b2.AllotmentBalance)
	}
	if bs.AllotmentBalance != 1 {
		t.Fatalf("solo everyone only: %d want 1", bs.AllotmentBalance)
	}
}

func TestPeriodKeyAndNext(t *testing.T) {
	now := time.Date(2026, 7, 20, 15, 0, 0, 0, time.UTC) // Monday
	if got := PeriodKey(store.IntervalDaily, now); got != "2026-07-20" {
		t.Fatalf("daily key=%s", got)
	}
	if got := PeriodKey(store.IntervalMonthly, now); got != "2026-07" {
		t.Fatalf("monthly key=%s", got)
	}
	wk := PeriodKey(store.IntervalWeekly, now)
	if !strings.HasPrefix(wk, "2026-W") {
		t.Fatalf("weekly key=%s", wk)
	}
	next := NextPeriodStart(store.IntervalDaily, now)
	if !next.Equal(time.Date(2026, 7, 21, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("next daily=%v", next)
	}
}

func TestWebhookRefusalDoesNotCredit(t *testing.T) {
	ctx := context.Background()
	svc, _ := testService(t)
	pkgs, _ := svc.Packages(ctx)
	co, err := svc.StartCheckout(ctx, "dev-refuse", pkgs[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	p, err := svc.HandleWebhook(ctx, WebhookPayload{
		ClientUniqueID: co.Purchase.ClientUniqueID,
		Status:         "Error",
		TransactionID:  "tx-refused",
	})
	if err != nil {
		t.Fatal(err)
	}
	if p.Status != store.PurchasePending {
		t.Fatalf("status=%s want pending", p.Status)
	}
	bal, _ := svc.Balance(ctx, "dev-refuse")
	if bal.Balance != 0 {
		t.Fatalf("balance=%d want 0", bal.Balance)
	}
}

func TestWebhookSuccessCredits(t *testing.T) {
	ctx := context.Background()
	svc, _ := testService(t)
	pkgs, _ := svc.Packages(ctx)
	co, err := svc.StartCheckout(ctx, "dev-hook", pkgs[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	p, err := svc.HandleWebhook(ctx, WebhookPayload{
		ClientUniqueID: co.Purchase.ClientUniqueID,
		Status:         "OK",
		TransactionID:  "tx-ok-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if p.Status != store.PurchasePaid {
		t.Fatalf("status=%s", p.Status)
	}
	bal, _ := svc.Balance(ctx, "dev-hook")
	if bal.Balance != pkgs[0].Credits {
		t.Fatalf("balance=%d want %d", bal.Balance, pkgs[0].Credits)
	}
}

