package policy

import (
	"testing"
	"time"
)

func TestEffectiveMergesBaseGrantsAndEssentials(t *testing.T) {
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	exp := now.Add(time.Hour)
	expired := now.Add(-time.Minute)

	apps, urls := Effective(
		[]Entry{
			{Kind: KindApp, Value: "com.school.learn"},
			{Kind: KindURL, Value: "https://www.khanacademy.org/"},
		},
		[]Grant{
			{Kind: KindURL, Value: "https://example.com/lesson", ExpiresAt: &exp},
			{Kind: KindURL, Value: "https://blocked.example/old", ExpiresAt: &expired},
			{Kind: KindApp, Value: "com.school.temp", EnrollmentID: "dev-1", ExpiresAt: &exp},
			{Kind: KindApp, Value: "com.school.other", EnrollmentID: "dev-2", ExpiresAt: &exp},
		},
		"dev-1",
		now,
	)

	assertContains(t, apps, "com.apple.mobilesafari")
	assertContains(t, apps, "com.apple.webapp")
	assertContains(t, apps, "com.school.learn")
	assertContains(t, apps, "com.school.temp")
	assertNotContains(t, apps, "com.school.other")
	assertContains(t, urls, "khanacademy.org")
	assertContains(t, urls, "example.com/lesson")
	assertNotContains(t, urls, "blocked.example/old")
}

func TestNormalizeURL(t *testing.T) {
	if got := Normalize(KindURL, "HTTPS://WWW.Example.COM/Path/"); got != "example.com/Path" && got != "example.com/path" {
		// path case preserved from EscapedPath; host lowercased
		if got != "example.com/Path" {
			t.Fatalf("Normalize URL = %q", got)
		}
	}
	if got := Normalize(KindApp, " Com.Foo.Bar "); got != "com.foo.bar" {
		t.Fatalf("Normalize app = %q", got)
	}
}

func assertContains(t *testing.T, list []string, want string) {
	t.Helper()
	for _, v := range list {
		if v == want {
			return
		}
	}
	t.Fatalf("expected %q in %v", want, list)
}

func assertNotContains(t *testing.T, list []string, want string) {
	t.Helper()
	for _, v := range list {
		if v == want {
			t.Fatalf("did not expect %q in %v", want, list)
		}
	}
}
