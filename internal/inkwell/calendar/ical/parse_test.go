package ical

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestParse_BasicEvents(t *testing.T) {
	f, err := os.Open("testdata/basic.ics")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if cerr := f.Close(); cerr != nil {
			t.Errorf("close fixture: %v", cerr)
		}
	}()

	events, err := Parse(f)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if len(events) != 2 {
		t.Fatalf("got %d events, want 2", len(events))
	}

	e := events[0]
	if e.UID != "evt-001@example.com" {
		t.Errorf("UID = %q, want %q", e.UID, "evt-001@example.com")
	}
	if e.Summary != "Team Standup" {
		t.Errorf("Summary = %q, want %q", e.Summary, "Team Standup")
	}
	wantStart := time.Date(2026, 4, 25, 9, 0, 0, 0, time.UTC)
	if !e.Start.Equal(wantStart) {
		t.Errorf("Start = %v, want %v", e.Start, wantStart)
	}
	wantEnd := time.Date(2026, 4, 25, 10, 0, 0, 0, time.UTC)
	if !e.End.Equal(wantEnd) {
		t.Errorf("End = %v, want %v", e.End, wantEnd)
	}
	if e.Location != "Room 42" {
		t.Errorf("Location = %q, want %q", e.Location, "Room 42")
	}
	if e.AllDay {
		t.Error("AllDay = true, want false")
	}

	e2 := events[1]
	if e2.UID != "evt-002@example.com" {
		t.Errorf("event 2 UID = %q", e2.UID)
	}
	if e2.Summary != "Sprint Planning" {
		t.Errorf("event 2 Summary = %q", e2.Summary)
	}
}

func TestParse_AllDayEvent(t *testing.T) {
	f, err := os.Open("testdata/allday.ics")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if cerr := f.Close(); cerr != nil {
			t.Errorf("close fixture: %v", cerr)
		}
	}()

	events, err := Parse(f)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}

	e := events[0]
	if !e.AllDay {
		t.Error("AllDay = false, want true")
	}
	wantStart := time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC)
	if !e.Start.Equal(wantStart) {
		t.Errorf("Start = %v, want %v", e.Start, wantStart)
	}
	wantEnd := time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)
	if !e.End.Equal(wantEnd) {
		t.Errorf("End = %v, want %v", e.End, wantEnd)
	}
}

// All-day VEVENTs without DTEND default to DTSTART+1 day per RFC 5545
// §3.6.1. The previous behavior of End == Start would drop the event
// from filterEventsForDay (which checks End.After(start)), making the
// event invisible on the dashboard.
func TestParse_AllDayEventNoDTEND(t *testing.T) {
	f, err := os.Open("testdata/allday_no_dtend.ics")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if cerr := f.Close(); cerr != nil {
			t.Errorf("close fixture: %v", cerr)
		}
	}()

	events, err := Parse(f)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}

	e := events[0]
	if !e.AllDay {
		t.Error("AllDay = false, want true")
	}
	wantStart := time.Date(2026, 6, 6, 0, 0, 0, 0, time.UTC)
	if !e.Start.Equal(wantStart) {
		t.Errorf("Start = %v, want %v", e.Start, wantStart)
	}
	wantEnd := wantStart.AddDate(0, 0, 1)
	if !e.End.Equal(wantEnd) {
		t.Errorf("End = %v, want %v (Start+1 day)", e.End, wantEnd)
	}
}

func TestParse_FoldedLines(t *testing.T) {
	f, err := os.Open("testdata/folded.ics")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if cerr := f.Close(); cerr != nil {
			t.Errorf("close fixture: %v", cerr)
		}
	}()

	events, err := Parse(f)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}

	want := "A very long event summary that needs to befolded across multiple lines"
	if events[0].Summary != want {
		t.Errorf("Summary = %q, want %q", events[0].Summary, want)
	}
}

func TestParse_Duration(t *testing.T) {
	f, err := os.Open("testdata/duration.ics")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if cerr := f.Close(); cerr != nil {
			t.Errorf("close fixture: %v", cerr)
		}
	}()

	events, err := Parse(f)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}

	e := events[0]
	wantEnd := time.Date(2026, 4, 25, 10, 30, 0, 0, time.UTC)
	if !e.End.Equal(wantEnd) {
		t.Errorf("End = %v, want %v (Start + 1h30m)", e.End, wantEnd)
	}
}

