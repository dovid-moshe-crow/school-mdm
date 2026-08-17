package activity_test

import (
	"context"
	"testing"

	"github.com/dwdmsh/school-mdm/internal/activity"
	"github.com/dwdmsh/school-mdm/internal/store"
	"github.com/dwdmsh/school-mdm/internal/store/memory"
)

func TestLoggerInsertAndList(t *testing.T) {
	mem := memory.New()
	log := &activity.Logger{Store: mem}
	ctx := context.Background()
	log.Log(ctx, activity.Event{
		Category:     store.ActivityCategoryMDM,
		Action:       "lock",
		ActorType:    store.ActivityActorAdmin,
		Actor:        "admin…oken",
		EnrollmentID: "dev-1",
		Result:       store.ActivityResultOK,
		Summary:      "נעילת מכשיר",
		Detail:       map[string]any{"pin_set": true},
	})
	out, err := mem.ListActivityEvents(ctx, store.ActivityFilter{Category: store.ActivityCategoryMDM, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 {
		t.Fatalf("len=%d", len(out))
	}
	if out[0].Action != "lock" || out[0].EnrollmentID != "dev-1" {
		t.Fatalf("%+v", out[0])
	}
	if got := activity.AdminFingerprint("dev-admin-token"); got != "admin…oken" {
		t.Fatalf("fingerprint=%s", got)
	}
}
