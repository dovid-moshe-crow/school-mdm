package profiles

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/google/uuid"
)

// BuildAllowlistProfile creates a configuration profile restricting apps and
// websites to the provided allowlists (not Single App Mode / kiosk).
func BuildAllowlistProfile(displayName string, apps, urls []string) ([]byte, error) {
	if displayName == "" {
		displayName = "School Allowlists"
	}
	appPayloadUUID := uuid.NewString()
	webPayloadUUID := uuid.NewString()
	profileUUID := uuid.NewString()

	var b strings.Builder
	b.WriteString(plistHeader)
	b.WriteString("<dict>\n")
	writeKeyString(&b, "PayloadDisplayName", displayName)
	writeKeyString(&b, "PayloadIdentifier", "com.schoolmdm.allowlists")
	writeKeyString(&b, "PayloadType", "Configuration")
	writeKeyString(&b, "PayloadUUID", profileUUID)
	writeKeyInt(&b, "PayloadVersion", 1)
	b.WriteString("\t<key>PayloadContent</key>\n\t<array>\n")

	// Application access allowlist
	b.WriteString("\t\t<dict>\n")
	writeKeyStringIndent(&b, 3, "PayloadType", "com.apple.applicationaccess")
	writeKeyStringIndent(&b, 3, "PayloadIdentifier", "com.schoolmdm.allowlists.apps")
	writeKeyStringIndent(&b, 3, "PayloadUUID", appPayloadUUID)
	writeKeyIntIndent(&b, 3, "PayloadVersion", 1)
	b.WriteString("\t\t\t<key>allowListedAppBundleIDs</key>\n\t\t\t<array>\n")
	for _, app := range apps {
		writeStringIndent(&b, 4, app)
	}
	b.WriteString("\t\t\t</array>\n\t\t</dict>\n")

	// Built-in web content filter: specific websites only style allowlist
	b.WriteString("\t\t<dict>\n")
	writeKeyStringIndent(&b, 3, "PayloadType", "com.apple.webcontent-filter")
	writeKeyStringIndent(&b, 3, "PayloadIdentifier", "com.schoolmdm.allowlists.web")
	writeKeyStringIndent(&b, 3, "PayloadUUID", webPayloadUUID)
	writeKeyIntIndent(&b, 3, "PayloadVersion", 1)
	writeKeyStringIndent(&b, 3, "FilterType", "BuiltIn")
	writeKeyBoolIndent(&b, 3, "AutoFilterEnabled", false)
	b.WriteString("\t\t\t<key>PermittedURLs</key>\n\t\t\t<array>\n")
	for _, u := range urls {
		normalized := u
		if !strings.Contains(normalized, "://") {
			normalized = "https://" + normalized
		}
		writeStringIndent(&b, 4, normalized)
	}
	b.WriteString("\t\t\t</array>\n")
	b.WriteString("\t\t\t<key>WhitelistedBookmarks</key>\n\t\t\t<array>\n")
	for _, u := range urls {
		normalized := u
		if !strings.Contains(normalized, "://") {
			normalized = "https://" + normalized
		}
		b.WriteString("\t\t\t\t<dict>\n")
		writeKeyStringIndent(&b, 5, "URL", normalized)
		writeKeyStringIndent(&b, 5, "Title", u)
		b.WriteString("\t\t\t\t</dict>\n")
	}
	b.WriteString("\t\t\t</array>\n\t\t</dict>\n")

	b.WriteString("\t</array>\n</dict>\n</plist>\n")
	out := []byte(b.String())
	if !strings.Contains(string(out), "allowListedAppBundleIDs") {
		return nil, fmt.Errorf("failed to build applicationaccess payload")
	}
	return out, nil
}

// DevicePortalURL builds the per-device portal path /d/{enrollmentID}.
func DevicePortalURL(portalBase, enrollmentID string) string {
	base := strings.TrimRight(portalBase, "/")
	id := strings.TrimSpace(enrollmentID)
	if id == "" {
		return base + "/"
	}
	return base + "/d/" + urlPathEscape(id)
}

func urlPathEscape(s string) string {
	return strings.ReplaceAll(url.PathEscape(s), "+", "%20")
}

// BuildRequestWebClipProfile installs a Home Screen Web Clip to the portal.
// portalURL should already include the device path when known (see DevicePortalURL).
func BuildRequestWebClipProfile(portalURL string) ([]byte, error) {
	if portalURL == "" {
		return nil, fmt.Errorf("portalURL is required")
	}
	payloadUUID := uuid.NewString()
	profileUUID := uuid.NewString()

	var b strings.Builder
	b.WriteString(plistHeader)
	b.WriteString("<dict>\n")
	writeKeyString(&b, "PayloadDisplayName", "Request Access")
	writeKeyString(&b, "PayloadIdentifier", "com.schoolmdm.webclip.request")
	writeKeyString(&b, "PayloadType", "Configuration")
	writeKeyString(&b, "PayloadUUID", profileUUID)
	writeKeyInt(&b, "PayloadVersion", 1)
	b.WriteString("\t<key>PayloadContent</key>\n\t<array>\n\t\t<dict>\n")
	writeKeyStringIndent(&b, 3, "PayloadType", "com.apple.webClip.managed")
	writeKeyStringIndent(&b, 3, "PayloadIdentifier", "com.schoolmdm.webclip.request.payload")
	writeKeyStringIndent(&b, 3, "PayloadUUID", payloadUUID)
	writeKeyIntIndent(&b, 3, "PayloadVersion", 1)
	writeKeyStringIndent(&b, 3, "Label", "Request Access")
	writeKeyStringIndent(&b, 3, "URL", portalURL)
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
