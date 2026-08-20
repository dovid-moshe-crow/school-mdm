package appmeta

import "testing"

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
}
