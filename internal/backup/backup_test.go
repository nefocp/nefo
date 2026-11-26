package backup

import (
	"testing"
	"time"

	"github.com/nefocp/nefo/internal/store"
)

func TestDue(t *testing.T) {
	loc := time.UTC
	at := func(day, hour, min int) time.Time { return time.Date(2026, 11, day, hour, min, 0, 0, loc) } // Nov 1 2026 is a Sunday
	daily := &store.Schedule{Frequency: "daily", Hour: 3, CreatedAt: at(1, 12, 0)}

	if Due(daily, at(2, 2, 59)) {
		t.Error("daily: due before its first planned time")
	}
	if !Due(daily, at(2, 3, 0)) {
		t.Error("daily: not due at 03:00 the next day")
	}
	daily.LastRunAt = at(2, 3, 1)
	if Due(daily, at(2, 23, 0)) {
		t.Error("daily: due twice on the same day")
	}
	if !Due(daily, at(5, 9, 0)) {
		t.Error("daily: a missed run (server was off) is not caught up")
	}

	// Weekly on Wednesday (3) at 04:00, created on Sunday Nov 1.
	weekly := &store.Schedule{Frequency: "weekly", Weekday: 3, Hour: 4, CreatedAt: at(1, 10, 0)}
	if Due(weekly, at(3, 23, 0)) {
		t.Error("weekly: due on Tuesday")
	}
	if !Due(weekly, at(4, 4, 30)) {
		t.Error("weekly: not due on Wednesday 04:30")
	}
	weekly.LastRunAt = at(4, 4, 31)
	if Due(weekly, at(10, 12, 0)) {
		t.Error("weekly: due again before the next Wednesday")
	}
	if !Due(weekly, at(11, 4, 0)) {
		t.Error("weekly: not due the following Wednesday")
	}
}

func TestCheckTarget(t *testing.T) {
	if checkTarget(KindSite, "../etc") == nil || checkTarget(KindDatabase, "a/b") == nil || checkTarget("x", "a") == nil {
		t.Fatal("bad target accepted")
	}
	if checkTarget(KindSite, "example.com") != nil || checkTarget(KindDatabase, "wp_shop") != nil {
		t.Fatal("good target refused")
	}
}
