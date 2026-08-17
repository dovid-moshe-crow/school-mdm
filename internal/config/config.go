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

	// MDM protocol (additive). When MDMEnqueue=live, DATABASE_URL + MDM_SCEP_CAPASS are required.
	MDMEnqueue      string // stub | live
	MDMPublicURL    string
	MDMTopic        string
	MDMCheckin      bool
	MDMCertHeader   string
	MDMSCEPPass      string
	MDMSCEPChallenge string
	MDMDebug         bool
	// MDMDepName seeds mdm_settings.dep_name once when the DB row is missing.
	// Runtime value is edited in admin UI / DB — not read from env after seed.
	MDMDepName string
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
		MDMEnqueue:         strings.ToLower(getenv("MDM_ENQUEUE", "stub")),
		MDMPublicURL:       strings.TrimRight(strings.TrimSpace(os.Getenv("MDM_PUBLIC_URL")), "/"),
		MDMTopic:           strings.TrimSpace(os.Getenv("MDM_TOPIC")),
		MDMCheckin:         getenvBool("MDM_CHECKIN", false),
		MDMCertHeader:      strings.TrimSpace(os.Getenv("MDM_CERT_HEADER")),
		MDMSCEPPass:        strings.TrimSpace(os.Getenv("MDM_SCEP_CAPASS")),
		MDMSCEPChallenge:   strings.TrimSpace(os.Getenv("MDM_SCEP_CHALLENGE")),
		MDMDebug:           getenvBool("MDM_DEBUG", false),
		MDMDepName:         getenv("MDM_DEP_NAME", "nanok"),
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
	if cfg.MDMEnqueue != "stub" && cfg.MDMEnqueue != "live" {
		return Config{}, fmt.Errorf("MDM_ENQUEUE must be stub or live")
	}
	if cfg.MDMEnqueue == "live" {
		if cfg.DatabaseURL == "" {
			return Config{}, fmt.Errorf("DATABASE_URL is required when MDM_ENQUEUE=live")
		}
		if cfg.MDMSCEPPass == "" {
			return Config{}, fmt.Errorf("MDM_SCEP_CAPASS is required when MDM_ENQUEUE=live")
		}
	}
	return cfg, nil
}

func getenvBool(key string, fallback bool) bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if v == "" {
		return fallback
	}
	switch v {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

// MDMLive reports whether the Apple MDM protocol plane should start.
func (c Config) MDMLive() bool {
	return c.MDMEnqueue == "live"
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
