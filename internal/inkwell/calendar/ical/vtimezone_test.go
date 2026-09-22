package ical

import (
	"strings"
	"testing"
	"time"
)

// A feed that ships a non-IANA TZID almost always ships the VTIMEZONE
// that defines it — Outlook emits "Eastern Standard Time" with the
// offsets right there in the file. Resolving the name with
// time.LoadLocation alone fails and drops the event to UTC, so it
// renders hours off in the wrong day column.
func TestParse_VTimezoneFixedOffset(t *testing.T) {
	input := `BEGIN:VCALENDAR
BEGIN:VTIMEZONE
TZID:Eastern Standard Time
BEGIN:STANDARD
DTSTART:16011104T020000
TZOFFSETFROM:-0400
TZOFFSETTO:-0500
TZNAME:EST
END:STANDARD
END:VTIMEZONE
BEGIN:VEVENT
UID:windows-zone
DTSTART;TZID=Eastern Standard Time:20260119T090000
SUMMARY:Outlook Meeting
END:VEVENT
END:VCALENDAR
`
	events, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
	// 09:00 at -05:00 is 14:00Z.
	want := time.Date(2026, 1, 19, 14, 0, 0, 0, time.UTC)
	if !events[0].Start.Equal(want) {
		t.Errorf("Start = %v (%v UTC), want %v", events[0].Start, events[0].Start.UTC(), want)
	}
}

// windowsEastern is the VTIMEZONE Outlook ships for "Eastern Standard
// Time": both halves, with the US switchover rules.
const windowsEastern = `BEGIN:VTIMEZONE
TZID:Eastern Standard Time
BEGIN:STANDARD
DTSTART:16011104T020000
TZOFFSETFROM:-0400
TZOFFSETTO:-0500
TZNAME:EST
RRULE:FREQ=YEARLY;BYDAY=1SU;BYMONTH=11
END:STANDARD
BEGIN:DAYLIGHT
DTSTART:16010311T020000
TZOFFSETFROM:-0500
TZOFFSETTO:-0400
TZNAME:EDT
RRULE:FREQ=YEARLY;BYDAY=2SU;BYMONTH=3
END:DAYLIGHT
END:VTIMEZONE
`

// A fixed offset taken from whichever subcomponent came first would be
// right for half the year and an hour out for the other half, so the
// DAYLIGHT/STANDARD rules have to be honoured rather than skipped.
func TestParse_VTimezoneHonoursDST(t *testing.T) {
	event := func(uid, dt string) string {
		return "BEGIN:VEVENT\nUID:" + uid +
			"\nDTSTART;TZID=Eastern Standard Time:" + dt +
			"\nSUMMARY:" + uid + "\nEND:VEVENT\n"
	}
	input := "BEGIN:VCALENDAR\n" + windowsEastern +
		// 2026 switches to EDT on Sunday 8 March and back on 1 November.
		event("winter", "20260119T090000") + // EST, -05:00
		event("summer", "20260715T090000") + // EDT, -04:00
		event("day-before-spring-forward", "20260307T090000") + // EST
		event("day-after-spring-forward", "20260309T090000") + // EDT
		event("day-before-fall-back", "20261031T090000") + // EDT
		event("day-after-fall-back", "20261102T090000") + // EST
		"END:VCALENDAR\n"

	events, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(events) != 6 {
		t.Fatalf("got %d events, want 6", len(events))
	}

	// Every event is 09:00 local; the UTC hour is what reveals which
	// side of a switchover the parser put it on.
	wantUTCHour := map[string]int{
		"winter":                    14,
		"day-before-spring-forward": 14,
		"day-after-spring-forward":  13,
		"summer":                    13,
		"day-before-fall-back":      13,
		"day-after-fall-back":       14,
	}
	for _, e := range events {
		if e.Start.Hour() != 9 {
			t.Errorf("%s: local hour = %d, want 9", e.UID, e.Start.Hour())
		}
		if got, want := e.Start.UTC().Hour(), wantUTCHour[e.UID]; got != want {
			t.Errorf("%s: UTC hour = %d, want %d (%v)", e.UID, got, want, e.Start)
		}
	}
}

