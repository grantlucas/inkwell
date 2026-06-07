package ical

import (
	"strings"
	"testing"
	"time"
)

// TestParseRRULE covers the RRULE forms the issue scope calls out:
// FREQ=DAILY/WEEKLY/MONTHLY with INTERVAL, BYDAY, COUNT and UNTIL.
// One row per non-trivial combination so a regression in any field's
// parsing fails at a specific row rather than a vague "wrong rule."
func TestParseRRULE(t *testing.T) {
	cases := []struct {
		label string
		input string
		want  Recurrence
	}{
		{
			label: "FREQ=DAILY only",
			input: "FREQ=DAILY",
			want:  Recurrence{Freq: FreqDaily},
		},
		{
			label: "FREQ=WEEKLY with INTERVAL and BYDAY",
			input: "FREQ=WEEKLY;INTERVAL=2;BYDAY=MO,WE,FR",
			want: Recurrence{
				Freq:     FreqWeekly,
				Interval: 2,
				ByDay:    []time.Weekday{time.Monday, time.Wednesday, time.Friday},
			},
		},
		{
			label: "FREQ=MONTHLY with COUNT",
			input: "FREQ=MONTHLY;COUNT=6",
			want:  Recurrence{Freq: FreqMonthly, Count: 6},
		},
		{
			label: "FREQ=WEEKLY with UNTIL (date-only form)",
			input: "FREQ=WEEKLY;UNTIL=20260601",
			want: Recurrence{
				Freq:  FreqWeekly,
				Until: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
			},
		},
		{
			label: "FREQ=DAILY with UNTIL (utc datetime form)",
			input: "FREQ=DAILY;UNTIL=20260601T235959Z",
			want: Recurrence{
				Freq:  FreqDaily,
				Until: time.Date(2026, 6, 1, 23, 59, 59, 0, time.UTC),
			},
		},
		{
			// Positional BYDAY ("2MO" = second Monday of month) is
			// out of scope semantically — but the parser still strips
			// the numeric prefix and resolves the weekday. That keeps
			// feeds with positional BYDAY from erroring out; they just
			// fall back to plain weekday filtering.
			label: "FREQ=MONTHLY with positional BYDAY (prefix stripped)",
			input: "FREQ=MONTHLY;BYDAY=2MO,-1FR",
			want: Recurrence{
				Freq:  FreqMonthly,
				ByDay: []time.Weekday{time.Monday, time.Friday},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			got, err := parseRRULE(tc.input)
			if err != nil {
				t.Fatalf("parseRRULE: %v", err)
			}
			if got.Freq != tc.want.Freq {
				t.Errorf("Freq = %v, want %v", got.Freq, tc.want.Freq)
			}
			if got.Interval != tc.want.Interval {
				t.Errorf("Interval = %d, want %d", got.Interval, tc.want.Interval)
			}
			if got.Count != tc.want.Count {
				t.Errorf("Count = %d, want %d", got.Count, tc.want.Count)
			}
			if !got.Until.Equal(tc.want.Until) {
				t.Errorf("Until = %v, want %v", got.Until, tc.want.Until)
			}
			if len(got.ByDay) != len(tc.want.ByDay) {
				t.Fatalf("ByDay len = %d, want %d (%v vs %v)", len(got.ByDay), len(tc.want.ByDay), got.ByDay, tc.want.ByDay)
			}
			for i, wd := range tc.want.ByDay {
				if got.ByDay[i] != wd {
					t.Errorf("ByDay[%d] = %v, want %v", i, got.ByDay[i], wd)
				}
			}
		})
	}
}

