package timers

import (
	"context"
	"testing"
	"time"

	"github.com/dwdmsh/school-mdm/internal/policy"
	"github.com/dwdmsh/school-mdm/internal/store"
	"github.com/dwdmsh/school-mdm/internal/store/memory"
)

type stubPush struct {
	n int
}

func (s *stubPush) ReconcileMany(context.Context, []string) error {
	s.n++
	return nil
}

func TestApplyAddThenRemove(t *testing.T) {
	ctx := context.Background()
	mem := memory.New()
	pack, err := mem.CreateWhitelistPack(ctx, store.WhitelistPack{Name: "games"})
	if err != nil {
		t.Fatal(err)
	}
	if err := mem.EnsureDevice(ctx, "dev-1"); err != nil {
		t.Fatal(err)
	}
	g, err := mem.CreateGroup(ctx, store.Group{Name: "class-a"})
	if err != nil {
		t.Fatal(err)
	}
	if err := mem.AddGroupMember(ctx, g.ID, "dev-1"); err != nil {
		t.Fatal(err)
	}

	push := &stubPush{}
	svc := &Service{Store: mem, Push: push, Loc: Location()}
	timer := store.PolicyTimer{
		Name:      "morning games",
		Action:    store.TimerActionAdd,
		PackIDs:   []string{pack.ID},
		DeviceIDs: []string{"dev-1"},
		GroupIDs:  []string{g.ID},
		Schedule:  store.TimerWeekly,
		Weekdays:  []int{0},
		TimeOfDay: "08:00",
		Enabled:   true,
	}
	created, err := svc.Create(ctx, timer)
	if err != nil {
		t.Fatal(err)
	}

	loc := Location()
	now := time.Date(2026, 8, 16, 8, 1, 0, 0, loc)
	fired, errs, err := svc.RunDue(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	if fired != 1 || errs != 0 {
		t.Fatalf("fired=%d errs=%d", fired, errs)
	}
	assigns, err := mem.ListWhitelistPackAssignments(ctx, pack.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(assigns) != 2 {
		t.Fatalf("want 2 assignments, got %#v", assigns)
	}
	if push.n == 0 {
		t.Fatal("expected reconcile")
	}

	got, err := svc.Get(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.LastRunKey != "2026-08-16" {
		t.Fatalf("last_run_key %q", got.LastRunKey)
	}

	fired, _, err = svc.RunDue(ctx, now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if fired != 0 {
		t.Fatalf("should be idempotent, fired=%d", fired)
	}

	got.Action = store.TimerActionRemove
	if _, err := svc.Update(ctx, got.PolicyTimer, true); err != nil {
		t.Fatal(err)
	}
	res, _, err := svc.RunNow(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if res.Assignments != 2 {
		t.Fatalf("remove assignments=%d", res.Assignments)
	}
	assigns, _ = mem.ListWhitelistPackAssignments(ctx, pack.ID)
	if len(assigns) != 0 {
		t.Fatalf("expected unassigned, got %#v", assigns)
	}

	// Group assignment used group target, not exploded devices.
	for _, a := range assigns {
		if a.TargetType == policy.TargetDevice && a.TargetID == "dev-1" && a.PackID == pack.ID {
			t.Fatal("unexpected leftover device assignment")
		}
	}
}

func TestOnceDisablesAfterFire(t *testing.T) {
	ctx := context.Background()
	mem := memory.New()
	pack, err := mem.CreateWhitelistPack(ctx, store.WhitelistPack{Name: "web"})
	if err != nil {
		t.Fatal(err)
	}
	runAt := time.Date(2026, 8, 16, 8, 0, 0, 0, Location())
	svc := &Service{Store: mem, Loc: Location()}
	created, err := svc.Create(ctx, store.PolicyTimer{
		Name:      "once",
		Action:    store.TimerActionAdd,
		PackIDs:   []string{pack.ID},
		DeviceIDs: []string{"dev-9"},
		Schedule:  store.TimerOnce,
		RunAt:     &runAt,
		Enabled:   true,
	})
	if err != nil {
		t.Fatal(err)
	}
	fired, _, err := svc.RunDue(ctx, runAt.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if fired != 1 {
		t.Fatalf("fired=%d", fired)
	}
	got, err := svc.Get(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Enabled {
		t.Fatal("once timer should disable after fire")
	}
	if got.LastRunKey != onceRunKey {
		t.Fatalf("last_run_key %q", got.LastRunKey)
	}
}
