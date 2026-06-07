// Package ical provides a minimal RFC 5545 iCalendar parser that extracts
// VEVENT components into Event values.
package ical

import (
	"fmt"
	"io"
	"log"
	"sort"
	"strings"
	"time"
)

// Parse reads iCalendar data from r and returns all VEVENT entries as
// Event values, sorted by start time. Events without a DTSTART are
// silently skipped.
func Parse(r io.Reader) ([]Event, error) {
	lines, err := unfold(r)
	if err != nil {
		return nil, fmt.Errorf("read iCal stream: %w", err)
	}

	var events []Event
	var cur *Event
	var curDuration time.Duration
	var hasDuration bool
	inEvent := false

	for _, line := range lines {
		switch {
		case line == "BEGIN:VEVENT":
			inEvent = true
			cur = &Event{}
			curDuration = 0
			hasDuration = false
		case line == "END:VEVENT":
			if inEvent && cur != nil && !cur.Start.IsZero() {
				switch {
				case hasDuration:
					cur.End = cur.Start.Add(curDuration)
				case cur.End.IsZero() && cur.AllDay:
					// RFC 5545 §3.6.1: an all-day VEVENT without DTEND
					// or DURATION ends at DTSTART + 1 day. Defaulting
					// to Start would otherwise drop the event from
					// filterEventsForDay's End.After(start) check.
					cur.End = cur.Start.AddDate(0, 0, 1)
				case cur.End.IsZero():
					cur.End = cur.Start
				}
				events = append(events, *cur)
			}
			inEvent = false
			cur = nil
			curDuration = 0
			hasDuration = false
		case inEvent && cur != nil:
			name, value := splitProperty(line)
			switch name {
			case "UID":
				cur.UID = value
			case "SUMMARY":
				cur.Summary = value
			case "LOCATION":
				cur.Location = value
			case "DTSTART":
				t, allDay, err := parseDateTime(line)
				if err != nil {
					return nil, fmt.Errorf("parse DTSTART: %w", err)
				}
				cur.Start = t
				cur.AllDay = allDay
			case "DTEND":
				t, _, err := parseDateTime(line)
				if err != nil {
					return nil, fmt.Errorf("parse DTEND: %w", err)
				}
				cur.End = t
			case "DURATION":
				d, err := parseDuration(value)
				if err != nil {
					return nil, fmt.Errorf("parse DURATION: %w", err)
				}
				curDuration = d
				hasDuration = true
			case "RRULE":
				rule, err := parseRRULE(value)
				if err != nil {
					return nil, fmt.Errorf("parse RRULE: %w", err)
				}
				// Carry forward any EXDATEs that arrived before this
				// RRULE — Google Calendar emits EXDATE-before-RRULE.
				if cur.Recurrence != nil {
					rule.ExDates = append(rule.ExDates, cur.Recurrence.ExDates...)
				}
				cur.Recurrence = &rule
			case "EXDATE":
				// EXDATE may appear multiple times and each line may
				// carry a comma-separated list. The full property line
				// (not just value) is needed so the TZID parameter
				// anchors naive datetimes to the right zone — without
				// this, a "EXDATE;TZID=America/Los_Angeles:..." would
				// parse as UTC and fail to match the corresponding
				// TZID-anchored occurrence instant.
				params, _, _ := strings.Cut(line, ":")
				loc := extractTZID(params)
				for v := range strings.SplitSeq(value, ",") {
					t, err := parseICSTime(v, loc)
					if err != nil {
						return nil, fmt.Errorf("parse EXDATE %q: %w", v, err)
					}
					if cur.Recurrence == nil {
						cur.Recurrence = &Recurrence{}
					}
					cur.Recurrence.ExDates = append(cur.Recurrence.ExDates, t)
				}
			}
		}
	}

	sort.Slice(events, func(i, j int) bool {
		return events[i].Start.Before(events[j].Start)
	})

	return events, nil
}

