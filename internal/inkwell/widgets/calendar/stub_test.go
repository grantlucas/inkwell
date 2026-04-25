package calendar

import (
	"io"
	"net/http"
	"strings"
	"time"

	cal "github.com/grantlucas/inkwell/internal/inkwell/calendar"
)

// stubHTTPClient returns an empty valid iCal feed for any request.
type stubHTTPClient struct{}

func (s *stubHTTPClient) Get(_ string) (*http.Response, error) {
	body := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nEND:VCALENDAR\r\n"
	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader(body)),
	}, nil
}

// staticSource is a Source that returns fixed events.
type staticSource struct {
	events []cal.Event
	err    error
}

func (s *staticSource) Events(_, _ time.Time) ([]cal.Event, error) {
	return s.events, s.err
}