func TestParseUTCOffset(t *testing.T) {
	tests := []struct {
		label string
		in    string
		want  int
		ok    bool
	}{
		{"negative hours", "-0500", -5 * 3600, true},
		{"positive hours and minutes", "+0530", 5*3600 + 30*60, true},
		{"with seconds", "-000030", -30, true},
		{"UTC", "+0000", 0, true},
		{"no sign", "0500", 0, false},
		// Right length, wrong shape: the sign is mandatory.
		{"right length but unsigned", "05000", 0, false},
		{"too short", "-050", 0, false},
		{"too long", "-05000000", 0, false},
		{"non-numeric hours", "-ab00", 0, false},
		{"non-numeric minutes", "-05cd", 0, false},
		{"non-numeric seconds", "-0500xy", 0, false},
		// Atoi accepts a sign, so a nested one would otherwise slip a
		// negative component into an already-signed offset.
		{"signed field", "+-50300", 0, false},
		// Atoi would read each of these as a plausible offset.
		{"doubled sign", "++530", 0, false},
		{"sign inside a field", "+05+0", 0, false},
		{"minutes out of range", "+0099", 0, false},
		{"hours out of range", "+2400", 0, false},
		{"seconds out of range", "+010060", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			got, ok := parseUTCOffset(tt.in)
			if ok != tt.ok || got != tt.want {
				t.Errorf("parseUTCOffset(%q) = %d, %v; want %d, %v", tt.in, got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestParseOrdinalWeekday(t *testing.T) {
	tests := []struct {
		label   string
		in      string
		wantWd  time.Weekday
		wantNth int
		ok      bool
	}{
		{"second Sunday", "2SU", time.Sunday, 2, true},
		{"explicitly positive", "+1MO", time.Monday, 1, true},
		{"last Sunday", "-1SU", time.Sunday, -1, true},
		{"second to last", "-2FR", time.Friday, -2, true},
		{"no ordinal", "SU", 0, 0, false},
		{"zero ordinal", "0SU", 0, 0, false},
		{"unknown weekday", "2XX", 0, 0, false},
		{"ordinal only", "2", 0, 0, false},
		{"sign only", "-", 0, 0, false},
		{"empty", "", 0, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			wd, nth, ok := parseOrdinalWeekday(tt.in)
			if ok != tt.ok || wd != tt.wantWd || nth != tt.wantNth {
				t.Errorf("parseOrdinalWeekday(%q) = %v, %d, %v; want %v, %d, %v",
					tt.in, wd, nth, ok, tt.wantWd, tt.wantNth, tt.ok)
			}
		})
	}
}

// A transition rule this parser cannot read must leave the rule
// ruleless rather than half-applied: a wrong switchover date is worse
// than treating the zone as a fixed offset.
func TestApplyTransitionRule(t *testing.T) {
	tests := []struct {
		label string
		in    string
		want  bool // hasRule
	}{
		{"the usual yearly form", "FREQ=YEARLY;BYDAY=2SU;BYMONTH=3", true},
		{"unknown keys are ignored", "FREQ=YEARLY;WKST=SU;BYDAY=2SU;BYMONTH=3", true},
		{"not yearly", "FREQ=MONTHLY;BYDAY=2SU;BYMONTH=3", false},
		{"no BYMONTH", "FREQ=YEARLY;BYDAY=2SU", false},
		{"no BYDAY", "FREQ=YEARLY;BYMONTH=3", false},
		{"non-numeric BYMONTH", "FREQ=YEARLY;BYMONTH=March;BYDAY=2SU", false},
		{"BYMONTH out of range", "FREQ=YEARLY;BYMONTH=13;BYDAY=2SU", false},
		{"unreadable BYDAY", "FREQ=YEARLY;BYMONTH=3;BYDAY=SU", false},
		{"malformed part", "FREQ=YEARLY;BYMONTH=3;BYDAY=2SU;JUNK", true},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			var r tzRule
			applyTransitionRule(&r, tt.in)
			if r.hasRule != tt.want {
				t.Errorf("hasRule = %v, want %v", r.hasRule, tt.want)
			}
		})
	}
}

func TestNthWeekdayOf(t *testing.T) {
	tests := []struct {
		label string
		year  int
		month time.Month
		wd    time.Weekday
		nth   int
		want  string
	}{
		{"second Sunday in March 2026", 2026, time.March, time.Sunday, 2, "2026-03-08"},
		{"first Sunday in November 2026", 2026, time.November, time.Sunday, 1, "2026-11-01"},
		{"last Sunday in October 2026", 2026, time.October, time.Sunday, -1, "2026-10-25"},
		// December exercises the month+1 rollover into the next year.
		{"last Wednesday in December 2026", 2026, time.December, time.Wednesday, -1, "2026-12-30"},
		{"second to last Sunday in October 2026", 2026, time.October, time.Sunday, -2, "2026-10-18"},
		// The 1st falling on the target weekday must not skip a week.
		{"first Thursday in January 2026", 2026, time.January, time.Thursday, 1, "2026-01-01"},
		{"fifth Monday in March 2026", 2026, time.March, time.Monday, 5, "2026-03-30"},
		// October 2026 has only four Sundays. Producers write 5SU to
		// mean "the last one", and letting it run on would compute the
		// fall-back transition a week late.
		{"fifth Sunday in a month with four", 2026, time.October, time.Sunday, 5, "2026-10-25"},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			got := nthWeekdayOf(tt.year, tt.month, tt.wd, tt.nth, 2, 0, 0)
			if got.Format("2006-01-02") != tt.want {
				t.Errorf("got %v, want %s", got.Format("2006-01-02"), tt.want)
			}
			if h := got.Hour(); h != 2 {
				t.Errorf("hour = %d, want 2 (DTSTART's clock time)", h)
			}
		})
	}
}

