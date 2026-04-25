// Package ical provides a minimal RFC 5545 iCalendar parser that extracts
// VEVENT components into calendar.Event values.
package ical

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar"
)

// Parse reads iCalendar data from r and returns all VEVENT entries as
// calendar.Event values, sorted by start time. Events without a DTSTART
// are silently skipped.
func Parse(r io.Reader) ([]calendar.Event, error) {
	lines := unfold(r)

	var events []calendar.Event
	var cur *calendar.Event
	inEvent := false

	for _, line := range lines {
		switch {
		case line == "BEGIN:VEVENT":
			inEvent = true
			cur = &calendar.Event{}
		case line == "END:VEVENT":
			if inEvent && cur != nil && !cur.Start.IsZero() {
				if cur.End.IsZero() {
					cur.End = cur.Start
				}
				events = append(events, *cur)
			}
			inEvent = false
			cur = nil
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
				// DURATION is applied after DTSTART is known;
				// store as End relative to zero, fix up below.
				cur.End = time.Time{}.Add(d)
			}
		}
	}

	// Fix up DURATION-based End times.
	for i := range events {
		if events[i].End.Before(time.Date(1, 1, 2, 0, 0, 0, 0, time.UTC)) && !events[i].End.IsZero() {
			d := events[i].End.Sub(time.Time{})
			events[i].End = events[i].Start.Add(d)
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
	colonIdx := strings.IndexByte(line, ':')
	if colonIdx < 0 {
		return line, ""
	}
	nameWithParams := line[:colonIdx]
	value := line[colonIdx+1:]

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
	colonIdx := strings.IndexByte(line, ':')
	if colonIdx < 0 {
		return time.Time{}, false, fmt.Errorf("missing colon in %q", line)
	}
	value := line[colonIdx+1:]
	params := line[:colonIdx]

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

	t, err := time.Parse("20060102T150405", value)
	if err != nil {
		return time.Time{}, false, fmt.Errorf("invalid datetime %q: %w", value, err)
	}
	return t, false, nil
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
