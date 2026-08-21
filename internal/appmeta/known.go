package appmeta

import (
	"strings"

	"github.com/dwdmsh/school-mdm/internal/policy"
	"github.com/dwdmsh/school-mdm/internal/store"
)

// Known returns a local display name for apps that are not in the App Store
// (system apps, the KFilter companion). Lookup is case-insensitive.
func Known(bundleID string) (store.AppMeta, bool) {
	key := policy.AppKey(bundleID)
	name, ok := knownNames[key]
	if !ok {
		return store.AppMeta{}, false
	}
	id := strings.TrimSpace(bundleID)
	if id == "" {
		id = key
	}
	return store.AppMeta{
		BundleID: id,
		Name:     name,
		Source:   "local",
	}, true
}

// KnownName is the Hebrew/friendly title for a well-known bundle ID.
func KnownName(bundleID string) (string, bool) {
	n, ok := knownNames[policy.AppKey(bundleID)]
	return n, ok
}

// SearchKnown returns built-in system/companion apps matching query (name or bundle id).
func SearchKnown(query string) []store.AppMeta {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return nil
	}
	var out []store.AppMeta
	seen := map[string]struct{}{}
	for id, name := range knownNames {
		if strings.Contains(id, q) || strings.Contains(strings.ToLower(name), q) {
			if m, ok := Known(id); ok {
				out = append(out, m)
				seen[id] = struct{}{}
			}
		}
	}
	if LooksLikeBundleID(query) {
		key := policy.AppKey(query)
		if _, ok := seen[key]; !ok {
			if m, ok := Known(query); ok {
				out = append([]store.AppMeta{m}, out...)
			} else {
				id := strings.TrimSpace(query)
				out = append([]store.AppMeta{{
					BundleID: id,
					Name:     id,
					Source:   "local",
				}}, out...)
			}
		}
	}
	return out
}

// LooksLikeBundleID reports whether s looks like an iOS bundle identifier.
func LooksLikeBundleID(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" || strings.ContainsAny(s, " \t") {
		return false
	}
	if !strings.Contains(s, ".") {
		return false
	}
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '-' {
			continue
		}
		return false
	}
	return true
}

var knownNames = map[string]string{
	"com.apple.mobilesafari":    "Safari",
	"com.apple.webapp":          "אפליקציית אינטרנט",
	"com.kfilter.portal":        "KFilter",
	"com.apple.appstore":        "App Store",
	"com.apple.mobilephone":     "טלפון",
	"com.apple.mobilemail":      "דואר",
	"com.apple.mobilesms":       "הודעות",
	"com.apple.mobilecal":       "לוח שנה",
	"com.apple.mobilenotes":     "פתקים",
	"com.apple.reminders":       "תזכורות",
	"com.apple.mobiletimer":     "שעון",
	"com.apple.calculator":      "מחשבון",
	"com.apple.camera":          "מצלמה",
	"com.apple.mobileslideshow": "תמונות",
	"com.apple.preferences":     "הגדרות",
	"com.apple.music":           "מוזיקה",
	"com.apple.maps":            "מפות",
	"com.apple.documentsapp":    "קבצים",
	"com.apple.weather":         "מזג אוויר",
	"com.apple.passbook":        "Wallet",
	"com.apple.health":          "בריאות",
	"com.apple.fitness":         "כושר",
	"com.apple.news":            "חדשות",
	"com.apple.tv":              "TV",
	"com.apple.podcasts":        "פודקאסטים",
	"com.apple.ibooks":          "ספרים",
	"com.apple.facetime":        "FaceTime",
	"com.apple.home":            "הבית",
	"com.apple.shortcuts":       "קיצורים",
	"com.apple.translate":       "תרגום",
	"com.apple.measure":         "מדידה",
	"com.apple.compass":         "מצפן",
	"com.apple.voicememos":      "תזכירים קוליים",
	"com.apple.tips":            "טיפים",
	"com.apple.magnifier":       "זכוכית מגדלת",
	"com.apple.findmy":          "חיפוש",
	"com.apple.contacts":        "אנשי קשר",
	"com.apple.keynote":         "Keynote",
	"com.apple.numbers":         "Numbers",
	"com.apple.pages":           "Pages",
	"com.apple.testflight":      "TestFlight",
}
