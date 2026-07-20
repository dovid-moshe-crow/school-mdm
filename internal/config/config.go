package config

import (
	"fmt"
	"os"
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
}

// Load reads configuration from the process environment.
func Load() (Config, error) {
	cfg := Config{
		HTTPAddr:      getenv("HTTP_ADDR", ":8080"),
		DatabaseURL:   strings.TrimSpace(os.Getenv("DATABASE_URL")),
		AdminTokens:   splitCSV(getenv("ADMIN_TOKENS", "dev-admin-token")),
		PortalBaseURL: strings.TrimRight(getenv("PORTAL_BASE_URL", "http://localhost:8080"), "/"),
		ItunesCountry: getenv("ITUNES_COUNTRY", "il"),
		ItunesLang:    strings.TrimSpace(os.Getenv("ITUNES_LANG")),
	}
	if cfg.HTTPAddr == "" {
		return Config{}, fmt.Errorf("HTTP_ADDR must not be empty")
	}
	return cfg, nil
}

func getenv(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
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
