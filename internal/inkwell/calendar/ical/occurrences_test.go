package ical

import (
	"testing"
	"time"
)

// utc constructs a UTC datetime for test brevity.
func utc(y int, m time.Month, d, hh, mm int) time.Time {
	return time.Date(y, m, d, hh, mm, 0, 0, time.UTC)
}

func TestOccurrences_NonRecurringPassesThroughWindow(t *testing.T) {
	events := []Event{
		{UID: "a", Start: utc(2026, 4, 27, 9, 0), End: utc(2026, 4, 27, 10, 0)},
		// Outside window:
		{UID: "b", Start: utc(2026, 3, 1, 9, 0), End: utc(2026, 3, 1, 10, 0)},
	}
	got := Occurrences(events, utc(2026, 4, 27, 0, 0), utc(2026, 4, 28, 0, 0))
	if len(got) != 1 || got[0].UID != "a" {
		t.Errorf("got %v, want only event a", got)
	}
}

func TestOccurrences_DailyCount(t *testing.T) {
	master := Event{
		UID:   "daily",
		Start: utc(2026, 4, 27, 9, 0),
		End:   utc(2026, 4, 27, 9, 30),
		Recurrence: &Recurrence{
			Freq:  FreqDaily,
			Count: 3,
		},
	}
	// Wide window so COUNT terminates before window-end does.
	got := Occurrences([]Event{master}, utc(2026, 1, 1, 0, 0), utc(2026, 12, 31, 0, 0))
	if len(got) != 3 {
		t.Fatalf("got %d occurrences, want 3", len(got))
	}
	wantStarts := []time.Time{
		utc(2026, 4, 27, 9, 0),
		utc(2026, 4, 28, 9, 0),
		utc(2026, 4, 29, 9, 0),
	}
	for i, want := range wantStarts {
		if !got[i].Start.Equal(want) {
			t.Errorf("occ[%d].Start = %v, want %v", i, got[i].Start, want)
		}
	}
	// The flattened occurrence must have Recurrence cleared so widgets
	// can treat them as plain Events without recursive expansion.
	if got[0].Recurrence != nil {
		t.Error("expanded occurrence still carries Recurrence")
	}
}

func TestOccurrences_DailyUntil(t *testing.T) {
	master := Event{
		UID:   "daily",
		Start: utc(2026, 4, 27, 9, 0),
		End:   utc(2026, 4, 27, 9, 30),
		Recurrence: &Recurrence{
			Freq:  FreqDaily,
			Until: utc(2026, 4, 29, 23, 59),
		},
	}
	got := Occurrences([]Event{master}, utc(2026, 4, 27, 0, 0), utc(2026, 5, 1, 0, 0))
	if len(got) != 3 { // Apr 27, 28, 29 (Apr 30 is past UNTIL)
		t.Fatalf("got %d occurrences, want 3", len(got))
	}
}

func TestOccurrences_DailyInterval(t *testing.T) {
	master := Event{
		Start: utc(2026, 4, 27, 9, 0),
		End:   utc(2026, 4, 27, 10, 0),
		Recurrence: &Recurrence{
			Freq:     FreqDaily,
			Interval: 3,
			Count:    3,
		},
	}
	got := Occurrences([]Event{master}, utc(2026, 4, 1, 0, 0), utc(2026, 5, 31, 0, 0))
	wantDays := []int{27, 30, 3} // Apr 27, Apr 30, May 3
	wantMonths := []time.Month{time.April, time.April, time.May}
	if len(got) != 3 {
		t.Fatalf("got %d occurrences, want 3", len(got))
	}
	for i := range got {
		if got[i].Start.Day() != wantDays[i] || got[i].Start.Month() != wantMonths[i] {
			t.Errorf("occ[%d] = %v, want %v %d", i, got[i].Start, wantMonths[i], wantDays[i])
		}
	}
}

