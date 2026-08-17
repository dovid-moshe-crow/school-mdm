package httpapi

import (
	"io"
	"net/http"
	"strings"

	"github.com/dwdmsh/school-mdm/internal/activity"
	"github.com/dwdmsh/school-mdm/internal/store"
)

// PUT /api/mdm/vpp/token — upload Apps & Books content token (raw text/bytes).
func (a *API) handleVPPTokenPut(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil || len(strings.TrimSpace(string(body))) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "vpp token body required"})
		return
	}
	filename := strings.TrimSpace(r.Header.Get("X-Filename"))
	if filename == "" {
		filename = r.URL.Query().Get("filename")
	}
	settings, err := a.Store.GetMDMSettings(r.Context())
	if err != nil {
		writeErr(w, err)
		return
	}
	settings.VPPToken = body
	settings.VPPTokenFilename = filename
	updated, err := a.Store.UpsertMDMSettings(r.Context(), settings)
	if err != nil {
		writeErr(w, err)
		return
	}
	actorType, actor := a.adminActor(r)
	a.audit(r, activity.Event{
		Category: store.ActivityCategoryABM, Action: "vpp_token_upload",
		ActorType: actorType, Actor: actor,
		Result: store.ActivityResultOK, Summary: "הועלה תוכן Apps & Books (VPP)",
		Detail: map[string]any{"filename": updated.VPPTokenFilename},
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":                   true,
		"has_vpp_token":        updated.HasVPPToken,
		"vpp_token_filename":   updated.VPPTokenFilename,
		"vpp_token_updated_at": updated.VPPTokenUpdatedAt,
	})
}

// DELETE /api/mdm/vpp/token
func (a *API) handleVPPTokenDelete(w http.ResponseWriter, r *http.Request) {
	settings, err := a.Store.GetMDMSettings(r.Context())
	if err != nil {
		writeErr(w, err)
		return
	}
	settings.VPPToken = []byte{}
	settings.VPPTokenFilename = ""
	settings.VPPTokenUpdatedAt = nil
	updated, err := a.Store.UpsertMDMSettings(r.Context(), settings)
	if err != nil {
		writeErr(w, err)
		return
	}
	a.auditAdmin(r, store.ActivityCategoryABM, "vpp_token_delete", "נמחק תוכן Apps & Books (VPP)", nil, "", "")
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "has_vpp_token": updated.HasVPPToken})
}

// POST /api/mdm/devices/{id}/install-companion — push KFilter install + managed config.
func (a *API) handleMDMInstallCompanion(w http.ResponseWriter, r *http.Request) {
	if a.Push == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "device push unavailable"})
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "device id required"})
		return
	}
	err := a.Push.EnsureCompanionApp(r.Context(), id)
	a.auditMDM(r, "install_companion", id, "התקנת אפליקציית KFilter", err, nil)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "queued"})
}

// POST /api/mdm/devices/{id}/configure-companion — Managed App Config only (TestFlight + MDM test).
func (a *API) handleMDMConfigureCompanion(w http.ResponseWriter, r *http.Request) {
	if a.Push == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "device push unavailable"})
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "device id required"})
		return
	}
	err := a.Push.PushCompanionConfig(r.Context(), id)
	a.auditMDM(r, "configure_companion", id, "שליחת הגדרת KFilter (Managed App Config)", err, map[string]any{
		"enrollment_id": id,
		"keys":          []string{"enrollment_id", "portal_base_url"},
	})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "queued"})
}