func TestParse_NoEvents(t *testing.T) {
	input := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nEND:VCALENDAR\r\n"
	events, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("got %d events, want 0", len(events))
	}
}

func TestParse_SortsByStart(t *testing.T) {
	input := `BEGIN:VCALENDAR
BEGIN:VEVENT
UID:late
DTSTART:20260425T140000Z
DTEND:20260425T150000Z
SUMMARY:Late
END:VEVENT
BEGIN:VEVENT
UID:early
DTSTART:20260425T090000Z
DTEND:20260425T100000Z
SUMMARY:Early
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
	if events[0].Summary != "Early" {
		t.Errorf("first event = %q, want %q", events[0].Summary, "Early")
	}
	if events[1].Summary != "Late" {
		t.Errorf("second event = %q, want %q", events[1].Summary, "Late")
	}
}

func TestParse_SkipsEventWithoutStart(t *testing.T) {
	input := `BEGIN:VCALENDAR
BEGIN:VEVENT
UID:no-start
SUMMARY:Missing Start
END:VEVENT
BEGIN:VEVENT
UID:good
DTSTART:20260425T090000Z
DTEND:20260425T100000Z
SUMMARY:Good Event
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
	if events[0].Summary != "Good Event" {
		t.Errorf("Summary = %q, want %q", events[0].Summary, "Good Event")
	}
}

func TestParse_InvalidDTSTART(t *testing.T) {
	input := `BEGIN:VCALENDAR
BEGIN:VEVENT
UID:bad
DTSTART:not-a-date
SUMMARY:Bad
END:VEVENT
END:VCALENDAR
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for invalid DTSTART")
	}
	if !strings.Contains(err.Error(), "DTSTART") {
		t.Errorf("error = %q, want mention of DTSTART", err.Error())
	}
}

func TestParse_InvalidDTEND(t *testing.T) {
	input := `BEGIN:VCALENDAR
BEGIN:VEVENT
UID:bad
DTSTART:20260425T090000Z
DTEND:not-a-date
SUMMARY:Bad
END:VEVENT
END:VCALENDAR
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for invalid DTEND")
	}
	if !strings.Contains(err.Error(), "DTEND") {
		t.Errorf("error = %q, want mention of DTEND", err.Error())
	}
}

func TestParse_InvalidDuration(t *testing.T) {
	input := `BEGIN:VCALENDAR
BEGIN:VEVENT
UID:bad
DTSTART:20260425T090000Z
DURATION:not-a-duration
SUMMARY:Bad
END:VEVENT
END:VCALENDAR
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for invalid DURATION")
	}
}

func TestParse_LocalDateTime(t *testing.T) {
	input := `BEGIN:VCALENDAR
BEGIN:VEVENT
UID:local
DTSTART:20260425T090000
DTEND:20260425T100000
SUMMARY:Local Time Event
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
	wantStart := time.Date(2026, 4, 25, 9, 0, 0, 0, time.UTC)
	if !events[0].Start.Equal(wantStart) {
		t.Errorf("Start = %v, want %v", events[0].Start, wantStart)
	}
}

// Verify that whole-day and whole-week DURATION values produce the
// correct End time. The old "End < 0001-01-02" sentinel hack only
// recovered durations under 24 hours, so P1D and P1W came out wrong.
func TestParse_DurationDaysAndWeeks(t *testing.T) {
	cases := []struct {
		name    string
		dur     string
		wantEnd time.Time
	}{
		{
			name:    "one day",
			dur:     "P1D",
			wantEnd: time.Date(2026, 4, 26, 9, 0, 0, 0, time.UTC),
		},
		{
			name:    "one week",
			dur:     "P1W",
			wantEnd: time.Date(2026, 5, 2, 9, 0, 0, 0, time.UTC),
		},
		{
			name:    "mixed week+day",
			dur:     "P1W2D",
			wantEnd: time.Date(2026, 5, 4, 9, 0, 0, 0, time.UTC),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := "BEGIN:VCALENDAR\r\nBEGIN:VEVENT\r\nUID:dur\r\nDTSTART:20260425T090000Z\r\nDURATION:" + tc.dur + "\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
			events, err := Parse(strings.NewReader(input))
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if len(events) != 1 {
				t.Fatalf("got %d events, want 1", len(events))
			}
			if !events[0].End.Equal(tc.wantEnd) {
				t.Errorf("End = %v, want %v", events[0].End, tc.wantEnd)
			}
		})
	}
}

