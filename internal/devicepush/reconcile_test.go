package devicepush

import (
	"context"
	"strings"
	"testing"

	"github.com/dwdmsh/school-mdm/internal/mdm"
	"github.com/dwdmsh/school-mdm/internal/profiles"
	"github.com/dwdmsh/school-mdm/internal/store"
	"github.com/dwdmsh/school-mdm/internal/store/memory"
)

func TestEnsureCompanionAppInstallsNotificationsProfile(t *testing.T) {
	ctx := context.Background()
	mem := memory.New()
	if _, err := mem.UpsertMDMSettings(ctx, store.MDMSettings{
		DepName:           "nanok",
		CompanionEnabled:  true,
		CompanionBundleID: "com.kfilter.portal",
	}); err != nil {
		t.Fatal(err)
	}
	stub := &mdm.StubEnqueuer{}
	svc := &Service{Store: mem, Enqueue: stub, PortalURL: "https://example.test"}
	if err := svc.EnsureCompanionApp(ctx, "dev-1"); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, c := range stub.Snapshot() {
		if strings.Contains(string(c.Profile), "com.apple.notificationsettings") &&
			strings.Contains(string(c.Profile), "com.kfilter.portal") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected notifications profile for KFilter, got %+v", stub.Snapshot())
	}
}

func TestEnsureCompanionAppDisabledRemovesNotificationsProfile(t *testing.T) {
	ctx := context.Background()
	mem := memory.New()
	if _, err := mem.UpsertMDMSettings(ctx, store.MDMSettings{
		DepName:          "nanok",
		CompanionEnabled: false,
	}); err != nil {
		t.Fatal(err)
	}
	stub := &mdm.StubEnqueuer{}
	svc := &Service{Store: mem, Enqueue: stub}
	if err := svc.EnsureCompanionApp(ctx, "dev-1"); err != nil {
		t.Fatal(err)
	}
	cmds := stub.Snapshot()
	if len(cmds) != 1 || cmds[0].Identifier != profiles.CompanionNotificationsIdentifier {
		t.Fatalf("expected remove of notifications profile, got %+v", cmds)
	}
}