func TestParseRRULE_Errors(t *testing.T) {
	cases := []struct {
		label string
		input string
	}{
		{"empty", ""},
		{"missing FREQ", "INTERVAL=2"},
		{"unknown FREQ", "FREQ=YEARLY"},
		{"invalid INTERVAL", "FREQ=DAILY;INTERVAL=abc"},
		{"invalid COUNT", "FREQ=DAILY;COUNT=-1"},
		{"invalid UNTIL", "FREQ=DAILY;UNTIL=not-a-date"},
		{"invalid BYDAY", "FREQ=WEEKLY;BYDAY=XX"},
		{"malformed part (no =)", "FREQ=DAILY;BROKEN"},
	}
	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			_, err := parseRRULE(tc.input)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

// TestParse_CapturesRRULEAndEXDATE confirms the parse loop hands the
// RRULE off to the rule parser and pulls EXDATEs into the event's
// exclusion list. Expansion happens later (TestOccurrences*); this
// just locks the iCal → struct mapping.
func TestParse_CapturesRRULEAndEXDATE(t *testing.T) {
	input := `BEGIN:VCALENDAR
BEGIN:VEVENT
UID:weekly@example.com
DTSTART:20260427T090000Z
DTEND:20260427T100000Z
SUMMARY:Weekly Sync
RRULE:FREQ=WEEKLY;BYDAY=MO;COUNT=4
EXDATE:20260504T090000Z
EXDATE:20260518T090000Z
END:VEVENT
END:VCALENDAR
`
	events, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
	e := events[0]
	if e.Recurrence == nil {
		t.Fatal("Recurrence = nil, want populated")
	}
	if e.Recurrence.Freq != FreqWeekly {
		t.Errorf("Freq = %v, want FreqWeekly", e.Recurrence.Freq)
	}
	if e.Recurrence.Count != 4 {
		t.Errorf("Count = %d, want 4", e.Recurrence.Count)
	}
	if len(e.Recurrence.ByDay) != 1 || e.Recurrence.ByDay[0] != time.Monday {
		t.Errorf("ByDay = %v, want [Monday]", e.Recurrence.ByDay)
	}
	if len(e.Recurrence.ExDates) != 2 {
		t.Fatalf("ExDates len = %d, want 2", len(e.Recurrence.ExDates))
	}
	want0 := time.Date(2026, 5, 4, 9, 0, 0, 0, time.UTC)
	if !e.Recurrence.ExDates[0].Equal(want0) {
		t.Errorf("ExDates[0] = %v, want %v", e.Recurrence.ExDates[0], want0)
	}
}

// TestParse_EXDATEBeforeRRULE confirms ordering doesn't matter: an
// EXDATE that appears before the RRULE in the VEVENT (Google Calendar
// emits this) must still attach to the resulting Recurrence rather
// than being dropped. Without the Recurrence-allocate-if-nil branch
// in the EXDATE handler, the EXDATE would silently disappear.
func TestParse_EXDATEBeforeRRULE(t *testing.T) {
	input := `BEGIN:VCALENDAR
BEGIN:VEVENT
UID:reordered
DTSTART:20260427T090000Z
EXDATE:20260504T090000Z
RRULE:FREQ=DAILY;COUNT=10
END:VEVENT
END:VCALENDAR
`
	events, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(events) != 1 || events[0].Recurrence == nil {
		t.Fatalf("Recurrence not populated: %+v", events)
	}
	if len(events[0].Recurrence.ExDates) != 1 {
		t.Errorf("ExDates len = %d, want 1", len(events[0].Recurrence.ExDates))
	}
}

func TestParse_InvalidRRULEErrors(t *testing.T) {
	input := `BEGIN:VCALENDAR
BEGIN:VEVENT
UID:bad
DTSTART:20260427T090000Z
RRULE:FREQ=YEARLY
END:VEVENT
END:VCALENDAR
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for unsupported RRULE")
	}
}

func TestParse_InvalidEXDATEErrors(t *testing.T) {
	input := `BEGIN:VCALENDAR
BEGIN:VEVENT
UID:bad
DTSTART:20260427T090000Z
RRULE:FREQ=DAILY
EXDATE:not-a-date
END:VEVENT
END:VCALENDAR
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for invalid EXDATE")
	}
}

