package calendar

import (
	"context"
	"net/http"
	"slices"
	"testing"
	"time"
)

// recurringFeedICS is a feed whose weekly series began in January, months
// before the windows the tests ask for, next to a one-off event in the
// same week as the series' first instance.
const recurringFeedICS = `BEGIN:VCALENDAR
BEGIN:VEVENT
UID:weekly-sync@example.com
DTSTART:20260105T090000Z
DTEND:20260105T093000Z
RRULE:FREQ=WEEKLY
SUMMARY:Jane Doe\nWeekly Sync
END:VEVENT
BEGIN:VEVENT
UID:kickoff@example.com
DTSTART:20260106T130000Z
DTEND:20260106T140000Z
SUMMARY:Kickoff
END:VEVENT
END:VCALENDAR
`

// occurrence is the part of an Event the window tests compare: what it
// says and when it is.
type occurrence struct {
	Summary string
	Start   time.Time
}

func occurrencesOf(events []Event) []occurrence {
	out := make([]occurrence, 0, len(events))
	for _, e := range events {
		out = append(out, occurrence{Summary: e.Summary, Start: e.Start})
	}
	return out
}

// TestCachedSource_Occurrences drives the fetch and the cache together
// through a fake HTTP transport, the way a widget does. A series is
// expanded before the window is applied, so a weekly meeting that began
// in January still turns up in October.
func TestCachedSource_Occurrences(t *testing.T) {
	const url = "https://example.com/cal.ics"
	stripName := mustRule(t, `^Jane Doe\n`, "", false)
	dropSync := mustRule(t, `Weekly Sync`, "", true)

	cases := []struct {
		label      string
		rules      []Rule
		start, end time.Time
		want       []occurrence
	}{
		{
			label: "series that began before the window",
			start: time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC),
			end:   time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC),
			want: []occurrence{
				{Summary: "Jane Doe\nWeekly Sync", Start: time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC)},
			},
		},
		{
			label: "rules apply to every occurrence of a series",
			rules: []Rule{stripName},
			start: time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC),
			end:   time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC),
			want: []occurrence{
				{Summary: "Weekly Sync", Start: time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC)},
				{Summary: "Weekly Sync", Start: time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)},
			},
		},
		{
			label: "exclude rules drop every occurrence of a series",
			rules: []Rule{dropSync},
			start: time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC),
			end:   time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC),
			want:  []occurrence{},
		},
		{
			label: "one-off event outside the window",
			start: time.Date(2026, 1, 7, 0, 0, 0, 0, time.UTC),
			end:   time.Date(2026, 1, 12, 0, 0, 0, 0, time.UTC),
			want:  []occurrence{},
		},
		{
			label: "one-off event inside the window",
			start: time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC),
			end:   time.Date(2026, 1, 12, 0, 0, 0, 0, time.UTC),
			want: []occurrence{
				{Summary: "Jane Doe\nWeekly Sync", Start: time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC)},
				{Summary: "Kickoff", Start: time.Date(2026, 1, 6, 13, 0, 0, 0, time.UTC)},
			},
		},
		{
			label: "window before the series began",
			start: time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC),
			end:   time.Date(2025, 12, 8, 0, 0, 0, 0, time.UTC),
			want:  []occurrence{},
		},
	}

	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			client := &mockHTTPClient{responses: map[string]*http.Response{url: newMockResponse(recurringFeedICS)}}
			now := tc.start
			src := NewCachedSource(NewHTTPSource([]Feed{{URL: url, Rules: tc.rules}}, client), 15*time.Minute, func() time.Time { return now })

			fresh, err := src.Events(context.Background(), tc.start, tc.end)
			if err != nil {
				t.Fatalf("fresh Events: %v", err)
			}
			cached, err := src.Events(context.Background(), tc.start, tc.end)
			if err != nil {
				t.Fatalf("cached Events: %v", err)
			}

			if got := occurrencesOf(fresh); !slices.Equal(got, tc.want) {
				t.Errorf("fresh fetch = %v, want %v", got, tc.want)
			}
			if got := occurrencesOf(cached); !slices.Equal(got, tc.want) {
				t.Errorf("cache hit = %v, want %v", got, tc.want)
			}
			if client.getCalls != 1 {
				t.Errorf("upstream requests = %d, want 1", client.getCalls)
			}
		})
	}
}
