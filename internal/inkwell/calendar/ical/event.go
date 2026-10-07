package ical

import (
	"strings"
	"time"
)

// Event represents a single calendar event parsed from an iCal feed.
// Recurring events carry a Recurrence value that describes how the
// master event expands into concrete occurrences; Occurrences walks
// the rule at filter time and produces flat Event values for each
// concrete instance (with Recurrence set to nil on the result).
//
// An event with a RecurrenceID is an override: it stands in for the
// occurrence of the series sharing its UID that would have started at
// RecurrenceID, wherever the override itself now starts. A Cancelled
// override removes that occurrence and is never shown itself.
//
// Declined holds the addresses of the attendees who have said no to
// the event. Declining leaves the event's STATUS alone, since the event
// still happens for everyone else, so it is recorded per attendee.
type Event struct {
	UID          string
	Summary      string
	Start        time.Time
	End          time.Time
	AllDay       bool
	Location     string
	Recurrence   *Recurrence
	RecurrenceID time.Time
	Cancelled    bool
	Declined     []string
}

// Frequency is the FREQ= value of an RRULE. Only the three the issue
// scope calls out are supported; YEARLY and others are rejected at
// parse time so unsupported feeds fail loudly instead of silently
// missing recurrences.
type Frequency int

const (
	// FreqDaily corresponds to FREQ=DAILY.
	FreqDaily Frequency = iota + 1
	// FreqWeekly corresponds to FREQ=WEEKLY.
	FreqWeekly
	// FreqMonthly corresponds to FREQ=MONTHLY.
	FreqMonthly
)

// Recurrence captures the subset of RFC 5545 RRULE/EXDATE that Inkwell
// supports. Defaults are zero-valued: Interval=0 is treated as 1,
// Count=0 means unbounded, zero Until means unbounded. ByDay applies
// to weekly rules (and acts as a day-of-week filter for daily rules);
// it's ignored for monthly rules — positional BYDAY (e.g. "2MO") is
// out of scope.
type Recurrence struct {
	Freq     Frequency
	Interval int
	Count    int
	Until    time.Time
	ByDay    []time.Weekday
	ExDates  []time.Time
}

// DeclinedBy reports whether the attendee at addr has declined e. An
// email address is matched without regard to case.
func (e Event) DeclinedBy(addr string) bool {
	for _, d := range e.Declined {
		if strings.EqualFold(d, addr) {
			return true
		}
	}
	return false
}

// IsOverride reports whether e edits one instance of a series rather
// than standing alone or being the series itself.
func (e Event) IsOverride() bool {
	return !e.RecurrenceID.IsZero()
}
