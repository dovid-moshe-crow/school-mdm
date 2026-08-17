package approvals

import (
	"context"
	"errors"
	"testing"

	"github.com/dwdmsh/school-mdm/internal/credits"
	"github.com/dwdmsh/school-mdm/internal/mdm"
	"github.com/dwdmsh/school-mdm/internal/nedarim"
	"github.com/dwdmsh/school-mdm/internal/policy"
	"github.com/dwdmsh/school-mdm/internal/store"
	"github.com/dwdmsh/school-mdm/internal/store/memory"
)

func TestApproveAccessCreatesGrantAndEnqueuesProfile(t *testing.T) {
	ctx := context.Background()
	mem := memory.New()
	stub := &mdm.StubEnqueuer{}
	svc := &Service{Store: mem, Enqueue: stub, PortalURL: "http://localhost:8080"}

	req, err := svc.CreateRequest(ctx, CreateRequestInput{
		Type:         store.TypeAccess,
		Kind:         policy.KindURL,
		Value:        "https://example.com/lesson",
		EnrollmentID: "device-1",
		Reason:       "homework",
	})
	if err != nil {
		t.Fatal(err)
	}

	decided, err := svc.Decide(ctx, DecideInput{RequestID: req.ID, Approve: true, Duration: "1h"})
	if err != nil {
		t.Fatal(err)
	}
	if decided.Status != store.StatusApproved {
		t.Fatalf("status=%s", decided.Status)
	}

	_, urls, err := svc.EffectiveAllowlist(ctx, "device-1")
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, urls, "example.com/lesson")
	// Reconcile installs school allowlist + help web clip + store web clip.
	if n := len(stub.Snapshot()); n != 3 {
		t.Fatalf("expected 3 stub commands (allowlist+help+store webclips), got %d", n)
	}
}

func TestApproveBugResolvesWithoutEnqueue(t *testing.T) {
	ctx := context.Background()
	mem := memory.New()
	stub := &mdm.StubEnqueuer{}
	svc := &Service{Store: mem, Enqueue: stub, PortalURL: "http://localhost:8080"}

	req, err := svc.CreateRequest(ctx, CreateRequestInput{
		Type:  store.TypeBug,
		Value: "Safari crashes on portal",
		Reason: "opens then blank",
	})
	if err != nil {
		t.Fatal(err)
	}
	decided, err := svc.Decide(ctx, DecideInput{RequestID: req.ID, Approve: true})
	if err != nil {
		t.Fatal(err)
	}
	if decided.Status != store.StatusResolved {
		t.Fatalf("status=%s", decided.Status)
	}
	if len(stub.Snapshot()) != 0 {
		t.Fatal("bug resolve should not enqueue MDM")
	}
}

func TestApproveGeneralWithoutEnqueue(t *testing.T) {
	ctx := context.Background()
	mem := memory.New()
	stub := &mdm.StubEnqueuer{}
	svc := &Service{Store: mem, Enqueue: stub, PortalURL: "http://localhost:8080"}

	req, err := svc.CreateRequest(ctx, CreateRequestInput{
		Type:  store.TypeGeneral,
		Value: "Need help with login",
	})
	if err != nil {
		t.Fatal(err)
	}
	decided, err := svc.Decide(ctx, DecideInput{RequestID: req.ID, Approve: true})
	if err != nil {
		t.Fatal(err)
	}
	if decided.Status != store.StatusResolved {
		t.Fatalf("status=%s want resolved", decided.Status)
	}
	if len(stub.Snapshot()) != 0 {
		t.Fatal("general approve should not enqueue MDM")
	}
}

