package httpapi

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type googleUserInfo struct {
	Email         string `json:"email"`
	EmailVerified any    `json:"email_verified"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
	HD            string `json:"hd"`
}

func (a *API) googleOAuth(redirectURL string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     a.Cfg.GoogleClientID,
		ClientSecret: a.Cfg.GoogleClientSecret,
		RedirectURL:  redirectURL,
		Scopes:       []string{"openid", "email", "profile"},
		Endpoint:     google.Endpoint,
	}
}

func (a *API) handleAuthConfig(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"google":        a.Cfg.GoogleEnabled(),
		"token_login":   !a.Cfg.GoogleEnabled(),
		"hosted_domain": a.Cfg.AdminGoogleDomain,
	})
}

func (a *API) handleAuthMe(w http.ResponseWriter, r *http.Request) {
	if u, ok := a.sessionFromRequest(r); ok {
		writeJSON(w, http.StatusOK, map[string]any{
			"email":  u.Email,
			"name":   u.Name,
			"method": "google",
		})
		return
	}
	// When Google sign-in is on, a leftover ADMIN_TOKENS value in the browser
	// must not count as a UI login.
	if a.Cfg.GoogleEnabled() {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "admin authorization required"})
		return
	}
	if tok := bearerToken(r); tok != "" && a.Cfg.ValidAdminToken(tok) {
		writeJSON(w, http.StatusOK, map[string]any{
			"email":  "api-token",
			"name":   "Admin",
			"method": "token",
		})
		return
	}
	if _, pass, ok := r.BasicAuth(); ok && a.Cfg.ValidAdminToken(pass) {
		writeJSON(w, http.StatusOK, map[string]any{
			"email":  "api-token",
			"name":   "Admin",
			"method": "token",
		})
		return
	}
	writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "admin authorization required"})
}

func (a *API) handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	a.clearSessionCookie(w, r)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (a *API) handleAuthGoogleStart(w http.ResponseWriter, r *http.Request) {
	if !a.Cfg.GoogleEnabled() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Google sign-in is not configured"})
		return
	}
	redirectURL := requestOrigin(r) + "/api/auth/google/callback"
	state := randomNonce()
	next := strings.TrimSpace(r.URL.Query().Get("next"))
	if next == "" || !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") {
		next = "/admin"
	}
	secure := cookieSecure(r)
	http.SetCookie(w, &http.Cookie{
		Name: oauthStateCookie, Value: a.signOpaque(state), Path: "/", MaxAge: 600,
		HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode,
	})
	http.SetCookie(w, &http.Cookie{
		Name: oauthNextCookie, Value: next, Path: "/", MaxAge: 600,
		HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode,
	})
	http.SetCookie(w, &http.Cookie{
		Name: oauthRedirectCookie, Value: a.signOpaque(redirectURL), Path: "/", MaxAge: 600,
		HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode,
	})
	opts := []oauth2.AuthCodeOption{oauth2.AccessTypeOnline, oauth2.SetAuthURLParam("prompt", "select_account")}
	if a.Cfg.AdminGoogleDomain != "" {
		opts = append(opts, oauth2.SetAuthURLParam("hd", a.Cfg.AdminGoogleDomain))
	}
	http.Redirect(w, r, a.googleOAuth(redirectURL).AuthCodeURL(state, opts...), http.StatusFound)
}

func (a *API) handleAuthGoogleCallback(w http.ResponseWriter, r *http.Request) {
	fail := func(status int, msg string) {
		q := url.Values{"error": {msg}}
		http.Redirect(w, r, "/admin?"+q.Encode(), http.StatusFound)
	}
	if !a.Cfg.GoogleEnabled() {
		fail(http.StatusServiceUnavailable, "google-disabled")
		return
	}
	stateCookie, err := r.Cookie(oauthStateCookie)
	if err != nil {
		fail(http.StatusBadRequest, "missing-state")
		return
	}
	wantState, ok := a.checkOpaque(stateCookie.Value)
	if !ok || wantState == "" || wantState != strings.TrimSpace(r.URL.Query().Get("state")) {
		fail(http.StatusBadRequest, "bad-state")
		return
	}
	redirCookie, err := r.Cookie(oauthRedirectCookie)
	if err != nil {
		fail(http.StatusBadRequest, "missing-redirect")
		return
	}
	redirectURL, ok := a.checkOpaque(redirCookie.Value)
	if !ok || redirectURL == "" {
		fail(http.StatusBadRequest, "bad-redirect")
		return
	}
	code := strings.TrimSpace(r.URL.Query().Get("code"))
	if code == "" {
		fail(http.StatusBadRequest, "missing-code")
		return
	}
	ctx := r.Context()
	tok, err := a.googleOAuth(redirectURL).Exchange(ctx, code)
	if err != nil {
		fail(http.StatusBadGateway, "token-exchange")
		return
	}
	client := a.googleOAuth(redirectURL).Client(ctx, tok)
	resp, err := client.Get("https://openidconnect.googleapis.com/v1/userinfo")
	if err != nil {
		fail(http.StatusBadGateway, "userinfo")
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		fail(http.StatusBadGateway, "userinfo")
		return
	}
	var info googleUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		fail(http.StatusBadGateway, "userinfo")
		return
	}
	email := strings.ToLower(strings.TrimSpace(info.Email))
	if email == "" || !googleEmailVerified(info.EmailVerified) {
		fail(http.StatusForbidden, "unverified-email")
		return
	}
	if !a.Cfg.AllowAdminEmail(email) {
		fail(http.StatusForbidden, "not-allowed")
		return
	}
	raw, err := a.signSession(email, strings.TrimSpace(info.Name))
	if err != nil {
		fail(http.StatusInternalServerError, "session")
		return
	}
	a.setSessionCookie(w, r, raw)
	clear := func(name string) {
		http.SetCookie(w, &http.Cookie{
			Name: name, Value: "", Path: "/", MaxAge: -1, HttpOnly: true,
			Secure: cookieSecure(r), SameSite: http.SameSiteLaxMode,
		})
	}
	clear(oauthStateCookie)
	clear(oauthRedirectCookie)
	next := "/admin"
	if c, err := r.Cookie(oauthNextCookie); err == nil && strings.HasPrefix(c.Value, "/") && !strings.HasPrefix(c.Value, "//") {
		next = c.Value
	}
	clear(oauthNextCookie)
	http.SetCookie(w, &http.Cookie{Name: oauthNextCookie, Value: "", Path: "/", MaxAge: -1, Expires: time.Unix(0, 0)})
	http.Redirect(w, r, next, http.StatusFound)
}

func googleEmailVerified(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return strings.EqualFold(t, "true")
	default:
		return false
	}
}