func TestParseDuration_Errors(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"empty", ""},
		{"no P prefix", "T1H"},
		{"trailing number", "PT1"},
		{"unknown unit", "P1X"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseDuration(tt.input)
			if err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestParseDuration_WeeksAndDays(t *testing.T) {
	d, err := parseDuration("P1W2D")
	if err != nil {
		t.Fatal(err)
	}
	want := 9 * 24 * time.Hour
	if d != want {
		t.Errorf("got %v, want %v", d, want)
	}
}

func TestParseDuration_Seconds(t *testing.T) {
	d, err := parseDuration("PT30S")
	if err != nil {
		t.Fatal(err)
	}
	if d != 30*time.Second {
		t.Errorf("got %v, want 30s", d)
	}
}

func TestParseDateTime_MissingColon(t *testing.T) {
	_, _, err := parseDateTime("DTSTART-NO-COLON", nil)
	if err == nil {
		t.Fatal("expected error for missing colon")
	}
}

func TestParseDateTime_InvalidDate(t *testing.T) {
	_, _, err := parseDateTime("DTSTART;VALUE=DATE:notadate", nil)
	if err == nil {
		t.Fatal("expected error for invalid date")
	}
}

func TestParseDateTime_InvalidUTC(t *testing.T) {
	_, _, err := parseDateTime("DTSTART:notadateZ", nil)
	if err == nil {
		t.Fatal("expected error for invalid UTC datetime")
	}
}

func TestParseDateTime_TZID(t *testing.T) {
	dt, allDay, err := parseDateTime("DTSTART;TZID=America/New_York:20260429T190000", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if allDay {
		t.Error("expected non-allday")
	}
	loc, _ := time.LoadLocation("America/New_York")
	want := time.Date(2026, 4, 29, 19, 0, 0, 0, loc)
	if !dt.Equal(want) {
		t.Errorf("got %v, want %v", dt, want)
	}
}

// A TZID must not rescue an unparseable value. The zone resolves fine
// here — America/New_York is IANA — so the error comes out of the
// TZID branch's own ParseInLocation rather than the no-TZID parse
// below it. Both wrap with the same message, so what this pins is that
// the value is rejected at all, instead of quietly becoming a zero
// time in the named zone.
func TestParseDateTime_TZID_InvalidValue(t *testing.T) {
	_, _, err := parseDateTime("DTSTART;TZID=America/New_York:notadatetime", nil)
	if err == nil {
		t.Fatal("expected error for invalid datetime under TZID")
	}
	if !strings.Contains(err.Error(), "invalid datetime") {
		t.Errorf("error = %q, want it to mention 'invalid datetime'", err.Error())
	}
}

func TestParseDateTime_TZID_Unknown(t *testing.T) {
	dt, _, err := parseDateTime("DTSTART;TZID=Fake/Zone:20260429T190000", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Date(2026, 4, 29, 19, 0, 0, 0, time.UTC)
	if !dt.Equal(want) {
		t.Errorf("got %v, want %v (should fall back to UTC)", dt, want)
	}
}

func TestExtractTZID(t *testing.T) {
	tests := []struct {
		label  string
		params string
		want   string // "" means nil location (fall back to UTC)
	}{
		{"bare IANA name", "DTSTART;TZID=America/Toronto", "America/Toronto"},
		{"no TZID parameter", "DTSTART;VALUE=DATE-TIME", ""},
		// RFC 5545 3.1 permits a quoted param value. Handing the
		// quotes to LoadLocation fails the lookup, so the event
		// silently renders hours off in UTC.
		{"quoted IANA name", `DTSTART;TZID="America/Toronto"`, "America/Toronto"},
		{"quoted, after another param", `DTSTART;VALUE=DATE-TIME;TZID="America/Toronto"`, "America/Toronto"},
		{"unknown name still falls back", "DTSTART;TZID=Fake/Zone", ""},
		// A quoted value may carry its own ';' (RFC 5545 3.1). Splitting
		// the parameter list on every semicolon tears such a value in
		// half, and the fragments are then scanned for a TZID= prefix
		// like any other segment — so a decoy inside the quotes wins
		// over the real parameter that follows it.
		{
			"semicolon inside a quoted value does not split it",
			`DTSTART;X-LIC-LOCATION="Foo;TZID=Fake/Zone";TZID=America/Toronto`,
			"America/Toronto",
		},
	}

	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			loc := extractTZID(tt.params, nil)
			switch {
			case tt.want == "":
				if loc != nil {
					t.Errorf("got %v, want nil", loc)
				}
			case loc == nil:
				t.Fatalf("got nil, want %s", tt.want)
			case loc.String() != tt.want:
				t.Errorf("got %v, want %s", loc, tt.want)
			}
		})
	}
}

