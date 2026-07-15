package approvals

import (
	"context"
	"testing"

	"github.com/dwdmsh/school-mdm/internal/mdm"
	"github.com/dwdmsh/school-mdm/internal/policy"
	"github.com/dwdmsh/school-mdm/internal/store"
	"github.com/dwdmsh/school-mdm/internal/store/memory"
)

func TestApproveCreatesGrantAndEnqueuesProfile(t *testing.T) {
	ctx := context.Background()
	mem := memory.New()
	stub := &mdm.StubEnqueuer{}
	svc := &Service{Store: mem, Enqueue: stub, PortalURL: "http://localhost:8080"}

	req, err := svc.CreateRequest(ctx, CreateRequestInput{
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

	apps, urls, err := svc.EffectiveAllowlist(ctx, "device-1")
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, urls, "example.com/lesson")
	assertContains(t, apps, "com.apple.webapp")

	cmds := stub.Snapshot()
	if len(cmds) != 1 {
		t.Fatalf("expected 1 stub command, got %d", len(cmds))
	}
	if cmds[0].EnrollmentID != "device-1" {
		t.Fatalf("enrollment=%s", cmds[0].EnrollmentID)
	}
	if len(cmds[0].Profile) == 0 {
		t.Fatal("empty profile")
	}
}

func TestDenyDoesNotEnqueue(t *testing.T) {
	ctx := context.Background()
	mem := memory.New()
	stub := &mdm.StubEnqueuer{}
	svc := &Service{Store: mem, Enqueue: stub, PortalURL: "http://localhost:8080"}

	req, err := svc.CreateRequest(ctx, CreateRequestInput{
		Kind:  policy.KindApp,
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

func assertContains(t *testing.T, list []string, want string) {
	t.Helper()
	for _, v := range list {
		if v == want {
			return
		}
	}
	t.Fatalf("expected %q in %v", want, list)
}
