package httpapi

import (
	"encoding/json"
	"errors"
	"html/template"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/dwdmsh/school-mdm/internal/appmeta"
	"github.com/dwdmsh/school-mdm/internal/approvals"
	"github.com/dwdmsh/school-mdm/internal/config"
	"github.com/dwdmsh/school-mdm/internal/mdm"
	"github.com/dwdmsh/school-mdm/internal/policy"
	"github.com/dwdmsh/school-mdm/internal/store"
)

// API serves product HTTP endpoints.
type API struct {
	Cfg     config.Config
	Service *approvals.Service
	Catalog *appmeta.Catalog
	Store   store.Store
	Stub    *mdm.StubEnqueuer
	Log     *slog.Logger
}

// Mount registers routes on mux.
func (a *API) Mount(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", a.handleHealthz)
	mux.HandleFunc("GET /api/allowlist", a.handleAllowlist)
	mux.HandleFunc("GET /api/allowances", a.requireAdmin(a.handleListAllowances))
	mux.HandleFunc("POST /api/allowances", a.requireAdmin(a.handleCreateAllowance))
	mux.HandleFunc("GET /api/devices", a.requireAdmin(a.handleListDevices))
	mux.HandleFunc("GET /api/groups", a.requireAdmin(a.handleListGroups))
	mux.HandleFunc("POST /api/groups", a.requireAdmin(a.handleCreateGroup))
	mux.HandleFunc("GET /api/groups/{id}", a.requireAdmin(a.handleGetGroup))
	mux.HandleFunc("PATCH /api/groups/{id}", a.requireAdmin(a.handleUpdateGroup))
	mux.HandleFunc("DELETE /api/groups/{id}", a.requireAdmin(a.handleDeleteGroup))
	mux.HandleFunc("GET /api/groups/{id}/members", a.requireAdmin(a.handleListGroupMembers))
	mux.HandleFunc("PUT /api/groups/{id}/members", a.requireAdmin(a.handleSetGroupMembers))
	mux.HandleFunc("GET /api/apps/search", a.handleAppSearch)
	mux.HandleFunc("GET /api/apps/{bundleID}", a.handleAppLookup)
	mux.HandleFunc("POST /api/requests", a.handleCreateRequest)
	mux.HandleFunc("GET /api/requests", a.requireAdmin(a.handleListRequests))
	mux.HandleFunc("POST /api/requests/{id}/approve", a.requireAdmin(a.handleApprove))
	mux.HandleFunc("POST /api/requests/{id}/deny", a.requireAdmin(a.handleDeny))
	mux.HandleFunc("GET /api/stub-commands", a.requireAdmin(a.handleStubCommands))
	mux.HandleFunc("GET /", a.handleHome)
	mux.HandleFunc("GET /d/{deviceID}", a.handleDevicePortal)
	mux.HandleFunc("GET /admin", a.handleAdminPage)
}

func (a *API) handleHealthz(w http.ResponseWriter, r *http.Request) {
	if err := a.Store.Ping(r.Context()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":    true,
		"store": a.Store.Kind(),
		"time":  time.Now().UTC().Format(time.RFC3339),
	})
}

func (a *API) handleAllowlist(w http.ResponseWriter, r *http.Request) {
	enrollment := r.URL.Query().Get("enrollment_id")
	apps, urls, err := a.Service.EffectiveAllowlist(r.Context(), enrollment)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"apps": apps, "urls": urls})
}

func (a *API) handleAppSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if a.Catalog == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "app catalog unavailable"})
		return
	}
	list, err := a.Catalog.Search(r.Context(), q, 12)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	if list == nil {
		list = []store.AppMeta{}
	}
	writeJSON(w, http.StatusOK, list)
}

func (a *API) handleAppLookup(w http.ResponseWriter, r *http.Request) {
	bundleID := r.PathValue("bundleID")
	if a.Catalog == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "app catalog unavailable"})
		return
	}
	meta, err := a.Catalog.LookupBundle(r.Context(), bundleID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, meta)
}

