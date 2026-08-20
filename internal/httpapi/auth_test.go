package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dwdmsh/school-mdm/internal/config"
	"github.com/dwdmsh/school-mdm/internal/store/memory"
)

func TestAdminRoutesRequireAuth(t *testing.T) {
	api := &API{Cfg: config.Config{AdminTokens: []string{"secret-token"}}, Store: memory.New()}
	mux := http.NewServeMux()
	api.Mount(mux)

	unauth := []string{
		"/api/devices",
		"/api/packs",
		"/api/profiles",
		"/api/groups",
		"/api/allowances",
		"/api/requests",
		"/api/admin/credits",
		"/api/stub-commands",
		"/api/webhooks/events",
		"/api/mdm/abm/profile",
	}
	for _, path := range unauth {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s unauth got %d", path, rec.Code)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/devices", nil)
	req.Header.Set("Authorization", "Bearer secret-token")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code == http.StatusUnauthorized {
		t.Fatal("bearer token should authorize /api/devices")
	}
}

func TestSessionCookieAuthorizesAdmin(t *testing.T) {
	api := &API{
		Cfg: config.Config{
			AdminTokens:   []string{"secret-token"},
			SessionSecret: "test-session-secret",
		},
		Store: memory.New(),
	}
	raw, err := api.signSession("admin@example.com", "Admin")
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	api.Mount(mux)
	req := httptest.NewRequest(http.MethodGet, "/api/devices", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: raw})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code == http.StatusUnauthorized {
		t.Fatal("session cookie should authorize /api/devices")
	}

	me := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	me.AddCookie(&http.Cookie{Name: sessionCookie, Value: raw})
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, me)
	if rec.Code != http.StatusOK {
		t.Fatalf("auth me %d %s", rec.Code, rec.Body.String())
	}
}

func TestAuthMeIgnoresBearerWhenGoogleEnabled(t *testing.T) {
	api := &API{
		Cfg: config.Config{
			AdminTokens:        []string{"secret-token"},
			GoogleClientID:     "id",
			GoogleClientSecret: "secret",
			SessionSecret:      "test-session-secret",
		},
		Store: memory.New(),
	}
	mux := http.NewServeMux()
	api.Mount(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	req.Header.Set("Authorization", "Bearer secret-token")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("bearer must not log into the UI when Google is on, got %d", rec.Code)
	}

	apiReq := httptest.NewRequest(http.MethodGet, "/api/devices", nil)
	apiReq.Header.Set("Authorization", "Bearer secret-token")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, apiReq)
	if rec.Code == http.StatusUnauthorized {
		t.Fatal("scripts can still use Bearer on APIs")
	}
}

func TestGoogleDisabledStart(t *testing.T) {
	api := &API{Cfg: config.Config{}, Store: memory.New()}
	mux := http.NewServeMux()
	api.Mount(mux)
	req := httptest.NewRequest(http.MethodGet, "/api/auth/google/start", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 got %d", rec.Code)
	}
}

func TestPublicRoutesStayOpen(t *testing.T) {
	api := &API{Cfg: config.Config{AdminTokens: []string{"secret-token"}}, Store: memory.New()}
	mux := http.NewServeMux()
	api.Mount(mux)
	for _, path := range []string{"/healthz", "/api/auth/config", "/api/openapi.json"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code == http.StatusUnauthorized {
			t.Fatalf("%s must stay public, got 401", path)
		}
	}
}