// Southern-hemisphere DST wraps the year boundary — daylight starts in
// October and ends the following April — so the "between the two
// onsets" test that works for the northern hemisphere is inverted.
// Getting this backwards puts every Australian event an hour out for
// the whole year rather than half of it.
func TestParse_VTimezoneSouthernHemisphere(t *testing.T) {
	input := `BEGIN:VCALENDAR
BEGIN:VTIMEZONE
TZID:AUS Eastern Standard Time
BEGIN:STANDARD
DTSTART:16010405T030000
TZOFFSETFROM:+1100
TZOFFSETTO:+1000
TZNAME:AEST
RRULE:FREQ=YEARLY;BYDAY=1SU;BYMONTH=4
END:STANDARD
BEGIN:DAYLIGHT
DTSTART:16011004T020000
TZOFFSETFROM:+1000
TZOFFSETTO:+1100
TZNAME:AEDT
RRULE:FREQ=YEARLY;BYDAY=1SU;BYMONTH=10
END:DAYLIGHT
END:VTIMEZONE
BEGIN:VEVENT
UID:january
DTSTART;TZID=AUS Eastern Standard Time:20260119T090000
SUMMARY:Summer Down Under
END:VEVENT
BEGIN:VEVENT
UID:july
DTSTART;TZID=AUS Eastern Standard Time:20260715T090000
SUMMARY:Winter Down Under
END:VEVENT
END:VCALENDAR
`
	events, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("got %d events, want 2", len(events))
	}
	// January is daylight time (+11:00), so 09:00 local is 22:00Z the
	// previous day; July is standard (+10:00), so 23:00Z.
	wantUTC := map[string]time.Time{
		"january": time.Date(2026, 1, 18, 22, 0, 0, 0, time.UTC),
		"july":    time.Date(2026, 7, 14, 23, 0, 0, 0, time.UTC),
	}
	for _, e := range events {
		if !e.Start.Equal(wantUTC[e.UID]) {
			t.Errorf("%s: Start = %v (%v UTC), want %v", e.UID, e.Start, e.Start.UTC(), wantUTC[e.UID])
		}
	}
}

