package devicepush

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/dwdmsh/school-mdm/internal/mdm"
	"github.com/dwdmsh/school-mdm/internal/policy"
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

func TestEnsureCustomProfilesInstallsAndRemoves(t *testing.T) {
	ctx := context.Background()
	mem := memory.New()
	if err := mem.EnsureDevice(ctx, "dev-1"); err != nil {
		t.Fatal(err)
	}
	xml := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>PayloadDisplayName</key>
	<string>Wi-Fi</string>
	<key>PayloadIdentifier</key>
	<string>com.example.wifi</string>
	<key>PayloadType</key>
	<string>Configuration</string>
	<key>PayloadUUID</key>
	<string>11111111-1111-4111-8111-111111111111</string>
	<key>PayloadVersion</key>
	<integer>1</integer>
	<key>PayloadContent</key>
	<array/>
</dict>
</plist>`)
	p, err := mem.CreateCustomProfile(ctx, store.CustomProfile{Name: "Wi-Fi", Payload: xml})
	if err != nil {
		t.Fatal(err)
	}
	if err := mem.SetCustomProfileAssignment(ctx, store.CustomProfileAssignment{
		ProfileID: p.ID, TargetType: policy.TargetDevice, TargetID: "dev-1",
	}); err != nil {
		t.Fatal(err)
	}
	stub := &mdm.StubEnqueuer{}
	svc := &Service{Store: mem, Enqueue: stub, PortalURL: "https://example.test"}
	if err := svc.EnsureCustomProfiles(ctx, "dev-1"); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, c := range stub.Snapshot() {
		if bytes.Contains(c.Profile, []byte("com.example.wifi")) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected install, got %+v", stub.Snapshot())
	}

	if err := mem.RemoveCustomProfileAssignment(ctx, p.ID, policy.Target{Type: policy.TargetDevice, ID: "dev-1"}); err != nil {
		t.Fatal(err)
	}
	stub2 := &mdm.StubEnqueuer{}
	svc.Enqueue = stub2
	if err := svc.EnsureCustomProfiles(ctx, "dev-1"); err != nil {
		t.Fatal(err)
	}
	if len(stub2.Snapshot()) != 1 || stub2.Snapshot()[0].Identifier != "com.example.wifi" {
		t.Fatalf("expected remove of custom profile, got %+v", stub2.Snapshot())
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
