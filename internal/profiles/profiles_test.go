package profiles

import (
	"strings"
	"testing"
)

func TestBuildLockScreenMessageProfile(t *testing.T) {
	raw, err := BuildLockScreenMessageProfile("iPad כיתה א", "מכשיר בית ספר · KFilter")
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	for _, want := range []string{
		"com.apple.shareddeviceconfiguration",
		"AssetTagInformation",
		"iPad כיתה א",
		"LockScreenFootnote",
		"מכשיר בית ספר · KFilter",
		LockScreenPayloadIdentifier,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("profile missing %q\n%s", want, s)
		}
	}
}

func TestBuildAllowlistProfile(t *testing.T) {
	raw, err := BuildAllowlistProfile("Test", []string{"com.apple.mobilesafari", "com.school.app"}, []string{"example.com"}, "")
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	for _, want := range []string{
		"com.apple.applicationaccess",
		"allowListedAppBundleIDs",
		"com.apple.mobilesafari",
		"com.apple.webcontent-filter",
		"https://example.com",
		"AllowListBookmarks",
		"WhitelistedBookmarks",
		allowlistProfileUUID,
		allowlistAppsUUID,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("profile missing %q\n%s", want, s)
		}
	}
}

func TestBuildAllowlistProfileEmptyURLsKeepsFilterActive(t *testing.T) {
	raw, err := BuildAllowlistProfile("Test", []string{"com.apple.mobilesafari"}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	// Store website hosts keep bookmarks non-empty so iOS stays in "specific sites only".
	if !strings.Contains(s, "https://apps.apple.com") {
		t.Fatalf("expected App Store website allow entry:\n%s", s)
	}
	if strings.Contains(s, blockAllSentinelURL) {
		t.Fatalf("sentinel not needed when store websites are always listed:\n%s", s)
	}
	if !strings.Contains(s, "AllowListBookmarks") {
		t.Fatal("expected AllowListBookmarks")
	}
}

func TestBuildAllowlistProfileEmptyURLsUsesPortal(t *testing.T) {
	portal := "https://mdm.example/d/ipad-42"
	raw, err := BuildAllowlistProfile("Test", []string{"com.apple.mobilesafari"}, nil, portal)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if !strings.Contains(s, portal) {
		t.Fatalf("expected portal URL:\n%s", s)
	}
	if !strings.Contains(s, "https://mdm.example") {
		t.Fatalf("expected portal host:\n%s", s)
	}
	if !strings.Contains(s, "https://apps.apple.com") {
		t.Fatalf("expected App Store website allow entry:\n%s", s)
	}
	if !strings.Contains(s, "https://matara.pro") {
		t.Fatalf("expected Nedarim website allow entry:\n%s", s)
	}
	if !strings.Contains(s, "https://www.gstatic.com") {
		t.Fatalf("expected gstatic captcha asset allow entry:\n%s", s)
	}
	if strings.Contains(s, "https://www.google.com") {
		t.Fatalf("www.google.com must not be allow-listed for every device:\n%s", s)
	}
	if strings.Contains(s, blockAllSentinelURL) {
		t.Fatalf("should not need sentinel when portal is set:\n%s", s)
	}
	if !strings.Contains(s, "בקשת עזרה") {
		t.Fatalf("expected Hebrew portal bookmark title:\n%s", s)
	}
}

func TestBuildAllowlistProfileAlwaysIncludesPortal(t *testing.T) {
	portal := "https://mdm.example/d/ipad-42"
	raw, err := BuildAllowlistProfile("Test", nil, []string{"khanacademy.org"}, portal)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if !strings.Contains(s, "https://khanacademy.org") {
		t.Fatalf("missing allowed site:\n%s", s)
	}
	if !strings.Contains(s, portal) {
		t.Fatalf("portal should stay reachable:\n%s", s)
	}
}

func TestBuildRequestWebClipProfile(t *testing.T) {
	portal := DevicePortalURL("https://mdm.example", "ipad-42")
	if portal != "https://mdm.example/d/ipad-42?tab=request" {
		t.Fatalf("portal=%s", portal)
	}
	raw, err := BuildRequestWebClipProfile(portal)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if !strings.Contains(s, "com.apple.webClip.managed") {
		t.Fatalf("missing webclip type: %s", s)
	}
	if !strings.Contains(s, "https://mdm.example/d/ipad-42?tab=request") {
		t.Fatalf("missing portal url: %s", s)
	}
	if !strings.Contains(s, "בקשת עזרה") {
		t.Fatalf("missing Hebrew webclip label: %s", s)
	}
}

func TestBuildStoreWebClipProfile(t *testing.T) {
	store := DeviceStoreURL("https://mdm.example", "ipad-42")
	if store != "https://mdm.example/d/ipad-42/store" {
		t.Fatalf("store=%s", store)
	}
	raw, err := BuildStoreWebClipProfile(store)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if !strings.Contains(s, store) {
		t.Fatalf("missing store url: %s", s)
	}
	if !strings.Contains(s, "חנות אפליקציות") {
		t.Fatalf("missing store label: %s", s)
	}
	if !strings.Contains(s, "com.schoolmdm.webclip.store") {
		t.Fatalf("missing store identifier: %s", s)
	}
}
