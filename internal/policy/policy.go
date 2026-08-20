package policy

import (
	"net/url"
	"sort"
	"strings"
	"time"
)

// Kind is an allowlist entry type.
type Kind string

const (
	KindApp Kind = "app"
	KindURL Kind = "url"
)

// TargetType is where an allowlist row or grant applies.
type TargetType string

const (
	TargetGlobal TargetType = "global"
	TargetGroup  TargetType = "group"
	TargetDevice TargetType = "device"
)

// Target identifies the scope of an allowlist or grant.
type Target struct {
	Type TargetType `json:"target_type"`
	ID   string     `json:"target_id"` // empty for global; group UUID or enrollment_id otherwise
}

// Entry is a durable allowlist item.
type Entry struct {
	ID     string `json:"id"`
	Kind   Kind   `json:"kind"`
	Value  string `json:"value"`
	Target Target `json:"target"`
}

// Grant is a time-boxed allowlist addition.
type Grant struct {
	ID        string     `json:"id"`
	Kind      Kind       `json:"kind"`
	Value     string     `json:"value"`
	Target    Target     `json:"target"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// Essentials are always merged into the app allowlist so students are not stranded.
var Essentials = []string{
	"com.apple.mobilesafari",
	"com.apple.webapp",
	"com.kfilter.portal", // KFilter companion — request portal + push
}

// SystemDefaults are built-in apps/services that every supervised device needs.
// Additional Apple system bundle IDs are stored in system_allowlist.
var SystemDefaults = []string{
	"com.apple.preferences",
	"com.apple.mobilephone",
}

// Applies reports whether target applies to enrollmentID given its group memberships.
func (t Target) Applies(enrollmentID string, groupIDs []string) bool {
	switch t.Type {
	case TargetGlobal, "":
		return true
	case TargetDevice:
		return enrollmentID != "" && t.ID == enrollmentID
	case TargetGroup:
		if t.ID == "" {
			return false
		}
		for _, g := range groupIDs {
			if g == t.ID {
				return true
			}
		}
		return false
	default:
		return false
	}
}

// Effective computes allowlists: essentials ∪ global ∪ groups ∪ device (+ non-expired grants).
func Effective(base []Entry, grants []Grant, groupIDs []string, enrollmentID string, now time.Time) (apps []string, urls []string) {
	// Apps: case-insensitive dedupe, but keep Apple's casing for MDM profiles
	// (allowListedAppBundleIDs is case-sensitive on iOS).
	appByKey := map[string]string{}
	urlSet := map[string]struct{}{}

	add := func(kind Kind, value string) {
		v := Normalize(kind, value)
		if v == "" {
			return
		}
		switch kind {
		case KindApp:
			key := AppKey(v)
			if prev, ok := appByKey[key]; ok {
				// Prefer a value that preserves uppercase (canonical App Store form).
				if prev == key && v != key {
					appByKey[key] = v
				}
				return
			}
			appByKey[key] = v
		case KindURL:
			urlSet[v] = struct{}{}
		}
	}

	for _, e := range Essentials {
		add(KindApp, e)
	}
	for _, e := range base {
		if e.Target.Applies(enrollmentID, groupIDs) {
			add(e.Kind, e.Value)
		}
	}
	for _, g := range grants {
		if g.ExpiresAt != nil && !g.ExpiresAt.After(now) {
			continue
		}
		if g.Target.Applies(enrollmentID, groupIDs) {
			add(g.Kind, g.Value)
		}
	}
	apps = make([]string, 0, len(appByKey))
	for _, v := range appByKey {
		apps = append(apps, v)
	}
	sort.Strings(apps)
	return apps, sortedKeys(urlSet)
}

// AppKey is a case-insensitive identity for bundle IDs (lookups / dedupe only).
func AppKey(bundleID string) string {
	return strings.ToLower(strings.TrimSpace(bundleID))
}

// Normalize cleans bundle IDs and URL/host values.
// App bundle IDs keep their original case — iOS matching is case-sensitive.
func Normalize(kind Kind, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	switch kind {
	case KindApp:
		return value
	case KindURL:
		return normalizeURL(value)
	default:
		return value
	}
}

func normalizeURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return strings.ToLower(strings.TrimRight(raw, "/"))
	}
	host := strings.ToLower(u.Host)
	host = strings.TrimPrefix(host, "www.")
	path := strings.TrimRight(u.EscapedPath(), "/")
	return host + path
}

func sortedKeys(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
