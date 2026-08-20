package httpapi

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dwdmsh/school-mdm/internal/activity"
	"github.com/dwdmsh/school-mdm/internal/config"
	"github.com/dwdmsh/school-mdm/internal/devicepush"
	"github.com/dwdmsh/school-mdm/internal/mdm"
	"github.com/dwdmsh/school-mdm/internal/store"
	"github.com/dwdmsh/school-mdm/internal/store/memory"
	"github.com/dwdmsh/school-mdm/internal/timers"
)

func sampleMobileconfig(identifier string) string {
	xml := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>PayloadDisplayName</key>
	<string>School Wi-Fi</string>
	<key>PayloadIdentifier</key>
	<string>` + identifier + `</string>
	<key>PayloadType</key>
	<string>Configuration</string>
	<key>PayloadUUID</key>
	<string>11111111-1111-4111-8111-111111111111</string>
	<key>PayloadVersion</key>
	<integer>1</integer>
	<key>PayloadContent</key>
	<array/>
</dict>
</plist>`
	return base64.StdEncoding.EncodeToString([]byte(xml))
}

func TestCustomProfileCRUDAndAssign(t *testing.T) {
	mem := memory.New()
	if err := mem.EnsureDevice(t.Context(), "iphone-1"); err != nil {
		t.Fatal(err)
	}
	stub := &mdm.StubEnqueuer{}
	push := &devicepush.Service{Store: mem, Enqueue: stub, PortalURL: "https://example.test"}
	api := &API{
		Cfg:      config.Config{AdminTokens: []string{"dev-admin-token"}},
		Store:    mem,
		Activity: &activity.Logger{Store: mem},
		Push:     push,
		Enqueue:  stub,
		Timers:   &timers.Service{Store: mem, Push: push},
	}
	mux := http.NewServeMux()
	api.Mount(mux)

	body, _ := json.Marshal(map[string]any{
		"name":           "Wi-Fi",
		"filename":       "wifi.mobileconfig",
		"payload_base64": sampleMobileconfig("com.example.wifi"),
	})
	req := httptest.NewRequest(http.MethodPost, "/api/profiles", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer dev-admin-token")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	var created store.CustomProfile
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.PayloadIdentifier != "com.example.wifi" {
		t.Fatalf("identifier %q", created.PayloadIdentifier)
	}

	assign, _ := json.Marshal(map[string]any{"scope": "device", "enrollment_id": "iphone-1"})
	req = httptest.NewRequest(http.MethodPost, "/api/profiles/"+created.ID+"/assignments", bytes.NewReader(assign))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer dev-admin-token")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("assign %d %s", rec.Code, rec.Body.String())
	}

	foundInstall := false
	for _, c := range stub.Snapshot() {
		if c.EnrollmentID == "iphone-1" && bytes.Contains(c.Profile, []byte("com.example.wifi")) {
			foundInstall = true
			break
		}
	}
	if !foundInstall {
		t.Fatalf("expected InstallProfile for custom profile, got %+v", stub.Snapshot())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/profiles?enrollment_id=iphone-1", nil)
	req.Header.Set("Authorization", "Bearer dev-admin-token")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list for device %d %s", rec.Code, rec.Body.String())
	}
	var forDevice []store.CustomProfile
	if err := json.Unmarshal(rec.Body.Bytes(), &forDevice); err != nil {
		t.Fatal(err)
	}
	if len(forDevice) != 1 {
		t.Fatalf("want 1 profile for device, got %#v", forDevice)
	}

	timerBody, _ := json.Marshal(map[string]any{
		"name":        "wifi off",
		"action":      "remove",
		"profile_ids": []string{created.ID},
		"device_ids":  []string{"iphone-1"},
		"schedule":    "once",
		"run_at":      "2026-08-21T08:00:00Z",
	})
	req = httptest.NewRequest(http.MethodPost, "/api/timers", bytes.NewReader(timerBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer dev-admin-token")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("timer create %d %s", rec.Code, rec.Body.String())
	}
	var timer map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &timer); err != nil {
		t.Fatal(err)
	}
	tid, _ := timer["id"].(string)
	req = httptest.NewRequest(http.MethodPost, "/api/timers/"+tid+"/run", nil)
	req.Header.Set("Authorization", "Bearer dev-admin-token")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("timer run %d %s", rec.Code, rec.Body.String())
	}
	assigns, err := mem.ListCustomProfileAssignments(t.Context(), created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(assigns) != 0 {
		t.Fatalf("expected unassigned after timer remove, got %#v", assigns)
	}
}
