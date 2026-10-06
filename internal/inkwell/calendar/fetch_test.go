package calendar

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/testutil/fakehttp"
)

const testICS = `BEGIN:VCALENDAR
BEGIN:VEVENT
UID:evt-001@example.com
DTSTART:20260425T090000Z
DTEND:20260425T100000Z
SUMMARY:Standup
END:VEVENT
END:VCALENDAR
`

// A feed that can't be fetched or read reports why, naming its URL, and
// contributes nothing.
func TestProvider_FetchErrors(t *testing.T) {
	const url = "https://example.com/cal.ics"
	badICS := "BEGIN:VCALENDAR\nBEGIN:VEVENT\nUID:bad\nDTSTART:not-a-date\nSUMMARY:Bad\nEND:VEVENT\nEND:VCALENDAR\n"

	tests := []struct {
		label   string
		reply   fakehttp.Reply
		ctx     func() context.Context
		wantErr string
	}{
		{label: "network error", reply: fakehttp.Reply{Err: errors.New("connection refused")}, wantErr: `fetch "` + url + `": connection refused`},
		{label: "non-200 status", reply: fakehttp.Reply{Status: http.StatusInternalServerError}, wantErr: `fetch "` + url + `": status 500`},
		{label: "invalid ICS", reply: fakehttp.Reply{Body: badICS}, wantErr: `parse "` + url + `"`},
		{label: "body fails to close", reply: fakehttp.Reply{Body: testICS, CloseErr: errors.New("simulated close failure")}, wantErr: `close "` + url + `": simulated close failure`},
		{
			label: "cancelled context",
			reply: fakehttp.Reply{Body: testICS},
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			},
			wantErr: context.Canceled.Error(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			tr := fakehttp.New()
			tr.Set(url, tt.reply)
			ctx := context.Background()
			if tt.ctx != nil {
				ctx = tt.ctx()
			}
			day := time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC)
			p := NewProvider(tr, func() time.Time { return day })

			got, err := p.Source([]Feed{{URL: url}}, time.Hour).Events(ctx, day, day.AddDate(0, 0, 1))
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("err = %v, want it to contain %q", err, tt.wantErr)
			}
			if len(got) != 0 {
				t.Errorf("got %d events, want none", len(got))
			}
		})
	}
}

// Building the request can only fail on a URL with control characters,
// which no config produces; the hook lets the error branch be exercised.
func TestProvider_BuildRequestError(t *testing.T) {
	orig := newRequestWithContext
	newRequestWithContext = func(context.Context, string, string, io.Reader) (*http.Request, error) {
		return nil, errors.New("synthetic build-request error")
	}
	defer func() { newRequestWithContext = orig }()

	day := time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC)
	p := NewProvider(fakehttp.New(), func() time.Time { return day })
	_, err := p.Source([]Feed{{URL: "https://example.com/cal.ics"}}, time.Hour).Events(context.Background(), day, day.AddDate(0, 0, 1))
	if err == nil || !strings.Contains(err.Error(), "build request") {
		t.Errorf("err = %v, want a build request error", err)
	}
}
