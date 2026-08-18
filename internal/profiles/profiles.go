package profiles

import (
	"fmt"
	"net/url"
	"strings"
)

// AllowlistPayloadIdentifier is the top-level PayloadIdentifier for school allowlists.
const AllowlistPayloadIdentifier = "com.schoolmdm.allowlists"

// Stable PayloadUUIDs so InstallProfile replaces the same profile on devices.
const (
	allowlistProfileUUID      = "a1000000-0000-4000-8000-000000000001"
	allowlistAppsUUID         = "a1000000-0000-4000-8000-000000000002"
	allowlistWebUUID          = "a1000000-0000-4000-8000-000000000003"
	webClipProfileUUID        = "a1000000-0000-4000-8000-000000000011"
	webClipPayloadUUID        = "a1000000-0000-4000-8000-000000000012"
	storeWebClipProfileUUID   = "a1000000-0000-4000-8000-000000000013"
	storeWebClipPayloadUUID   = "a1000000-0000-4000-8000-000000000014"
	lockScreenProfileUUID     = "a1000000-0000-4000-8000-000000000021"
	lockScreenPayloadUUID     = "a1000000-0000-4000-8000-000000000022"
	companionNotifProfileUUID = "a1000000-0000-4000-8000-000000000031"
	companionNotifPayloadUUID = "a1000000-0000-4000-8000-000000000032"
)

// LockScreenPayloadIdentifier is the top-level PayloadIdentifier for lock screen text.
const LockScreenPayloadIdentifier = "com.schoolmdm.lockscreen"

// CompanionNotificationsIdentifier is the profile that forces KFilter notification settings.
const CompanionNotificationsIdentifier = "com.schoolmdm.companion.notifications"

const DefaultCompanionBundleID = "com.kfilter.portal"

// blockAllSentinelURL keeps Safari in "specific websites only" mode when the
// school allowlist has zero URLs. iOS treats a missing/empty bookmark allow-list
// as unrestricted (all sites allowed).
const blockAllSentinelURL = "https://localhost"

// BuildAllowlistProfile creates a configuration profile restricting apps and
// websites to the provided allowlists (not Single App Mode / kiosk).
//
// portalURL, when set, is always included in the website allow-list so students
// can open the request portal even when no other sites are permitted.
func BuildAllowlistProfile(displayName string, apps, urls []string, portalURL string) ([]byte, error) {
	if displayName == "" {
		displayName = "School Allowlists"
	}

	bookmarks := websiteBookmarks(urls, portalURL)

	var b strings.Builder
	b.WriteString(plistHeader)
	b.WriteString("<dict>\n")
	writeKeyString(&b, "PayloadDisplayName", displayName)
	writeKeyString(&b, "PayloadIdentifier", AllowlistPayloadIdentifier)
	writeKeyString(&b, "PayloadType", "Configuration")
	writeKeyString(&b, "PayloadUUID", allowlistProfileUUID)
	writeKeyInt(&b, "PayloadVersion", 1)
	b.WriteString("\t<key>PayloadContent</key>\n\t<array>\n")

	// Application access allowlist
	b.WriteString("\t\t<dict>\n")
	writeKeyStringIndent(&b, 3, "PayloadType", "com.apple.applicationaccess")
	writeKeyStringIndent(&b, 3, "PayloadIdentifier", "com.schoolmdm.allowlists.apps")
	writeKeyStringIndent(&b, 3, "PayloadUUID", allowlistAppsUUID)
	writeKeyIntIndent(&b, 3, "PayloadVersion", 1)
	b.WriteString("\t\t\t<key>allowListedAppBundleIDs</key>\n\t\t\t<array>\n")
	for _, app := range apps {
		writeStringIndent(&b, 4, app)
	}
	b.WriteString("\t\t\t</array>\n\t\t</dict>\n")

	// Built-in web content filter: specific websites only (AllowListBookmarks).
	// PermittedURLs only applies when AutoFilterEnabled is true (adult filter
	// exceptions) — it does not implement a school allow-list by itself.
	b.WriteString("\t\t<dict>\n")
	writeKeyStringIndent(&b, 3, "PayloadType", "com.apple.webcontent-filter")
	writeKeyStringIndent(&b, 3, "PayloadIdentifier", "com.schoolmdm.allowlists.web")
	writeKeyStringIndent(&b, 3, "PayloadUUID", allowlistWebUUID)
	writeKeyIntIndent(&b, 3, "PayloadVersion", 1)
	writeKeyStringIndent(&b, 3, "FilterType", "BuiltIn")
	writeKeyBoolIndent(&b, 3, "AutoFilterEnabled", false)
	writeBookmarkArray(&b, "AllowListBookmarks", bookmarks)
	// Legacy key for older iOS; same entries.
	writeBookmarkArray(&b, "WhitelistedBookmarks", bookmarks)
	b.WriteString("\t\t</dict>\n")

	b.WriteString("\t</array>\n</dict>\n</plist>\n")
	out := []byte(b.String())
	if !strings.Contains(string(out), "allowListedAppBundleIDs") {
		return nil, fmt.Errorf("failed to build applicationaccess payload")
	}
	return out, nil
}

