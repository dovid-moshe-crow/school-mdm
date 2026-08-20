package timers

import (
	"time"

	"github.com/dwdmsh/school-mdm/internal/store"
)

const (
	tzName     = "Asia/Jerusalem"
	onceRunKey = "once"
)

// Location is Asia/Jerusalem (weekly clocks). Falls back to UTC if tzdata is missing.
func Location() *time.Location {
	loc, err := time.LoadLocation(tzName)
	if err != nil {
		return time.UTC
	}
	return loc
}

func weekdaySet(days []int) map[time.Weekday]struct{} {
	out := make(map[time.Weekday]struct{}, len(days))
	for _, d := range days {
		if d < 0 || d > 6 {
			continue
		}
		out[time.Weekday(d)] = struct{}{}
	}
	return out
}

func dateKey(t time.Time, loc *time.Location) string {
	return t.In(loc).Format("2006-01-02")
}

func weeklySlot(day time.Time, loc *time.Location, hour, min int) time.Time {
	local := day.In(loc)
	return time.Date(local.Year(), local.Month(), local.Day(), hour, min, 0, 0, loc)
}

// Due reports whether t should fire at now, and the idempotency key to record.
func Due(t store.PolicyTimer, now time.Time, loc *time.Location) (key string, ok bool) {
	if !t.Enabled {
		return "", false
	}
	if loc == nil {
		loc = Location()
	}
	now = now.In(loc)
	switch t.Schedule {
	case store.TimerOnce:
		if t.RunAt == nil || t.RunAt.IsZero() {
			return "", false
		}
		if t.LastRunKey == onceRunKey {
			return "", false
		}
		if now.Before(t.RunAt.In(loc)) {
			return "", false
		}
		return onceRunKey, true
	case store.TimerWeekly:
		hour, min, err := store.ParseTimerTimeOfDay(t.TimeOfDay)
		if err != nil {
			return "", false
		}
		if _, allowed := weekdaySet(t.Weekdays)[now.Weekday()]; !allowed {
			return "", false
		}
		slot := weeklySlot(now, loc, hour, min)
		if now.Before(slot) {
			return "", false
		}
		key = dateKey(now, loc)
		if t.LastRunKey == key {
			return "", false
		}
		return key, true
	default:
		return "", false
	}
}

// NextRun is the next (or overdue) fire time. Nil when disabled or already completed.
func NextRun(t store.PolicyTimer, now time.Time, loc *time.Location) *time.Time {
	if !t.Enabled {
		return nil
	}
	if loc == nil {
		loc = Location()
	}
	now = now.In(loc)
	switch t.Schedule {
	case store.TimerOnce:
		if t.LastRunKey == onceRunKey || t.RunAt == nil || t.RunAt.IsZero() {
			return nil
		}
		at := t.RunAt.In(loc)
		return &at
	case store.TimerWeekly:
		hour, min, err := store.ParseTimerTimeOfDay(t.TimeOfDay)
		if err != nil {
			return nil
		}
		allowed := weekdaySet(t.Weekdays)
		if len(allowed) == 0 {
			return nil
		}
		todayKey := dateKey(now, loc)
		for offset := 0; offset <= 8; offset++ {
			day := now.AddDate(0, 0, offset)
			if _, ok := allowed[day.Weekday()]; !ok {
				continue
			}
			cand := weeklySlot(day, loc, hour, min)
			if offset == 0 {
				if !cand.After(now) {
					if t.LastRunKey != todayKey {
						return &cand
					}
					continue
				}
				return &cand
			}
			if cand.After(now) {
				return &cand
			}
		}
		return nil
	default:
		return nil
	}
}
