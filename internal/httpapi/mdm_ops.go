package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/dwdmsh/school-mdm/internal/activity"
	"github.com/dwdmsh/school-mdm/internal/mdm/commands"
	"github.com/dwdmsh/school-mdm/internal/store"
)

func (a *API) handleMDMLock(w http.ResponseWriter, r *http.Request) {
	live, ok := a.live()
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "live enqueuer unavailable"})
		return
	}
	var body struct {
		PIN     string `json:"pin"`
		Message string `json:"message"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	id := r.PathValue("id")
	cmd, err := commands.DeviceLock(body.PIN, body.Message)
	if err != nil {
		a.auditMDM(r, "lock", id, "נעילת מכשיר", err, nil)
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	err = live.EnqueueRaw(r.Context(), id, cmd)
	a.auditMDM(r, "lock", id, "נעילת מכשיר", err, nil)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "queued"})
}

func (a *API) handleMDMClearPasscode(w http.ResponseWriter, r *http.Request) {
	a.enqueueNamed(w, r, "clear_passcode", "איפוס קוד גישה", commands.ClearPasscode)
}

func (a *API) handleMDMRestart(w http.ResponseWriter, r *http.Request) {
	a.enqueueNamed(w, r, "restart", "הפעלה מחדש של המכשיר", commands.RestartDevice)
}

func (a *API) handleMDMShutDown(w http.ResponseWriter, r *http.Request) {
	a.enqueueNamed(w, r, "shutdown", "כיבוי המכשיר", commands.ShutDownDevice)
}

func (a *API) handleMDMErase(w http.ResponseWriter, r *http.Request) {
	live, ok := a.live()
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "live enqueuer unavailable"})
		return
	}
	var body struct {
		PIN string `json:"pin"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	id := r.PathValue("id")
	cmd, err := commands.EraseDevice(body.PIN)
	if err != nil {
		a.auditMDM(r, "erase", id, "מחיקת מכשיר", err, nil)
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	err = live.EnqueueRaw(r.Context(), id, cmd)
	a.auditMDM(r, "erase", id, "מחיקת מכשיר", err, nil)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "queued"})
}

