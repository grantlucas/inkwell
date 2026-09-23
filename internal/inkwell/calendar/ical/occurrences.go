package ical

import (
	"slices"
	"sort"
	"time"
)

// occurrenceSafetyCap bounds rule expansion so a malformed feed (no
// COUNT, no UNTIL, sparse BYDAY skipping every candidate) can't make
// the dashboard hang. 50k iterations is well past any realistic
// dashboard window (decades of daily occurrences). Declared as a var
// (not a const) so safety-cap exhaustion can be exercised in tests
// without burning real iteration time.
//
// Bounds the outer iteration of each walker, not the yielded count —
// walkWeekly with BYDAY can therefore emit up to cap × len(BYDAY)
// candidates before bailing. Tests that mutate this value MUST NOT run
// in parallel (no t.Parallel() inside ical package tests).
var occurrenceSafetyCap = 50_000

// Occurrences expands recurring events in [start, end) and returns all
// concrete occurrences — recurring and non-recurring — that overlap
// the window, sorted by start time. Returned Events have Recurrence
// set to nil so downstream code can treat each occurrence as a plain
// event; the source event is unchanged.
func Occurrences(events []Event, start, end time.Time) []Event {
	var out []Event
	for _, e := range events {
		if e.Recurrence == nil {
			if e.End.After(start) && e.Start.Before(end) {
				out = append(out, e)
			}
			continue
		}
		out = append(out, expand(e, start, end)...)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Start.Before(out[j].Start)
	})
	return out
}

// expand walks a recurring master event and returns concrete
// occurrences whose [Start, End) overlap [winStart, winEnd).
//
// DST: every walker steps with time.Time.AddDate, which works in the
// receiver's own location, so a recurrence inherits the anchoring of
// whatever zone the parser attached to DTSTART. That is deliberate and
// it is what RFC 5545 asks for in both directions:
//
//   - A TZID-qualified master recurs in that zone, so a 09:00 weekly
//     meeting stays 09:00 local across a transition and the UTC
//     instant moves. This is what a user means by "every Wednesday at
//     nine", and Google emits TZID for any recurrence given a zone, so
//     it is the case the dashboard actually meets.
//   - A Z-suffixed master (RFC 5545 3.3.5 form 2) is an instant, so it
//     recurs at a fixed UTC offset and the local wall time moves
//     instead.
//
// Normalising the walk to UTC would collapse the first case into the
// second and shift every zoned recurring event by an hour for half the
// year. TestOccurrences_DSTTransition pins both readings, across all
// three walkers.
//
// A floating master (form 1 — no Z, no TZID) is a third case, and the
// one place this deviates from the RFC. Form 1 is local time that
// tracks the observer's zone; parseDateTime instead reads it as UTC
// (see its "local datetime, treated as UTC" note), so a floating 09:00
// renders at 09:00 UTC rather than 09:00 wherever the panel is. An
// unresolvable TZID lands in the same bucket. That is a deliberate
// simplification, not conformance — worth knowing before treating the
// behaviour as a specification.
func expand(master Event, winStart, winEnd time.Time) []Event {
	r := master.Recurrence
	interval := r.Interval
	if interval == 0 {
		interval = 1
	}
	duration := master.End.Sub(master.Start)

	// EXDATEs match an occurrence by instant. Unix seconds is enough
	// precision for iCal datetimes (no sub-second values).
	exdates := make(map[int64]struct{}, len(r.ExDates))
	for _, t := range r.ExDates {
		exdates[t.Unix()] = struct{}{}
	}

	var out []Event
	yielded := 0

	// handle is invoked for each candidate occurrence start that
	// survives the per-FREQ filters (FREQ stride, BYDAY filter).
	// It enforces COUNT/UNTIL/window-end, applies EXDATE exclusion,
	// and appends to the result slice. Returns false to stop the walk.
	handle := func(occStart time.Time) bool {
		if r.Count > 0 && yielded >= r.Count {
			return false
		}
		if !r.Until.IsZero() && occStart.After(r.Until) {
			return false
		}
		if !occStart.Before(winEnd) {
			return false
		}
		if _, excluded := exdates[occStart.Unix()]; !excluded {
			occEnd := occStart.Add(duration)
			if occEnd.After(winStart) {
				occ := master
				occ.Start = occStart
				occ.End = occEnd
				occ.Recurrence = nil
				out = append(out, occ)
			}
		}
		// COUNT counts every generated occurrence, including those
		// excluded by EXDATE — matches the common rrule semantics
		// (Google, dateutil) so user expectations are stable.
		yielded++
		return true
	}

	switch r.Freq {
	case FreqDaily:
		walkDaily(master.Start, r, interval, handle)
	case FreqWeekly:
		walkWeekly(master.Start, r, interval, handle)
	case FreqMonthly:
		walkMonthly(master.Start, r, interval, handle)
	}
	return out
}

