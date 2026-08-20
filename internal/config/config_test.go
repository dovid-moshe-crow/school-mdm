package config

import (
	"os"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("ADMIN_TOKENS", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("PORTAL_BASE_URL", "")
	t.Setenv("NEDARIM_MODE", "")
	t.Setenv("CREDITS_ACCESS_COST", "")
	// clear then load with defaults via unset empty - Load uses getenv fallbacks
	_ = os.Unsetenv("HTTP_ADDR")
	_ = os.Unsetenv("ADMIN_TOKENS")
	_ = os.Unsetenv("DATABASE_URL")
	_ = os.Unsetenv("PORTAL_BASE_URL")
	_ = os.Unsetenv("NEDARIM_MODE")
	_ = os.Unsetenv("CREDITS_ACCESS_COST")
	_ = os.Unsetenv("MDM_ENQUEUE")
	_ = os.Unsetenv("MDM_SCEP_CAPASS")
	_ = os.Unsetenv("MDM_PUBLIC_URL")
	_ = os.Unsetenv("MDM_TOPIC")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MDMEnqueue != "stub" {
		t.Fatalf("MDMEnqueue=%s", cfg.MDMEnqueue)
	}
	if cfg.HTTPAddr != ":8080" {
		t.Fatalf("HTTPAddr=%s", cfg.HTTPAddr)
	}
	if !cfg.ValidAdminToken("dev-admin-token") {
		t.Fatal("expected default admin token")
	}
	if cfg.NedarimMode != "fake" {
		t.Fatalf("NedarimMode=%s", cfg.NedarimMode)
	}
	if cfg.CreditsAccessCost != 1 {
		t.Fatalf("CreditsAccessCost=%d", cfg.CreditsAccessCost)
	}
}

func TestAllowAdminEmail(t *testing.T) {
	onlyList := Config{AdminEmails: []string{"Ada@School.com"}}
	if !onlyList.AllowAdminEmail("ada@school.com") {
		t.Fatal("list should match")
	}
	if onlyList.AllowAdminEmail("other@school.com") {
		t.Fatal("unknown email")
	}
	domain := Config{AdminGoogleDomain: "school.com"}
	if !domain.AllowAdminEmail("ada@school.com") {
		t.Fatal("domain should match")
	}
	if domain.AllowAdminEmail("ada@gmail.com") {
		t.Fatal("wrong domain")
	}
	empty := Config{}
	if empty.AllowAdminEmail("ada@school.com") {
		t.Fatal("fail closed with no allowlist")
	}
}
