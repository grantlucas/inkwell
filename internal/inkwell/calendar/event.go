// Package calendar provides calendar event types, data sources, and iCal
// parsing for the Inkwell calendar widget.
package calendar

import "time"

// Event represents a single calendar event.
type Event struct {
	UID      string
	Summary  string
	Start    time.Time
	End      time.Time
	AllDay   bool
	Location string
}

// Source provides calendar events for a time range.
// Implementations must be safe for concurrent use.
type Source interface {
	// Events returns events overlapping [start, end), sorted by Start time.
	Events(start, end time.Time) ([]Event, error)
}