type bookmark struct {
	URL   string
	Title string
}

func websiteBookmarks(urls []string, portalURL string) []bookmark {
	out := make([]bookmark, 0, len(urls)+2)
	seen := map[string]struct{}{}
	add := func(raw, title string) {
		normalized := normalizeWebsiteURL(raw)
		if normalized == "" {
			return
		}
		key := strings.ToLower(normalized)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		if title == "" {
			title = raw
		}
		out = append(out, bookmark{URL: normalized, Title: title})
	}
	for _, u := range urls {
		add(u, u)
	}
	if p := strings.TrimSpace(portalURL); p != "" {
		add(p, "בקשת עזרה")
		// Store path when portalURL is the device portal root (…/d/{id}).
		if !strings.Contains(p, "/store") {
			add(strings.TrimRight(p, "/")+"/store", "חנות אפליקציות")
		}
		// Host root so other portal paths stay reachable under Safari allow-list matching.
		if u, err := url.Parse(p); err == nil && u.Host != "" {
			scheme := u.Scheme
			if scheme == "" {
				scheme = "https"
			}
			add(scheme+"://"+u.Host, "פורטל בית ספר")
		}
	}
	// Safari’s built-in allow-list blocks apps.apple.com unless listed; Chrome is not
	// filtered the same way. Allow the App Store *website* only — not the App Store app.
	add("https://apps.apple.com", "App Store")
	add("https://itunes.apple.com", "iTunes")
	// Nedarim Plus DebitIframe (+ captcha static assets). Do not allow www.google.com
	// for every device — search would become reachable under Safari's site allow-list.
	add("https://matara.pro", "נדרים פלוס")
	add("https://www.matara.pro", "נדרים פלוס")
	add("https://www.gstatic.com", "reCAPTCHA")
	// Empty allow-list bookmarks ⇒ iOS allows all sites. Keep one inert entry
	// so "specific websites only" stays active and everything else is blocked.
	if len(out) == 0 {
		add(blockAllSentinelURL, "Blocked")
	}
	return out
}

func normalizeWebsiteURL(u string) string {
	u = strings.TrimSpace(u)
	if u == "" {
		return ""
	}
	if !strings.Contains(u, "://") {
		return "https://" + u
	}
	return u
}

func writeBookmarkArray(b *strings.Builder, key string, bookmarks []bookmark) {
	tab := strings.Repeat("\t", 3)
	b.WriteString(tab)
	b.WriteString("<key>")
	b.WriteString(escape(key))
	b.WriteString("</key>\n")
	b.WriteString(tab)
	b.WriteString("<array>\n")
	for _, bm := range bookmarks {
		b.WriteString("\t\t\t\t<dict>\n")
		writeKeyStringIndent(b, 5, "URL", bm.URL)
		writeKeyStringIndent(b, 5, "Title", bm.Title)
		b.WriteString("\t\t\t\t</dict>\n")
	}
	b.WriteString(tab)
	b.WriteString("</array>\n")
}

// DevicePortalRoot builds the per-device portal base path /d/{enrollmentID}.
func DevicePortalRoot(portalBase, enrollmentID string) string {
	base := strings.TrimRight(portalBase, "/")
	id := strings.TrimSpace(enrollmentID)
	if id == "" {
		return base + "/"
	}
	return base + "/d/" + urlPathEscape(id)
}

// DevicePortalURL builds the help/request web-clip URL (/d/{id}?tab=request).
func DevicePortalURL(portalBase, enrollmentID string) string {
	base := DevicePortalRoot(portalBase, enrollmentID)
	if base == "" || strings.HasSuffix(base, "/") {
		return base
	}
	return base + "?tab=request"
}

// DeviceStoreURL builds the app-store web-clip URL (/d/{id}/store).
func DeviceStoreURL(portalBase, enrollmentID string) string {
	root := DevicePortalRoot(portalBase, enrollmentID)
	if root == "" || strings.HasSuffix(root, "/") {
		return root
	}
	return root + "/store"
}

