// Package calendar provides calendar event types, data sources, and iCal
// parsing for the Inkwell calendar widget.
package calendar

import (
	"context"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar/ical"
)

// Event is an alias for ical.Event, re-exported for convenience.
type Event = ical.Event

// Recurrence and Frequency are re-exported so consumers can construct
// recurring events (chiefly tests and synthetic feeds) without
// importing the ical package directly.
type (
	Recurrence = ical.Recurrence
	Frequency  = ical.Frequency
)

// Frequency constants re-exported for the same reason.
const (
	FreqDaily   = ical.FreqDaily
	FreqWeekly  = ical.FreqWeekly
	FreqMonthly = ical.FreqMonthly
)

// Source provides calendar events for a time range.
// Implementations must be safe for concurrent use.
type Source interface {
	// Events returns events overlapping [start, end), sorted by Start time.
	// ctx bounds any underlying I/O — implementations that perform HTTP or
	// other cancellable work must honor it.
	Events(ctx context.Context, start, end time.Time) ([]Event, error)
}