func TestOccurrences_DailyByDayFilter(t *testing.T) {
	master := Event{
		Start: utc(2026, 4, 27, 9, 0), // Monday
		End:   utc(2026, 4, 27, 10, 0),
		Recurrence: &Recurrence{
			Freq:  FreqDaily,
			ByDay: []time.Weekday{time.Monday, time.Wednesday, time.Friday},
		},
	}
	// One week window: Mon Apr 27 through Sun May 3.
	got := Occurrences([]Event{master}, utc(2026, 4, 27, 0, 0), utc(2026, 5, 4, 0, 0))
	wantDays := []int{27, 29, 1} // Mon Apr 27, Wed Apr 29, Fri May 1
	if len(got) != 3 {
		t.Fatalf("got %d occurrences, want 3 (%v)", len(got), got)
	}
	for i, d := range wantDays {
		if got[i].Start.Day() != d {
			t.Errorf("occ[%d].Day = %d, want %d", i, got[i].Start.Day(), d)
		}
	}
}

func TestOccurrences_WeeklyWithoutByDay(t *testing.T) {
	master := Event{
		Start: utc(2026, 4, 27, 9, 0), // Monday
		End:   utc(2026, 4, 27, 10, 0),
		Recurrence: &Recurrence{
			Freq:  FreqWeekly,
			Count: 4,
		},
	}
	got := Occurrences([]Event{master}, utc(2026, 4, 1, 0, 0), utc(2026, 6, 1, 0, 0))
	wantStarts := []time.Time{
		utc(2026, 4, 27, 9, 0),
		utc(2026, 5, 4, 9, 0),
		utc(2026, 5, 11, 9, 0),
		utc(2026, 5, 18, 9, 0),
	}
	if len(got) != 4 {
		t.Fatalf("got %d, want 4 (%v)", len(got), got)
	}
	for i, want := range wantStarts {
		if !got[i].Start.Equal(want) {
			t.Errorf("occ[%d] = %v, want %v", i, got[i].Start, want)
		}
	}
}

func TestOccurrences_WeeklyWithByDay(t *testing.T) {
	// DTSTART = Mon Apr 27. RRULE = WEEKLY;BYDAY=MO,WE;COUNT=4.
	// Expected: Mon Apr 27, Wed Apr 29, Mon May 4, Wed May 6.
	master := Event{
		Start: utc(2026, 4, 27, 9, 0),
		End:   utc(2026, 4, 27, 10, 0),
		Recurrence: &Recurrence{
			Freq:  FreqWeekly,
			ByDay: []time.Weekday{time.Monday, time.Wednesday},
			Count: 4,
		},
	}
	got := Occurrences([]Event{master}, utc(2026, 4, 1, 0, 0), utc(2026, 6, 1, 0, 0))
	wantDays := []int{27, 29, 4, 6}
	wantMonths := []time.Month{time.April, time.April, time.May, time.May}
	if len(got) != 4 {
		t.Fatalf("got %d, want 4 (%v)", len(got), got)
	}
	for i := range got {
		if got[i].Start.Day() != wantDays[i] || got[i].Start.Month() != wantMonths[i] {
			t.Errorf("occ[%d] = %v, want %v %d", i, got[i].Start, wantMonths[i], wantDays[i])
		}
	}
}

func TestOccurrences_WeeklyByDay_DTStartNotInByDay(t *testing.T) {
	// DTSTART = Wed Apr 29. BYDAY=MO,FR.
	// The first valid occurrence is Fri May 1 (DTSTART isn't yielded
	// because Wed isn't in BYDAY). Following: Mon May 4, Fri May 8, ...
	master := Event{
		Start: utc(2026, 4, 29, 9, 0), // Wednesday
		End:   utc(2026, 4, 29, 10, 0),
		Recurrence: &Recurrence{
			Freq:  FreqWeekly,
			ByDay: []time.Weekday{time.Monday, time.Friday},
			Count: 3,
		},
	}
	got := Occurrences([]Event{master}, utc(2026, 4, 1, 0, 0), utc(2026, 6, 1, 0, 0))
	wantStarts := []time.Time{
		utc(2026, 5, 1, 9, 0), // Fri
		utc(2026, 5, 4, 9, 0), // Mon
		utc(2026, 5, 8, 9, 0), // Fri
	}
	if len(got) != 3 {
		t.Fatalf("got %d, want 3 (%v)", len(got), got)
	}
	for i, want := range wantStarts {
		if !got[i].Start.Equal(want) {
			t.Errorf("occ[%d] = %v, want %v", i, got[i].Start, want)
		}
	}
}

