package httpapi

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	sessionCookie     = "school_mdm_session"
	oauthStateCookie  = "school_mdm_oauth_state"
	oauthNextCookie   = "school_mdm_oauth_next"
	oauthRedirectCookie = "school_mdm_oauth_redirect"
	sessionTTL        = 7 * 24 * time.Hour
)

type sessionClaims struct {
	Email string `json:"email"`
	Name  string `json:"name"`
	jwt.RegisteredClaims
}

func (a *API) sessionKey() []byte {
	if s := strings.TrimSpace(a.Cfg.SessionSecret); s != "" {
		sum := sha256.Sum256([]byte(s))
		return sum[:]
	}
	h := sha256.New()
	h.Write([]byte("school-mdm-session"))
	h.Write([]byte(a.Cfg.GoogleClientSecret))
	for _, t := range a.Cfg.AdminTokens {
		h.Write([]byte{0})
		h.Write([]byte(t))
	}
	return h.Sum(nil)
}

func (a *API) signSession(email, name string) (string, error) {
	now := time.Now()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, sessionClaims{
		Email: email,
		Name:  name,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   email,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(sessionTTL)),
		},
	})
	return tok.SignedString(a.sessionKey())
}

func (a *API) parseSession(raw string) (sessionClaims, bool) {
	var claims sessionClaims
	tok, err := jwt.ParseWithClaims(raw, &claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return a.sessionKey(), nil
	})
	if err != nil || !tok.Valid || strings.TrimSpace(claims.Email) == "" {
		return sessionClaims{}, false
	}
	return claims, true
}

func (a *API) sessionFromRequest(r *http.Request) (sessionClaims, bool) {
	c, err := r.Cookie(sessionCookie)
	if err != nil || c.Value == "" {
		return sessionClaims{}, false
	}
	return a.parseSession(c.Value)
}

func (a *API) setSessionCookie(w http.ResponseWriter, r *http.Request, raw string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    raw,
		Path:     "/",
		MaxAge:   int(sessionTTL.Seconds()),
		HttpOnly: true,
		Secure:   cookieSecure(r),
		SameSite: http.SameSiteLaxMode,
	})
}

func (a *API) clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   cookieSecure(r),
		SameSite: http.SameSiteLaxMode,
	})
}

func cookieSecure(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	return strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

func requestOrigin(r *http.Request) string {
	proto := "http"
	if cookieSecure(r) {
		proto = "https"
	}
	host := strings.TrimSpace(r.Header.Get("X-Forwarded-Host"))
	if host == "" {
		host = r.Host
	}
	return proto + "://" + host
}

func randomNonce() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return hex.EncodeToString([]byte(time.Now().String()))
	}
	return hex.EncodeToString(b)
}

func (a *API) signOpaque(v string) string {
	mac := hmac.New(sha256.New, a.sessionKey())
	mac.Write([]byte(v))
	return v + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (a *API) checkOpaque(v string) (string, bool) {
	i := strings.LastIndex(v, ".")
	if i < 1 {
		return "", false
	}
	raw, sig := v[:i], v[i+1:]
	mac := hmac.New(sha256.New, a.sessionKey())
	mac.Write([]byte(raw))
	want := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(sig), []byte(want)) {
		return "", false
	}
	return raw, true
}
