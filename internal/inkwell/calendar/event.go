// Package calendar provides calendar event types, data sources, and iCal
// parsing for the Inkwell calendar widget.
package calendar

import (
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar/ical"
)

// Event is an alias for ical.Event, re-exported for convenience.
type Event = ical.Event

// Source provides calendar events for a time range.
// Implementations must be safe for concurrent use.
type Source interface {
	// Events returns events overlapping [start, end), sorted by Start time.
	Events(start, end time.Time) ([]Event, error)
}