// walkDaily yields candidates every INTERVAL days starting from
// DTSTART. BYDAY (when set) filters which candidates are kept —
// filtered candidates do NOT count toward COUNT, matching common
// rrule semantics.
func walkDaily(dtstart time.Time, r *Recurrence, interval int, handle func(time.Time) bool) {
	for i := range occurrenceSafetyCap {
		// Recomputed from DTSTART rather than stepped from the previous
		// candidate: a candidate landing in the hour spring-forward
		// skips gets normalised by time.Date, and stepping from it
		// would carry that hour into every occurrence after it.
		cur := dtstart.AddDate(0, 0, i*interval)
		if len(r.ByDay) == 0 || weekdayIn(r.ByDay, cur.Weekday()) {
			if !handle(cur) {
				return
			}
		}
	}
}

// walkWeekly yields candidates per the WEEKLY rule. Without BYDAY,
// occurrences are DTSTART + N*INTERVAL weeks. With BYDAY, each
// INTERVAL-week period yields one occurrence per BYDAY weekday whose
// date falls in [DTSTART, …), preserving DTSTART's clock time.
func walkWeekly(dtstart time.Time, r *Recurrence, interval int, handle func(time.Time) bool) {
	if len(r.ByDay) == 0 {
		for i := range occurrenceSafetyCap {
			// Anchored on DTSTART, not stepped — see walkDaily.
			if !handle(dtstart.AddDate(0, 0, 7*interval*i)) {
				return
			}
		}
		return
	}

	// BYDAY path: anchor at Monday of DTSTART's week, preserving
	// DTSTART's hour/minute/second/nanosecond. We use Monday as week
	// start (WKST default per RFC 5545). The first period's
	// candidates before DTSTART are skipped explicitly.
	weekdayOffset := int(dtstart.Weekday()-time.Monday+7) % 7
	firstAnchor := dtstart.AddDate(0, 0, -weekdayOffset)

	sorted := make([]time.Weekday, len(r.ByDay))
	copy(sorted, r.ByDay)
	sort.Slice(sorted, func(i, j int) bool {
		oi := int(sorted[i]-time.Monday+7) % 7
		oj := int(sorted[j]-time.Monday+7) % 7
		return oi < oj
	})

	for week := range occurrenceSafetyCap {
		// Each week's anchor is recomputed from the first one rather
		// than stepped, for the same reason as walkDaily — though this
		// branch was not observably affected: the anchor is the Monday
		// of the week and the transitions that matter fall on Sundays,
		// so it never landed in a skipped hour. Consistency, and the
		// latent case of a zone that switches midweek.
		weekAnchor := firstAnchor.AddDate(0, 0, 7*interval*week)
		for _, wd := range sorted {
			offset := int(wd-time.Monday+7) % 7
			occ := weekAnchor.AddDate(0, 0, offset)
			if occ.Before(dtstart) {
				continue
			}
			if !handle(occ) {
				return
			}
		}
	}
}

// walkMonthly yields candidates every INTERVAL months from DTSTART,
// using the same day-of-month. Months that don't have that day (Jan 31
// + 1 month) are skipped rather than normalized — matching dateutil
// and Google Calendar.
func walkMonthly(dtstart time.Time, r *Recurrence, interval int, handle func(time.Time) bool) {
	for i := range occurrenceSafetyCap {
		cand := dtstart.AddDate(0, i*interval, 0)
		// AddDate normalizes Jan 31 + 1 month → Mar 3. If the day
		// changed, the candidate month doesn't have the original day;
		// skip without yielding.
		if cand.Day() != dtstart.Day() {
			continue
		}
		if !handle(cand) {
			return
		}
	}
}

func weekdayIn(days []time.Weekday, wd time.Weekday) bool {
	return slices.Contains(days, wd)
}
