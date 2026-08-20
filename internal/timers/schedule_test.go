package timers

import (
	"testing"
	"time"

	"github.com/dwdmsh/school-mdm/internal/store"
)

func TestDueOnceAndWeekly(t *testing.T) {
	loc := Location()
	if loc.String() != tzName {
		t.Fatalf("expected %s, got %s", tzName, loc)
	}

	// Sunday 16 Aug 2026 08:00 Israel.
	sunMorning := time.Date(2026, 8, 16, 8, 0, 0, 0, loc)
	runAt := time.Date(2026, 8, 16, 7, 0, 0, 0, loc)

	once := store.PolicyTimer{
		Enabled:  true,
		Schedule: store.TimerOnce,
		RunAt:    &runAt,
	}
	key, ok := Due(once, sunMorning, loc)
	if !ok || key != onceRunKey {
		t.Fatalf("once due: ok=%v key=%q", ok, key)
	}
	once.LastRunKey = onceRunKey
	if _, ok := Due(once, sunMorning, loc); ok {
		t.Fatal("once should not re-fire")
	}

	future := runAt.Add(2 * time.Hour)
	once.LastRunKey = ""
	once.RunAt = &future
	if _, ok := Due(once, sunMorning, loc); ok {
		t.Fatal("future once should not fire")
	}

	weekly := store.PolicyTimer{
		Enabled:   true,
		Schedule:  store.TimerWeekly,
		Weekdays:  []int{0}, // Sunday
		TimeOfDay: "08:00",
	}
	key, ok = Due(weekly, sunMorning, loc)
	if !ok || key != "2026-08-16" {
		t.Fatalf("weekly due: ok=%v key=%q", ok, key)
	}
	weekly.LastRunKey = key
	if _, ok := Due(weekly, sunMorning.Add(time.Hour), loc); ok {
		t.Fatal("weekly should not re-fire same day")
	}

	before := time.Date(2026, 8, 16, 7, 59, 0, 0, loc)
	weekly.LastRunKey = ""
	if _, ok := Due(weekly, before, loc); ok {
		t.Fatal("weekly before slot should not fire")
	}

	monday := time.Date(2026, 8, 17, 9, 0, 0, 0, loc)
	if _, ok := Due(weekly, monday, loc); ok {
		t.Fatal("weekly should not fire on Monday")
	}
}

func TestNextRunWeeklyOverdueThenTomorrow(t *testing.T) {
	loc := Location()
	weekly := store.PolicyTimer{
		Enabled:   true,
		Schedule:  store.TimerWeekly,
		Weekdays:  []int{0, 1}, // Sun, Mon
		TimeOfDay: "08:00",
	}
	sunLate := time.Date(2026, 8, 16, 9, 0, 0, 0, loc)
	next := NextRun(weekly, sunLate, loc)
	if next == nil || !next.Equal(time.Date(2026, 8, 16, 8, 0, 0, 0, loc)) {
		t.Fatalf("overdue Sunday slot, got %v", next)
	}
	weekly.LastRunKey = "2026-08-16"
	next = NextRun(weekly, sunLate, loc)
	want := time.Date(2026, 8, 17, 8, 0, 0, 0, loc)
	if next == nil || !next.Equal(want) {
		t.Fatalf("next Monday, got %v want %v", next, want)
	}

	disabled := weekly
	disabled.Enabled = false
	if NextRun(disabled, sunLate, loc) != nil {
		t.Fatal("disabled timer has no next run")
	}
}
