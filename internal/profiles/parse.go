package profiles

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"github.com/micromdm/plist"
)

// MaxMobileconfigBytes is the upload size cap for custom configuration profiles.
const MaxMobileconfigBytes = 2 << 20

// Parsed is the top-level metadata extracted from a .mobileconfig.
type Parsed struct {
	Identifier  string
	UUID        string
	DisplayName string
	PayloadType string
}

var (
	xmlPlistStart = []byte("<?xml")
	xmlPlistEnd   = []byte("</plist>")
	keyStringRe   = regexp.MustCompile(`(?s)<key>(PayloadIdentifier|PayloadUUID|PayloadDisplayName|PayloadType)</key>\s*<string>([^<]*)</string>`)
)

// ParseMobileconfig reads an unsigned XML/binary plist or a signed profile and
// returns the top-level PayloadIdentifier (required for RemoveProfile).
func ParseMobileconfig(data []byte) (Parsed, error) {
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return Parsed{}, fmt.Errorf("profile is empty")
	}
	if len(data) > MaxMobileconfigBytes {
		return Parsed{}, fmt.Errorf("profile is larger than %d bytes", MaxMobileconfigBytes)
	}

	var p Parsed
	if top, ok := unmarshalProfileDict(data); ok {
		p = parsedFromMap(top)
	}
	if p.Identifier == "" {
		p = mergeParsed(p, parsedFromRaw(data))
	}
	if p.Identifier == "" {
		if inner := extractEmbeddedXML(data); len(inner) > 0 && !bytes.Equal(inner, data) {
			if top, ok := unmarshalProfileDict(inner); ok {
				p = mergeParsed(p, parsedFromMap(top))
			}
			if p.Identifier == "" {
				p = mergeParsed(p, parsedFromRaw(inner))
			}
		}
	}
	p.Identifier = strings.TrimSpace(p.Identifier)
	p.UUID = strings.TrimSpace(p.UUID)
	p.DisplayName = strings.TrimSpace(p.DisplayName)
	p.PayloadType = strings.TrimSpace(p.PayloadType)
	if p.Identifier == "" {
		return Parsed{}, fmt.Errorf("PayloadIdentifier is required")
	}
	if p.PayloadType != "" && !strings.EqualFold(p.PayloadType, "Configuration") {
		return Parsed{}, fmt.Errorf("PayloadType must be Configuration, got %s", p.PayloadType)
	}
	if IsManagedIdentifier(p.Identifier) {
		return Parsed{}, fmt.Errorf("identifier %s is reserved for school-mdm system profiles", p.Identifier)
	}
	return p, nil
}

// IsManagedIdentifier reports whether id belongs to a built-in school-mdm profile.
func IsManagedIdentifier(id string) bool {
	id = strings.ToLower(strings.TrimSpace(id))
	return strings.HasPrefix(id, "com.schoolmdm.")
}

func unmarshalProfileDict(data []byte) (map[string]any, bool) {
	var top map[string]any
	if err := plist.Unmarshal(data, &top); err != nil || top == nil {
		return nil, false
	}
	return top, true
}

func parsedFromMap(m map[string]any) Parsed {
	return Parsed{
		Identifier:  mapString(m, "PayloadIdentifier"),
		UUID:        mapString(m, "PayloadUUID"),
		DisplayName: mapString(m, "PayloadDisplayName"),
		PayloadType: mapString(m, "PayloadType"),
	}
}

func mapString(m map[string]any, key string) string {
	v, _ := m[key].(string)
	return strings.TrimSpace(v)
}

func parsedFromRaw(data []byte) Parsed {
	var p Parsed
	for _, m := range keyStringRe.FindAllSubmatch(data, -1) {
		if len(m) < 3 {
			continue
		}
		key := string(m[1])
		val := strings.TrimSpace(string(m[2]))
		if val == "" {
			continue
		}
		switch key {
		case "PayloadIdentifier":
			if p.Identifier == "" {
				p.Identifier = val
			}
		case "PayloadUUID":
			if p.UUID == "" {
				p.UUID = val
			}
		case "PayloadDisplayName":
			if p.DisplayName == "" {
				p.DisplayName = val
			}
		case "PayloadType":
			if p.PayloadType == "" {
				p.PayloadType = val
			}
		}
	}
	return p
}

func extractEmbeddedXML(data []byte) []byte {
	i := bytes.Index(data, xmlPlistStart)
	if i < 0 {
		return nil
	}
	j := bytes.Index(data[i:], xmlPlistEnd)
	if j < 0 {
		return nil
	}
	return bytes.TrimSpace(data[i : i+j+len(xmlPlistEnd)])
}

func mergeParsed(base, extra Parsed) Parsed {
	if base.Identifier == "" {
		base.Identifier = extra.Identifier
	}
	if base.UUID == "" {
		base.UUID = extra.UUID
	}
	if base.DisplayName == "" {
		base.DisplayName = extra.DisplayName
	}
	if base.PayloadType == "" {
		base.PayloadType = extra.PayloadType
	}
	return base
}
