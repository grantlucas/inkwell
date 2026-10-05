package calendar

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

// mockHTTPClient implements HTTPClient for testing.
type mockHTTPClient struct {
	responses map[string]*http.Response
	errors    map[string]error
	getCalls  int
}

func (m *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	m.getCalls++
	url := req.URL.String()
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
	src := NewHTTPSource(FeedsFromURLs([]string{"https://example.com/cal.ics"}), client)

	events, err := src.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
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
	src := NewHTTPSource(FeedsFromURLs([]string{
		"https://example.com/a.ics",
		"https://example.com/b.ics",
	}), client)

	events, err := src.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
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
	src := NewHTTPSource(FeedsFromURLs([]string{
		"https://example.com/a.ics",
		"https://example.com/b.ics",
	}), client)

	events, err := src.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1 (deduplicated)", len(events))
	}
}

func TestHTTPSource_HTTPError(t *testing.T) {
	client := &mockHTTPClient{
		errors: map[string]error{
			"https://example.com/cal.ics": fmt.Errorf("network error"),
		},
	}
	src := NewHTTPSource(FeedsFromURLs([]string{"https://example.com/cal.ics"}), client)

	_, err := src.Fetch(context.Background())
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
	src := NewHTTPSource(FeedsFromURLs([]string{"https://example.com/cal.ics"}), client)

	_, err := src.Fetch(context.Background())
	if err == nil {
		t.Fatal("expected error for invalid ICS content")
	}
}

// errCloser wraps a Reader so its Close() returns a specific error.
// This pins the per-iteration body-close error wrapping in fetchFeed.
type errCloser struct {
	io.Reader
}

func (errCloser) Close() error { return fmt.Errorf("simulated close failure") }

func TestHTTPSource_BodyCloseErrorSurfaced(t *testing.T) {
	client := &mockHTTPClient{
		responses: map[string]*http.Response{
			"https://example.com/cal.ics": {
				StatusCode: 200,
				Body:       errCloser{Reader: strings.NewReader(testICS)},
			},
		},
	}
	src := NewHTTPSource(FeedsFromURLs([]string{"https://example.com/cal.ics"}), client)

	_, err := src.Fetch(context.Background())
	if err == nil {
		t.Fatal("expected error from failing Body.Close")
	}
	if !strings.Contains(err.Error(), "simulated close failure") {
		t.Errorf("error = %q, want it to wrap simulated close failure", err.Error())
	}
}

// Build-request error path is unreachable in production because the
// feed URL is opaque to the source — http.NewRequestWithContext only
// rejects URLs with invalid method/control characters, neither of
// which we generate. Exercise it via the newRequestWithContext hook
// for coverage parity with weather/openmeteo.
func TestHTTPSource_BuildRequestError(t *testing.T) {
	orig := newRequestWithContext
	newRequestWithContext = func(_ context.Context, _, _ string, _ io.Reader) (*http.Request, error) {
		return nil, fmt.Errorf("synthetic build-request error")
	}
	defer func() { newRequestWithContext = orig }()

	client := &mockHTTPClient{}
	src := NewHTTPSource(FeedsFromURLs([]string{"https://example.com/cal.ics"}), client)

	_, err := src.Fetch(context.Background())
	if err == nil {
		t.Fatal("expected build-request error")
	}
	if !strings.Contains(err.Error(), "build request") {
		t.Errorf("error = %q, want it to mention 'build request'", err.Error())
	}
}

// Ctx cancellation flows through to the HTTP client now that the
// Source carries it. A client that returns req.Context().Err()
// simulates a transport honoring the deadline.
func TestHTTPSource_HonorsContextCancellation(t *testing.T) {
	client := &ctxAwareCalClient{}
	src := NewHTTPSource(FeedsFromURLs([]string{"https://example.com/cal.ics"}), client)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := src.Fetch(ctx)
	if err == nil {
		t.Fatal("expected ctx cancellation error")
	}
}

type ctxAwareCalClient struct{}

func (c *ctxAwareCalClient) Do(req *http.Request) (*http.Response, error) {
	if err := req.Context().Err(); err != nil {
		return nil, err
	}
	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(""))}, nil
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
	src := NewHTTPSource(FeedsFromURLs([]string{"https://example.com/cal.ics"}), client)

	_, err := src.Fetch(context.Background())
	if err == nil {
		t.Fatal("expected error for non-200 status")
	}
}
