package ical

import (
	"strconv"
	"strings"
	"time"
)

// A feed that uses a non-IANA TZID — Outlook emits Windows zone names
// like "Eastern Standard Time" — almost always ships the VTIMEZONE that
// defines it, with the offsets and switchover rules right there in the
// file. time.LoadLocation only understands IANA names, so without
// reading VTIMEZONE those feeds fall back to UTC and render hours off.
//
// A component that defines both halves and says when they swap is
// compiled into a real DST-aware *time.Location (see tzif.go), because
// a recurrence has to step in one — a fixed offset picked per instant
// gets single events right and drifts recurrences by an hour after the
// next switchover. Components that define less than that fall back to
// a single fixed offset, which is all they gave us.

// tzRule is one STANDARD or DAYLIGHT subcomponent: the offset it
// switches to, and the yearly rule saying when it takes effect.
type tzRule struct {
	name      string // TZNAME (e.g. "EST"); cosmetic, names the zone
	offset    int    // TZOFFSETTO, seconds east of UTC
	hasOffset bool
	// from is TZOFFSETFROM: the offset in force *before* this rule
	// takes effect. A switchover is written in the outgoing zone's
	// wall time, so this is what converts it to a UTC instant.
	from     int
	daylight bool // from BEGIN:DAYLIGHT rather than BEGIN:STANDARD

	// The switchover, from DTSTART's clock time plus the RRULE. A
	// subcomponent with no usable RRULE defines a zone that never
	// switches, which is legal and common in hand-rolled feeds.
	hasRule bool
	month   time.Month
	weekday time.Weekday
	nth     int // 1..5, or negative counting back from the last
	hour    int
	min     int
	sec     int
}

// vtimezone is a parsed VTIMEZONE component: a TZID and its rules.
type vtimezone struct {
	id    string
	rules []tzRule

	// loc is the DST-aware Location compiled from the rules, when they
	// describe a switching zone. Built once when the component closes,
	// because a recurrence has to step in a real Location — a fixed
	// offset chosen per instant gets single events right and drifts
	// recurrences by an hour after the next switchover (issue #105).
	loc *time.Location
}

// parseVTimezones collects every VTIMEZONE in the stream into a name →
// component map. It runs as its own pass so a VTIMEZONE declared after
// the VEVENT that references it still resolves; RFC 5545 does not
// require any particular order.
func parseVTimezones(lines []string) map[string]*vtimezone {
	var zones map[string]*vtimezone
	var cur *vtimezone
	var rule *tzRule

	for _, line := range lines {
		switch {
		case line == "BEGIN:VTIMEZONE":
			cur = &vtimezone{}
			rule = nil
		case cur == nil:
			// Outside a VTIMEZONE; VEVENT parsing handles these.
		case line == "END:VTIMEZONE":
			// A component with no id or no usable rule tells us
			// nothing, so it is not worth keeping — resolution falls
			// through to the UTC default and its log line.
			if cur.id != "" && len(cur.rules) > 0 {
				if loc, ok := cur.buildLocation(); ok {
					cur.loc = loc
				}
				if zones == nil {
					zones = make(map[string]*vtimezone)
				}
				zones[cur.id] = cur
			}
			cur = nil
			rule = nil
		case line == "BEGIN:STANDARD", line == "BEGIN:DAYLIGHT":
			rule = &tzRule{daylight: line == "BEGIN:DAYLIGHT"}
		case line == "END:STANDARD", line == "END:DAYLIGHT":
			// TZOFFSETTO is the whole point of the subcomponent; one
			// without it cannot place an event.
			if rule != nil && rule.hasOffset {
				cur.rules = append(cur.rules, *rule)
			}
			rule = nil
		default:
			applyTZProperty(cur, rule, line)
		}
	}
	return zones
}

// applyTZProperty folds one property line into the VTIMEZONE being
// parsed, or into its current STANDARD/DAYLIGHT subcomponent.
func applyTZProperty(zone *vtimezone, rule *tzRule, line string) {
	name, value := splitProperty(line)
	if rule == nil {
		if name == "TZID" {
			zone.id = value
		}
		return
	}
	switch name {
	case "TZOFFSETTO":
		if off, ok := parseUTCOffset(value); ok {
			rule.offset = off
			rule.hasOffset = true
		}
	case "TZOFFSETFROM":
		if off, ok := parseUTCOffset(value); ok {
			rule.from = off
		}
	case "TZNAME":
		rule.name = value
	case "DTSTART":
		// Only the clock time matters: the date is a rule anchor
		// (typically year 1601) that the RRULE supersedes.
		if t, err := time.Parse("20060102T150405", value); err == nil {
			rule.hour, rule.min, rule.sec = t.Clock()
		}
	case "RRULE":
		applyTransitionRule(rule, value)
	}
}

// applyTransitionRule reads the FREQ=YEARLY;BYMONTH=n;BYDAY=nDD form
// every real feed uses for a switchover. Anything else leaves hasRule
// false, and the zone is then treated as a fixed offset rather than
// guessed at — a wrong transition date is worse than none.
func applyTransitionRule(rule *tzRule, value string) {
	var (
		yearly bool
		month  time.Month
		wd     time.Weekday
		nth    int
	)
	for part := range strings.SplitSeq(value, ";") {
		k, v, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		switch k {
		case "FREQ":
			yearly = v == "YEARLY"
		case "BYMONTH":
			n, err := strconv.Atoi(v)
			if err != nil || n < 1 || n > 12 {
				return
			}
			month = time.Month(n)
		case "BYDAY":
			w, n, ok := parseOrdinalWeekday(v)
			if !ok {
				return
			}
			wd, nth = w, n
		}
	}
	if !yearly || month == 0 || nth == 0 {
		return
	}
	rule.month, rule.weekday, rule.nth, rule.hasRule = month, wd, nth, true
}

