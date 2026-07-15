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

// Entry is a durable allowlist item.
type Entry struct {
	ID    string
	Kind  Kind
	Value string
	Scope string // e.g. "global"
}

// Grant is a time-boxed allowlist addition.
type Grant struct {
	ID           string
	Kind         Kind
	Value        string
	EnrollmentID string
	ExpiresAt    *time.Time
}

// Essentials are always merged into the app allowlist so students are not stranded.
var Essentials = []string{
	"com.apple.mobilesafari",
	"com.apple.webapp",
}

// Effective computes the allowlists at now for an optional enrollment.
func Effective(base []Entry, grants []Grant, enrollmentID string, now time.Time) (apps []string, urls []string) {
	appSet := map[string]struct{}{}
	urlSet := map[string]struct{}{}

	for _, e := range Essentials {
		appSet[e] = struct{}{}
	}
	for _, e := range base {
		v := Normalize(e.Kind, e.Value)
		if v == "" {
			continue
		}
		switch e.Kind {
		case KindApp:
			appSet[v] = struct{}{}
		case KindURL:
			urlSet[v] = struct{}{}
		}
	}
	for _, g := range grants {
		if g.ExpiresAt != nil && !g.ExpiresAt.After(now) {
			continue
		}
		if g.EnrollmentID != "" && enrollmentID != "" && g.EnrollmentID != enrollmentID {
			continue
		}
		v := Normalize(g.Kind, g.Value)
		if v == "" {
			continue
		}
		switch g.Kind {
		case KindApp:
			appSet[v] = struct{}{}
		case KindURL:
			urlSet[v] = struct{}{}
		}
	}
	return sortedKeys(appSet), sortedKeys(urlSet)
}

// Normalize cleans bundle IDs and URL/host values.
func Normalize(kind Kind, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	switch kind {
	case KindApp:
		return strings.ToLower(value)
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
		// fall back to trimmed lowercase path-ish string
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
