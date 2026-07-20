package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config is loaded from environment variables.
type Config struct {
	HTTPAddr      string
	DatabaseURL   string
	AdminTokens   []string
	PortalBaseURL string
	// ItunesCountry is the App Store storefront (default il).
	ItunesCountry string
	// ItunesLang is optional; only en_us / ja_jp are valid. Leave empty for IL storefront.
	ItunesLang string

	// NedarimMode is "fake" (local simulation) or "live" (real DebitIframe).
	NedarimMode        string
	NedarimMosadID     string
	NedarimApiPassword string
	NedarimApiValid    string
	// CreditsAccessCost is how many credits an access request costs (default 1).
	CreditsAccessCost int
}

// Load reads configuration from the process environment.
func Load() (Config, error) {
	cfg := Config{
		HTTPAddr:           getenv("HTTP_ADDR", ":8080"),
		DatabaseURL:        strings.TrimSpace(os.Getenv("DATABASE_URL")),
		AdminTokens:        splitCSV(getenv("ADMIN_TOKENS", "dev-admin-token")),
		PortalBaseURL:      strings.TrimRight(getenv("PORTAL_BASE_URL", "http://localhost:8080"), "/"),
		ItunesCountry:      getenv("ITUNES_COUNTRY", "il"),
		ItunesLang:         strings.TrimSpace(os.Getenv("ITUNES_LANG")),
		NedarimMode:        strings.ToLower(getenv("NEDARIM_MODE", "fake")),
		NedarimMosadID:     strings.TrimSpace(os.Getenv("NEDARIM_MOSAD_ID")),
		NedarimApiPassword: strings.TrimSpace(os.Getenv("NEDARIM_API_PASSWORD")),
		NedarimApiValid:    strings.TrimSpace(os.Getenv("NEDARIM_API_VALID")),
		CreditsAccessCost:  getenvInt("CREDITS_ACCESS_COST", 1),
	}
	if cfg.HTTPAddr == "" {
		return Config{}, fmt.Errorf("HTTP_ADDR must not be empty")
	}
	if cfg.NedarimMode != "fake" && cfg.NedarimMode != "live" {
		return Config{}, fmt.Errorf("NEDARIM_MODE must be fake or live")
	}
	if cfg.CreditsAccessCost < 1 {
		return Config{}, fmt.Errorf("CREDITS_ACCESS_COST must be >= 1")
	}
	return cfg, nil
}

func getenv(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func getenvInt(key string, fallback int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// ValidAdminToken reports whether token is configured.
func (c Config) ValidAdminToken(token string) bool {
	for _, t := range c.AdminTokens {
		if t == token {
			return true
		}
	}
	return false
}

// NedarimLiveReady reports whether live Nedarim credentials are present.
func (c Config) NedarimLiveReady() bool {
	return c.NedarimMosadID != "" && c.NedarimApiValid != ""
}
