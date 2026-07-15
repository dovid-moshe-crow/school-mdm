package profiles

import (
	"strings"
	"testing"
)

func TestBuildAllowlistProfile(t *testing.T) {
	raw, err := BuildAllowlistProfile("Test", []string{"com.apple.mobilesafari", "com.school.app"}, []string{"example.com"})
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
		"WhitelistedBookmarks",
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("profile missing %q\n%s", want, s)
		}
	}
}

func TestBuildRequestWebClipProfile(t *testing.T) {
	portal := DevicePortalURL("https://mdm.example", "ipad-42")
	if portal != "https://mdm.example/d/ipad-42" {
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
	if !strings.Contains(s, "https://mdm.example/d/ipad-42") {
		t.Fatalf("missing portal url: %s", s)
	}
}