// A param value only *needs* quoting when it contains ':', ';' or ','
// (RFC 5545 3.1). Cutting the property at its first colon therefore
// lands inside the quoted value, leaving a value of
// `Eastern":20260919T104500` — which fails to parse and takes the
// whole feed down with it, rather than degrading to UTC.
func TestParse_QuotedTZIDContainingColon(t *testing.T) {
	input := `BEGIN:VCALENDAR
BEGIN:VEVENT
UID:quoted-colon
DTSTART;TZID="Customized Time Zone: Eastern":20260919T104500
SUMMARY:Quoted Zone
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
	// The zone name is not IANA, so it falls back to UTC — but the
	// wall-clock time must survive intact.
	want := time.Date(2026, 9, 19, 10, 45, 0, 0, time.UTC)
	if !events[0].Start.Equal(want) {
		t.Errorf("Start = %v, want %v", events[0].Start, want)
	}
}

func TestParse_EventWithoutEnd(t *testing.T) {
	input := `BEGIN:VCALENDAR
BEGIN:VEVENT
UID:no-end
DTSTART:20260425T090000Z
SUMMARY:No End Time
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
	if !events[0].End.Equal(events[0].Start) {
		t.Errorf("End = %v, want Start (%v)", events[0].End, events[0].Start)
	}
}

func TestSplitProperty_NoColon(t *testing.T) {
	name, value := splitProperty("NOCOLON")
	if name != "NOCOLON" {
		t.Errorf("name = %q, want %q", name, "NOCOLON")
	}
	if value != "" {
		t.Errorf("value = %q, want empty", value)
	}
}

// Verify Parse returns ical.Event values.
func TestParse_ReturnsEvents(t *testing.T) {
	input := `BEGIN:VCALENDAR
BEGIN:VEVENT
UID:test
DTSTART:20260425T090000Z
DTEND:20260425T100000Z
SUMMARY:Test
END:VEVENT
END:VCALENDAR
`
	events, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	var _ []Event = events
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
}

// eventWithStatus builds a single-VEVENT feed carrying the given STATUS
// line (empty status omits the property entirely).
func eventWithStatus(status string) string {
	statusLine := ""
	if status != "" {
		statusLine = "STATUS:" + status + "\r\n"
	}
	return "BEGIN:VCALENDAR\r\n" +
		"BEGIN:VEVENT\r\n" +
		"UID:evt@example.com\r\n" +
		"SUMMARY:Practice\r\n" +
		"DTSTART:20260919T130000Z\r\n" +
		"DTEND:20260919T140000Z\r\n" +
		statusLine +
		"END:VEVENT\r\n" +
		"END:VCALENDAR\r\n"
}

func TestParse_StatusFiltering(t *testing.T) {
	cases := []struct {
		label  string
		status string
		want   int
	}{
		{label: "cancelled event is dropped", status: "CANCELLED", want: 0},
		{label: "lowercase cancelled is dropped", status: "cancelled", want: 0},
		{label: "confirmed event is kept", status: "CONFIRMED", want: 1},
		{label: "tentative event is kept", status: "TENTATIVE", want: 1},
		{label: "missing status is kept", status: "", want: 1},
	}

	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			events, err := Parse(strings.NewReader(eventWithStatus(tc.status)))
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if len(events) != tc.want {
				t.Fatalf("got %d events, want %d", len(events), tc.want)
			}
		})
	}
}

func TestParse_CancelledEventDoesNotSuppressFollowingEvent(t *testing.T) {
	const feed = "BEGIN:VCALENDAR\r\n" +
		"BEGIN:VEVENT\r\n" +
		"UID:cancelled@example.com\r\n" +
		"SUMMARY:Cancelled Practice\r\n" +
		"DTSTART:20260919T130000Z\r\n" +
		"DTEND:20260919T140000Z\r\n" +
		"STATUS:CANCELLED\r\n" +
		"END:VEVENT\r\n" +
		"BEGIN:VEVENT\r\n" +
		"UID:live@example.com\r\n" +
		"SUMMARY:Game\r\n" +
		"DTSTART:20260919T150000Z\r\n" +
		"DTEND:20260919T160000Z\r\n" +
		"END:VEVENT\r\n" +
		"END:VCALENDAR\r\n"

	events, err := Parse(strings.NewReader(feed))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
	if events[0].UID != "live@example.com" {
		t.Errorf("UID = %q, want the non-cancelled event", events[0].UID)
	}
}

