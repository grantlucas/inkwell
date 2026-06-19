package inkwell

import (
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/widget"
)

// defaultRefreshCadence is the cadence applied to a widget that declares none.
// One minute is also the hard floor: nothing refreshes faster than once a
// minute, so a shorter declared or configured cadence is clamped up to it.
const defaultRefreshCadence = time.Minute

// resolveRefreshCadence picks a widget's effective refresh cadence. An explicit
// per-instance override (from config) wins; otherwise the widget's declared
// RefreshEvery is used, falling back to defaultRefreshCadence for widgets that
// don't implement widget.RefreshCadence. A non-positive result marks the widget
// static (never due on its own); a positive result below the floor clamps up.
func resolveRefreshCadence(w widget.Widget, override time.Duration) time.Duration {
	c := override
	if c == 0 {
		if rc, ok := w.(widget.RefreshCadence); ok {
			c = rc.RefreshEvery()
		} else {
			c = defaultRefreshCadence
		}
	}
	if c <= 0 {
		return 0 // static
	}
	if c < defaultRefreshCadence {
		return defaultRefreshCadence
	}
	return c
}

// refreshSchedule holds the resolved refresh cadence of every widget on a
// screen and decides, for a given wall-clock time, whether any widget is due
// to refresh. It is the "refresh queue": rather than pushing a panel refresh
// the instant any widget's content changes, the render loop only lets a change
// reach the panel when at least one widget is due this minute — so widgets on
// independent cadences coalesce instead of each triggering its own refresh.
//
// A cadence <= 0 marks a static widget that never triggers a refresh on its
// own; stored positive cadences are always >= one minute (the hard floor),
// clamped during resolution.
type refreshSchedule struct {
	cadences []time.Duration
}

// buildSchedule resolves a cadence for each widget. overrides[i], when present
// and non-zero, overrides widget i's declared cadence; a nil or short overrides
// slice leaves the remaining widgets on their declared (or default) cadence.
func buildSchedule(widgets []widget.Widget, overrides []time.Duration) refreshSchedule {
	cadences := make([]time.Duration, len(widgets))
	for i, w := range widgets {
		var override time.Duration
		if i < len(overrides) {
			override = overrides[i]
		}
		cadences[i] = resolveRefreshCadence(w, override)
	}
	return refreshSchedule{cadences: cadences}
}

// anyDue reports whether any non-static widget is due to refresh at now.
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