// splitProperty splits "NAME;params:value" into (NAME, value).
func splitProperty(line string) (string, string) {
	// Find the first colon to split name (with params) from value.
	before, after, ok := strings.Cut(line, ":")
	if !ok {
		return line, ""
	}
	nameWithParams := before
	value := after

	// Strip parameters: "DTSTART;VALUE=DATE" → "DTSTART"
	if semiIdx := strings.IndexByte(nameWithParams, ';'); semiIdx >= 0 {
		nameWithParams = nameWithParams[:semiIdx]
	}
	return nameWithParams, value
}

// parseDateTime parses an iCal date or datetime value. It handles:
//   - 20060102T150405Z (UTC datetime)
//   - 20060102T150405  (local datetime, treated as UTC)
//   - 20060102         (all-day date)
//
// The full property line is passed to detect VALUE=DATE parameters.
func parseDateTime(line string) (time.Time, bool, error) {
	before, after, ok := strings.Cut(line, ":")
	if !ok {
		return time.Time{}, false, fmt.Errorf("missing colon in %q", line)
	}
	value := after
	params := before

	isDate := strings.Contains(params, "VALUE=DATE") && !strings.Contains(params, "VALUE=DATE-TIME")

	if isDate || len(value) == 8 {
		t, err := time.Parse("20060102", value)
		if err != nil {
			return time.Time{}, false, fmt.Errorf("invalid date %q: %w", value, err)
		}
		return t, true, nil
	}

	if strings.HasSuffix(value, "Z") {
		t, err := time.Parse("20060102T150405Z", value)
		if err != nil {
			return time.Time{}, false, fmt.Errorf("invalid UTC datetime %q: %w", value, err)
		}
		return t, false, nil
	}

	if loc := extractTZID(params); loc != nil {
		t, err := time.ParseInLocation("20060102T150405", value, loc)
		if err != nil {
			return time.Time{}, false, fmt.Errorf("invalid datetime %q: %w", value, err)
		}
		return t, false, nil
	}

	t, err := time.Parse("20060102T150405", value)
	if err != nil {
		return time.Time{}, false, fmt.Errorf("invalid datetime %q: %w", value, err)
	}
	return t, false, nil
}

// extractTZID extracts and loads a timezone from a TZID parameter.
// Returns nil if no TZID is found or if the timezone is unknown. An
// unknown TZID gets a log line so an operator can spot timezone bugs
// in the feed (e.g. a Toronto event suddenly rendering in UTC) instead
// of silently mis-bucketing the event into the wrong column.
func extractTZID(params string) *time.Location {
	for part := range strings.SplitSeq(params, ";") {
		if strings.HasPrefix(part, "TZID=") {
			name := part[5:]
			loc, err := time.LoadLocation(name)
			if err != nil {
				log.Printf("ical: unknown TZID %q, treating as UTC", name)
				return nil
			}
			return loc
		}
	}
	return nil
}

// parseDuration parses an iCal DURATION value like "PT1H30M", "P1D", etc.
func parseDuration(s string) (time.Duration, error) {
	if len(s) == 0 || s[0] != 'P' {
		return 0, fmt.Errorf("invalid duration %q: must start with P", s)
	}
	s = s[1:] // strip P

	var d time.Duration
	inTime := false

	for len(s) > 0 {
		if s[0] == 'T' {
			inTime = true
			s = s[1:]
			continue
		}

		// Read numeric value.
		i := 0
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
		}
		if i == 0 || i >= len(s) {
			return 0, fmt.Errorf("invalid duration component in %q", s)
		}

		n := 0
		for _, c := range s[:i] {
			n = n*10 + int(c-'0')
		}
		unit := s[i]
		s = s[i+1:]

		switch {
		case unit == 'D' && !inTime:
			d += time.Duration(n) * 24 * time.Hour
		case unit == 'W' && !inTime:
			d += time.Duration(n) * 7 * 24 * time.Hour
		case unit == 'H' && inTime:
			d += time.Duration(n) * time.Hour
		case unit == 'M' && inTime:
			d += time.Duration(n) * time.Minute
		case unit == 'S' && inTime:
			d += time.Duration(n) * time.Second
		default:
			return 0, fmt.Errorf("unknown duration unit %q (inTime=%v)", string(unit), inTime)
		}
	}

	return d, nil
}
