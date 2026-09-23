package daygrid

import (
	"testing"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar/ical"
	"github.com/grantlucas/inkwell/internal/inkwell/weather"
)

func day(y int, m time.Month, d int, loc *time.Location) Day {
	return Day{Start: time.Date(y, m, d, 0, 0, 0, 0, loc)}
}

// Days are built from the clock's own zone, because the dashboard hands
// every widget a time already in the display zone. Re-resolving one
// here would let a widget disagree with the rest of the panel about
// which day it is.
func TestDays(t *testing.T) {
	loc := time.FixedZone("UTC-5", -5*60*60)
	now := time.Date(2026, 4, 28, 14, 30, 0, 0, loc)

	days := Days(now, 5)
	if len(days) != 5 {
		t.Fatalf("got %d days, want 5", len(days))
	}
	if !days[0].IsToday {
		t.Error("the first day is not marked today")
	}
	for i, d := range days[1:] {
		if d.IsToday {
			t.Errorf("day %d is also marked today", i+1)
		}
	}

	// Each day runs local midnight to local midnight, in the clock's
	// zone rather than UTC.
	for i, d := range days {
		if d.Start.Location() != loc {
			t.Errorf("day %d start is in %v, want %v", i, d.Start.Location(), loc)
		}
		if h, m, sec := d.Start.Clock(); h != 0 || m != 0 || sec != 0 {
			t.Errorf("day %d starts at %02d:%02d:%02d, want local midnight", i, h, m, sec)
		}
		if !d.End().Equal(d.Start.AddDate(0, 0, 1)) {
			t.Errorf("day %d does not end one day after it starts", i)
		}
	}
	if got := days[0].Start.Day(); got != 28 {
		t.Errorf("first day = %d, want 28", got)
	}
	if got := days[4].Start.Day(); got != 2 {
		t.Errorf("last day = %d, want 2 (May)", got)
	}
}

func TestDays_NonePlanned(t *testing.T) {
	for _, n := range []int{0, -1} {
		if got := Days(time.Now(), n); len(got) != 0 {
			t.Errorf("Days(n=%d) returned %d days, want none", n, len(got))
		}
	}
}

// Window is what a calendar or forecast fetch asks for, so it has to
// span the whole grid rather than the first day.
func TestWindow(t *testing.T) {
	now := time.Date(2026, 4, 28, 14, 30, 0, 0, time.UTC)
	start, end := Window(Days(now, 7))

	if want := time.Date(2026, 4, 28, 0, 0, 0, 0, time.UTC); !start.Equal(want) {
		t.Errorf("start = %v, want %v", start, want)
	}
	if got := end.Sub(start); got != 7*24*time.Hour {
		t.Errorf("window = %v, want 168h", got)
	}
}

func TestWindow_NoDays(t *testing.T) {
	start, end := Window(nil)
	if !start.IsZero() || !end.IsZero() {
		t.Errorf("Window(nil) = %v, %v; want zero times", start, end)
	}
}

func TestFilterEventsForDay(t *testing.T) {
	events := []ical.Event{
		{
			UID: "before", Summary: "Yesterday",
			Start: time.Date(2026, 4, 27, 9, 0, 0, 0, time.UTC),
			End:   time.Date(2026, 4, 27, 10, 0, 0, 0, time.UTC),
		},
		{
			UID: "during", Summary: "Today event",
			Start: time.Date(2026, 4, 28, 14, 0, 0, 0, time.UTC),
			End:   time.Date(2026, 4, 28, 15, 0, 0, 0, time.UTC),
		},
		{
			UID: "allday", Summary: "All Day", AllDay: true,
			Start: time.Date(2026, 4, 28, 0, 0, 0, 0, time.UTC),
			End:   time.Date(2026, 4, 29, 0, 0, 0, 0, time.UTC),
		},
		{
			UID: "after", Summary: "Tomorrow",
			Start: time.Date(2026, 4, 29, 9, 0, 0, 0, time.UTC),
			End:   time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC),
		},
	}

	filtered := FilterEventsForDay(events, day(2026, 4, 28, time.UTC))
	if len(filtered) != 2 {
		t.Fatalf("got %d events, want 2", len(filtered))
	}
	if !filtered[0].AllDay {
		t.Error("all-day event should sort first")
	}
	if filtered[1].Summary != "Today event" {
		t.Errorf("second event = %q, want 'Today event'", filtered[1].Summary)
	}
}

