package abm

import (
	"testing"

	"github.com/micromdm/nanodep/godep"
)

func TestSerialsNeedingProfile(t *testing.T) {
	uuid := "school-profile"
	other := "other-profile"
	assigned := godep.DeviceJsonProfileStatusAssigned
	pushed := godep.DeviceJsonProfileStatusPushed
	empty := godep.DeviceJsonProfileStatusEmpty
	removed := godep.DeviceJsonProfileStatusRemoved
	devices := []godep.DeviceJson{
		{SerialNumber: "HAS-ASSIGNED", ProfileUuid: &uuid, ProfileStatus: &assigned},
		{SerialNumber: "HAS-PUSHED", ProfileUuid: &uuid, ProfileStatus: &pushed},
		{SerialNumber: "EMPTY", ProfileStatus: &empty},
		{SerialNumber: "REMOVED", ProfileUuid: &uuid, ProfileStatus: &removed},
		{SerialNumber: "OTHER", ProfileUuid: &other, ProfileStatus: &assigned},
		{SerialNumber: "NO-STATUS", ProfileUuid: &uuid},
		{SerialNumber: ""},
	}
	got := SerialsNeedingProfile(devices, uuid)
	want := map[string]bool{"EMPTY": true, "REMOVED": true, "OTHER": true, "NO-STATUS": true}
	if len(got) != len(want) {
		t.Fatalf("got %#v", got)
	}
	for _, s := range got {
		if !want[s] {
			t.Fatalf("unexpected serial %s in %#v", s, got)
		}
	}
	if SerialsNeedingProfile(devices, "") != nil {
		t.Fatal("empty profile uuid should skip")
	}
}
