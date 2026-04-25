package calendar

import (
	"io"
	"net/http"
	"strings"
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
