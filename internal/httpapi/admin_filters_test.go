package httpapi

import (
	"testing"

	"github.com/dwdmsh/school-mdm/internal/store"
)

func TestMatchRequestStatus(t *testing.T) {
	cases := []struct {
		got  store.RequestStatus
		want string
		ok   bool
	}{
		{store.StatusPending, "all", true},
		{store.StatusPending, "pending", true},
		{store.StatusPending, "open", true},
		{store.StatusApproved, "open", false},
		{store.StatusApproved, "closed", true},
		{store.StatusDenied, "closed", true},
		{store.StatusResolved, "closed", true},
		{store.StatusPending, "closed", false},
	}
	for _, tc := range cases {
		if got := matchRequestStatus(tc.got, tc.want); got != tc.ok {
			t.Fatalf("%s/%s: got %v want %v", tc.got, tc.want, got, tc.ok)
		}
	}
}
