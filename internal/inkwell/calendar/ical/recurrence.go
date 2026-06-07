package ical

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// parseRRULE parses an RRULE property value (everything after the
// "RRULE:" prefix) into a Recurrence. Returns an error if FREQ is
// missing, unsupported, or any field fails to parse — that's
// deliberate: silently dropping an unrecognized rule would let
// recurring events disappear from the dashboard with no diagnostic.
func parseRRULE(value string) (Recurrence, error) {
	if value == "" {
		return Recurrence{}, fmt.Errorf("empty RRULE")
	}
	var r Recurrence
	var sawFreq bool
	for part := range strings.SplitSeq(value, ";") {
		k, v, ok := strings.Cut(part, "=")
		if !ok {
			return Recurrence{}, fmt.Errorf("malformed RRULE part %q", part)
		}
		switch k {
		case "FREQ":
			f, err := parseFrequency(v)
			if err != nil {
				return Recurrence{}, err
			}
			r.Freq = f
			sawFreq = true
		case "INTERVAL":
			n, err := strconv.Atoi(v)
			if err != nil || n < 1 {
				return Recurrence{}, fmt.Errorf("invalid INTERVAL %q", v)
			}
			r.Interval = n
		case "COUNT":
			n, err := strconv.Atoi(v)
			if err != nil || n < 1 {
				return Recurrence{}, fmt.Errorf("invalid COUNT %q", v)
			}
			r.Count = n
		case "UNTIL":
			t, err := parseICSTime(v)
			if err != nil {
				return Recurrence{}, fmt.Errorf("invalid UNTIL %q: %w", v, err)
			}
			r.Until = t
		case "BYDAY":
			wds, err := parseByDay(v)
			if err != nil {
				return Recurrence{}, err
			}
			r.ByDay = wds
		}
		// Unrecognized keys are ignored — RFC 5545 allows extensions
		// and the issue scope is explicit about what's covered.
	}
	if !sawFreq {
		return Recurrence{}, fmt.Errorf("RRULE missing FREQ")
	}
	return r, nil
}

func parseFrequency(s string) (Frequency, error) {
	switch s {
	case "DAILY":
		return FreqDaily, nil
	case "WEEKLY":
		return FreqWeekly, nil
	case "MONTHLY":
		return FreqMonthly, nil
	default:
		return 0, fmt.Errorf("unsupported FREQ %q (only DAILY/WEEKLY/MONTHLY)", s)
	}
}

// byDayCodes maps the two-letter iCal weekday codes to time.Weekday.
var byDayCodes = map[string]time.Weekday{
	"SU": time.Sunday,
	"MO": time.Monday,
	"TU": time.Tuesday,
	"WE": time.Wednesday,
	"TH": time.Thursday,
	"FR": time.Friday,
	"SA": time.Saturday,
}

func parseByDay(s string) ([]time.Weekday, error) {
	var out []time.Weekday
	for code := range strings.SplitSeq(s, ",") {
		// Strip leading +/- and digits (e.g. "2MO" = second Monday).
		// Positional BYDAY is out of scope, so we just look at the
		// suffix — if it's not a known code, reject.
		suffix := code
		for len(suffix) > 0 && (suffix[0] == '+' || suffix[0] == '-' || (suffix[0] >= '0' && suffix[0] <= '9')) {
			suffix = suffix[1:]
		}
		wd, ok := byDayCodes[suffix]
		if !ok {
			return nil, fmt.Errorf("invalid BYDAY code %q", code)
		}
		out = append(out, wd)
	}
	return out, nil
}

// parseICSTime parses an RRULE UNTIL value or an EXDATE value, both of
// which use the same date/datetime forms as DTSTART (without the
// property-line wrapping parseDateTime needs).
func parseICSTime(v string) (time.Time, error) {
	if len(v) == 8 {
		return time.Parse("20060102", v)
	}
	if strings.HasSuffix(v, "Z") {
		return time.Parse("20060102T150405Z", v)
	}
	return time.Parse("20060102T150405", v)
}