func urlPathEscape(s string) string {
	return strings.ReplaceAll(url.PathEscape(s), "+", "%20")
}

// BuildRequestWebClipProfile installs a Home Screen Web Clip to the help/request portal.
func BuildRequestWebClipProfile(portalURL string) ([]byte, error) {
	return buildWebClipProfile(webClipSpec{
		displayName:       "בקשת עזרה",
		profileIdentifier: "com.schoolmdm.webclip.request",
		profileUUID:       webClipProfileUUID,
		payloadIdentifier: "com.schoolmdm.webclip.request.payload",
		payloadUUID:       webClipPayloadUUID,
		label:             "בקשת עזרה",
		url:               portalURL,
	})
}

// BuildStoreWebClipProfile installs a Home Screen Web Clip to the student app store.
func BuildStoreWebClipProfile(storeURL string) ([]byte, error) {
	return buildWebClipProfile(webClipSpec{
		displayName:       "חנות אפליקציות",
		profileIdentifier: "com.schoolmdm.webclip.store",
		profileUUID:       storeWebClipProfileUUID,
		payloadIdentifier: "com.schoolmdm.webclip.store.payload",
		payloadUUID:       storeWebClipPayloadUUID,
		label:             "חנות אפליקציות",
		url:               storeURL,
	})
}

// BuildLockScreenMessageProfile builds com.apple.shareddeviceconfiguration text
// shown on the supervised device lock screen (asset tag + footnote).
func BuildLockScreenMessageProfile(assetTag, footnote string) ([]byte, error) {
	assetTag = strings.TrimSpace(assetTag)
	footnote = strings.TrimSpace(footnote)
	if assetTag == "" && footnote == "" {
		return nil, fmt.Errorf("lock screen asset tag or footnote required")
	}
	var b strings.Builder
	b.WriteString(plistHeader)
	b.WriteString("<dict>\n")
	writeKeyString(&b, "PayloadDisplayName", "Lock Screen Message")
	writeKeyString(&b, "PayloadIdentifier", LockScreenPayloadIdentifier)
	writeKeyString(&b, "PayloadType", "Configuration")
	writeKeyString(&b, "PayloadUUID", lockScreenProfileUUID)
	writeKeyInt(&b, "PayloadVersion", 1)
	b.WriteString("\t<key>PayloadContent</key>\n\t<array>\n\t\t<dict>\n")
	writeKeyStringIndent(&b, 3, "PayloadType", "com.apple.shareddeviceconfiguration")
	writeKeyStringIndent(&b, 3, "PayloadIdentifier", "com.schoolmdm.lockscreen.payload")
	writeKeyStringIndent(&b, 3, "PayloadUUID", lockScreenPayloadUUID)
	writeKeyIntIndent(&b, 3, "PayloadVersion", 1)
	if assetTag != "" {
		writeKeyStringIndent(&b, 3, "AssetTagInformation", assetTag)
	}
	if footnote != "" {
		writeKeyStringIndent(&b, 3, "LockScreenFootnote", footnote)
	}
	b.WriteString("\t\t</dict>\n\t</array>\n</dict>\n</plist>\n")
	return []byte(b.String()), nil
}

// BuildCompanionNotificationsProfile enables Notification Center for the companion
// app on supervised devices (ADE). AlertType 1 = banners.
func BuildCompanionNotificationsProfile(bundleID string) ([]byte, error) {
	bundleID = strings.TrimSpace(bundleID)
	if bundleID == "" {
		bundleID = DefaultCompanionBundleID
	}
	var b strings.Builder
	b.WriteString(plistHeader)
	b.WriteString("<dict>\n")
	writeKeyString(&b, "PayloadDisplayName", "KFilter Notifications")
	writeKeyString(&b, "PayloadIdentifier", CompanionNotificationsIdentifier)
	writeKeyString(&b, "PayloadType", "Configuration")
	writeKeyString(&b, "PayloadUUID", companionNotifProfileUUID)
	writeKeyInt(&b, "PayloadVersion", 1)
	b.WriteString("\t<key>PayloadContent</key>\n\t<array>\n\t\t<dict>\n")
	writeKeyStringIndent(&b, 3, "PayloadType", "com.apple.notificationsettings")
	writeKeyStringIndent(&b, 3, "PayloadIdentifier", "com.schoolmdm.companion.notifications.payload")
	writeKeyStringIndent(&b, 3, "PayloadUUID", companionNotifPayloadUUID)
	writeKeyIntIndent(&b, 3, "PayloadVersion", 1)
	b.WriteString("\t\t\t<key>NotificationSettings</key>\n\t\t\t<array>\n\t\t\t\t<dict>\n")
	writeKeyStringIndent(&b, 5, "BundleIdentifier", bundleID)
	writeKeyBoolIndent(&b, 5, "NotificationsEnabled", true)
	writeKeyBoolIndent(&b, 5, "ShowInLockScreen", true)
	writeKeyBoolIndent(&b, 5, "ShowInNotificationCenter", true)
	writeKeyBoolIndent(&b, 5, "SoundsEnabled", true)
	writeKeyBoolIndent(&b, 5, "BadgesEnabled", true)
	writeKeyIntIndent(&b, 5, "AlertType", 1)
	writeKeyIntIndent(&b, 5, "GroupingType", 0)
	b.WriteString("\t\t\t\t</dict>\n\t\t\t</array>\n")
	b.WriteString("\t\t</dict>\n\t</array>\n</dict>\n</plist>\n")
	return []byte(b.String()), nil
}