type createRequestBody struct {
	Type         string `json:"type"`
	Kind         string `json:"kind"`
	Value        string `json:"value"`
	EnrollmentID string `json:"enrollment_id"`
	Reason       string `json:"reason"`
}

func (a *API) handleCreateRequest(w http.ResponseWriter, r *http.Request) {
	var body createRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if body.EnrollmentID == "" {
		body.EnrollmentID = r.Header.Get("X-Device-ID")
	}
	req, err := a.Service.CreateRequest(r.Context(), approvals.CreateRequestInput{
		Type:         store.RequestType(body.Type),
		Kind:         policy.Kind(body.Kind),
		Value:        body.Value,
		EnrollmentID: body.EnrollmentID,
		Reason:       body.Reason,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if req.Type == store.TypeAccess && req.TargetKind == policy.KindApp && a.Catalog != nil {
		_, _ = a.Catalog.LookupBundle(r.Context(), req.Value)
	}
	writeJSON(w, http.StatusCreated, req)
}

type decideBody struct {
	Duration string `json:"duration"`
	Scope    string `json:"scope"`
	GroupID  string `json:"group_id"`
}

func (a *API) handleApprove(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var body decideBody
	_ = json.NewDecoder(r.Body).Decode(&body)
	req, err := a.Service.Decide(r.Context(), approvals.DecideInput{
		RequestID: id,
		Approve:   true,
		Duration:  body.Duration,
		Scope:     body.Scope,
		GroupID:   body.GroupID,
	})
	if err != nil {
		writeDecideErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, req)
}

func (a *API) handleDeny(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	req, err := a.Service.Decide(r.Context(), approvals.DecideInput{
		RequestID: id,
		Approve:   false,
	})
	if err != nil {
		writeDecideErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, req)
}

func (a *API) handleStubCommands(w http.ResponseWriter, r *http.Request) {
	if a.Stub == nil {
		writeJSON(w, http.StatusOK, []any{})
		return
	}
	cmds := a.Stub.Snapshot()
	type view struct {
		EnrollmentID   string    `json:"enrollment_id"`
		At             time.Time `json:"at"`
		ProfileBytes   int       `json:"profile_bytes"`
		ProfilePreview string    `json:"profile_preview"`
	}
	out := make([]view, 0, len(cmds))
	for _, c := range cmds {
		preview := string(c.Profile)
		if len(preview) > 400 {
			preview = preview[:400] + "…"
		}
		out = append(out, view{
			EnrollmentID:   c.EnrollmentID,
			At:             c.At,
			ProfileBytes:   len(c.Profile),
			ProfilePreview: preview,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *API) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		token := strings.TrimPrefix(auth, "Bearer ")
		token = strings.TrimSpace(token)
		if token == "" {
			token = r.URL.Query().Get("token")
		}
		if !a.Cfg.ValidAdminToken(token) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		next(w, r)
	}
}

func (a *API) handleHome(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = homeTmpl.Execute(w, nil)
}

func (a *API) handleDevicePortal(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = portalTmpl.Execute(w, map[string]any{
		"DeviceID": r.PathValue("deviceID"),
		"URL":      r.URL.Query().Get("url"),
	})
}

func (a *API) handleAdminPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	token := r.URL.Query().Get("token")
	if token == "" && len(a.Cfg.AdminTokens) > 0 {
		token = a.Cfg.AdminTokens[0]
	}
	if err := adminTmpl.Execute(w, map[string]string{"Token": token}); err != nil {
		a.Log.Error("admin template", "err", err)
	}
}

func writeDecideErr(w http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
}

func writeErr(w http.ResponseWriter, err error) {
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func toJSON(v any) template.JS {
	b, err := json.Marshal(v)
	if err != nil {
		return template.JS("null")
	}
	return template.JS(b)
}

func mustTemplate(name, body string) *template.Template {
	return template.Must(template.New(name).Funcs(template.FuncMap{
		"json": toJSON,
	}).Parse(body))
}
