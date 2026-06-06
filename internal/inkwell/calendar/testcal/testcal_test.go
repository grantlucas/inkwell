package testcal_test

import (
	"strings"
	"testing"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar/ical"
	"github.com/grantlucas/inkwell/internal/inkwell/calendar/testcal"
)

var refTime = time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC)

func parse(t *testing.T) []ical.Event {
	t.Helper()
	out := testcal.Generate(refTime)
	events, err := ical.Parse(strings.NewReader(out))
	if err != nil {
		t.Fatalf("ical.Parse failed: %v", err)
	}
	return events
}

func TestGenerate_ParseableICS(t *testing.T) {
	events := parse(t)
	if len(events) == 0 {
		t.Fatal("expected events, got none")
	}
}

func TestGenerate_IncludesAllDayEvents(t *testing.T) {
	events := parse(t)
	for _, e := range events {
		if e.AllDay {
			return
		}
	}
	t.Fatal("expected at least one all-day event")
}

func TestGenerate_IncludesEventsWithLocations(t *testing.T) {
	events := parse(t)
	for _, e := range events {
		if e.Location != "" {
			return
		}
	}
	t.Fatal("expected at least one event with a location")
}

func TestGenerate_IncludesEventsWithoutLocations(t *testing.T) {
	events := parse(t)
	for _, e := range events {
		if !e.AllDay && e.Location == "" {
			return
		}
	}
	t.Fatal("expected at least one timed event without a location")
}

func TestGenerate_IncludesOverlappingEvents(t *testing.T) {
	events := parse(t)
	for i := range events {
		for j := i + 1; j < len(events); j++ {
			a, b := events[i], events[j]
			if a.Start.Before(b.End) && b.Start.Before(a.End) && !a.AllDay && !b.AllDay {
				return
			}
		}
	}
	t.Fatal("expected at least one pair of overlapping timed events")
}

func TestGenerate_IncludesEarlyAndLateEvents(t *testing.T) {
	events := parse(t)
	var hasEarly, hasLate bool
	for _, e := range events {
		if e.AllDay {
			continue
		}
		h := e.Start.Hour()
		if h < 8 {
			hasEarly = true
		}
		if h >= 20 {
			hasLate = true
		}
	}
	if !hasEarly {
		t.Error("expected at least one event before 8:00")
	}
	if !hasLate {
		t.Error("expected at least one event at or after 20:00")
	}
}

func TestGenerate_IncludesShortAndLongEvents(t *testing.T) {
	events := parse(t)
	var hasShort, hasLong bool
	for _, e := range events {
		if e.AllDay {
			continue
		}
		dur := e.End.Sub(e.Start)
		if dur <= 30*time.Minute {
			hasShort = true
		}
		if dur >= 2*time.Hour {
			hasLong = true
		}
	}
	if !hasShort {
		t.Error("expected at least one short event (≤30min)")
	}
	if !hasLong {
		t.Error("expected at least one long event (≥2h)")
	}
}

func TestGenerate_DatesRelativeToNow(t *testing.T) {
	other := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)
	out := testcal.Generate(other)
	events, err := ical.Parse(strings.NewReader(out))
	if err != nil {
		t.Fatalf("ical.Parse failed: %v", err)
	}

	otherDay := time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)
	for _, e := range events {
		if e.Start.Before(otherDay) || e.Start.After(otherDay.AddDate(0, 0, 7)) {
			t.Errorf("event %q at %v is outside 7-day window starting %v", e.Summary, e.Start, otherDay)
		}
	}
}

func TestGenerate_SpansSevenDays(t *testing.T) {
	events := parse(t)

	today := time.Date(2026, 4, 29, 0, 0, 0, 0, time.UTC)
	covered := make(map[int]bool)
	for _, e := range events {
		day := int(e.Start.Sub(today).Hours() / 24)
		if day >= 0 && day < 7 {
			covered[day] = true
		}
	}
	for d := range 7 {
		if !covered[d] {
			t.Errorf("no events on day %d (%s)", d, today.AddDate(0, 0, d).Format("Mon Jan 2"))
		}
	}
}