// All-day events are calendar date labels, not instants. A multi-day
// all-day event (a trip starting Thursday) must land in exactly the
// columns for the dates it spans, even when the viewer's columns are
// built in a negative-UTC zone. Parsed all-day dates are anchored to
// UTC midnight, so an instant-overlap comparison against local-zone
// columns leaks the event into the previous local day. This reproduces
// inkwell-9f0: the trip showed up a day early in America/Toronto.
//
// This is the case issue #92 named as the reason the bucketing must not
// be copied per widget — the failure is silent and off by one day.
func TestFilterEventsForDay_AllDayMultiDayNegativeTimezone(t *testing.T) {
	// DTSTART;VALUE=DATE:20260625 / DTEND;VALUE=DATE:20260628 (exclusive)
	// parses to UTC midnight, matching ical.parseDateTime.
	trip := ical.Event{
		UID: "trip", Summary: "Winnipeg", AllDay: true,
		Start: time.Date(2026, 6, 25, 0, 0, 0, 0, time.UTC),
		End:   time.Date(2026, 6, 28, 0, 0, 0, 0, time.UTC),
	}
	loc := time.FixedZone("UTC-5", -5*60*60)

	cases := []struct {
		label string
		date  int
		want  bool
	}{
		{"day before start", 24, false},
		{"first day (Thursday)", 25, true},
		{"middle day", 26, true},
		{"last spanned day", 27, true},
		{"exclusive end day", 28, false},
	}
	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			got := len(FilterEventsForDay([]ical.Event{trip}, day(2026, 6, tc.date, loc))) == 1
			if got != tc.want {
				t.Errorf("trip present on %s = %v, want %v", tc.label, got, tc.want)
			}
		})
	}
}

func TestFilterEventsForDay_SortByStart(t *testing.T) {
	events := []ical.Event{
		{
			UID: "late", Summary: "Late",
			Start: time.Date(2026, 4, 28, 15, 0, 0, 0, time.UTC),
			End:   time.Date(2026, 4, 28, 16, 0, 0, 0, time.UTC),
		},
		{
			UID: "early", Summary: "Early",
			Start: time.Date(2026, 4, 28, 9, 0, 0, 0, time.UTC),
			End:   time.Date(2026, 4, 28, 10, 0, 0, 0, time.UTC),
		},
	}
	filtered := FilterEventsForDay(events, day(2026, 4, 28, time.UTC))
	if len(filtered) != 2 {
		t.Fatalf("got %d events, want 2", len(filtered))
	}
	if filtered[0].Summary != "Early" {
		t.Errorf("first event = %q, want 'Early'", filtered[0].Summary)
	}
}

func TestFilterEventsForDay_Empty(t *testing.T) {
	if got := FilterEventsForDay(nil, day(2026, 4, 28, time.UTC)); len(got) != 0 {
		t.Errorf("got %d events, want 0", len(got))
	}
}

func TestFindForecast(t *testing.T) {
	days := []weather.DailyForecast{
		{Date: time.Date(2026, 4, 27, 0, 0, 0, 0, time.UTC), High: 12},
		{Date: time.Date(2026, 4, 28, 0, 0, 0, 0, time.UTC), High: 14},
	}

	tests := []struct {
		label    string
		days     []weather.DailyForecast
		day      Day
		wantHigh float64
	}{
		{"found", days, day(2026, 4, 28, time.UTC), 14},
		{"not found", days, day(2026, 5, 10, time.UTC), 0},
		{"no forecast at all", nil, day(2026, 4, 27, time.UTC), 0},
		// Matching is by calendar date, not instant: a column built in
		// a negative-UTC zone starts hours after the forecast's UTC
		// midnight, so comparing instants would miss every day.
		{"local-zone column still matches", days, day(2026, 4, 28, time.FixedZone("UTC-5", -5*60*60)), 14},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			got := FindForecast(tt.days, tt.day)
			if got.High != tt.wantHigh {
				t.Errorf("High = %v, want %v", got.High, tt.wantHigh)
			}
		})
	}
}