type webClipSpec struct {
	displayName       string
	profileIdentifier string
	profileUUID       string
	payloadIdentifier string
	payloadUUID       string
	label             string
	url               string
}

func buildWebClipProfile(spec webClipSpec) ([]byte, error) {
	if strings.TrimSpace(spec.url) == "" {
		return nil, fmt.Errorf("webclip URL is required")
	}

	var b strings.Builder
	b.WriteString(plistHeader)
	b.WriteString("<dict>\n")
	writeKeyString(&b, "PayloadDisplayName", spec.displayName)
	writeKeyString(&b, "PayloadIdentifier", spec.profileIdentifier)
	writeKeyString(&b, "PayloadType", "Configuration")
	writeKeyString(&b, "PayloadUUID", spec.profileUUID)
	writeKeyInt(&b, "PayloadVersion", 1)
	b.WriteString("\t<key>PayloadContent</key>\n\t<array>\n\t\t<dict>\n")
	writeKeyStringIndent(&b, 3, "PayloadType", "com.apple.webClip.managed")
	writeKeyStringIndent(&b, 3, "PayloadIdentifier", spec.payloadIdentifier)
	writeKeyStringIndent(&b, 3, "PayloadUUID", spec.payloadUUID)
	writeKeyIntIndent(&b, 3, "PayloadVersion", 1)
	writeKeyStringIndent(&b, 3, "Label", spec.label)
	writeKeyStringIndent(&b, 3, "URL", spec.url)
	writeKeyBoolIndent(&b, 3, "IsRemovable", false)
	writeKeyBoolIndent(&b, 3, "FullScreen", true)
	b.WriteString("\t\t</dict>\n\t</array>\n</dict>\n</plist>\n")
	return []byte(b.String()), nil
}

const plistHeader = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
`

func writeKeyString(b *strings.Builder, key, value string) {
	writeKeyStringIndent(b, 1, key, value)
}

func writeKeyInt(b *strings.Builder, key string, value int) {
	writeKeyIntIndent(b, 1, key, value)
}

func writeKeyStringIndent(b *strings.Builder, indent int, key, value string) {
	tab := strings.Repeat("\t", indent)
	b.WriteString(tab)
	b.WriteString("<key>")
	b.WriteString(escape(key))
	b.WriteString("</key>\n")
	b.WriteString(tab)
	b.WriteString("<string>")
	b.WriteString(escape(value))
	b.WriteString("</string>\n")
}

func writeKeyIntIndent(b *strings.Builder, indent int, key string, value int) {
	tab := strings.Repeat("\t", indent)
	fmt.Fprintf(b, "%s<key>%s</key>\n%s<integer>%d</integer>\n", tab, escape(key), tab, value)
}

func writeKeyBoolIndent(b *strings.Builder, indent int, key string, value bool) {
	tab := strings.Repeat("\t", indent)
	v := "<false/>"
	if value {
		v = "<true/>"
	}
	fmt.Fprintf(b, "%s<key>%s</key>\n%s%s\n", tab, escape(key), tab, v)
}

func writeStringIndent(b *strings.Builder, indent int, value string) {
	tab := strings.Repeat("\t", indent)
	b.WriteString(tab)
	b.WriteString("<string>")
	b.WriteString(escape(value))
	b.WriteString("</string>\n")
}

func escape(s string) string {
	r := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&apos;",
	)
	return r.Replace(s)
}