func TestOccurrences_Monthly(t *testing.T) {
	master := Event{
		Start: utc(2026, 4, 15, 9, 0),
		End:   utc(2026, 4, 15, 10, 0),
		Recurrence: &Recurrence{
			Freq:  FreqMonthly,
			Count: 3,
		},
	}
	got := Occurrences([]Event{master}, utc(2026, 1, 1, 0, 0), utc(2026, 12, 1, 0, 0))
	wantMonths := []time.Month{time.April, time.May, time.June}
	if len(got) != 3 {
		t.Fatalf("got %d, want 3", len(got))
	}
	for i, m := range wantMonths {
		if got[i].Start.Month() != m || got[i].Start.Day() != 15 {
			t.Errorf("occ[%d] = %v, want %v 15", i, got[i].Start, m)
		}
	}
}

// Jan 31 + 1 month doesn't exist in Feb, so the Feb occurrence should be
// skipped (not normalized to Mar 3) per common rrule semantics.
func TestOccurrences_MonthlyDayOverflow(t *testing.T) {
	master := Event{
		Start: utc(2026, 1, 31, 9, 0),
		End:   utc(2026, 1, 31, 10, 0),
		Recurrence: &Recurrence{
			Freq:  FreqMonthly,
			Count: 4,
		},
	}
	got := Occurrences([]Event{master}, utc(2026, 1, 1, 0, 0), utc(2027, 1, 1, 0, 0))
	if len(got) != 4 {
		t.Fatalf("got %d, want 4 (%v)", len(got), got)
	}
	// Jan 31, Mar 31, May 31, Jul 31 (Feb, Apr, Jun skipped — no day 31).
	wantMonths := []time.Month{time.January, time.March, time.May, time.July}
	for i, m := range wantMonths {
		if got[i].Start.Month() != m {
			t.Errorf("occ[%d].Month = %v, want %v", i, got[i].Start.Month(), m)
		}
		if got[i].Start.Day() != 31 {
			t.Errorf("occ[%d].Day = %d, want 31", i, got[i].Start.Day())
		}
	}
}

func TestOccurrences_EXDATEExcludesOccurrence(t *testing.T) {
	master := Event{
		Start: utc(2026, 4, 27, 9, 0),
		End:   utc(2026, 4, 27, 10, 0),
		Recurrence: &Recurrence{
			Freq:    FreqDaily,
			Count:   5,
			ExDates: []time.Time{utc(2026, 4, 29, 9, 0)}, // exclude the third one
		},
	}
	// Window must include May 1 (where the 5th occurrence lands) since
	// COUNT counts EXDATE-excluded candidates per Google/dateutil
	// semantics — Apr 29 is counted-but-excluded, May 1 is the 5th.
	got := Occurrences([]Event{master}, utc(2026, 4, 1, 0, 0), utc(2026, 5, 2, 0, 0))
	wantDays := []int{27, 28, 30, 1}
	if len(got) != 4 {
		t.Fatalf("got %d, want 4 (%v)", len(got), got)
	}
	for i, d := range wantDays {
		if got[i].Start.Day() != d {
			t.Errorf("occ[%d].Day = %d, want %d", i, got[i].Start.Day(), d)
		}
	}
}

func TestOccurrences_EXDATEExcludesMaster(t *testing.T) {
	master := Event{
		Start: utc(2026, 4, 27, 9, 0),
		End:   utc(2026, 4, 27, 10, 0),
		Recurrence: &Recurrence{
			Freq:    FreqDaily,
			Count:   3,
			ExDates: []time.Time{utc(2026, 4, 27, 9, 0)},
		},
	}
	got := Occurrences([]Event{master}, utc(2026, 4, 1, 0, 0), utc(2026, 5, 1, 0, 0))
	// Master itself excluded; Apr 28 and Apr 29 remain.
	if len(got) != 2 {
		t.Fatalf("got %d, want 2 (%v)", len(got), got)
	}
	if got[0].Start.Day() != 28 || got[1].Start.Day() != 29 {
		t.Errorf("days = (%d, %d), want (28, 29)", got[0].Start.Day(), got[1].Start.Day())
	}
}

func TestOccurrences_WindowExcludesOutOfRange(t *testing.T) {
	master := Event{
		Start: utc(2026, 4, 1, 9, 0),
		End:   utc(2026, 4, 1, 10, 0),
		Recurrence: &Recurrence{
			Freq:  FreqDaily,
			Count: 100,
		},
	}
	got := Occurrences([]Event{master}, utc(2026, 4, 10, 0, 0), utc(2026, 4, 12, 0, 0))
	// Only Apr 10 and Apr 11 land in the window.
	if len(got) != 2 {
		t.Fatalf("got %d, want 2 (%v)", len(got), got)
	}
	if got[0].Start.Day() != 10 || got[1].Start.Day() != 11 {
		t.Errorf("days = (%d, %d), want (10, 11)", got[0].Start.Day(), got[1].Start.Day())
	}
}

