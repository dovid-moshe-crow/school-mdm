package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/dwdmsh/school-mdm/internal/store"
)

// POST /api/device/{deviceID}/push-token — register Expo push token for the student app.
// Auth matches other device portal routes: enrollment id in the path is the trust boundary.
func (a *API) handleDevicePushToken(w http.ResponseWriter, r *http.Request) {
	enrollment := strings.TrimSpace(r.PathValue("deviceID"))
	if enrollment == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "device id required"})
		return
	}
	var body struct {
		Token    string `json:"token"`
		Platform string `json:"platform"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	token := strings.TrimSpace(body.Token)
	if token == "" || len(token) > 512 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "token required"})
		return
	}
	platform := strings.TrimSpace(strings.ToLower(body.Platform))
	if platform == "" {
		platform = "ios"
	}
	if platform != "ios" && platform != "android" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "platform must be ios or android"})
		return
	}
	_ = a.Store.EnsureDevice(r.Context(), enrollment)
	if err := a.Store.UpsertPushToken(r.Context(), store.DevicePushToken{
		EnrollmentID: enrollment,
		Token:        token,
		Platform:     platform,
	}); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// Alias for plan path POST /api/devices/{id}/push-token.
func (a *API) handleDevicePushTokenAlias(w http.ResponseWriter, r *http.Request) {
	r.SetPathValue("deviceID", r.PathValue("id"))
	a.handleDevicePushToken(w, r)
}
