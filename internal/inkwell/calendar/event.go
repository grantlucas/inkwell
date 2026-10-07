// Package calendar is the calendar module every calendar widget reads
// through: given a widget's feeds, a window and a refresh setting, its
// Provider returns the occurrences that overlap the window, with
// recurrences expanded, overrides honoured, declined events removed,
// rules applied and duplicates collapsed, from one cache per feed shared
// across widgets and screens.
package calendar

import (
	"context"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar/ical"
)

// Event is an alias for ical.Event, re-exported for convenience.
type Event = ical.Event

// Source provides calendar events for a time range.
// Implementations must be safe for concurrent use.
type Source interface {
	// Events returns events overlapping [start, end), sorted by Start time.
	// ctx bounds any underlying I/O — implementations that perform HTTP or
	// other cancellable work must honor it.
	Events(ctx context.Context, start, end time.Time) ([]Event, error)
}
