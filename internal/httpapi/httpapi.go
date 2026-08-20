package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/dwdmsh/school-mdm/internal/abm"
	"github.com/dwdmsh/school-mdm/internal/activity"
	"github.com/dwdmsh/school-mdm/internal/appmeta"
	"github.com/dwdmsh/school-mdm/internal/approvals"
	"github.com/dwdmsh/school-mdm/internal/config"
	"github.com/dwdmsh/school-mdm/internal/credits"
	"github.com/dwdmsh/school-mdm/internal/devicepush"
	"github.com/dwdmsh/school-mdm/internal/mdm"
	"github.com/dwdmsh/school-mdm/internal/mdmstore"
	"github.com/dwdmsh/school-mdm/internal/notify"
	"github.com/dwdmsh/school-mdm/internal/policy"
	"github.com/dwdmsh/school-mdm/internal/store"
	"github.com/dwdmsh/school-mdm/internal/timers"
	"github.com/dwdmsh/school-mdm/internal/webhooks"
	"github.com/dwdmsh/school-mdm/internal/webui"
)

// API serves product HTTP endpoints.
type API struct {
	Cfg      config.Config
	Service  *approvals.Service
	Credits  *credits.Service
	Catalog  *appmeta.Catalog
	Store    store.Store
	MDMStore mdmstore.Store
	Push     *devicepush.Service
	Enqueue  mdm.CommandEnqueuer
	Stub     *mdm.StubEnqueuer
	ABM      *abm.Service
	Notify   *notify.Service
	Activity *activity.Logger
	Webhooks *webhooks.Service
	Timers   *timers.Service
	Log      *slog.Logger

	accessMu    sync.Mutex
	accessCache map[string]cachedAccessIndex
}