func TestOccurrences_MixedEventsSortedByStart(t *testing.T) {
	events := []Event{
		// Non-recurring later event.
		{UID: "late", Start: utc(2026, 4, 28, 14, 0), End: utc(2026, 4, 28, 15, 0)},
		// Daily recurring earlier event.
		{
			UID:   "daily",
			Start: utc(2026, 4, 27, 9, 0),
			End:   utc(2026, 4, 27, 10, 0),
			Recurrence: &Recurrence{
				Freq:  FreqDaily,
				Count: 2,
			},
		},
	}
	got := Occurrences(events, utc(2026, 4, 1, 0, 0), utc(2026, 5, 1, 0, 0))
	// Expected order: daily/Apr27, daily/Apr28, late/Apr28.
	if len(got) != 3 {
		t.Fatalf("got %d, want 3", len(got))
	}
	for i := 0; i < len(got)-1; i++ {
		if got[i+1].Start.Before(got[i].Start) {
			t.Errorf("not sorted: got[%d]=%v, got[%d]=%v", i, got[i].Start, i+1, got[i+1].Start)
		}
	}
}

// TestOccurrences_SafetyCapTerminates covers the defensive bound on
// each walker — if a malformed rule and a far-future window ever leave
// handle() always returning true, the walker must still exit cleanly
// rather than spinning. We lower the cap to a tiny number so the
// test stays fast.
func TestOccurrences_SafetyCapTerminates(t *testing.T) {
	old := occurrenceSafetyCap
	occurrenceSafetyCap = 5
	defer func() { occurrenceSafetyCap = old }()

	master := Event{
		Start: utc(2026, 4, 1, 9, 0),
		End:   utc(2026, 4, 1, 10, 0),
		Recurrence: &Recurrence{
			Freq: FreqWeekly, // no BYDAY → simplest walker
		},
	}
	// Window large enough that the window check never trips before cap.
	got := Occurrences([]Event{master}, utc(2026, 4, 1, 0, 0), utc(3000, 1, 1, 0, 0))
	if len(got) != 5 {
		t.Errorf("got %d occurrences, want 5 (safety cap)", len(got))
	}
}

// Defensive: a recurring event with no termination condition (no COUNT,
// no UNTIL) must still be bounded by the requested window. Otherwise
// a dashboard config asking for "next month" of a forever-weekly event
// would walk forever — caught by the safety cap, but the window check
// makes it cheap.
func TestOccurrences_UnboundedRRULEStopsAtWindow(t *testing.T) {
	master := Event{
		Start: utc(2026, 4, 1, 9, 0),
		End:   utc(2026, 4, 1, 10, 0),
		Recurrence: &Recurrence{
			Freq: FreqDaily,
		},
	}
	got := Occurrences([]Event{master}, utc(2026, 4, 1, 0, 0), utc(2026, 4, 8, 0, 0))
	// 7-day window starting on DTSTART → 7 occurrences (Apr 1..7).
	if len(got) != 7 {
		t.Fatalf("got %d, want 7", len(got))
	}
}

// Recurring all-day events must propagate AllDay through expansion;
// otherwise a recurring birthday/holiday would render as a timed event
// at midnight on the device. A one-line regression guard for the
// occ := master copy in expand().
func TestOccurrences_AllDayPropagatesThroughExpansion(t *testing.T) {
	master := Event{
		UID:    "holiday",
		Start:  utc(2026, 4, 27, 0, 0),
		End:    utc(2026, 4, 28, 0, 0),
		AllDay: true,
		Recurrence: &Recurrence{
			Freq:  FreqDaily,
			Count: 3,
		},
	}
	got := Occurrences([]Event{master}, utc(2026, 4, 27, 0, 0), utc(2026, 5, 1, 0, 0))
	if len(got) != 3 {
		t.Fatalf("got %d, want 3", len(got))
	}
	for i, e := range got {
		if !e.AllDay {
			t.Errorf("occ[%d].AllDay = false, want true", i)
		}
	}
}
