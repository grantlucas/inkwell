package calendar

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/testutil/fakehttp"
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

// TestProvider_Occurrences drives the fetch and the cache together
// through a fake HTTP client, the way a widget does. A series is
// expanded before the window is applied, so a weekly meeting that began
// in January still turns up in October.
func TestProvider_Occurrences(t *testing.T) {
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
			tr := fakehttp.New()
			tr.Serve(url, recurringFeedICS)
			now := tc.start
			src := NewProvider(tr, func() time.Time { return now }).Source([]Feed{{URL: url, Rules: tc.rules}}, 15*time.Minute)

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
			if got := tr.Total(); got != 1 {
				t.Errorf("upstream requests = %d, want 1", got)
			}
		})
	}
}

// weeklySeries is a weekly series on Mondays at 09:00 UTC from January,
// for the override fixtures to edit single instances of.
const weeklySeries = `BEGIN:VEVENT
UID:weekly-sync@example.com
DTSTART:20260105T090000Z
DTEND:20260105T093000Z
RRULE:FREQ=WEEKLY
SUMMARY:Weekly Sync
END:VEVENT
`

// icsCalendar wraps VEVENT blocks in a VCALENDAR.
func icsCalendar(events ...string) string {
	return "BEGIN:VCALENDAR\r\n" + strings.Join(events, "") + "END:VCALENDAR\r\n"
}

