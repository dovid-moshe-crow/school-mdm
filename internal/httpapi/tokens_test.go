package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dwdmsh/school-mdm/internal/config"
	"github.com/dwdmsh/school-mdm/internal/store/memory"
)

func TestAPITokensCreateAndAuthorize(t *testing.T) {
	api := &API{Cfg: config.Config{AdminTokens: []string{"env-token"}}, Store: memory.New()}
	mux := http.NewServeMux()
	api.Mount(mux)

	req := httptest.NewRequest(http.MethodPost, "/api/admin/tokens", strings.NewReader(`{"name":"scripts"}`))
	req.Header.Set("Authorization", "Bearer env-token")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		ID     string `json:"id"`
		Prefix string `json:"prefix"`
		Token  string `json:"token"`
		Name   string `json:"name"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.ID == "" || created.Token == "" || !strings.HasPrefix(created.Token, "smdm_") {
		t.Fatalf("created %#v", created)
	}
	if created.Name != "scripts" {
		t.Fatalf("name %q", created.Name)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/admin/tokens", nil)
	listReq.Header.Set("Authorization", "Bearer "+created.Token)
	listRec := httptest.NewRecorder()
	mux.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list with new token %d %s", listRec.Code, listRec.Body.String())
	}
	if strings.Contains(listRec.Body.String(), created.Token) {
		t.Fatal("list must not include the raw token")
	}

	del := httptest.NewRequest(http.MethodDelete, "/api/admin/tokens/"+created.ID, nil)
	del.Header.Set("Authorization", "Bearer env-token")
	delRec := httptest.NewRecorder()
	mux.ServeHTTP(delRec, del)
	if delRec.Code != http.StatusOK {
		t.Fatalf("delete %d %s", delRec.Code, delRec.Body.String())
	}

	again := httptest.NewRequest(http.MethodGet, "/api/devices", nil)
	again.Header.Set("Authorization", "Bearer "+created.Token)
	againRec := httptest.NewRecorder()
	mux.ServeHTTP(againRec, again)
	if againRec.Code != http.StatusUnauthorized {
		t.Fatalf("revoked token still worked: %d", againRec.Code)
	}
}