// feedWithSummary builds a single-VEVENT feed whose SUMMARY carries the
// given raw (still-escaped) property value.
func feedWithSummary(summary string) string {
	return "BEGIN:VCALENDAR\r\n" +
		"BEGIN:VEVENT\r\n" +
		"UID:evt@example.com\r\n" +
		"SUMMARY:" + summary + "\r\n" +
		"DTSTART:20260919T130000Z\r\n" +
		"DTEND:20260919T140000Z\r\n" +
		"END:VEVENT\r\n" +
		"END:VCALENDAR\r\n"
}

func TestParse_UnescapesTextValues(t *testing.T) {
	cases := []struct {
		label string
		raw   string
		want  string
	}{
		{label: "escaped newline", raw: `Jane Doe\nRavens`, want: "Jane Doe\nRavens"},
		{label: "uppercase escaped newline", raw: `Line\NBreak`, want: "Line\nBreak"},
		{label: "escaped comma", raw: `Hamilton\, ON`, want: "Hamilton, ON"},
		{label: "escaped semicolon", raw: `Practice\; Game`, want: "Practice; Game"},
		{label: "escaped backslash", raw: `Back\\slash`, want: `Back\slash`},
		{label: "escaped backslash before n is not a newline", raw: `Back\\nope`, want: `Back\nope`},
		{label: "unknown escape is preserved", raw: `Odd\qEscape`, want: `Odd\qEscape`},
		{label: "trailing backslash is preserved", raw: `Trailing\`, want: `Trailing\`},
		{label: "plain text is unchanged", raw: `Team Standup`, want: `Team Standup`},
	}

	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			events, err := Parse(strings.NewReader(feedWithSummary(tc.raw)))
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if len(events) != 1 {
				t.Fatalf("got %d events, want 1", len(events))
			}
			if events[0].Summary != tc.want {
				t.Errorf("Summary = %q, want %q", events[0].Summary, tc.want)
			}
		})
	}
}

func TestParse_UnescapesLocation(t *testing.T) {
	feed := "BEGIN:VCALENDAR\r\n" +
		"BEGIN:VEVENT\r\n" +
		"UID:evt@example.com\r\n" +
		"SUMMARY:Practice\r\n" +
		`LOCATION:70 HEMPSTEAD DR\, HAMILTON` + "\r\n" +
		"DTSTART:20260919T130000Z\r\n" +
		"END:VEVENT\r\n" +
		"END:VCALENDAR\r\n"

	events, err := Parse(strings.NewReader(feed))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got, want := events[0].Location, "70 HEMPSTEAD DR, HAMILTON"; got != want {
		t.Errorf("Location = %q, want %q", got, want)
	}
}

// An unbalanced quote must not be worse than no quote handling at all.
// Scanning for a colon "outside quotes" never finds one when a stray
// DQUOTE leaves the scanner quoted to end of line, and splitProperty
// then hands back the whole line as the property name — which matches
// no case in Parse's switch, so DTSTART never lands and the event is
// dropped at END:VEVENT with no error and no log line. Degrading to
// UTC is recoverable; vanishing silently is not.
//
// Here the naive fallback cut plus the quote trim actually recovers the
// intended zone, so the event lands at the right instant rather than
// merely surviving.
func TestParse_UnbalancedQuoteInParams(t *testing.T) {
	input := `BEGIN:VCALENDAR
BEGIN:VEVENT
UID:unbalanced
DTSTART;TZID=America/Toronto":20260919T104500
SUMMARY:Stray Quote
END:VEVENT
END:VCALENDAR
`
	events, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1 (the event must not vanish)", len(events))
	}
	toronto, err := time.LoadLocation("America/Toronto")
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 9, 19, 10, 45, 0, 0, toronto)
	if !events[0].Start.Equal(want) {
		t.Errorf("Start = %v, want %v", events[0].Start, want)
	}
}
