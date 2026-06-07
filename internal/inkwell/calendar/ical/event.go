package ical

import "time"

// Event represents a single calendar event parsed from an iCal feed.
// Recurring events carry a Recurrence value that describes how the
// master event expands into concrete occurrences; Occurrences walks
// the rule at filter time and produces flat Event values for each
// concrete instance (with Recurrence set to nil on the result).
type Event struct {
	UID        string
	Summary    string
	Start      time.Time
	End        time.Time
	AllDay     bool
	Location   string
	Recurrence *Recurrence
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
