package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/dwdmsh/school-mdm/internal/store"
)

func (a *API) handleListAPITokens(w http.ResponseWriter, r *http.Request) {
	if a.Store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "store unavailable"})
		return
	}
	list, err := a.Store.ListAPITokens(r.Context())
	if err != nil {
		writeErr(w, err)
		return
	}
	if list == nil {
		list = []store.APIToken{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"tokens": list})
}

func (a *API) handleCreateAPIToken(w http.ResponseWriter, r *http.Request) {
	if a.Store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "store unavailable"})
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && err != io.EOF {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		name = "API token"
	}
	raw, hash, prefix, err := store.NewAPITokenSecret()
	if err != nil {
		writeErr(w, err)
		return
	}
	createdBy := ""
	if u, ok := a.sessionFromRequest(r); ok {
		createdBy = u.Email
	}
	tok, err := a.Store.CreateAPIToken(r.Context(), store.APIToken{
		Name:      name,
		Prefix:    prefix,
		TokenHash: hash,
		CreatedBy: createdBy,
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	a.auditAdmin(r, store.ActivityCategorySystem, "api_token_create", "נוצר אסימון API",
		map[string]any{"token_id": tok.ID, "name": tok.Name, "prefix": tok.Prefix}, "", "")
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":          tok.ID,
		"name":        tok.Name,
		"prefix":      tok.Prefix,
		"created_by":  tok.CreatedBy,
		"created_at":  tok.CreatedAt,
		"token":       raw,
		"token_hint":  "Copy this token now. It is not shown again.",
	})
}

func (a *API) handleDeleteAPIToken(w http.ResponseWriter, r *http.Request) {
	if a.Store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "store unavailable"})
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	if err := a.Store.DeleteAPIToken(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		writeErr(w, err)
		return
	}
	a.auditAdmin(r, store.ActivityCategorySystem, "api_token_delete", "נמחק אסימון API",
		map[string]any{"token_id": id}, "", "")
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
