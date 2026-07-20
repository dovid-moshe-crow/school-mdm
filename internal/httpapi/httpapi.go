package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/dwdmsh/school-mdm/internal/appmeta"
	"github.com/dwdmsh/school-mdm/internal/approvals"
	"github.com/dwdmsh/school-mdm/internal/config"
	"github.com/dwdmsh/school-mdm/internal/mdm"
	"github.com/dwdmsh/school-mdm/internal/policy"
	"github.com/dwdmsh/school-mdm/internal/store"
	"github.com/dwdmsh/school-mdm/internal/webui"
)

// API serves product HTTP endpoints.
type API struct {
	Cfg     config.Config
	Service *approvals.Service
	Catalog *appmeta.Catalog
	Store   store.Store
	Stub    *mdm.StubEnqueuer
	Log     *slog.Logger

	accessMu    sync.Mutex
	accessCache map[string]cachedAccessIndex
}

// Mount registers routes on mux.
func (a *API) Mount(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", a.handleHealthz)
	mux.HandleFunc("GET /api/allowlist", a.handleAllowlist)
	mux.HandleFunc("GET /api/allowances", a.handleListAllowances)
	mux.HandleFunc("POST /api/allowances", a.handleCreateAllowance)
	mux.HandleFunc("DELETE /api/allowances", a.handleDeleteAllowance)
	mux.HandleFunc("GET /api/devices", a.handleListDevices)
	mux.HandleFunc("PATCH /api/devices/{id}", a.handleUpdateDevice)
	mux.HandleFunc("GET /api/groups", a.handleListGroups)
	mux.HandleFunc("POST /api/groups", a.handleCreateGroup)
	mux.HandleFunc("GET /api/groups/{id}", a.handleGetGroup)
	mux.HandleFunc("PATCH /api/groups/{id}", a.handleUpdateGroup)
	mux.HandleFunc("DELETE /api/groups/{id}", a.handleDeleteGroup)
	mux.HandleFunc("GET /api/groups/{id}/members", a.handleListGroupMembers)
	mux.HandleFunc("PUT /api/groups/{id}/members", a.handleSetGroupMembers)
	mux.HandleFunc("GET /api/apps/search", a.handleAppSearch)
	mux.HandleFunc("GET /api/apps/{bundleID}", a.handleAppLookup)
	mux.HandleFunc("GET /api/access-status", a.handleAccessStatus)
	mux.HandleFunc("GET /api/device/{deviceID}/requests", a.handleDeviceRequests)
	mux.HandleFunc("POST /api/device/{deviceID}/requests/{id}/messages", a.handleDevicePostMessage)
	mux.HandleFunc("POST /api/requests", a.handleCreateRequest)
	mux.HandleFunc("GET /api/requests", a.handleListRequests)
	mux.HandleFunc("GET /api/requests/{id}", a.handleGetRequest)
	mux.HandleFunc("GET /api/requests/{id}/messages", a.handleListMessages)
	mux.HandleFunc("POST /api/requests/{id}/messages", a.handleAdminPostMessage)
	mux.HandleFunc("POST /api/requests/{id}/approve", a.handleApprove)
	mux.HandleFunc("POST /api/requests/{id}/deny", a.handleDeny)
	mux.HandleFunc("GET /api/stub-commands", a.handleStubCommands)
	mux.Handle("/", webui.Handler())
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
	enrollment := strings.TrimSpace(r.URL.Query().Get("enrollment_id"))
	log := a.Log
	if log == nil {
		log = slog.Default()
	}
	log.Info("GET /api/apps/search", "q", q, "enrollment_id", enrollment)

	if a.Catalog == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "app catalog unavailable"})
		return
	}
	start := time.Now()
	list, err := a.Catalog.Search(r.Context(), q, 25)
	if err != nil {
		log.Error("app search handler failed", "q", q, "err", err, "ms", time.Since(start).Milliseconds())
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	if list == nil {
		list = []store.AppMeta{}
	}
	annotateStart := time.Now()
	out := a.annotateApps(r, list, enrollment)
	annotateMS := time.Since(annotateStart).Milliseconds()
	log.Info("GET /api/apps/search done",
		"q", q,
		"enrollment_id", enrollment,
		"results", len(out),
		"annotate_ms", annotateMS,
		"ms", time.Since(start).Milliseconds(),
	)
	writeJSON(w, http.StatusOK, out)
}

func (a *API) handleAppLookup(w http.ResponseWriter, r *http.Request) {
	bundleID := r.PathValue("bundleID")
	if a.Catalog == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "app catalog unavailable"})
		return
	}
	refresh := r.URL.Query().Get("refresh") == "1" || r.URL.Query().Get("full") == "1"
	meta, err := a.Catalog.LookupBundleOpt(r.Context(), bundleID, refresh)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	row := appMetaJSON(meta)
	if enrollment := strings.TrimSpace(r.URL.Query().Get("enrollment_id")); enrollment != "" {
		if st, err := a.accessStatus(r, enrollment, policy.KindApp, meta.BundleID); err == nil {
			row["access_status"] = st
		}
	}
	writeJSON(w, http.StatusOK, row)
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
	a.invalidateAccessIndex(req.EnrollmentID)
	writeJSON(w, http.StatusCreated, req)
}

func (a *API) handleGetRequest(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	req, err := a.Store.GetRequest(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeErr(w, err)
		return
	}
	item := a.enrichRequest(r, req)
	writeJSON(w, http.StatusOK, item)
}

func (a *API) handleListMessages(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	// Optional device ownership check for portal privacy
	if device := strings.TrimSpace(r.URL.Query().Get("enrollment_id")); device != "" {
		req, err := a.Store.GetRequest(r.Context(), id)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
				return
			}
			writeErr(w, err)
			return
		}
		if req.EnrollmentID != device {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "device does not own this request"})
			return
		}
	}
	msgs, err := a.Store.ListRequestMessages(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, msgs)
}

type postMessageBody struct {
	Body string `json:"body"`
}

func (a *API) handleAdminPostMessage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var body postMessageBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	msg, err := a.Service.PostMessage(r.Context(), approvals.PostMessageInput{
		RequestID:  id,
		AuthorRole: store.AuthorAdmin,
		Body:       body.Body,
	})
	if err != nil {
		writeDecideErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, msg)
}

func (a *API) handleDevicePostMessage(w http.ResponseWriter, r *http.Request) {
	deviceID := strings.TrimSpace(r.PathValue("deviceID"))
	id := r.PathValue("id")
	var body postMessageBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	msg, err := a.Service.PostMessage(r.Context(), approvals.PostMessageInput{
		RequestID:    id,
		AuthorRole:   store.AuthorStudent,
		Body:         body.Body,
		EnrollmentID: deviceID,
	})
	if err != nil {
		writeDecideErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, msg)
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
	a.invalidateAccessIndex(req.EnrollmentID)
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
	a.invalidateAccessIndex(req.EnrollmentID)
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
