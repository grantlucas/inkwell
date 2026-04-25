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
	defer f.Close()

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
	defer f.Close()

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

func TestParse_FoldedLines(t *testing.T) {
	f, err := os.Open("testdata/folded.ics")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

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
	defer f.Close()

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
	_, _, err := parseDateTime("DTSTART-NO-COLON")
	if err == nil {
		t.Fatal("expected error for missing colon")
	}
}

func TestParseDateTime_InvalidDate(t *testing.T) {
	_, _, err := parseDateTime("DTSTART;VALUE=DATE:notadate")
	if err == nil {
		t.Fatal("expected error for invalid date")
	}
}

func TestParseDateTime_InvalidUTC(t *testing.T) {
	_, _, err := parseDateTime("DTSTART:notadateZ")
	if err == nil {
		t.Fatal("expected error for invalid UTC datetime")
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
