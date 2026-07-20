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

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
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
