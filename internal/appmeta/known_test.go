package appmeta

import (
	"strings"
	"testing"
)

func TestKnownSystemApps(t *testing.T) {
	m, ok := Known("com.apple.mobilesafari")
	if !ok || m.Name != "Safari" {
		t.Fatalf("safari: ok=%v name=%q", ok, m.Name)
	}
	if name, ok := KnownName("COM.KFILTER.PORTAL"); !ok || name != "KFilter" {
		t.Fatalf("kfilter case: ok=%v name=%q", ok, name)
	}
	if _, ok := Known("com.whatsapp.WhatsApp"); ok {
		t.Fatal("store apps should not be in the known table")
	}
	found := false
	for _, m := range SearchKnown("הודעות") {
		if strings.EqualFold(m.BundleID, "com.apple.mobilesms") {
			found = true
		}
	}
	if !found {
		t.Fatal("search known by Hebrew name")
	}
	if !LooksLikeBundleID("com.apple.MobileSMS") {
		t.Fatal("bundle id shape")
	}
	manual := SearchKnown("com.example.customapp")
	if len(manual) == 0 || manual[0].BundleID != "com.example.customapp" {
		t.Fatalf("manual bundle: %#v", manual)
	}
}
