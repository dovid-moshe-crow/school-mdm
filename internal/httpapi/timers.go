package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/dwdmsh/school-mdm/internal/store"
	"github.com/dwdmsh/school-mdm/internal/timers"
)

func (a *API) timersOrErr(w http.ResponseWriter) bool {
	if a == nil || a.Timers == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "timers unavailable"})
		return false
	}
	return true
}

func (a *API) handleListTimers(w http.ResponseWriter, r *http.Request) {
	if !a.timersOrErr(w) {
		return
	}
	list, err := a.Timers.List(r.Context())
	if err != nil {
		writeErr(w, err)
		return
	}
	if list == nil {
		list = []timers.View{}
	}
	writeJSON(w, http.StatusOK, list)
}

func (a *API) handleCreateTimer(w http.ResponseWriter, r *http.Request) {
	if !a.timersOrErr(w) {
		return
	}
	var body timerWrite
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	t, err := body.toTimer()
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	t.Enabled = true
	if body.Enabled != nil {
		t.Enabled = *body.Enabled
	}
	created, err := a.Timers.Create(r.Context(), t)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	a.auditAdmin(r, store.ActivityCategoryPolicy, "timer_create", "נוצר טיימר רשימה מותרת",
		map[string]any{"timer_id": created.ID, "name": created.Name, "action": created.Action, "schedule": created.Schedule}, "", "")
	writeJSON(w, http.StatusCreated, created)
}

func (a *API) handleGetTimer(w http.ResponseWriter, r *http.Request) {
	if !a.timersOrErr(w) {
		return
	}
	v, err := a.Timers.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (a *API) handleUpdateTimer(w http.ResponseWriter, r *http.Request) {
	if !a.timersOrErr(w) {
		return
	}
	existing, err := a.Timers.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		writeErr(w, err)
		return
	}
	var body timerPatch
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	next, reset, err := body.apply(existing.PolicyTimer)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	updated, err := a.Timers.Update(r.Context(), next, reset)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	a.auditAdmin(r, store.ActivityCategoryPolicy, "timer_update", "עודכן טיימר רשימה מותרת",
		map[string]any{"timer_id": updated.ID, "name": updated.Name, "enabled": updated.Enabled}, "", "")
	writeJSON(w, http.StatusOK, updated)
}

func (a *API) handleDeleteTimer(w http.ResponseWriter, r *http.Request) {
	if !a.timersOrErr(w) {
		return
	}
	id := r.PathValue("id")
	if err := a.Timers.Delete(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		writeErr(w, err)
		return
	}
	a.auditAdmin(r, store.ActivityCategoryPolicy, "timer_delete", "נמחק טיימר רשימה מותרת",
		map[string]any{"timer_id": id}, "", "")
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (a *API) handleRunTimer(w http.ResponseWriter, r *http.Request) {
	if !a.timersOrErr(w) {
		return
	}
	res, t, err := a.Timers.RunNow(r.Context(), r.PathValue("id"))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		writeErr(w, err)
		return
	}
	a.auditAdmin(r, store.ActivityCategoryPolicy, "timer_run", "טיימר רשימה מותרת הורץ ידנית",
		map[string]any{"timer_id": t.ID, "name": t.Name, "action": t.Action, "assignments": res.Assignments, "devices": res.Devices, "errors": res.Errors}, "", "")
	writeJSON(w, http.StatusOK, res)
}

func (a *API) handleRunDueTimers(w http.ResponseWriter, r *http.Request) {
	if !a.timersOrErr(w) {
		return
	}
	fired, errs, err := a.Timers.RunDue(r.Context(), time.Now())
	if err != nil {
		writeErr(w, err)
		return
	}
	a.auditAdmin(r, store.ActivityCategoryPolicy, "timer_run", "הרצת טיימרים שמגיע זמנם",
		map[string]any{"fired": fired, "errors": errs}, "", "")
	writeJSON(w, http.StatusOK, map[string]any{"fired": fired, "errors": errs})
}

type timerWrite struct {
	Name       string   `json:"name"`
	Action     string   `json:"action"`
	PackIDs    []string `json:"pack_ids"`
	ProfileIDs []string `json:"profile_ids"`
	DeviceIDs  []string `json:"device_ids"`
	GroupIDs   []string `json:"group_ids"`
	Schedule   string   `json:"schedule"`
	RunAt      string   `json:"run_at"`
	Weekdays   []int    `json:"weekdays"`
	TimeOfDay  string   `json:"time_of_day"`
	Enabled    *bool    `json:"enabled"`
}

func (b timerWrite) toTimer() (store.PolicyTimer, error) {
	t := store.PolicyTimer{
		Name:       b.Name,
		Action:     b.Action,
		PackIDs:    b.PackIDs,
		ProfileIDs: b.ProfileIDs,
		DeviceIDs:  b.DeviceIDs,
		GroupIDs:   b.GroupIDs,
		Schedule:   b.Schedule,
		Weekdays:   b.Weekdays,
		TimeOfDay:  b.TimeOfDay,
	}
	runAt, err := parseRunAt(b.RunAt)
	if err != nil {
		return store.PolicyTimer{}, err
	}
	t.RunAt = runAt
	return t, nil
}

type timerPatch struct {
	Name       *string   `json:"name"`
	Action     *string   `json:"action"`
	PackIDs    *[]string `json:"pack_ids"`
	ProfileIDs *[]string `json:"profile_ids"`
	DeviceIDs  *[]string `json:"device_ids"`
	GroupIDs   *[]string `json:"group_ids"`
	Schedule   *string   `json:"schedule"`
	RunAt      *string   `json:"run_at"`
	Weekdays   *[]int    `json:"weekdays"`
	TimeOfDay  *string   `json:"time_of_day"`
	Enabled    *bool     `json:"enabled"`
}

func (p timerPatch) apply(cur store.PolicyTimer) (store.PolicyTimer, bool, error) {
	reset := p.Schedule != nil || p.RunAt != nil || p.Weekdays != nil || p.TimeOfDay != nil
	if p.Name != nil {
		cur.Name = *p.Name
	}
	if p.Action != nil {
		cur.Action = *p.Action
	}
	if p.PackIDs != nil {
		cur.PackIDs = *p.PackIDs
	}
	if p.ProfileIDs != nil {
		cur.ProfileIDs = *p.ProfileIDs
	}
	if p.DeviceIDs != nil {
		cur.DeviceIDs = *p.DeviceIDs
	}
	if p.GroupIDs != nil {
		cur.GroupIDs = *p.GroupIDs
	}
	if p.Schedule != nil {
		cur.Schedule = *p.Schedule
	}
	if p.RunAt != nil {
		runAt, err := parseRunAt(*p.RunAt)
		if err != nil {
			return store.PolicyTimer{}, false, err
		}
		cur.RunAt = runAt
	}
	if p.Weekdays != nil {
		cur.Weekdays = *p.Weekdays
	}
	if p.TimeOfDay != nil {
		cur.TimeOfDay = *p.TimeOfDay
	}
	if p.Enabled != nil {
		cur.Enabled = *p.Enabled
	}
	return cur, reset, nil
}

func parseRunAt(s string) (*time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t, err = time.Parse(time.RFC3339Nano, s)
	}
	if err != nil {
		return nil, errors.New("run_at must be RFC3339")
	}
	u := t.UTC()
	return &u, nil
}