// TZNAME is optional. Without it the zone still has to resolve — the
// name is only a label — so it falls back to the component's TZID.
func TestParse_VTimezoneWithoutTZName(t *testing.T) {
	input := `BEGIN:VCALENDAR
BEGIN:VTIMEZONE
TZID:Custom Zone
BEGIN:STANDARD
DTSTART:16011104T020000
TZOFFSETFROM:+0000
TZOFFSETTO:+0900
END:STANDARD
END:VTIMEZONE
BEGIN:VEVENT
UID:nameless
DTSTART;TZID=Custom Zone:20260119T090000
SUMMARY:Nameless Zone
END:VEVENT
END:VCALENDAR
`
	events, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
	want := time.Date(2026, 1, 19, 0, 0, 0, 0, time.UTC)
	if !events[0].Start.Equal(want) {
		t.Errorf("Start = %v, want %v", events[0].Start.UTC(), want)
	}
	if name := events[0].Start.Format("MST"); name != "Custom Zone" {
		t.Errorf("zone name = %q, want the TZID as fallback", name)
	}
}

// RFC 5545 fixes no order between components, and the zone is needed
// while the VEVENT referencing it is being read — which is why zones
// are collected in their own pass first. A single-pass parser would
// resolve this feed to UTC.
func TestParse_VTimezoneDeclaredAfterEvent(t *testing.T) {
	input := "BEGIN:VCALENDAR\n" +
		"BEGIN:VEVENT\nUID:early-reference\n" +
		"DTSTART;TZID=Eastern Standard Time:20260119T090000\n" +
		"SUMMARY:Before The Zone\nEND:VEVENT\n" +
		windowsEastern +
		"END:VCALENDAR\n"

	events, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
	want := time.Date(2026, 1, 19, 14, 0, 0, 0, time.UTC)
	if !events[0].Start.Equal(want) {
		t.Errorf("Start = %v, want %v", events[0].Start.UTC(), want)
	}
}

// A VTIMEZONE that defines nothing usable must not be preferred over
// the existing UTC fallback — it should behave as if it were absent.
func TestParse_VTimezoneUnusable(t *testing.T) {
	tests := []struct {
		label string
		zone  string
	}{
		{
			"subcomponent without TZOFFSETTO",
			"BEGIN:VTIMEZONE\nTZID:Broken Zone\nBEGIN:STANDARD\nDTSTART:16011104T020000\nTZNAME:XXX\nEND:STANDARD\nEND:VTIMEZONE\n",
		},
		{
			"no TZID",
			"BEGIN:VTIMEZONE\nBEGIN:STANDARD\nDTSTART:16011104T020000\nTZOFFSETTO:-0500\nEND:STANDARD\nEND:VTIMEZONE\n",
		},
		{
			"no subcomponents at all",
			"BEGIN:VTIMEZONE\nTZID:Broken Zone\nEND:VTIMEZONE\n",
		},
		{
			"unparseable TZOFFSETTO",
			"BEGIN:VTIMEZONE\nTZID:Broken Zone\nBEGIN:STANDARD\nTZOFFSETTO:nonsense\nEND:STANDARD\nEND:VTIMEZONE\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			input := "BEGIN:VCALENDAR\n" + tt.zone +
				"BEGIN:VEVENT\nUID:x\nDTSTART;TZID=Broken Zone:20260119T090000\n" +
				"SUMMARY:Broken\nEND:VEVENT\nEND:VCALENDAR\n"
			events, err := Parse(strings.NewReader(input))
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if len(events) != 1 {
				t.Fatalf("got %d events, want 1", len(events))
			}
			// Falls back to UTC: the wall clock is kept as written.
			want := time.Date(2026, 1, 19, 9, 0, 0, 0, time.UTC)
			if !events[0].Start.Equal(want) {
				t.Errorf("Start = %v, want %v (UTC fallback)", events[0].Start, want)
			}
		})
	}
}