// Mount registers routes on mux.
func (a *API) Mount(mux *http.ServeMux) {
	admin := func(pattern string, h http.HandlerFunc) {
		mux.HandleFunc(pattern, a.requireAdmin(h))
	}

	mux.HandleFunc("GET /healthz", a.handleHealthz)
	mux.HandleFunc("GET /api/openapi.json", a.handleOpenAPI)
	mux.HandleFunc("GET /api/auth/config", a.handleAuthConfig)
	mux.HandleFunc("GET /api/auth/me", a.handleAuthMe)
	mux.HandleFunc("POST /api/auth/logout", a.handleAuthLogout)
	mux.HandleFunc("GET /api/auth/google/start", a.handleAuthGoogleStart)
	mux.HandleFunc("GET /api/auth/google/callback", a.handleAuthGoogleCallback)

	mux.HandleFunc("GET /api/allowlist", a.handleAllowlist)
	admin("GET /api/allowances", a.handleListAllowances)
	admin("POST /api/allowances", a.handleCreateAllowance)
	admin("DELETE /api/allowances", a.handleDeleteAllowance)
	admin("GET /api/system-allowlist", a.handleListSystemAllowlist)
	admin("POST /api/system-allowlist", a.handleUpsertSystemAllowlist)
	admin("PATCH /api/system-allowlist", a.handlePatchSystemAllowlist)
	admin("DELETE /api/system-allowlist", a.handleDeleteSystemAllowlist)
	admin("GET /api/devices", a.handleListDevices)
	admin("PATCH /api/devices/{id}", a.handleUpdateDevice)
	admin("GET /api/admin/activity", a.handleListActivity)
	admin("GET /api/packs", a.handleListPacks)
	admin("POST /api/packs", a.handleCreatePack)
	admin("GET /api/packs/{id}", a.handleGetPack)
	admin("PATCH /api/packs/{id}", a.handleUpdatePack)
	admin("DELETE /api/packs/{id}", a.handleDeletePack)
	admin("POST /api/packs/{id}/items", a.handleAddPackItem)
	admin("DELETE /api/packs/{id}/items", a.handleRemovePackItem)
	admin("POST /api/packs/{id}/assignments", a.handleAddPackAssignment)
	admin("DELETE /api/packs/{id}/assignments", a.handleRemovePackAssignment)

	admin("GET /api/profiles", a.handleListProfiles)
	admin("POST /api/profiles", a.handleCreateProfile)
	admin("GET /api/profiles/{id}", a.handleGetProfile)
	admin("PATCH /api/profiles/{id}", a.handleUpdateProfile)
	admin("DELETE /api/profiles/{id}", a.handleDeleteProfile)
	admin("PUT /api/profiles/{id}/payload", a.handleReplaceProfilePayload)
	admin("GET /api/profiles/{id}/file", a.handleDownloadProfile)
	admin("POST /api/profiles/{id}/assignments", a.handleAddProfileAssignment)
	admin("DELETE /api/profiles/{id}/assignments", a.handleRemoveProfileAssignment)

	mux.HandleFunc("GET /api/timers", a.requireAdmin(a.handleListTimers))
	mux.HandleFunc("POST /api/timers", a.requireAdmin(a.handleCreateTimer))
	mux.HandleFunc("POST /api/timers/run-due", a.requireAdmin(a.handleRunDueTimers))
	mux.HandleFunc("GET /api/timers/{id}", a.requireAdmin(a.handleGetTimer))
	mux.HandleFunc("PATCH /api/timers/{id}", a.requireAdmin(a.handleUpdateTimer))
	mux.HandleFunc("DELETE /api/timers/{id}", a.requireAdmin(a.handleDeleteTimer))
	mux.HandleFunc("POST /api/timers/{id}/run", a.requireAdmin(a.handleRunTimer))

	admin("GET /api/groups", a.handleListGroups)
	admin("POST /api/groups", a.handleCreateGroup)
	admin("GET /api/groups/{id}", a.handleGetGroup)
	admin("PATCH /api/groups/{id}", a.handleUpdateGroup)
	admin("DELETE /api/groups/{id}", a.handleDeleteGroup)
	admin("GET /api/groups/{id}/members", a.handleListGroupMembers)
	admin("PUT /api/groups/{id}/members", a.handleSetGroupMembers)
	mux.HandleFunc("GET /api/apps/search", a.handleAppSearch)
	mux.HandleFunc("GET /api/apps/lookup", a.handleAppLookupMany)
	mux.HandleFunc("GET /api/apps/{bundleID}", a.handleAppLookup)
	mux.HandleFunc("GET /api/access-status", a.handleAccessStatus)
	mux.HandleFunc("GET /api/device/{deviceID}/requests", a.handleDeviceRequests)
	mux.HandleFunc("POST /api/device/{deviceID}/requests/{id}/messages", a.handleDevicePostMessage)
	mux.HandleFunc("POST /api/device/{deviceID}/push-token", a.handleDevicePushToken)
	mux.HandleFunc("POST /api/devices/{id}/push-token", a.handleDevicePushTokenAlias)
	mux.HandleFunc("POST /api/requests", a.handleCreateRequest)
	admin("GET /api/requests", a.handleListRequests)
	admin("GET /api/requests/{id}", a.handleGetRequest)
	mux.HandleFunc("GET /api/requests/{id}/messages", a.handleListMessages)
	admin("POST /api/requests/{id}/messages", a.handleAdminPostMessage)
	admin("POST /api/requests/{id}/approve", a.handleApprove)
	admin("POST /api/requests/{id}/deny", a.handleDeny)
	admin("GET /api/stub-commands", a.handleStubCommands)

	mux.HandleFunc("GET /api/credits/balance", a.handleCreditBalance)
	mux.HandleFunc("GET /api/credits/packages", a.handleCreditPackages)
	mux.HandleFunc("GET /api/credits/settings", a.handleCreditSettings)
	mux.HandleFunc("POST /api/credits/checkout", a.handleCreditCheckout)
	mux.HandleFunc("POST /api/credits/confirm", a.handleCreditConfirm)
	mux.HandleFunc("GET /api/credits/fake-iframe", a.handleFakeIframe)
	mux.HandleFunc("POST /api/credits/fake-pay", a.handleFakePay)
	mux.HandleFunc("GET /api/credits/nedarim-bridge", a.handleNedarimBridge)
	mux.HandleFunc("POST /api/webhooks/nedarim", a.handleNedarimWebhook)
	mux.HandleFunc("POST /api/webhooks/nedarim/fake", a.handleNedarimWebhook)
	admin("POST /api/admin/credits/gift", a.handleAdminGiftCredits)
	admin("POST /api/admin/credits/adjust", a.handleAdminAdjustCredits)
	admin("GET /api/admin/credits", a.handleAdminListCredits)
	admin("GET /api/admin/credits/ledger", a.handleAdminCreditLedger)
	admin("GET /api/admin/credits/purchases", a.handleAdminListPurchases)
	admin("GET /api/admin/credits/settings", a.handleAdminGetCreditSettings)
	admin("PUT /api/admin/credits/settings", a.handleAdminPutCreditSettings)
	admin("GET /api/admin/credits/packages", a.handleAdminListPackages)
	admin("POST /api/admin/credits/packages", a.handleAdminCreatePackage)
	admin("PUT /api/admin/credits/packages/{id}", a.handleAdminUpdatePackage)
	admin("DELETE /api/admin/credits/packages/{id}", a.handleAdminDeletePackage)
	admin("GET /api/admin/credits/allotments", a.handleAdminListAllotments)
	admin("POST /api/admin/credits/allotments", a.handleAdminCreateAllotment)
	admin("PUT /api/admin/credits/allotments/{id}", a.handleAdminUpdateAllotment)
	admin("DELETE /api/admin/credits/allotments/{id}", a.handleAdminDeleteAllotment)
	admin("POST /api/admin/credits/allotments/run", a.handleAdminRunAllotments)

	// MDM admin (thin; requires Bearer admin token)
	mux.HandleFunc("GET /api/mdm/status", a.requireAdmin(a.handleMDMStatus))
	mux.HandleFunc("GET /api/mdm/devices", a.requireAdmin(a.handleMDMListDevices))
	mux.HandleFunc("GET /api/mdm/devices/{id}", a.requireAdmin(a.handleMDMGetDevice))
	mux.HandleFunc("DELETE /api/mdm/devices/{id}", a.requireAdmin(a.handleMDMDeleteDevice))
	mux.HandleFunc("POST /api/mdm/devices/{id}/push", a.requireAdmin(a.handleMDMPush))
	mux.HandleFunc("POST /api/mdm/devices/{id}/install-profile", a.requireAdmin(a.handleMDMInstallProfile))
	mux.HandleFunc("POST /api/mdm/devices/{id}/remove-profile", a.requireAdmin(a.handleMDMRemoveProfile))
	mux.HandleFunc("POST /api/mdm/devices/{id}/device-information", a.requireAdmin(a.handleMDMDeviceInformation))
	mux.HandleFunc("GET /api/mdm/devices/{id}/commands/{commandUUID}", a.requireAdmin(a.handleMDMCommandResult))
	mux.HandleFunc("POST /api/mdm/devices/{id}/profile-list", a.requireAdmin(a.handleMDMProfileList))
	mux.HandleFunc("POST /api/mdm/devices/{id}/installed-apps", a.requireAdmin(a.handleMDMInstalledApps))
	mux.HandleFunc("POST /api/mdm/devices/{id}/reconcile", a.requireAdmin(a.handleMDMReconcile))
	mux.HandleFunc("POST /api/mdm/devices/{id}/install-companion", a.requireAdmin(a.handleMDMInstallCompanion))
	mux.HandleFunc("POST /api/mdm/devices/{id}/configure-companion", a.requireAdmin(a.handleMDMConfigureCompanion))
	mux.HandleFunc("POST /api/mdm/devices/{id}/clear-allowlist", a.requireAdmin(a.handleMDMClearAllowlist))
	mux.HandleFunc("POST /api/mdm/devices/{id}/lock", a.requireAdmin(a.handleMDMLock))
	mux.HandleFunc("POST /api/mdm/devices/{id}/clear-passcode", a.requireAdmin(a.handleMDMClearPasscode))
	mux.HandleFunc("POST /api/mdm/devices/{id}/restart", a.requireAdmin(a.handleMDMRestart))
	mux.HandleFunc("POST /api/mdm/devices/{id}/shutdown", a.requireAdmin(a.handleMDMShutDown))
	mux.HandleFunc("POST /api/mdm/devices/{id}/erase", a.requireAdmin(a.handleMDMErase))
	mux.HandleFunc("POST /api/mdm/devices/{id}/lost-mode/enable", a.requireAdmin(a.handleMDMEnableLostMode))
	mux.HandleFunc("POST /api/mdm/devices/{id}/lost-mode/disable", a.requireAdmin(a.handleMDMDisableLostMode))
	mux.HandleFunc("POST /api/mdm/devices/{id}/lost-mode/play-sound", a.requireAdmin(a.handleMDMPlayLostModeSound))
	mux.HandleFunc("POST /api/mdm/devices/{id}/lost-mode/location", a.requireAdmin(a.handleMDMDeviceLocation))
	mux.HandleFunc("POST /api/mdm/devices/{id}/security-info", a.requireAdmin(a.handleMDMSecurityInfo))
	mux.HandleFunc("POST /api/mdm/devices/bulk", a.requireAdmin(a.handleMDMBulk))
	mux.HandleFunc("PUT /api/mdm/pushcert", a.requireAdmin(a.handleMDMPushCert))

	mux.HandleFunc("GET /api/mdm/abm/account", a.requireAdmin(a.handleABMAccount))
	mux.HandleFunc("GET /api/mdm/abm/settings", a.requireAdmin(a.handleABMSettingsGet))
	mux.HandleFunc("PUT /api/mdm/abm/settings", a.requireAdmin(a.handleABMSettingsPut))
	mux.HandleFunc("PUT /api/mdm/vpp/token", a.requireAdmin(a.handleVPPTokenPut))
	mux.HandleFunc("DELETE /api/mdm/vpp/token", a.requireAdmin(a.handleVPPTokenDelete))
	mux.HandleFunc("GET /api/mdm/abm/dep-names", a.requireAdmin(a.handleABMDEPNames))
	mux.HandleFunc("GET /api/mdm/abm/devices", a.requireAdmin(a.handleABMListDevices))
	mux.HandleFunc("POST /api/mdm/abm/sync", a.requireAdmin(a.handleABMSync))
	// Creating or replacing an Apple profile must stay admin-only.
	admin("GET /api/mdm/abm/profile", a.handleABMGetProfile)
	mux.HandleFunc("POST /api/mdm/abm/profile", a.requireAdmin(a.handleABMDefineProfile))
	mux.HandleFunc("POST /api/mdm/abm/assign", a.requireAdmin(a.handleABMAssignProfile))

	admin("GET /api/webhooks/events", a.handleWebhookEvents)
	mux.HandleFunc("GET /api/webhooks", a.requireAdmin(a.handleListWebhooks))
	mux.HandleFunc("POST /api/webhooks", a.requireAdmin(a.handleCreateWebhook))
	mux.HandleFunc("GET /api/webhooks/{id}", a.requireAdmin(a.handleGetWebhook))
	mux.HandleFunc("PATCH /api/webhooks/{id}", a.requireAdmin(a.handleUpdateWebhook))
	mux.HandleFunc("DELETE /api/webhooks/{id}", a.requireAdmin(a.handleDeleteWebhook))
	mux.HandleFunc("GET /api/webhooks/{id}/deliveries", a.requireAdmin(a.handleListWebhookDeliveries))
	mux.HandleFunc("POST /api/webhooks/{id}/test", a.requireAdmin(a.handleTestWebhook))

	mux.Handle("/", webui.Handler())
}