// parseOrdinalWeekday reads a positional BYDAY code such as "2SU" (the
// second Sunday) or "-1SU" (the last). A bare "SU" has no ordinal and
// so cannot name a single transition date; it is rejected.
func parseOrdinalWeekday(s string) (time.Weekday, int, bool) {
	i, sign := 0, 1
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		if s[i] == '-' {
			sign = -1
		}
		i++
	}
	start := i
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i == start {
		return 0, 0, false
	}
	n, err := strconv.Atoi(s[start:i])
	if err != nil || n == 0 {
		return 0, 0, false
	}
	wd, ok := byDayCodes[s[i:]]
	if !ok {
		return 0, 0, false
	}
	return wd, sign * n, true
}

// parseUTCOffset parses a UTC-offset value (RFC 5545 3.3.14): a sign
// then HHMM, optionally with seconds.
func parseUTCOffset(s string) (int, bool) {
	if len(s) != 5 && len(s) != 7 {
		return 0, false
	}
	sign := 1
	switch s[0] {
	case '+':
	case '-':
		sign = -1
	default:
		return 0, false
	}
	var total int
	for _, field := range []struct {
		from, to, scale, limit int
	}{{1, 3, 3600, 24}, {3, 5, 60, 60}, {5, 7, 1, 60}} {
		if field.to > len(s) {
			break
		}
		// Parsed digit by digit rather than with Atoi: Atoi accepts a
		// sign inside the field, so "++530" and "+05+0" would come back
		// as plausible offsets rather than as rejects.
		n, ok := twoDigits(s[field.from:field.to])
		if !ok || n >= field.limit {
			return 0, false
		}
		total += n * field.scale
	}
	return sign * total, true
}

// twoDigits reads exactly two ASCII digits.
func twoDigits(s string) (int, bool) {
	if len(s) != 2 || s[0] < '0' || s[0] > '9' || s[1] < '0' || s[1] > '9' {
		return 0, false
	}
	return int(s[0]-'0')*10 + int(s[1]-'0'), true
}

// locationFor returns the zone this component defines.
//
// Normally that is the DST-aware Location compiled from the rules. The
// fallback is for components that could not be compiled — one half
// missing, or a switchover rule in a form this parser cannot read —
// and those have only one offset to give, so it does not depend on the
// instant. Standard time is preferred: it is what the zone is for most
// of the year, and a feed that lists DAYLIGHT first would otherwise
// run on the daylight offset all winter.
func (v *vtimezone) locationFor() *time.Location {
	if v.loc != nil {
		return v.loc
	}
	if std, dst := v.pair(); std != nil {
		return v.fixedZone(*std)
	} else {
		return v.fixedZone(*dst)
	}
}

// pair returns the last STANDARD and DAYLIGHT rule in the component.
// Feeds may repeat a subcomponent per era; the most recent definition
// is the one that applies to a dashboard's near-future window.
func (v *vtimezone) pair() (std, dst *tzRule) {
	for i := range v.rules {
		if v.rules[i].daylight {
			dst = &v.rules[i]
		} else {
			std = &v.rules[i]
		}
	}
	return std, dst
}

// fixedZone builds the location for one rule, falling back to the
// component's TZID when the subcomponent carries no TZNAME.
func (v *vtimezone) fixedZone(r tzRule) *time.Location {
	name := r.name
	if name == "" {
		name = v.id
	}
	return time.FixedZone(name, r.offset)
}

// onsetIn returns the rule's switchover for the given year, as a naive
// wall time comparable with the value being resolved.
func (r tzRule) onsetIn(year int) time.Time {
	return nthWeekdayOf(year, r.month, r.weekday, r.nth, r.hour, r.min, r.sec)
}

// nthWeekdayOf returns the nth weekday of a month — the 2nd Sunday in
// March, the last Sunday in October. A negative nth counts back from
// the end of the month.
func nthWeekdayOf(year int, m time.Month, wd time.Weekday, nth, hour, min, sec int) time.Time {
	if nth < 0 {
		// time.Date normalises month 13 into January of the next year,
		// so December needs no special case.
		last := time.Date(year, m+1, 1, hour, min, sec, 0, time.UTC).AddDate(0, 0, -1)
		back := int(last.Weekday()-wd+7) % 7
		return last.AddDate(0, 0, -back+7*(nth+1))
	}
	first := time.Date(year, m, 1, hour, min, sec, 0, time.UTC)
	fwd := int(wd-first.Weekday()+7) % 7
	day := first.AddDate(0, 0, fwd+7*(nth-1))
	// Producers emit BYDAY=5SU for "the last Sunday", and a month with
	// only four of them would otherwise push the transition into the
	// next month — computing the fall-back a week late and giving every
	// event in that week the wrong offset. Clamping matches both that
	// intent and RFC 5545, which yields no occurrence at all.
	if day.Month() != m {
		day = day.AddDate(0, 0, -7)
	}
	return day
}
