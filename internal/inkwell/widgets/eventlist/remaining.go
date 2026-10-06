package eventlist

import (
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar"
)

// Remaining drops the events that finished before now, for a list that
// spends its room on what is left of the day. An event still running
// counts as remaining: it is the one you most want to see.
func Remaining(events []calendar.Event, now time.Time) []calendar.Event {
	var out []calendar.Event
	for _, e := range events {
		// Not After: a timed VEVENT with neither DTEND nor DURATION is
		// parsed with End == Start, so After would drop a reminder at
		// the very minute it fires — and if it were the last one, the
		// list would say the day is done over an event happening now.
		// An all-day event applies to the whole day and never finishes
		// partway through it.
		if e.AllDay || !e.End.Before(now) {
			out = append(out, e)
		}
	}
	return out
}
