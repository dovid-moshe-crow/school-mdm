package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dwdmsh/school-mdm/internal/activity"
	"github.com/dwdmsh/school-mdm/internal/config"
	"github.com/dwdmsh/school-mdm/internal/store/memory"
	"github.com/dwdmsh/school-mdm/internal/webhooks"
)

func TestOpenAPIIncludesAdminAndWebhooks(t *testing.T) {
	spec := OpenAPISpec()
	paths, _ := spec["paths"].(map[string]any)
	if _, ok := paths["/api/mdm/devices/{id}/lock"]; !ok {
		t.Fatal("missing lock path")
	}
	if _, ok := paths["/api/webhooks"]; !ok {
		t.Fatal("missing webhooks path")
	}
	if _, ok := paths["/api/auth/google/start"]; !ok {
		t.Fatal("missing google start path")
	}
	if _, ok := paths["/api/timers"]; !ok {
		t.Fatal("missing timers path")
	}
	wh := paths["/api/webhooks"].(map[string]any)
	post := wh["post"].(map[string]any)
	sec, _ := post["security"].([]map[string]any)
	if len(sec) == 0 {
		t.Fatal("create webhook should require bearer")
	}
}

func TestWebhookCRUDAndOpenAPIRoute(t *testing.T) {
	mem := memory.New()
	svc := webhooks.New(mem, nil)
	svc.Sync = true
	svc.Attempts = 1
	api := &API{
		Cfg:      config.Config{AdminTokens: []string{"dev-admin-token"}},
		Store:    mem,
		Activity: &activity.Logger{Store: mem, Webhooks: svc},
		Webhooks: svc,
	}
	mux := http.NewServeMux()
	api.Mount(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/openapi.json", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("openapi status %d", rec.Code)
	}

	// unauthenticated create is 401
	req = httptest.NewRequest(http.MethodPost, "/api/webhooks", strings.NewReader(`{"url":"http://127.0.0.1:1/h"}`))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauth create status %d", rec.Code)
	}

	recv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer recv.Close()

	body, _ := json.Marshal(map[string]any{"url": recv.URL, "events": []string{"webhooks.ping"}, "description": "test"})
	req = httptest.NewRequest(http.MethodPost, "/api/webhooks", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer dev-admin-token")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	var ep map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &ep); err != nil {
		t.Fatal(err)
	}
	id, _ := ep["id"].(string)
	if id == "" || ep["secret"] == "" {
		t.Fatalf("endpoint %+v", ep)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/webhooks/"+id+"/test", nil)
	req.Header.Set("Authorization", "Bearer dev-admin-token")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("test %d %s", rec.Code, rec.Body.String())
	}
	raw, _ := io.ReadAll(rec.Body)
	if !bytes.Contains(raw, []byte(`"status":"success"`)) && !bytes.Contains(raw, []byte(`"status": "success"`)) {
		t.Fatalf("delivery %s", raw)
	}
}
