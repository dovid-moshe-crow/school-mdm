package httpapi

import (
	"testing"

	"github.com/micromdm/nanodep/godep"
)

func TestApplySchoolDEPProfileDefaultsSkipsSetupPanes(t *testing.T) {
	var p godep.ProfileJson
	applySchoolDEPProfileDefaults(&p)
	if len(p.SkipSetupItems) == 0 {
		t.Fatal("expected skip_setup_items")
	}
	seen := map[string]bool{}
	for _, item := range p.SkipSetupItems {
		seen[item] = true
		if item == "Welcome" {
			t.Fatal("Welcome should stay so the student sees the last screen")
		}
	}
	for _, want := range []string{"Restore", "DeviceToDeviceMigration", "AppleID", "TOS", "Privacy", "Passcode"} {
		if !seen[want] {
			t.Fatalf("missing skip key %s", want)
		}
	}
	if p.DoNotUseProfileFromBackup == nil || !*p.DoNotUseProfileFromBackup {
		t.Fatal("expected do_not_use_profile_from_backup")
	}
}

func TestApplySchoolDEPProfileDefaultsKeepsExplicitSkips(t *testing.T) {
	p := godep.ProfileJson{SkipSetupItems: []string{"AppleID"}}
	applySchoolDEPProfileDefaults(&p)
	if len(p.SkipSetupItems) != 1 || p.SkipSetupItems[0] != "AppleID" {
		t.Fatalf("explicit skip list was replaced: %#v", p.SkipSetupItems)
	}
}
