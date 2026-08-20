package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dwdmsh/school-mdm/internal/activity"
	"github.com/dwdmsh/school-mdm/internal/config"
	"github.com/dwdmsh/school-mdm/internal/store"
	"github.com/dwdmsh/school-mdm/internal/store/memory"
	"github.com/dwdmsh/school-mdm/internal/timers"
)

func TestTimerCRUDAndRun(t *testing.T) {
	mem := memory.New()
	pack, err := mem.CreateWhitelistPack(t.Context(), store.WhitelistPack{Name: "games"})
	if err != nil {
		t.Fatal(err)
	}
	svc := &timers.Service{Store: mem, Loc: timers.Location()}
	api := &API{
		Cfg:      config.Config{AdminTokens: []string{"dev-admin-token"}},
		Store:    mem,
		Activity: &activity.Logger{Store: mem},
		Timers:   svc,
	}
	mux := http.NewServeMux()
	api.Mount(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/timers", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauth list %d", rec.Code)
	}

	runAt := time.Date(2026, 8, 20, 8, 0, 0, 0, time.UTC)
	body, _ := json.Marshal(map[string]any{
		"name":       "after school",
		"action":     "add",
		"pack_ids":   []string{pack.ID},
		"device_ids": []string{"iphone-1"},
		"schedule":   "once",
		"run_at":     runAt.Format(time.RFC3339),
	})
	req = httptest.NewRequest(http.MethodPost, "/api/timers", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer dev-admin-token")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	var created map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	id, _ := created["id"].(string)
	if id == "" {
		t.Fatalf("missing id: %v", created)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/timers/"+id+"/run", nil)
	req.Header.Set("Authorization", "Bearer dev-admin-token")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("run %d %s", rec.Code, rec.Body.String())
	}
	assigns, err := mem.ListWhitelistPackAssignments(t.Context(), pack.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(assigns) != 1 || assigns[0].TargetID != "iphone-1" {
		t.Fatalf("assignments %#v", assigns)
	}

	patch, _ := json.Marshal(map[string]any{"enabled": false})
	req = httptest.NewRequest(http.MethodPatch, "/api/timers/"+id, bytes.NewReader(patch))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer dev-admin-token")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch %d %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/timers", nil)
	req.Header.Set("Authorization", "Bearer dev-admin-token")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list %d %s", rec.Code, rec.Body.String())
	}
	var listed []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 {
		t.Fatalf("listed %d", len(listed))
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/timers/"+id, nil)
	req.Header.Set("Authorization", "Bearer dev-admin-token")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete %d %s", rec.Code, rec.Body.String())
	}
}
