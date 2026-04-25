package ical

import "time"

// Event represents a single calendar event parsed from an iCal feed.
type Event struct {
	UID      string
	Summary  string
	Start    time.Time
	End      time.Time
	AllDay   bool
	Location string
}
