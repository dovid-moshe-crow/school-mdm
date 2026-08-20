package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/dwdmsh/school-mdm/internal/policy"
	"github.com/dwdmsh/school-mdm/internal/store"
)

func (a *API) enabledSystemKeys(r *http.Request) map[string]struct{} {
	out := map[string]struct{}{}
	if a.Store == nil {
		return out
	}
	list, err := a.Store.ListSystemAllowlist(r.Context())
	if err != nil {
		return out
	}
	for _, it := range list {
		if !it.Enabled {
			continue
		}
		if it.Kind == policy.KindApp {
			out[policy.AppKey(it.Value)] = struct{}{}
			continue
		}
		out[policy.Normalize(it.Kind, it.Value)] = struct{}{}
	}
	return out
}

func (a *API) handleListSystemAllowlist(w http.ResponseWriter, r *http.Request) {
	list, err := a.Store.ListSystemAllowlist(r.Context())
	if err != nil {
		writeErr(w, err)
		return
	}
	if list == nil {
		list = []store.SystemAllowlistItem{}
	}
	writeJSON(w, http.StatusOK, list)
}

func (a *API) handleUpsertSystemAllowlist(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Kind    string `json:"kind"`
		Value   string `json:"value"`
		Enabled *bool  `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	kind := policy.Kind(strings.TrimSpace(body.Kind))
	if kind == "" {
		kind = policy.KindApp
	}
	enabled := true
	if body.Enabled != nil {
		enabled = *body.Enabled
	}
	item := store.SystemAllowlistItem{Kind: kind, Value: body.Value, Enabled: enabled}
	if err := a.Store.UpsertSystemAllowlist(r.Context(), item); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (a *API) handlePatchSystemAllowlist(w http.ResponseWriter, r *http.Request) {
	kind := policy.Kind(strings.TrimSpace(r.URL.Query().Get("kind")))
	value := strings.TrimSpace(r.URL.Query().Get("value"))
	if kind == "" {
		kind = policy.KindApp
	}
	var body struct {
		Enabled *bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Enabled == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "enabled is required"})
		return
	}
	if err := a.Store.SetSystemAllowlistEnabled(r.Context(), kind, value, *body.Enabled); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (a *API) handleDeleteSystemAllowlist(w http.ResponseWriter, r *http.Request) {
	kind := policy.Kind(strings.TrimSpace(r.URL.Query().Get("kind")))
	value := strings.TrimSpace(r.URL.Query().Get("value"))
	if kind == "" {
		kind = policy.KindApp
	}
	if err := a.Store.DeleteSystemAllowlist(r.Context(), kind, value); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