// When one half of the pair is missing or carries a rule this parser
// cannot read, the zone collapses to a single offset — and it has to be
// the standard one. A feed listing DAYLIGHT first would otherwise hand
// back the daylight offset all year, which is an hour wrong for the
// whole winter rather than for none of it.
func TestParse_VTimezoneIncompletePairPrefersStandard(t *testing.T) {
	input := `BEGIN:VCALENDAR
BEGIN:VTIMEZONE
TZID:Half Known Zone
BEGIN:DAYLIGHT
DTSTART:16010311T020000
TZOFFSETFROM:-0500
TZOFFSETTO:-0400
TZNAME:XDT
RRULE:FREQ=YEARLY;BYDAY=2SU;BYMONTH=3
END:DAYLIGHT
BEGIN:STANDARD
DTSTART:16011104T020000
TZOFFSETFROM:-0400
TZOFFSETTO:-0500
TZNAME:XST
RRULE:FREQ=YEARLY;BYDAY=SU;BYSETPOS=-1;BYMONTH=11
END:STANDARD
END:VTIMEZONE
BEGIN:VEVENT
UID:midsummer
DTSTART;TZID=Half Known Zone:20260715T090000
SUMMARY:Midsummer
END:VEVENT
END:VCALENDAR
`
	events, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
	// Standard (-05:00), so 09:00 local is 14:00Z. Taking the first
	// subcomponent instead would give -04:00 and 13:00Z.
	want := time.Date(2026, 7, 15, 14, 0, 0, 0, time.UTC)
	if !events[0].Start.Equal(want) {
		t.Errorf("Start = %v, want %v", events[0].Start.UTC(), want)
	}
}

// A zone defining only DAYLIGHT has no standard half to fall back on,
// so its single offset is all there is.
func TestParse_VTimezoneDaylightOnly(t *testing.T) {
	input := `BEGIN:VCALENDAR
BEGIN:VTIMEZONE
TZID:Daylight Only Zone
BEGIN:DAYLIGHT
DTSTART:16010311T020000
TZOFFSETFROM:-0500
TZOFFSETTO:-0400
TZNAME:XDT
END:DAYLIGHT
END:VTIMEZONE
BEGIN:VEVENT
UID:only
DTSTART;TZID=Daylight Only Zone:20260715T090000
SUMMARY:Only Daylight
END:VEVENT
END:VCALENDAR
`
	events, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
	want := time.Date(2026, 7, 15, 13, 0, 0, 0, time.UTC) // 09:00 at -04:00
	if !events[0].Start.Equal(want) {
		t.Errorf("Start = %v, want %v", events[0].Start.UTC(), want)
	}
}

// An EXDATE line carries every value under one TZID, so the zone is
// resolved once for the line. These pin both halves of that: a list of
// datetimes reuses the first resolution, and a date-only list never
// resolves a zone at all (a date needs none).
func TestParse_EXDATEZoneResolvedOncePerLine(t *testing.T) {
	tests := []struct {
		label     string
		exdate    string
		wantCount int
	}{
		{
			"several datetimes share one zone",
			"EXDATE;TZID=Eastern Standard Time:20260120T090000,20260121T090000,20260122T090000",
			2, // three of five weekdays excluded
		},
		{
			"date-only values need no zone",
			"EXDATE;VALUE=DATE;TZID=Eastern Standard Time:20260120,20260121",
			5, // dates never match the datetime instants, so none drop
		},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			input := "BEGIN:VCALENDAR\n" + windowsEastern +
				"BEGIN:VEVENT\nUID:recurring\n" +
				"DTSTART;TZID=Eastern Standard Time:20260119T090000\n" +
				"DTEND;TZID=Eastern Standard Time:20260119T100000\n" +
				"RRULE:FREQ=DAILY;COUNT=5\n" +
				tt.exdate + "\n" +
				"SUMMARY:Daily\nEND:VEVENT\nEND:VCALENDAR\n"

			events, err := Parse(strings.NewReader(input))
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if len(events) != 1 {
				t.Fatalf("got %d master events, want 1", len(events))
			}
			occ := Occurrences(events, utc(2026, 1, 1, 0, 0), utc(2026, 2, 1, 0, 0))
			if len(occ) != tt.wantCount {
				t.Errorf("got %d occurrences, want %d", len(occ), tt.wantCount)
			}
		})
	}
}