func (a *API) handleHealthz(w http.ResponseWriter, r *http.Request) {
	if err := a.Store.Ping(r.Context()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":          true,
		"store":       a.Store.Kind(),
		"mdm_enqueue": a.Cfg.MDMEnqueue,
		"mdm_live":    a.Cfg.MDMLive(),
		"time":        time.Now().UTC().Format(time.RFC3339),
	})
}

func (a *API) handleAllowlist(w http.ResponseWriter, r *http.Request) {
	enrollment := r.URL.Query().Get("enrollment_id")
	apps, urls, err := a.Service.EffectiveAllowlist(r.Context(), enrollment)
	if err != nil {
		writeErr(w, err)
		return
	}
	sys := a.enabledSystemKeys(r)
	if len(sys) > 0 {
		visible := make([]string, 0, len(apps))
		for _, app := range apps {
			if _, hide := sys[policy.AppKey(app)]; hide {
				continue
			}
			visible = append(visible, app)
		}
		apps = visible
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

func (a *API) handleAppLookupMany(w http.ResponseWriter, r *http.Request) {
	ids := r.URL.Query()["id"]
	if extra := strings.TrimSpace(r.URL.Query().Get("ids")); extra != "" {
		ids = append(ids, strings.Split(extra, ",")...)
	}
	if len(ids) > 80 {
		ids = ids[:80]
	}
	remote := r.URL.Query().Get("fetch") != "0"
	var list []store.AppMeta
	if a.Catalog != nil {
		list = a.Catalog.LookupMany(r.Context(), ids, remote)
	} else {
		seen := map[string]struct{}{}
		for _, raw := range ids {
			id := strings.TrimSpace(raw)
			if id == "" {
				continue
			}
			key := strings.ToLower(id)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			if a.Store != nil {
				if meta, err := a.Store.GetAppMeta(r.Context(), id); err == nil {
					list = append(list, meta)
					continue
				}
			}
			if meta, ok := appmeta.Known(id); ok {
				list = append(list, meta)
				continue
			}
			list = append(list, store.AppMeta{BundleID: id, Name: id, Source: "unknown"})
		}
	}
	if list == nil {
		list = []store.AppMeta{}
	}
	out := make([]map[string]any, 0, len(list))
	for _, m := range list {
		out = append(out, appMetaJSON(m))
	}
	writeJSON(w, http.StatusOK, out)
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
		status := http.StatusBadRequest
		if errors.Is(err, store.ErrInsufficientCredits) {
			status = http.StatusPaymentRequired
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	if req.Type == store.TypeAccess && req.TargetKind == policy.KindApp && a.Catalog != nil {
		_, _ = a.Catalog.LookupBundle(r.Context(), req.Value)
	}
	a.invalidateAccessIndex(req.EnrollmentID)
	a.audit(r, activity.Event{
		Category:     store.ActivityCategoryRequests,
		Action:       "request_create",
		ActorType:    store.ActivityActorDevice,
		Actor:        req.EnrollmentID,
		EnrollmentID: req.EnrollmentID,
		RequestID:    req.ID,
		Result:       store.ActivityResultOK,
		Summary:      "נוצרה בקשה חדשה",
		Detail: map[string]any{
			"type":  req.Type,
			"kind":  req.TargetKind,
			"value": req.Value,
		},
	})
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
	if !a.authorizedAdmin(r) {
		device := strings.TrimSpace(r.URL.Query().Get("enrollment_id"))
		if device == "" {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "admin authorization required"})
			return
		}
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
	actorType, actor := a.adminActor(r)
	a.audit(r, activity.Event{
		Category:     store.ActivityCategoryRequests,
		Action:       "request_approve",
		ActorType:    actorType,
		Actor:        actor,
		EnrollmentID: req.EnrollmentID,
		RequestID:    req.ID,
		Result:       store.ActivityResultOK,
		Summary:      "בקשה אושרה",
		Detail:       map[string]any{"scope": body.Scope, "duration": body.Duration, "group_id": body.GroupID},
	})
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
	actorType, actor := a.adminActor(r)
	a.audit(r, activity.Event{
		Category:     store.ActivityCategoryRequests,
		Action:       "request_deny",
		ActorType:    actorType,
		Actor:        actor,
		EnrollmentID: req.EnrollmentID,
		RequestID:    req.ID,
		Result:       store.ActivityResultOK,
		Summary:      "בקשה נדחתה",
	})
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
