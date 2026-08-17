package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/dwdmsh/school-mdm/internal/mdm/commands"
	mdmenqueue "github.com/dwdmsh/school-mdm/internal/mdm/enqueue"
)

func (a *API) handleMDMStatus(w http.ResponseWriter, r *http.Request) {
	out := map[string]any{
		"enqueue":    a.Cfg.MDMEnqueue,
		"live":       a.Cfg.MDMLive(),
		"public_url": a.Cfg.MDMPublicURL,
		"topic":      a.Cfg.MDMTopic,
		"checkin":    a.Cfg.MDMCheckin,
	}
	if a.MDMStore != nil && a.Cfg.MDMTopic != "" {
		if info, err := a.MDMStore.GetPushCertInfo(r.Context(), a.Cfg.MDMTopic); err == nil && info != nil {
			out["push_cert"] = info
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *API) handleMDMListDevices(w http.ResponseWriter, r *http.Request) {
	if a.MDMStore == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "mdm store unavailable (set MDM_ENQUEUE=live)"})
		return
	}
	list, err := a.MDMStore.ListEnrollments(r.Context())
	if err != nil {
		writeErr(w, err)
		return
	}
	if list == nil {
		list = nil
	}
	writeJSON(w, http.StatusOK, list)
}

func (a *API) handleMDMGetDevice(w http.ResponseWriter, r *http.Request) {
	if a.MDMStore == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "mdm store unavailable"})
		return
	}
	e, err := a.MDMStore.GetEnrollment(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	if e == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "device not found"})
		return
	}
	hasPush, _ := a.Store.HasPushToken(r.Context(), e.ID)
	writeJSON(w, http.StatusOK, map[string]any{
		"id":                 e.ID,
		"device_id":          e.DeviceID,
		"serial_number":      e.SerialNumber,
		"type":               e.Type,
		"topic":              e.Topic,
		"push_magic":         e.PushMagic,
		"enabled":            e.Enabled,
		"token_update_tally": e.TokenUpdateTally,
		"last_seen_at":       e.LastSeenAt,
		"created_at":         e.CreatedAt,
		"updated_at":         e.UpdatedAt,
		"has_push_token":     hasPush,
	})
}

func (a *API) handleMDMDeleteDevice(w http.ResponseWriter, r *http.Request) {
	if a.MDMStore == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "mdm store unavailable"})
		return
	}
	if err := a.MDMStore.DeleteEnrollment(r.Context(), r.PathValue("id")); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
}

func (a *API) live() (*mdmenqueue.LiveEnqueuer, bool) {
	live, ok := a.Enqueue.(*mdmenqueue.LiveEnqueuer)
	return live, ok
}

func (a *API) handleMDMPush(w http.ResponseWriter, r *http.Request) {
	live, ok := a.live()
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "live enqueuer unavailable"})
		return
	}
	id := r.PathValue("id")
	err := live.Push(r.Context(), id)
	a.auditMDM(r, "push", id, "דחיפת פקודה למכשיר", err, nil)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "pushed"})
}

func (a *API) handleMDMInstallProfile(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil || len(body) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "profile payload required"})
		return
	}
	if a.Enqueue == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "enqueuer unavailable"})
		return
	}
	id := r.PathValue("id")
	err = a.Enqueue.InstallProfile(r.Context(), id, body)
	a.auditMDM(r, "install_profile", id, "הותקן פרופיל במכשיר", err, map[string]any{"bytes": len(body)})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "queued"})
}

func (a *API) handleMDMRemoveProfile(w http.ResponseWriter, r *http.Request) {
	live, ok := a.live()
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "live enqueuer unavailable"})
		return
	}
	var req struct {
		Identifier string `json:"identifier"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Identifier) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "identifier is required"})
		return
	}
	id := r.PathValue("id")
	err := live.RemoveProfile(r.Context(), id, req.Identifier)
	a.auditMDM(r, "remove_profile", id, "הוסר פרופיל מהמכשיר", err, map[string]any{"identifier": req.Identifier})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "queued"})
}

func (a *API) enqueueNamed(w http.ResponseWriter, r *http.Request, action, summary string, build func() ([]byte, error)) {
	live, ok := a.live()
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "live enqueuer unavailable"})
		return
	}
	id := r.PathValue("id")
	cmd, err := build()
	if err != nil {
		a.auditMDM(r, action, id, summary, err, nil)
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	err = live.EnqueueRaw(r.Context(), id, cmd)
	a.auditMDM(r, action, id, summary, err, nil)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "queued"})
}

func (a *API) handleMDMDeviceInformation(w http.ResponseWriter, r *http.Request) {
	live, ok := a.live()
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "live enqueuer unavailable"})
		return
	}
	id := r.PathValue("id")
	cmd, commandUUID, err := commands.DeviceInformation()
	if err != nil {
		a.auditMDM(r, "device_information", id, "בקשת מידע על המכשיר", err, nil)
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	err = live.EnqueueRaw(r.Context(), id, cmd)
	a.auditMDM(r, "device_information", id, "בקשת מידע על המכשיר", err, map[string]any{"command_uuid": commandUUID})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{
		"status":       "queued",
		"command_uuid": commandUUID,
	})
}

func (a *API) handleMDMCommandResult(w http.ResponseWriter, r *http.Request) {
	if a.MDMStore == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "mdm store unavailable"})
		return
	}
	res, err := a.MDMStore.GetCommandResult(r.Context(), r.PathValue("id"), r.PathValue("commandUUID"))
	if err != nil {
		writeErr(w, err)
		return
	}
	if res == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "command not found"})
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (a *API) handleMDMProfileList(w http.ResponseWriter, r *http.Request) {
	a.enqueueWithUUID(w, r, "profile_list", "בקשת רשימת פרופילים", commands.ProfileListWithUUID)
}

func (a *API) handleMDMInstalledApps(w http.ResponseWriter, r *http.Request) {
	a.enqueueWithUUID(w, r, "installed_apps", "בקשת רשימת אפליקציות", commands.InstalledApplicationListWithUUID)
}

func (a *API) handleMDMReconcile(w http.ResponseWriter, r *http.Request) {
	if a.Push == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "devicepush unavailable"})
		return
	}
	id := r.PathValue("id")
	err := a.Push.Reconcile(r.Context(), id)
	a.auditMDM(r, "reconcile", id, "סנכרון מדיניות למכשיר", err, nil)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "reconciled"})
}

func (a *API) handleMDMClearAllowlist(w http.ResponseWriter, r *http.Request) {
	if a.Push == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "devicepush unavailable"})
		return
	}
	id := r.PathValue("id")
	err := a.Push.ClearAllowlist(r.Context(), id)
	a.auditMDM(r, "clear_allowlist", id, "נוקה פרופיל הרשימה המותרת", err, nil)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "queued"})
}

func (a *API) handleMDMPushCert(w http.ResponseWriter, r *http.Request) {
	if a.MDMStore == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "mdm store unavailable"})
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil || len(body) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "PEM body required (cert + key concatenated)"})
		return
	}
	topic := strings.TrimSpace(r.URL.Query().Get("topic"))
	if topic == "" {
		topic = a.Cfg.MDMTopic
	}
	if topic == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "topic required"})
		return
	}
	// Split concatenated PEMs into cert then key (first CERTIFICATE, then PRIVATE KEY).
	certPEM, keyPEM, err := splitCertKeyPEM(body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := a.MDMStore.UpsertPushCert(r.Context(), topic, certPEM, keyPEM); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"ok": "true", "topic": topic})
}
