package httpapi

import (
	"net/http"
	"strings"
)

// requireAdmin wraps a handler with admin session cookie or Bearer token check.
func (a *API) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !a.authorizedAdmin(r) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "admin authorization required"})
			return
		}
		next(w, r)
	}
}

// DepAuth wraps NanoDEP /dep handlers: session cookie, Bearer admin token, or Basic (password = admin token).
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
	if _, ok := a.sessionFromRequest(r); ok {
		return true
	}
	if tok := bearerToken(r); a.validBearer(r, tok) {
		return true
	}
	if _, pass, ok := r.BasicAuth(); ok && a.validBearer(r, pass) {
		return true
	}
	return false
}

func (a *API) validBearer(r *http.Request, tok string) bool {
	tok = strings.TrimSpace(tok)
	if tok == "" {
		return false
	}
	if a.Cfg.ValidAdminToken(tok) {
		return true
	}
	if a.Store == nil {
		return false
	}
	_, err := a.Store.TouchAPIToken(r.Context(), tok)
	return err == nil
}

func bearerToken(r *http.Request) string {
	h := strings.TrimSpace(r.Header.Get("Authorization"))
	const p = "Bearer "
	if strings.HasPrefix(strings.ToLower(h), strings.ToLower(p)) {
		return strings.TrimSpace(h[len(p):])
	}
	return ""
}
