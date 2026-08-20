package profiles

import (
	"strings"
	"testing"
)

func sampleProfile(identifier, name string) []byte {
	return []byte(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>PayloadDisplayName</key>
	<string>` + name + `</string>
	<key>PayloadIdentifier</key>
	<string>` + identifier + `</string>
	<key>PayloadType</key>
	<string>Configuration</string>
	<key>PayloadUUID</key>
	<string>11111111-1111-4111-8111-111111111111</string>
	<key>PayloadVersion</key>
	<integer>1</integer>
	<key>PayloadContent</key>
	<array/>
</dict>
</plist>
`)
}

func TestParseMobileconfig(t *testing.T) {
	p, err := ParseMobileconfig(sampleProfile("com.example.wifi", "School Wi-Fi"))
	if err != nil {
		t.Fatal(err)
	}
	if p.Identifier != "com.example.wifi" {
		t.Fatalf("identifier %q", p.Identifier)
	}
	if p.DisplayName != "School Wi-Fi" {
		t.Fatalf("name %q", p.DisplayName)
	}
	if p.UUID != "11111111-1111-4111-8111-111111111111" {
		t.Fatalf("uuid %q", p.UUID)
	}
	if !strings.EqualFold(p.PayloadType, "Configuration") {
		t.Fatalf("type %q", p.PayloadType)
	}
}

func TestParseMobileconfigRejectsReserved(t *testing.T) {
	_, err := ParseMobileconfig(sampleProfile(AllowlistPayloadIdentifier, "Allowlists"))
	if err == nil {
		t.Fatal("expected reserved identifier error")
	}
}

func TestParseMobileconfigEmpty(t *testing.T) {
	if _, err := ParseMobileconfig(nil); err == nil {
		t.Fatal("expected empty error")
	}
}

func TestIsManagedIdentifier(t *testing.T) {
	if !IsManagedIdentifier("com.schoolmdm.allowlists") {
		t.Fatal("allowlists should be managed")
	}
	if IsManagedIdentifier("com.example.wifi") {
		t.Fatal("example should not be managed")
	}
}
