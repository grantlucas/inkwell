package inkwell

import "time"

// refreshSchedule holds the configured refresh cadence of every widget on a
// screen and decides, for a given wall-clock time, whether any widget is due
// to refresh. It is the "refresh queue": rather than pushing a panel refresh
// the instant any widget's content changes, the render loop only lets a change
// reach the panel when at least one widget is due this minute — so widgets on
// independent cadences coalesce instead of each triggering its own refresh.
//
// Cadences come straight from each widget's required config entry (validated
// to be >= one minute by LoadConfig). A cadence < one minute is treated as
// non-participating (it never opens the gate on its own).
type refreshSchedule struct {
	cadences []time.Duration
}

// anyDue reports whether any widget is due to refresh at now.
//
// A widget with cadence c (whole minutes, >= 1) is due when the minute-of-day
// is divisible by c. Aligning to the wall-clock minute-of-day — rather than to
// an arbitrary start time — means widgets sharing a cadence always fall due on
// the same boundary (e.g. two 5m widgets both fire on :00/:05/:10) and so
// coalesce into a single refresh.
func (s refreshSchedule) anyDue(now time.Time) bool {
	mins := now.Hour()*60 + now.Minute()
	for _, c := range s.cadences {
		cm := int(c / time.Minute)
		if cm < 1 {
			continue // static (or sub-minute): never opens the gate on its own
		}
		if mins%cm == 0 {
			return true
		}
	}
	return false
}