func TestDenyDoesNotEnqueue(t *testing.T) {
	ctx := context.Background()
	mem := memory.New()
	stub := &mdm.StubEnqueuer{}
	svc := &Service{Store: mem, Enqueue: stub, PortalURL: "http://localhost:8080"}

	req, err := svc.CreateRequest(ctx, CreateRequestInput{
		Kind:  policy.KindApp, // back-compat implies access
		Value: "com.game.fun",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Decide(ctx, DecideInput{RequestID: req.ID, Approve: false}); err != nil {
		t.Fatal(err)
	}
	if len(stub.Snapshot()) != 0 {
		t.Fatal("deny should not enqueue")
	}
}

func TestApproveGroupScopeFansOut(t *testing.T) {
	ctx := context.Background()
	mem := memory.New()
	stub := &mdm.StubEnqueuer{}
	svc := &Service{Store: mem, Enqueue: stub, PortalURL: "http://localhost:8080"}

	g, err := mem.CreateGroup(ctx, store.Group{Name: "Math"})
	if err != nil {
		t.Fatal(err)
	}
	if err := mem.SetGroupMembers(ctx, g.ID, []string{"device-a", "device-b"}); err != nil {
		t.Fatal(err)
	}

	req, err := svc.CreateRequest(ctx, CreateRequestInput{
		Type:         store.TypeAccess,
		Kind:         policy.KindURL,
		Value:        "https://khanacademy.org",
		EnrollmentID: "device-a",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Decide(ctx, DecideInput{
		RequestID: req.ID,
		Approve:   true,
		Duration:  "permanent",
		Scope:     "group",
		GroupID:   g.ID,
	}); err != nil {
		t.Fatal(err)
	}

	_, urlsA, err := svc.EffectiveAllowlist(ctx, "device-a")
	if err != nil {
		t.Fatal(err)
	}
	_, urlsB, err := svc.EffectiveAllowlist(ctx, "device-b")
	if err != nil {
		t.Fatal(err)
	}
	_, urlsC, err := svc.EffectiveAllowlist(ctx, "device-c")
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, urlsA, "khanacademy.org")
	assertContains(t, urlsB, "khanacademy.org")
	for _, u := range urlsC {
		if u == "khanacademy.org" {
			t.Fatal("group allowlist leaked to non-member")
		}
	}
	if len(stub.Snapshot()) < 2 {
		t.Fatalf("expected fan-out enqueue for both members, got %d", len(stub.Snapshot()))
	}
}

func TestCreateAccessWithoutReasonDoesNotSeedBundleID(t *testing.T) {
	ctx := context.Background()
	mem := memory.New()
	svc := &Service{Store: mem, Enqueue: &mdm.StubEnqueuer{}, PortalURL: "http://localhost:8080"}

	req, err := svc.CreateRequest(ctx, CreateRequestInput{
		Type:         store.TypeAccess,
		Kind:         policy.KindApp,
		Value:        "com.google.ios.youtube",
		EnrollmentID: "device-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	msgs, err := mem.ListRequestMessages(ctx, req.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 0 {
		t.Fatalf("expected no seeded message when reason is empty, got %#v", msgs)
	}
}

func TestRequestMessageThreadAndReopen(t *testing.T) {
	ctx := context.Background()
	mem := memory.New()
	svc := &Service{Store: mem, Enqueue: &mdm.StubEnqueuer{}, PortalURL: "http://localhost:8080"}

	req, err := svc.CreateRequest(ctx, CreateRequestInput{
		Type:         store.TypeGeneral,
		Value:        "Wifi down",
		EnrollmentID: "dev-1",
		Reason:       "Cannot connect in room 12",
	})
	if err != nil {
		t.Fatal(err)
	}
	msgs, err := mem.ListRequestMessages(ctx, req.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 || msgs[0].AuthorRole != store.AuthorStudent {
		t.Fatalf("expected seeded student message, got %#v", msgs)
	}
	if msgs[0].Body != "Cannot connect in room 12" {
		t.Fatalf("seeded body = %q", msgs[0].Body)
	}

	if _, err := svc.Decide(ctx, DecideInput{RequestID: req.ID, Approve: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.PostMessage(ctx, PostMessageInput{
		RequestID:  req.ID,
		AuthorRole: store.AuthorAdmin,
		Body:       "Router was reset — try again",
	}); err != nil {
		t.Fatal(err)
	}
	reopened, err := svc.PostMessage(ctx, PostMessageInput{
		RequestID:    req.ID,
		AuthorRole:   store.AuthorStudent,
		Body:         "Still broken",
		EnrollmentID: "dev-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if reopened.Body != "Still broken" {
		t.Fatal(reopened.Body)
	}
	got, err := mem.GetRequest(ctx, req.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != store.StatusPending {
		t.Fatalf("student reply should reopen, status=%s", got.Status)
	}
}

func TestAccessRequiresCreditsAndDenyRefunds(t *testing.T) {
	ctx := context.Background()
	mem := memory.New()
	stub := &mdm.StubEnqueuer{}
	creditSvc := &credits.Service{
		Store:      mem,
		Nedarim:    &nedarim.Client{Cfg: nedarim.Config{Mode: nedarim.ModeFake, PortalBase: "http://localhost:8080"}},
		AccessCost: 1,
		PortalBase: "http://localhost:8080",
	}
	svc := &Service{Store: mem, Enqueue: stub, PortalURL: "http://localhost:8080", Credits: creditSvc}

	_, err := svc.CreateRequest(ctx, CreateRequestInput{
		Type:         store.TypeAccess,
		Kind:         policy.KindURL,
		Value:        "https://example.com/paid",
		EnrollmentID: "device-credits",
		Reason:       "need it",
	})
	if !errors.Is(err, store.ErrInsufficientCredits) {
		t.Fatalf("expected insufficient credits, got %v", err)
	}

	if _, err := creditSvc.Gift(ctx, "device-credits", 2, ""); err != nil {
		t.Fatal(err)
	}
	req, err := svc.CreateRequest(ctx, CreateRequestInput{
		Type:         store.TypeAccess,
		Kind:         policy.KindURL,
		Value:        "https://example.com/paid",
		EnrollmentID: "device-credits",
		Reason:       "need it",
	})
	if err != nil {
		t.Fatal(err)
	}
	bal, _ := creditSvc.Balance(ctx, "device-credits")
	if bal.Balance != 1 {
		t.Fatalf("balance after spend=%d want 1", bal.Balance)
	}

	if _, err := svc.Decide(ctx, DecideInput{RequestID: req.ID, Approve: false}); err != nil {
		t.Fatal(err)
	}
	bal, _ = creditSvc.Balance(ctx, "device-credits")
	if bal.Balance != 2 {
		t.Fatalf("balance after deny refund=%d want 2", bal.Balance)
	}
}

func TestGeneralRequestIsFree(t *testing.T) {
	ctx := context.Background()
	mem := memory.New()
	stub := &mdm.StubEnqueuer{}
	creditSvc := &credits.Service{Store: mem, AccessCost: 1}
	svc := &Service{Store: mem, Enqueue: stub, PortalURL: "http://localhost:8080", Credits: creditSvc}

	req, err := svc.CreateRequest(ctx, CreateRequestInput{
		Type:         store.TypeGeneral,
		Value:        "How do I print?",
		EnrollmentID: "device-free",
	})
	if err != nil {
		t.Fatal(err)
	}
	if req.Status != store.StatusPending {
		t.Fatalf("status=%s", req.Status)
	}
	bal, _ := creditSvc.Balance(ctx, "device-free")
	if bal.Balance != 0 {
		t.Fatalf("general should not spend, balance=%d", bal.Balance)
	}
}

func assertContains(t *testing.T, list []string, want string) {
	t.Helper()
	for _, v := range list {
		if v == want {
			return
		}
	}
	t.Fatalf("expected %q in %v", want, list)
}
