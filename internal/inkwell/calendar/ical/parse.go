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

	// VTIMEZONE is collected in its own pass so a component declared
	// after the VEVENT referencing it still resolves — RFC 5545 fixes no
	// order, and the zone is needed while the VEVENT is being read.
	zones := parseVTimezones(lines)

	var events []Event
	var cur *Event
	var curDuration time.Duration
	var hasDuration bool
	var cancelled bool
	inEvent := false

	for _, line := range lines {
		switch {
		case line == "BEGIN:VEVENT":
			inEvent = true
			cur = &Event{}
			curDuration = 0
			hasDuration = false
			cancelled = false
		case line == "END:VEVENT":
			if inEvent && cur != nil && !cur.Start.IsZero() && !cancelled {
				// EXDATE without RRULE would otherwise leave a
				// Recurrence with Freq=0; Occurrences would route the
				// event through expand(), the switch on Freq would
				// match no case, and the event would vanish silently.
				// Orphan EXDATEs have no meaning per RFC 5545 — drop
				// the recurrence metadata and keep the single instance.
				if cur.Recurrence != nil && cur.Recurrence.Freq == 0 {
					cur.Recurrence = nil
				}
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
			cancelled = false
		case inEvent && cur != nil:
			name, value := splitProperty(line)
			switch name {
			case "UID":
				cur.UID = value
			case "SUMMARY":
				cur.Summary = unescapeText(value)
			case "LOCATION":
				cur.Location = unescapeText(value)
			case "STATUS":
				// RFC 5545 3.8.1.11: a CANCELLED VEVENT has been
				// called off, so it must never reach the screen —
				// league team feeds routinely keep cancelled practices
				// in the feed alongside live ones. TENTATIVE and
				// CONFIRMED are both still happening, so only
				// CANCELLED is dropped.
				cancelled = strings.EqualFold(value, "CANCELLED")
			case "DTSTART":
				t, allDay, err := parseDateTime(line, zones)
				if err != nil {
					return nil, fmt.Errorf("parse DTSTART: %w", err)
				}
				cur.Start = t
				cur.AllDay = allDay
			case "DTEND":
				t, _, err := parseDateTime(line, zones)
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
				params, _, _ := cutProperty(line)
				// Resolved once for the line: every value on it shares
				// the same TZID, and the zone no longer depends on
				// which instant is being placed.
				loc := extractTZID(params, zones)
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

// unescapeText reverses the TEXT escaping of RFC 5545 3.3.11: a
// backslash escapes another backslash, a semicolon, a comma, or (as \n
// or \N) a newline. Feeds lean on this heavily — a league team feed
// packs a whole event heading into one SUMMARY separated by escaped
// newlines — and
// leaving the escapes in place puts literal backslash-n on the panel
// and defeats word wrapping, which splits on real whitespace.
//
// An undefined escape (say \q) keeps both bytes: the sequence has no
// RFC meaning, so dropping the backslash would silently corrupt a
// summary rather than pass it through untouched.
func unescapeText(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 == len(s) {
			b.WriteByte(s[i])
			continue
		}
		i++
		switch s[i] {
		case 'n', 'N':
			b.WriteByte('\n')
		case '\\', ';', ',':
			b.WriteByte(s[i])
		default:
			b.WriteByte('\\')
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

// cutProperty splits a content line into its "NAME;params" prefix and
// its value at the first colon lying outside a quoted parameter value.
// RFC 5545 3.1 requires a param value containing ':', ';' or ',' to be
// DQUOTE-wrapped, so cutting at the first colon outright lands inside
// such a value — `TZID="Customized Time Zone: Eastern"` would yield a
// value of ` Eastern":...`, which fails to parse and errors the whole
// feed out rather than degrading that one event to UTC.
func cutProperty(line string) (params, value string, ok bool) {
	inQuotes := false
	for i := range len(line) {
		switch line[i] {
		case '"':
			inQuotes = !inQuotes
		case ':':
			if !inQuotes {
				return line[:i], line[i+1:], true
			}
		}
	}
	// A stray DQUOTE — an inch mark in an unquoted value, a truncated
	// parameter — leaves the scan quoted to end of line, so no colon
	// was ever accepted. Reporting "no colon" would send splitProperty
	// down its fallback, which returns the whole line as the property
	// name; that matches no case in Parse's switch, so DTSTART never
	// lands and the event is dropped at END:VEVENT with no error and no
	// log line. Quotes that never closed were never really quotes, so
	// fall back to the naive cut and let the event degrade to UTC —
	// recoverable, and visible in the log.
	if inQuotes {
		return strings.Cut(line, ":")
	}
	return line, "", false
}

// splitParams splits a "NAME;a=1;b=2" prefix on the semicolons lying
// outside quoted values, so a quoted param value carrying its own ';'
// survives as one segment.
func splitParams(params string) []string {
	var parts []string
	inQuotes := false
	start := 0
	for i := range len(params) {
		switch params[i] {
		case '"':
			inQuotes = !inQuotes
		case ';':
			if !inQuotes {
				parts = append(parts, params[start:i])
				start = i + 1
			}
		}
	}
	return append(parts, params[start:])
}

// splitProperty splits "NAME;params:value" into (NAME, value).
func splitProperty(line string) (string, string) {
	// Find the value-delimiting colon to split name (with params)
	// from value.
	before, after, ok := cutProperty(line)
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
func parseDateTime(line string, zones map[string]*vtimezone) (time.Time, bool, error) {
	before, after, ok := cutProperty(line)
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

	if loc := extractTZID(params, zones); loc != nil {
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
func extractTZID(params string, zones map[string]*vtimezone) *time.Location {
	for _, part := range splitParams(params) {
		if strings.HasPrefix(part, "TZID=") {
			// RFC 5545 3.1 lets a param value be DQUOTE-wrapped, and
			// a value containing ':', ';' or ',' must be. The quotes
			// delimit the value and are not part of it, so they have
			// to come off before the lookup.
			name := strings.Trim(part[5:], `"`)
			if loc, err := time.LoadLocation(name); err == nil {
				return loc
			}
			// Not an IANA name. Feeds that ship one (Outlook emits
			// Windows zone names) define it in a VTIMEZONE, so the
			// offsets are usually right there in the file.
			if vt := zones[name]; vt != nil {
				return vt.locationFor()
			}
			log.Printf("ical: unknown TZID %q, treating as UTC", name)
			return nil
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