// TestProvider_OverridesAndDuplicates drives single-instance edits
// and events carried by more than one feed through the fake HTTP
// client, the way a widget sees them.
func TestProvider_OverridesAndDuplicates(t *testing.T) {
	const (
		urlA = "https://a.example/cal.ics"
		urlB = "https://b.example/cal.ics"
	)
	oct := func(day, hour int) time.Time { return time.Date(2026, 10, day, hour, 0, 0, 0, time.UTC) }

	movedOct5 := `BEGIN:VEVENT
UID:weekly-sync@example.com
RECURRENCE-ID:20261005T090000Z
DTSTART:20261006T150000Z
DTEND:20261006T153000Z
SUMMARY:Weekly Sync
END:VEVENT
`
	// The Oct 5 instance moved to the Sunday before the window, and the
	// Oct 19 instance (outside the window) moved into it.
	movedOutOct5 := strings.ReplaceAll(movedOct5, "20261006T15", "20261004T15")
	movedInOct19 := `BEGIN:VEVENT
UID:weekly-sync@example.com
RECURRENCE-ID:20261019T090000Z
DTSTART:20261016T090000Z
DTEND:20261016T093000Z
SUMMARY:Weekly Sync
END:VEVENT
`
	// oneOff is a one-off event on Tuesday 6 October; uid, summary and
	// start hour vary so two feeds can carry it with or without
	// matching.
	oneOff := func(uid, summary string, hour int) string {
		return fmt.Sprintf(`BEGIN:VEVENT
UID:%s
DTSTART:20261006T%02d0000Z
DTEND:20261006T%02d3000Z
SUMMARY:%s
END:VEVENT
`, uid, hour, hour, summary)
	}

	cases := []struct {
		label string
		feeds map[string]string
		rules map[string][]Rule
		want  []occurrence
	}{
		{
			label: "an override listed before its series does not hide the series",
			feeds: map[string]string{urlA: icsCalendar(movedOct5, weeklySeries)},
			want: []occurrence{
				{Summary: "Weekly Sync", Start: oct(6, 15)},
				{Summary: "Weekly Sync", Start: oct(12, 9)},
			},
		},
		{
			label: "an instance moved out of the window leaves it",
			feeds: map[string]string{urlA: icsCalendar(weeklySeries, movedOutOct5)},
			want: []occurrence{
				{Summary: "Weekly Sync", Start: oct(12, 9)},
			},
		},
		{
			label: "an instance moved into the window joins it",
			feeds: map[string]string{urlA: icsCalendar(weeklySeries, movedInOct19)},
			want: []occurrence{
				{Summary: "Weekly Sync", Start: oct(5, 9)},
				{Summary: "Weekly Sync", Start: oct(12, 9)},
				{Summary: "Weekly Sync", Start: oct(16, 9)},
			},
		},
		{
			label: "the same title at different times stays separate",
			feeds: map[string]string{
				urlA: icsCalendar(oneOff("kickoff@a.example", "Kickoff", 13)),
				urlB: icsCalendar(oneOff("kickoff@b.example", "Kickoff", 15)),
			},
			want: []occurrence{
				{Summary: "Kickoff", Start: oct(6, 13)},
				{Summary: "Kickoff", Start: oct(6, 15)},
			},
		},
		{
			label: "the same title and start with a different end stays separate",
			feeds: map[string]string{
				urlA: icsCalendar(oneOff("kickoff@a.example", "Kickoff", 13)),
				urlB: icsCalendar(strings.ReplaceAll(oneOff("kickoff@b.example", "Kickoff", 13), "DTEND:20261006T133000Z", "DTEND:20261006T140000Z")),
			},
			want: []occurrence{
				{Summary: "Kickoff", Start: oct(6, 13)},
				{Summary: "Kickoff", Start: oct(6, 13)},
			},
		},
		{
			label: "two feeds whose rules rewrite to the same title collapse",
			feeds: map[string]string{
				urlA: icsCalendar(oneOff("practice@a.example", `Jane Doe\nPractice`, 13)),
				urlB: icsCalendar(oneOff("practice@b.example", `John Doe\nPractice`, 13)),
			},
			rules: map[string][]Rule{
				urlA: {mustRule(t, `^Jane Doe\n`, "", false)},
				urlB: {mustRule(t, `^John Doe\n`, "", false)},
			},
			want: []occurrence{
				{Summary: "Practice", Start: oct(6, 13)},
			},
		},
		{
			label: "an override an exclude rule drops still replaces its occurrence",
			feeds: map[string]string{
				urlA: icsCalendar(weeklySeries, strings.ReplaceAll(movedOct5, "SUMMARY:Weekly Sync", "SUMMARY:Weekly Sync (skipped)")),
			},
			rules: map[string][]Rule{
				urlA: {mustRule(t, `skipped`, "", true)},
			},
			want: []occurrence{
				{Summary: "Weekly Sync", Start: oct(12, 9)},
			},
		},
		{
			// Overrides apply within their own feed: feed A's rules
			// cancelling its Oct 5 instance leave feed B's copy of the
			// same series alone.
			label: "one feed's cancelled override leaves another feed's series alone",
			feeds: map[string]string{
				urlA: icsCalendar(weeklySeries, strings.ReplaceAll(movedOct5, "SUMMARY:Weekly Sync", "SUMMARY:Weekly Sync (skipped)")),
				urlB: icsCalendar(weeklySeries),
			},
			rules: map[string][]Rule{
				urlA: {mustRule(t, `skipped`, "", true)},
			},
			want: []occurrence{
				{Summary: "Weekly Sync", Start: oct(5, 9)},
				{Summary: "Weekly Sync", Start: oct(12, 9)},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			tr := fakehttp.New()
			var feeds []Feed
			for _, url := range []string{urlA, urlB} {
				body, ok := tc.feeds[url]
				if !ok {
					continue
				}
				tr.Serve(url, body)
				feeds = append(feeds, Feed{URL: url, Rules: tc.rules[url]})
			}
			start, end := oct(5, 0), oct(19, 0)
			src := NewProvider(tr, func() time.Time { return start }).Source(feeds, 15*time.Minute)

			got, err := src.Events(context.Background(), start, end)
			if err != nil {
				t.Fatalf("Events: %v", err)
			}
			if got := occurrencesOf(got); !slices.Equal(got, tc.want) {
				t.Errorf("occurrences = %v, want %v", got, tc.want)
			}
		})
	}
}
