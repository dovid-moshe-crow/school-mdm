package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/dwdmsh/school-mdm/internal/policy"
	"github.com/dwdmsh/school-mdm/internal/store"
)

func (a *API) handleListPacks(w http.ResponseWriter, r *http.Request) {
	list, err := a.Store.ListWhitelistPacks(r.Context())
	if err != nil {
		writeErr(w, err)
		return
	}
	if list == nil {
		list = []store.WhitelistPack{}
	}
	writeJSON(w, http.StatusOK, list)
}

func (a *API) handleCreatePack(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	p, err := a.Store.CreateWhitelistPack(r.Context(), store.WhitelistPack{
		Name: body.Name, Description: body.Description,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	a.auditAdmin(r, store.ActivityCategoryPolicy, "pack_create", "נוצרה חבילת רשימה מותרת",
		map[string]any{"pack_id": p.ID, "name": p.Name}, "", "")
	writeJSON(w, http.StatusCreated, p)
}

func (a *API) handleGetPack(w http.ResponseWriter, r *http.Request) {
	p, err := a.Store.GetWhitelistPack(r.Context(), r.PathValue("id"))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		writeErr(w, err)
		return
	}
	items, _ := a.Store.ListWhitelistPackItems(r.Context(), p.ID)
	assigns, _ := a.Store.ListWhitelistPackAssignments(r.Context(), p.ID)
	if items == nil {
		items = []store.WhitelistPackItem{}
	}
	if assigns == nil {
		assigns = []store.WhitelistPackAssignment{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"pack":         p,
		"items":        items,
		"assignments":  assigns,
	})
}

func (a *API) handleUpdatePack(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	err := a.Store.UpdateWhitelistPack(r.Context(), store.WhitelistPack{
		ID: r.PathValue("id"), Name: body.Name, Description: body.Description,
	})
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	p, _ := a.Store.GetWhitelistPack(r.Context(), r.PathValue("id"))
	a.auditAdmin(r, store.ActivityCategoryPolicy, "pack_update", "עודכנה חבילת רשימה מותרת",
		map[string]any{"pack_id": r.PathValue("id"), "name": body.Name}, "", "")
	writeJSON(w, http.StatusOK, p)
}

func (a *API) handleDeletePack(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	devices, _ := a.devicesAffectedByPack(r, id)
	if err := a.Store.DeleteWhitelistPack(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		writeErr(w, err)
		return
	}
	a.auditAdmin(r, store.ActivityCategoryPolicy, "pack_delete", "נמחקה חבילת רשימה מותרת",
		map[string]any{"pack_id": id, "devices": len(devices)}, "", "")
	if a.Push != nil && len(devices) > 0 {
		_ = a.Push.ReconcileMany(r.Context(), devices)
	}
	writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
}

func (a *API) handleAddPackItem(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Kind  string `json:"kind"`
		Value string `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	kind := policy.Kind(strings.TrimSpace(body.Kind))
	if kind != policy.KindApp && kind != policy.KindURL {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "kind must be app or url"})
		return
	}
	packID := r.PathValue("id")
	value := policy.Normalize(kind, body.Value)
	if kind == policy.KindApp && a.Catalog != nil {
		if meta, err := a.Catalog.LookupBundle(r.Context(), value); err == nil && meta.BundleID != "" {
			value = meta.BundleID
		}
	}
	if err := a.Store.AddWhitelistPackItem(r.Context(), store.WhitelistPackItem{
		PackID: packID, Kind: kind, Value: value,
	}); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	devices, _ := a.devicesAffectedByPack(r, packID)
	a.auditAdmin(r, store.ActivityCategoryPolicy, "pack_item_add", "נוסף פריט לחבילה",
		map[string]any{"pack_id": packID, "kind": string(kind), "value": value, "devices": len(devices)}, "", "")
	if a.Push != nil && len(devices) > 0 {
		_ = a.Push.ReconcileMany(r.Context(), devices)
	}
	writeJSON(w, http.StatusCreated, map[string]string{"ok": "true"})
}

func (a *API) handleRemovePackItem(w http.ResponseWriter, r *http.Request) {
	kind := policy.Kind(strings.TrimSpace(r.URL.Query().Get("kind")))
	value := strings.TrimSpace(r.URL.Query().Get("value"))
	packID := r.PathValue("id")
	if err := a.Store.RemoveWhitelistPackItem(r.Context(), packID, kind, value); err != nil {
		writeErr(w, err)
		return
	}
	devices, _ := a.devicesAffectedByPack(r, packID)
	a.auditAdmin(r, store.ActivityCategoryPolicy, "pack_item_remove", "הוסר פריט מחבילה",
		map[string]any{"pack_id": packID, "kind": string(kind), "value": value, "devices": len(devices)}, "", "")
	if a.Push != nil && len(devices) > 0 {
		_ = a.Push.ReconcileMany(r.Context(), devices)
	}
	writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
}

func (a *API) handleAddPackAssignment(w http.ResponseWriter, r *http.Request) {
	var body struct {
		TargetType string `json:"target_type"`
		TargetID   string `json:"target_id"`
		Scope      string `json:"scope"` // alias: global|group|device
		GroupID    string `json:"group_id"`
		Enrollment string `json:"enrollment_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	tt := policy.TargetType(strings.TrimSpace(body.TargetType))
	tid := strings.TrimSpace(body.TargetID)
	if tt == "" {
		switch strings.ToLower(strings.TrimSpace(body.Scope)) {
		case "global", "":
			tt = policy.TargetGlobal
		case "group":
			tt = policy.TargetGroup
			tid = strings.TrimSpace(body.GroupID)
		case "device":
			tt = policy.TargetDevice
			tid = strings.TrimSpace(body.Enrollment)
		default:
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid scope"})
			return
		}
	}
	packID := r.PathValue("id")
	if err := a.Store.SetWhitelistPackAssignment(r.Context(), store.WhitelistPackAssignment{
		PackID: packID, TargetType: tt, TargetID: tid,
	}); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	devices, _ := a.devicesForTarget(r, policy.Target{Type: tt, ID: tid})
	enrollID, groupID := "", ""
	switch tt {
	case policy.TargetDevice:
		enrollID = tid
	case policy.TargetGroup:
		groupID = tid
	}
	a.auditAdmin(r, store.ActivityCategoryPolicy, "pack_assign", "שויכה חבילה",
		map[string]any{"pack_id": packID, "target_type": string(tt), "target_id": tid, "devices": len(devices)},
		enrollID, groupID)
	if a.Push != nil && len(devices) > 0 {
		_ = a.Push.ReconcileMany(r.Context(), devices)
	}
	writeJSON(w, http.StatusCreated, map[string]string{"ok": "true"})
}

func (a *API) handleRemovePackAssignment(w http.ResponseWriter, r *http.Request) {
	tt := policy.TargetType(strings.TrimSpace(r.URL.Query().Get("target_type")))
	tid := strings.TrimSpace(r.URL.Query().Get("target_id"))
	packID := r.PathValue("id")
	target := policy.Target{Type: tt, ID: tid}
	devices, _ := a.devicesForTarget(r, target)
	if err := a.Store.RemoveWhitelistPackAssignment(r.Context(), packID, target); err != nil {
		writeErr(w, err)
		return
	}
	enrollID, groupID := "", ""
	switch tt {
	case policy.TargetDevice:
		enrollID = tid
	case policy.TargetGroup:
		groupID = tid
	}
	a.auditAdmin(r, store.ActivityCategoryPolicy, "pack_unassign", "בוטל שיוך חבילה",
		map[string]any{"pack_id": packID, "target_type": string(tt), "target_id": tid, "devices": len(devices)},
		enrollID, groupID)
	if a.Push != nil && len(devices) > 0 {
		_ = a.Push.ReconcileMany(r.Context(), devices)
	}
	writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
}

func (a *API) devicesAffectedByPack(r *http.Request, packID string) ([]string, error) {
	assigns, err := a.Store.ListWhitelistPackAssignments(r.Context(), packID)
	if err != nil {
		return nil, err
	}
	seen := map[string]struct{}{}
	var out []string
	for _, as := range assigns {
		ids, err := a.devicesForTarget(r, policy.Target{Type: as.TargetType, ID: as.TargetID})
		if err != nil {
			continue
		}
		for _, id := range ids {
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			out = append(out, id)
		}
	}
	return out, nil
}