// TestParse_EXDATEWithTZID confirms an EXDATE qualified by a TZID
// parameter is parsed in that zone — so its instant matches the
// corresponding occurrence generated from a TZID-qualified DTSTART.
// Without TZID threading, EXDATE values are parsed as naive UTC, the
// instants diverge, and the EXDATE silently fails to exclude. Real
// Google Calendar feeds emit this form for non-UTC recurring events.
func TestParse_EXDATEWithTZID(t *testing.T) {
	input := `BEGIN:VCALENDAR
BEGIN:VEVENT
UID:zoned
DTSTART;TZID=America/Los_Angeles:20260427T090000
DTEND;TZID=America/Los_Angeles:20260427T093000
RRULE:FREQ=DAILY;COUNT=3
EXDATE;TZID=America/Los_Angeles:20260428T090000
END:VEVENT
END:VCALENDAR
`
	events, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(events) != 1 || events[0].Recurrence == nil {
		t.Fatalf("Recurrence not populated: %+v", events)
	}
	loc, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Skipf("tzdata unavailable: %v", err)
	}
	wantExDate := time.Date(2026, 4, 28, 9, 0, 0, 0, loc)
	got := events[0].Recurrence.ExDates
	if len(got) != 1 {
		t.Fatalf("ExDates len = %d, want 1", len(got))
	}
	// Compare absolute instants — the EXDATE must land at 09:00 LA,
	// not 09:00 UTC.
	if !got[0].Equal(wantExDate) {
		t.Errorf("ExDates[0] = %v (unix %d), want %v (unix %d)",
			got[0], got[0].Unix(), wantExDate, wantExDate.Unix())
	}

	// End-to-end: the EXDATE must actually exclude the Apr 28 occurrence.
	winStart := time.Date(2026, 4, 27, 0, 0, 0, 0, loc)
	winEnd := time.Date(2026, 4, 30, 0, 0, 0, 0, loc)
	occs := Occurrences(events, winStart, winEnd)
	if len(occs) != 2 {
		t.Fatalf("Occurrences len = %d, want 2 (Apr 28 excluded by EXDATE)", len(occs))
	}
	if occs[0].Start.Day() != 27 || occs[1].Start.Day() != 29 {
		t.Errorf("days = (%d, %d), want (27, 29)", occs[0].Start.Day(), occs[1].Start.Day())
	}
}

// TestParse_EXDATEWithoutRRULE pins the EXDATE-only VEVENT case: a
// VEVENT with EXDATE but no RRULE is malformed by RFC 5545 (EXDATE is
// only meaningful relative to a recurrence rule). The event itself
// should still survive — without the END:VEVENT cleanup, parse would
// leave a zero-frequency Recurrence and Occurrences would drop the
// event entirely through the no-default-case switch in expand.
func TestParse_EXDATEWithoutRRULE(t *testing.T) {
	input := `BEGIN:VCALENDAR
BEGIN:VEVENT
UID:lonely-exdate
DTSTART:20260427T090000Z
DTEND:20260427T100000Z
SUMMARY:One-off
EXDATE:20260428T090000Z
END:VEVENT
END:VCALENDAR
`
	events, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
	// The orphan EXDATE has no meaning without an RRULE; the parser
	// must drop the zero-frequency Recurrence so the single instance
	// flows through Occurrences as a non-recurring event.
	if events[0].Recurrence != nil {
		t.Errorf("Recurrence = %+v, want nil (orphan EXDATE)", events[0].Recurrence)
	}

	// End-to-end: the event must show up in a covering window.
	occs := Occurrences(events, time.Date(2026, 4, 27, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 4, 28, 0, 0, 0, 0, time.UTC))
	if len(occs) != 1 {
		t.Fatalf("Occurrences len = %d, want 1 (event must not be dropped)", len(occs))
	}
}

// TestParse_EXDATEUnknownTZIDFallsBackToUTC pins the failure mode for
// a TZID we can't resolve: the existing extractTZID logs a warning and
// returns nil, so the EXDATE parses as a naive UTC datetime. This keeps
// the parser from rejecting otherwise-valid feeds with an exotic TZID.
func TestParse_EXDATEUnknownTZIDFallsBackToUTC(t *testing.T) {
	input := `BEGIN:VCALENDAR
BEGIN:VEVENT
UID:exotic-tz
DTSTART:20260427T090000Z
RRULE:FREQ=DAILY;COUNT=5
EXDATE;TZID=Fake/Zone:20260428T090000
END:VEVENT
END:VCALENDAR
`
	events, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(events) != 1 || events[0].Recurrence == nil {
		t.Fatalf("Recurrence not populated: %+v", events)
	}
	// Unknown TZID → UTC fallback → 09:00 UTC.
	want := time.Date(2026, 4, 28, 9, 0, 0, 0, time.UTC)
	got := events[0].Recurrence.ExDates
	if len(got) != 1 || !got[0].Equal(want) {
		t.Errorf("ExDates = %v, want [%v]", got, want)
	}
}
