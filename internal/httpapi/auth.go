package httpapi

import (
	"net/http"
	"strings"
)

// requireAdmin wraps a handler with Bearer admin token check (shared for school + MDM mutators).
func (a *API) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !a.authorizedAdmin(r) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "admin authorization required"})
			return
		}
		next(w, r)
	}
}

// DepAuth wraps NanoDEP /dep handlers: Bearer admin token or Basic (password = admin token).
func (a *API) DepAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a.authorizedAdmin(r) {
			next.ServeHTTP(w, r)
			return
		}
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "admin authorization required"})
	})
}

func (a *API) authorizedAdmin(r *http.Request) bool {
	if tok := bearerToken(r); tok != "" && a.Cfg.ValidAdminToken(tok) {
		return true
	}
	if _, pass, ok := r.BasicAuth(); ok && a.Cfg.ValidAdminToken(pass) {
		return true
	}
	return false
}

func bearerToken(r *http.Request) string {
	h := strings.TrimSpace(r.Header.Get("Authorization"))
	if h == "" {
		return strings.TrimSpace(r.URL.Query().Get("admin_token"))
	}
	const p = "Bearer "
	if strings.HasPrefix(strings.ToLower(h), strings.ToLower(p)) {
		return strings.TrimSpace(h[len(p):])
	}
	return ""
}