func (a *API) handleMDMEnableLostMode(w http.ResponseWriter, r *http.Request) {
	live, ok := a.live()
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "live enqueuer unavailable"})
		return
	}
	var body struct {
		Message  string `json:"message"`
		Phone    string `json:"phone"`
		Footnote string `json:"footnote"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	id := r.PathValue("id")
	cmd, err := commands.EnableLostMode(body.Message, body.Phone, body.Footnote)
	if err != nil {
		a.auditMDM(r, "enable_lost_mode", id, "הפעלת מצב אבוד", err, nil)
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	err = live.EnqueueRaw(r.Context(), id, cmd)
	a.auditMDM(r, "enable_lost_mode", id, "הפעלת מצב אבוד", err, nil)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "queued"})
}

func (a *API) handleMDMDisableLostMode(w http.ResponseWriter, r *http.Request) {
	a.enqueueNamed(w, r, "disable_lost_mode", "כיבוי מצב אבוד", commands.DisableLostMode)
}

func (a *API) handleMDMPlayLostModeSound(w http.ResponseWriter, r *http.Request) {
	a.enqueueNamed(w, r, "play_lost_mode_sound", "צליל במצב אבוד", commands.PlayLostModeSound)
}

func (a *API) handleMDMDeviceLocation(w http.ResponseWriter, r *http.Request) {
	a.enqueueWithUUID(w, r, "device_location", "בקשת מיקום", commands.DeviceLocation)
}

func (a *API) handleMDMSecurityInfo(w http.ResponseWriter, r *http.Request) {
	a.enqueueWithUUID(w, r, "security_info", "בקשת מידע אבטחה", commands.SecurityInfo)
}

func (a *API) enqueueWithUUID(w http.ResponseWriter, r *http.Request, action, summary string, build func() ([]byte, string, error)) {
	live, ok := a.live()
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "live enqueuer unavailable"})
		return
	}
	id := r.PathValue("id")
	cmd, commandUUID, err := build()
	if err != nil {
		a.auditMDM(r, action, id, summary, err, nil)
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	err = live.EnqueueRaw(r.Context(), id, cmd)
	a.auditMDM(r, action, id, summary, err, map[string]any{"command_uuid": commandUUID})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{
		"status":       "queued",
		"command_uuid": commandUUID,
	})
}

func (a *API) handleMDMBulk(w http.ResponseWriter, r *http.Request) {
	var body struct {
		EnrollmentIDs []string `json:"enrollment_ids"`
		Op            string   `json:"op"` // unrestricted | restrict | lock | clear-passcode | restart | shutdown | erase | enable-lost-mode | disable-lost-mode | play-lost-mode-sound | add-group
		PIN           string   `json:"pin"`
		Message       string   `json:"message"`
		Phone         string   `json:"phone"`
		Footnote      string   `json:"footnote"`
		GroupID       string   `json:"group_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	op := strings.ToLower(strings.TrimSpace(body.Op))
	if len(body.EnrollmentIDs) == 0 || op == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "enrollment_ids and op required"})
		return
	}
	type result struct {
		ID    string `json:"id"`
		OK    bool   `json:"ok"`
		Error string `json:"error,omitempty"`
	}
	out := make([]result, 0, len(body.EnrollmentIDs))
	for _, id := range body.EnrollmentIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		res := result{ID: id, OK: true}
		var err error
		switch op {
		case "unrestricted":
			err = a.Store.SetDeviceUnrestricted(r.Context(), id, true)
			if err == nil {
				a.auditAction(r, actionDeviceUnrestricted, map[string]any{
					"enrollment_id": id, "unrestricted": true, "bulk": true,
				})
				if a.Notify != nil {
					a.Notify.UnrestrictedChanged(r.Context(), id, true)
				}
				if a.Push != nil {
					err = a.Push.Reconcile(r.Context(), id)
				}
			}
		case "restrict":
			err = a.Store.SetDeviceUnrestricted(r.Context(), id, false)
			if err == nil {
				a.auditAction(r, actionDeviceUnrestricted, map[string]any{
					"enrollment_id": id, "unrestricted": false, "bulk": true,
				})
				if a.Notify != nil {
					a.Notify.UnrestrictedChanged(r.Context(), id, false)
				}
				if a.Push != nil {
					err = a.Push.Reconcile(r.Context(), id)
				}
			}
		case "lock":
			if live, ok := a.live(); ok {
				var cmd []byte
				cmd, err = commands.DeviceLock(body.PIN, body.Message)
				if err == nil {
					err = live.EnqueueRaw(r.Context(), id, cmd)
				}
			} else {
				err = errUnavailable
			}
		case "clear-passcode":
			err = a.bulkEnqueue(r, id, commands.ClearPasscode)
		case "restart":
			err = a.bulkEnqueue(r, id, commands.RestartDevice)
		case "shutdown":
			err = a.bulkEnqueue(r, id, commands.ShutDownDevice)
		case "erase":
			if live, ok := a.live(); ok {
				var cmd []byte
				cmd, err = commands.EraseDevice(body.PIN)
				if err == nil {
					err = live.EnqueueRaw(r.Context(), id, cmd)
				}
			} else {
				err = errUnavailable
			}
		case "enable-lost-mode":
			if live, ok := a.live(); ok {
				var cmd []byte
				cmd, err = commands.EnableLostMode(body.Message, body.Phone, body.Footnote)
				if err == nil {
					err = live.EnqueueRaw(r.Context(), id, cmd)
				}
			} else {
				err = errUnavailable
			}
		case "disable-lost-mode":
			err = a.bulkEnqueue(r, id, commands.DisableLostMode)
		case "play-lost-mode-sound":
			err = a.bulkEnqueue(r, id, commands.PlayLostModeSound)
		case "add-group":
			gid := strings.TrimSpace(body.GroupID)
			if gid == "" {
				err = errGroupRequired
			} else {
				err = a.Store.AddGroupMember(r.Context(), gid, id)
				if err == nil {
					a.auditAction(r, actionGroupMemberAdd, map[string]any{
						"group_id": gid, "enrollment_id": id, "bulk": true,
					})
					if a.Push != nil {
						err = a.Push.Reconcile(r.Context(), id)
					}
				}
			}
		// Legacy ops kept for API compatibility; not shown in UI.
		case "push":
			if live, ok := a.live(); ok {
				err = live.Push(r.Context(), id)
			} else {
				err = errUnavailable
			}
		case "reconcile":
			if a.Push != nil {
				err = a.Push.Reconcile(r.Context(), id)
			} else {
				err = errUnavailable
			}
		case "clear-allowlist":
			if a.Push != nil {
				err = a.Push.ClearAllowlist(r.Context(), id)
			} else {
				err = errUnavailable
			}
		default:
			err = errUnknownOp
		}
		if err != nil {
			res.OK = false
			res.Error = err.Error()
		}
		out = append(out, res)
	}
	okN, failN := 0, 0
	for _, res := range out {
		if res.OK {
			okN++
		} else {
			failN++
		}
	}
	actorType, actor := a.adminActor(r)
	auditResult := store.ActivityResultOK
	if failN > 0 && okN == 0 {
		auditResult = store.ActivityResultError
	} else if failN > 0 {
		auditResult = store.ActivityResultInfo
	}
	bulkSummary := "פעולה מרובה: " + op
	switch op {
	case "unrestricted":
		bulkSummary = "פעולה מרובה: הופעל מצב ללא הגבלות"
	case "restrict":
		bulkSummary = "פעולה מרובה: בוטל מצב ללא הגבלות"
	case "add-group":
		bulkSummary = "פעולה מרובה: הוספה לקבוצה"
	case "lock":
		bulkSummary = "פעולה מרובה: נעילת מכשירים"
	}
	a.audit(r, activity.Event{
		Category:  store.ActivityCategoryMDM,
		Action:    "bulk_" + op,
		ActorType: actorType,
		Actor:     actor,
		Result:    auditResult,
		Summary:   bulkSummary,
		Detail:    map[string]any{"ok": okN, "failed": failN, "total": len(out), "op": op},
	})
	writeJSON(w, http.StatusOK, map[string]any{"results": out})
}

func (a *API) bulkEnqueue(r *http.Request, id string, build func() ([]byte, error)) error {
	live, ok := a.live()
	if !ok {
		return errUnavailable
	}
	cmd, err := build()
	if err != nil {
		return err
	}
	return live.EnqueueRaw(r.Context(), id, cmd)
}

var (
	errUnavailable   = &bulkErr{"unavailable"}
	errUnknownOp     = &bulkErr{"unknown op"}
	errGroupRequired = &bulkErr{"group_id required"}
)

type bulkErr struct{ s string }

func (e *bulkErr) Error() string { return e.s }
