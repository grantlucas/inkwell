package calendar

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// mockHTTPClient implements HTTPClient for testing.
type mockHTTPClient struct {
	responses map[string]*http.Response
	errors    map[string]error
	getCalls  int
}

func (m *mockHTTPClient) Get(url string) (*http.Response, error) {
	m.getCalls++
	if err, ok := m.errors[url]; ok {
		return nil, err
	}
	if resp, ok := m.responses[url]; ok {
		return resp, nil
	}
	return &http.Response{StatusCode: 404, Body: io.NopCloser(strings.NewReader(""))}, nil
}

func newMockResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

const testICS = `BEGIN:VCALENDAR
BEGIN:VEVENT
UID:evt-001@example.com
DTSTART:20260425T090000Z
DTEND:20260425T100000Z
SUMMARY:Standup
END:VEVENT
END:VCALENDAR
`

const testICS2 = `BEGIN:VCALENDAR
BEGIN:VEVENT
UID:evt-002@example.com
DTSTART:20260425T140000Z
DTEND:20260425T150000Z
SUMMARY:Planning
END:VEVENT
END:VCALENDAR
`

func TestHTTPSource_SingleFeed(t *testing.T) {
	client := &mockHTTPClient{
		responses: map[string]*http.Response{
			"https://example.com/cal.ics": newMockResponse(testICS),
		},
	}
	src := NewHTTPSource([]string{"https://example.com/cal.ics"}, client)

	start := time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)
	events, err := src.Events(start, end)
	if err != nil {
		t.Fatalf("Events: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
	if events[0].Summary != "Standup" {
		t.Errorf("Summary = %q, want %q", events[0].Summary, "Standup")
	}
}

func TestHTTPSource_MultipleFeeds(t *testing.T) {
	client := &mockHTTPClient{
		responses: map[string]*http.Response{
			"https://example.com/a.ics": newMockResponse(testICS),
			"https://example.com/b.ics": newMockResponse(testICS2),
		},
	}
	src := NewHTTPSource([]string{
		"https://example.com/a.ics",
		"https://example.com/b.ics",
	}, client)

	start := time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)
	events, err := src.Events(start, end)
	if err != nil {
		t.Fatalf("Events: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("got %d events, want 2", len(events))
	}
	// Should be sorted by start time.
	if events[0].Summary != "Standup" {
		t.Errorf("first event = %q, want %q", events[0].Summary, "Standup")
	}
	if events[1].Summary != "Planning" {
		t.Errorf("second event = %q, want %q", events[1].Summary, "Planning")
	}
}

func TestHTTPSource_DeduplicatesByUID(t *testing.T) {
	client := &mockHTTPClient{
		responses: map[string]*http.Response{
			"https://example.com/a.ics": newMockResponse(testICS),
			"https://example.com/b.ics": newMockResponse(testICS), // same event
		},
	}
	src := NewHTTPSource([]string{
		"https://example.com/a.ics",
		"https://example.com/b.ics",
	}, client)

	start := time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)
	events, err := src.Events(start, end)
	if err != nil {
		t.Fatalf("Events: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1 (deduplicated)", len(events))
	}
}

func TestHTTPSource_FiltersToRange(t *testing.T) {
	ics := `BEGIN:VCALENDAR
BEGIN:VEVENT
UID:in-range
DTSTART:20260425T090000Z
DTEND:20260425T100000Z
SUMMARY:In Range
END:VEVENT
BEGIN:VEVENT
UID:out-range
DTSTART:20260426T090000Z
DTEND:20260426T100000Z
SUMMARY:Out of Range
END:VEVENT
END:VCALENDAR
`
	client := &mockHTTPClient{
		responses: map[string]*http.Response{
			"https://example.com/cal.ics": newMockResponse(ics),
		},
	}
	src := NewHTTPSource([]string{"https://example.com/cal.ics"}, client)

	start := time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)
	events, err := src.Events(start, end)
	if err != nil {
		t.Fatalf("Events: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
	if events[0].Summary != "In Range" {
		t.Errorf("Summary = %q, want %q", events[0].Summary, "In Range")
	}
}

func TestHTTPSource_HTTPError(t *testing.T) {
	client := &mockHTTPClient{
		errors: map[string]error{
			"https://example.com/cal.ics": fmt.Errorf("network error"),
		},
	}
	src := NewHTTPSource([]string{"https://example.com/cal.ics"}, client)

	start := time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)
	_, err := src.Events(start, end)
	if err == nil {
		t.Fatal("expected error for HTTP failure")
	}
}

func TestHTTPSource_InvalidICS(t *testing.T) {
	badICS := `BEGIN:VCALENDAR
BEGIN:VEVENT
UID:bad
DTSTART:not-a-date
SUMMARY:Bad
END:VEVENT
END:VCALENDAR
`
	client := &mockHTTPClient{
		responses: map[string]*http.Response{
			"https://example.com/cal.ics": newMockResponse(badICS),
		},
	}
	src := NewHTTPSource([]string{"https://example.com/cal.ics"}, client)

	start := time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)
	_, err := src.Events(start, end)
	if err == nil {
		t.Fatal("expected error for invalid ICS content")
	}
}

func TestHTTPSource_Non200Status(t *testing.T) {
	client := &mockHTTPClient{
		responses: map[string]*http.Response{
			"https://example.com/cal.ics": {
				StatusCode: 500,
				Body:       io.NopCloser(strings.NewReader("")),
			},
		},
	}
	src := NewHTTPSource([]string{"https://example.com/cal.ics"}, client)

	start := time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)
	_, err := src.Events(start, end)
	if err == nil {
		t.Fatal("expected error for non-200 status")
	}
}
