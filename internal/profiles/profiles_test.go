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
	raw, err := BuildRequestWebClipProfile("https://mdm.example/portal")
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if !strings.Contains(s, "com.apple.webClip.managed") {
		t.Fatalf("missing webclip type: %s", s)
	}
	if !strings.Contains(s, "https://mdm.example/portal") {
		t.Fatalf("missing portal url: %s", s)
	}
}
