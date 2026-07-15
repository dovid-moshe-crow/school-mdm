package policy

import (
	"testing"
	"time"
)

func TestEffectiveUnionAcrossGroupsAndDevice(t *testing.T) {
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	exp := now.Add(time.Hour)
	expired := now.Add(-time.Minute)

	base := []Entry{
		{Kind: KindURL, Value: "school.edu", Target: Target{Type: TargetGlobal}},
		{Kind: KindApp, Value: "com.school.learn", Target: Target{Type: TargetGroup, ID: "g-math"}},
		{Kind: KindURL, Value: "khanacademy.org", Target: Target{Type: TargetGroup, ID: "g-math"}},
		{Kind: KindApp, Value: "com.school.art", Target: Target{Type: TargetGroup, ID: "g-art"}},
		{Kind: KindApp, Value: "com.school.onlya", Target: Target{Type: TargetDevice, ID: "dev-a"}},
	}
	grants := []Grant{
		{Kind: KindURL, Value: "example.com/lesson", Target: Target{Type: TargetDevice, ID: "dev-a"}, ExpiresAt: &exp},
		{Kind: KindURL, Value: "expired.example", Target: Target{Type: TargetDevice, ID: "dev-a"}, ExpiresAt: &expired},
		{Kind: KindApp, Value: "com.school.other", Target: Target{Type: TargetDevice, ID: "dev-b"}, ExpiresAt: &exp},
	}

	apps, urls := Effective(base, grants, []string{"g-math", "g-art"}, "dev-a", now)
	assertContains(t, apps, "com.apple.mobilesafari")
	assertContains(t, apps, "com.school.learn")
	assertContains(t, apps, "com.school.art")
	assertContains(t, apps, "com.school.onlya")
	assertNotContains(t, apps, "com.school.other")
	assertContains(t, urls, "school.edu")
	assertContains(t, urls, "khanacademy.org")
	assertContains(t, urls, "example.com/lesson")
	assertNotContains(t, urls, "expired.example")

	_, urlsB := Effective(base, grants, []string{"g-math"}, "dev-b", now)
	assertNotContains(t, urlsB, "example.com/lesson")
	appsB, _ := Effective(base, grants, nil, "dev-b", now)
	assertContains(t, appsB, "com.school.other")
	assertNotContains(t, appsB, "com.school.onlya")
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
