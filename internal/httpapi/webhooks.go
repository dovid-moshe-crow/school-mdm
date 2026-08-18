package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/dwdmsh/school-mdm/internal/store"
	"github.com/dwdmsh/school-mdm/internal/webhooks"
)

func (a *API) handleWebhookEvents(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"events": webhooks.Catalog(),
		"filters": []string{
			"* — every event",
			"category.* — every action in a category (example: mdm.*)",
			"category.action — one event (example: requests.request_approve)",
		},
	})
}

func (a *API) handleListWebhooks(w http.ResponseWriter, r *http.Request) {
	list, err := a.Store.ListWebhookEndpoints(r.Context())
	if err != nil {
		writeErr(w, err)
		return
	}
	if list == nil {
		list = []store.WebhookEndpoint{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"endpoints": list})
}

type webhookBody struct {
	URL         *string  `json:"url"`
	Secret      *string  `json:"secret"`
	Description *string  `json:"description"`
	Events      []string `json:"events"`
	Enabled     *bool    `json:"enabled"`
}

func (a *API) handleCreateWebhook(w http.ResponseWriter, r *http.Request) {
	var body webhookBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	url := ""
	if body.URL != nil {
		url = strings.TrimSpace(*body.URL)
	}
	if err := webhooks.ValidateURL(url); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	secret := ""
	if body.Secret != nil {
		secret = strings.TrimSpace(*body.Secret)
	}
	if secret == "" {
		secret = randomWebhookSecret()
	}
	desc := ""
	if body.Description != nil {
		desc = strings.TrimSpace(*body.Description)
	}
	events := normalizeWebhookEvents(body.Events)
	enabled := true
	if body.Enabled != nil {
		enabled = *body.Enabled
	}
	ep, err := a.Store.CreateWebhookEndpoint(r.Context(), store.WebhookEndpoint{
		URL:         url,
		Secret:      secret,
		Description: desc,
		Events:      events,
		Enabled:     enabled,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	a.auditAdmin(r, store.ActivityCategoryWebhooks, "endpoint_create", "Webhook endpoint created",
		map[string]any{"webhook_id": ep.ID, "url": ep.URL, "events": ep.Events}, "", "")
	writeJSON(w, http.StatusCreated, ep)
}

func (a *API) handleGetWebhook(w http.ResponseWriter, r *http.Request) {
	ep, err := a.Store.GetWebhookEndpoint(r.Context(), r.PathValue("id"))
	if err != nil {
		writeWebhookErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ep)
}

func (a *API) handleUpdateWebhook(w http.ResponseWriter, r *http.Request) {
	existing, err := a.Store.GetWebhookEndpoint(r.Context(), r.PathValue("id"))
	if err != nil {
		writeWebhookErr(w, err)
		return
	}
	var body webhookBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if body.URL != nil {
		if err := webhooks.ValidateURL(*body.URL); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		existing.URL = strings.TrimSpace(*body.URL)
	}
	if body.Secret != nil && strings.TrimSpace(*body.Secret) != "" {
		existing.Secret = strings.TrimSpace(*body.Secret)
	}
	if body.Description != nil {
		existing.Description = strings.TrimSpace(*body.Description)
	}
	if body.Events != nil {
		existing.Events = normalizeWebhookEvents(body.Events)
	}
	if body.Enabled != nil {
		existing.Enabled = *body.Enabled
	}
	updated, err := a.Store.UpdateWebhookEndpoint(r.Context(), existing)
	if err != nil {
		writeWebhookErr(w, err)
		return
	}
	a.auditAdmin(r, store.ActivityCategoryWebhooks, "endpoint_update", "Webhook endpoint updated",
		map[string]any{"webhook_id": updated.ID, "url": updated.URL, "enabled": updated.Enabled}, "", "")
	writeJSON(w, http.StatusOK, updated)
}

func (a *API) handleDeleteWebhook(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := a.Store.DeleteWebhookEndpoint(r.Context(), id); err != nil {
		writeWebhookErr(w, err)
		return
	}
	a.auditAdmin(r, store.ActivityCategoryWebhooks, "endpoint_delete", "Webhook endpoint deleted",
		map[string]any{"webhook_id": id}, "", "")
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (a *API) handleListWebhookDeliveries(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	list, err := a.Store.ListWebhookDeliveries(r.Context(), r.PathValue("id"), limit)
	if err != nil {
		writeErr(w, err)
		return
	}
	if list == nil {
		list = []store.WebhookDelivery{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"deliveries": list})
}

func (a *API) handleTestWebhook(w http.ResponseWriter, r *http.Request) {
	if a.Webhooks == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "webhooks unavailable"})
		return
	}
	ep, err := a.Store.GetWebhookEndpoint(r.Context(), r.PathValue("id"))
	if err != nil {
		writeWebhookErr(w, err)
		return
	}
	d, err := a.Webhooks.SendTest(r.Context(), ep)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

func writeWebhookErr(w http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	writeErr(w, err)
}

func normalizeWebhookEvents(in []string) []string {
	if in == nil {
		return []string{"*"}
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, raw := range in {
		v := strings.TrimSpace(raw)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	if len(out) == 0 {
		return []string{"*"}
	}
	return out
}

func randomWebhookSecret() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
