package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/dwdmsh/school-mdm/internal/policy"
	"github.com/dwdmsh/school-mdm/internal/store"
)

func (a *API) handleListGroups(w http.ResponseWriter, r *http.Request) {
	list, err := a.Store.ListGroups(r.Context())
	if err != nil {
		writeErr(w, err)
		return
	}
	if list == nil {
		list = []store.Group{}
	}
	for i := range list {
		members, err := a.Store.ListGroupMembers(r.Context(), list[i].ID)
		if err == nil {
			list[i].MemberCount = len(members)
		}
	}
	writeJSON(w, http.StatusOK, list)
}

type groupBody struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (a *API) handleCreateGroup(w http.ResponseWriter, r *http.Request) {
	var body groupBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	g, err := a.Store.CreateGroup(r.Context(), store.Group{
		Name:        body.Name,
		Description: body.Description,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, g)
}

func (a *API) handleGetGroup(w http.ResponseWriter, r *http.Request) {
	g, err := a.Store.GetGroup(r.Context(), r.PathValue("id"))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, g)
}

func (a *API) handleUpdateGroup(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	existing, err := a.Store.GetGroup(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeErr(w, err)
		return
	}
	var body groupBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if strings.TrimSpace(body.Name) != "" {
		existing.Name = body.Name
	}
	existing.Description = body.Description
	if err := a.Store.UpdateGroup(r.Context(), existing); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, existing)
}

func (a *API) handleDeleteGroup(w http.ResponseWriter, r *http.Request) {
	if err := a.Store.DeleteGroup(r.Context(), r.PathValue("id")); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
}

func (a *API) handleListGroupMembers(w http.ResponseWriter, r *http.Request) {
	members, err := a.Store.ListGroupMembers(r.Context(), r.PathValue("id"))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeErr(w, err)
		return
	}
	if members == nil {
		members = []string{}
	}
	writeJSON(w, http.StatusOK, members)
}

type membersBody struct {
	EnrollmentIDs []string `json:"enrollment_ids"`
}

func (a *API) handleSetGroupMembers(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var body membersBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if err := a.Store.SetGroupMembers(r.Context(), id, body.EnrollmentIDs); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	members, err := a.Store.ListGroupMembers(r.Context(), id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, members)
}

type createAllowanceBody struct {
	Kind         string `json:"kind"`
	Value        string `json:"value"`
	Scope        string `json:"scope"` // global | group | device
	GroupID      string `json:"group_id"`
	EnrollmentID string `json:"enrollment_id"`
	Duration     string `json:"duration"` // permanent | 15m | 1h | 24h | today
}

func (a *API) handleCreateAllowance(w http.ResponseWriter, r *http.Request) {
	var body createAllowanceBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	kind := policy.Kind(strings.TrimSpace(body.Kind))
	if kind != policy.KindApp && kind != policy.KindURL {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "kind must be app or url"})
		return
	}
	value := policy.Normalize(kind, body.Value)
	if value == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "value is required"})
		return
	}
	target, err := parseAllowanceTarget(body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	now := time.Now().UTC()
	expires, permanent, err := parseAllowanceDuration(body.Duration, now)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if permanent {
		if err := a.Store.UpsertAllowlist(r.Context(), policy.Entry{
			Kind: kind, Value: value, Target: target,
		}); err != nil {
			writeErr(w, err)
			return
		}
	} else {
		if err := a.Store.AddGrant(r.Context(), policy.Grant{
			Kind: kind, Value: value, Target: target, ExpiresAt: expires,
		}); err != nil {
			writeErr(w, err)
			return
		}
	}
	if kind == policy.KindApp && a.Catalog != nil {
		_, _ = a.Catalog.LookupBundle(r.Context(), value)
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"kind":        kind,
		"value":       value,
		"target_type": target.Type,
		"target_id":   target.ID,
		"permanent":   permanent,
		"expires_at":  expires,
	})
}

func parseAllowanceTarget(body createAllowanceBody) (policy.Target, error) {
	scope := strings.ToLower(strings.TrimSpace(body.Scope))
	if scope == "" {
		scope = "global"
	}
	switch scope {
	case "global":
		return policy.Target{Type: policy.TargetGlobal}, nil
	case "group":
		gid := strings.TrimSpace(body.GroupID)
		if gid == "" {
			return policy.Target{}, errors.New("group_id is required when scope=group")
		}
		return policy.Target{Type: policy.TargetGroup, ID: gid}, nil
	case "device":
		id := strings.TrimSpace(body.EnrollmentID)
		if id == "" {
			return policy.Target{}, errors.New("enrollment_id is required when scope=device")
		}
		return policy.Target{Type: policy.TargetDevice, ID: id}, nil
	default:
		return policy.Target{}, errors.New("scope must be global, group, or device")
	}
}

func parseAllowanceDuration(d string, now time.Time) (expires *time.Time, permanent bool, err error) {
	switch strings.TrimSpace(d) {
	case "", "permanent":
		return nil, true, nil
	case "15m":
		t := now.Add(15 * time.Minute)
		return &t, false, nil
	case "1h":
		t := now.Add(time.Hour)
		return &t, false, nil
	case "24h":
		t := now.Add(24 * time.Hour)
		return &t, false, nil
	case "today":
		y, m, day := now.Date()
		end := time.Date(y, m, day, 23, 59, 59, 0, time.UTC)
		return &end, false, nil
	default:
		return nil, false, errors.New("unsupported duration (use 15m, 1h, 24h, today, permanent)")
	}
}

func (a *API) handleDeleteAllowance(w http.ResponseWriter, r *http.Request) {
	kind := policy.Kind(strings.TrimSpace(r.URL.Query().Get("kind")))
	value := strings.TrimSpace(r.URL.Query().Get("value"))
	tt := policy.TargetType(strings.TrimSpace(r.URL.Query().Get("target_type")))
	tid := strings.TrimSpace(r.URL.Query().Get("target_id"))
	if tt == "" {
		tt = policy.TargetGlobal
	}
	if kind != policy.KindApp && kind != policy.KindURL {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "kind must be app or url"})
		return
	}
	for _, e := range policy.Essentials {
		if kind == policy.KindApp && policy.Normalize(kind, value) == e {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "cannot remove essential app"})
			return
		}
	}
	target := policy.Target{Type: tt, ID: tid}
	if err := a.Store.DeleteAllowlist(r.Context(), kind, value, target); err != nil {
		writeErr(w, err)
		return
	}
	if err := a.Store.DeleteGrants(r.Context(), kind, value, target); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
}
