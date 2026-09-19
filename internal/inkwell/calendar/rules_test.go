package calendar

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

const teamICS = `BEGIN:VCALENDAR
BEGIN:VEVENT
UID:team-001@example.com
DTSTART:20260919T130000Z
DTEND:20260919T140000Z
SUMMARY:Jane Doe\nRavens\nPractice\nEast Rink
END:VEVENT
END:VCALENDAR
`

const personalICS = `BEGIN:VCALENDAR
BEGIN:VEVENT
UID:personal-001@example.com
DTSTART:20260919T150000Z
DTEND:20260919T160000Z
SUMMARY:Jane Doe\\nDentist
END:VEVENT
END:VCALENDAR
`

func TestHTTPSource_ReplaceRuleRewritesSummary(t *testing.T) {
	client := &mockHTTPClient{
		responses: map[string]*http.Response{"https://team.example/cal.ics": newMockResponse(teamICS)},
	}
	rule, err := NewRule(`^Jane Doe\n(Ravens\n)?`, "", false)
	if err != nil {
		t.Fatalf("NewRule: %v", err)
	}
	src := NewHTTPSource([]Feed{{URL: "https://team.example/cal.ics", Rules: []Rule{rule}}}, client)

	events, err := src.Events(context.Background(),
		time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("Events: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
	if got, want := events[0].Summary, "Practice\nEast Rink"; got != want {
		t.Errorf("Summary = %q, want %q", got, want)
	}
}

// eventsFrom runs a one-feed source over a day-wide window.
func eventsFrom(t *testing.T, feeds []Feed, responses map[string]*http.Response) []Event {
	t.Helper()
	src := NewHTTPSource(feeds, &mockHTTPClient{responses: responses})
	events, err := src.Events(context.Background(),
		time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("Events: %v", err)
	}
	return events
}

func mustRule(t *testing.T, match, replace string, exclude bool) Rule {
	t.Helper()
	r, err := NewRule(match, replace, exclude)
	if err != nil {
		t.Fatalf("NewRule(%q): %v", match, err)
	}
	return r
}

func TestApplyRules(t *testing.T) {
	const url = "https://team.example/cal.ics"

	cases := []struct {
		label       string
		rules       func(t *testing.T) []Rule
		wantSummary string // empty means the event is expected to be dropped
	}{
		{
			label:       "no rules leaves the summary untouched",
			rules:       func(*testing.T) []Rule { return nil },
			wantSummary: "Jane Doe\nRavens\nPractice\nEast Rink",
		},
		{
			label: "exclude drops a matching event",
			rules: func(t *testing.T) []Rule {
				return []Rule{mustRule(t, `Practice`, "", true)}
			},
			wantSummary: "",
		},
		{
			label: "exclude keeps a non-matching event",
			rules: func(t *testing.T) []Rule {
				return []Rule{mustRule(t, `Tournament`, "", true)}
			},
			wantSummary: "Jane Doe\nRavens\nPractice\nEast Rink",
		},
		{
			label: "capture groups expand in the replacement",
			rules: func(t *testing.T) []Rule {
				return []Rule{mustRule(t, `^Jane Doe\n(Ravens)\n`, "$1: ", false)}
			},
			wantSummary: "Ravens: Practice\nEast Rink",
		},
		{
			label: "rules apply in order as a pipeline",
			rules: func(t *testing.T) []Rule {
				return []Rule{
					mustRule(t, `^Jane Doe\nRavens\n`, "", false),
					mustRule(t, `^Practice`, "Skate", false),
				}
			},
			wantSummary: "Skate\nEast Rink",
		},
		{
			// Proves the exclude matches against the rewritten summary:
			// "^Skate" cannot match the original, which starts with
			// "Jane Doe", only the text the two replaces leave behind.
			label: "an exclude sees the summary the earlier replaces left",
			rules: func(t *testing.T) []Rule {
				return []Rule{
					mustRule(t, `^Jane Doe\nRavens\n`, "", false),
					mustRule(t, `Practice`, "Skate", false),
					mustRule(t, `^Skate`, "", true),
				}
			},
			wantSummary: "",
		},
		{
			// A summary now holds real newlines, so it matters that "^"
			// anchors to the whole value and not to each line. Operators
			// who want per-line anchoring opt in with Go's (?m) flag.
			label: "caret anchors the whole summary, not each line",
			rules: func(t *testing.T) []Rule {
				return []Rule{mustRule(t, `^Practice`, "Skate", false)}
			},
			wantSummary: "Jane Doe\nRavens\nPractice\nEast Rink",
		},
		{
			label: "the (?m) flag opts in to per-line anchoring",
			rules: func(t *testing.T) []Rule {
				return []Rule{mustRule(t, `(?m)^Practice$`, "Skate", false)}
			},
			wantSummary: "Jane Doe\nRavens\nSkate\nEast Rink",
		},
		{
			label: "a rewritten summary is trimmed",
			rules: func(t *testing.T) []Rule {
				return []Rule{mustRule(t, `East Rink$`, "", false)}
			},
			wantSummary: "Jane Doe\nRavens\nPractice",
		},
	}

	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			events := eventsFrom(t,
				[]Feed{{URL: url, Rules: tc.rules(t)}},
				map[string]*http.Response{url: newMockResponse(teamICS)})

			if tc.wantSummary == "" {
				if len(events) != 0 {
					t.Fatalf("got %d events, want 0 (event should be excluded)", len(events))
				}
				return
			}
			if len(events) != 1 {
				t.Fatalf("got %d events, want 1", len(events))
			}
			if events[0].Summary != tc.wantSummary {
				t.Errorf("Summary = %q, want %q", events[0].Summary, tc.wantSummary)
			}
		})
	}
}

func TestHTTPSource_RulesApplyOnlyToTheirOwnFeed(t *testing.T) {
	const teamURL = "https://team.example/cal.ics"
	const personalURL = "https://personal.example/cal.ics"

	events := eventsFrom(t,
		[]Feed{
			{URL: teamURL, Name: "Team calendar", Rules: []Rule{mustRule(t, `^Jane Doe\nRavens\n`, "", false)}},
			{URL: personalURL},
		},
		map[string]*http.Response{
			teamURL:     newMockResponse(teamICS),
			personalURL: newMockResponse(personalICS),
		})

	if len(events) != 2 {
		t.Fatalf("got %d events, want 2", len(events))
	}
	// Sorted by start: the team event is at 13:00, the personal one at 15:00.
	if got, want := events[0].Summary, "Practice\nEast Rink"; got != want {
		t.Errorf("ramp Summary = %q, want %q", got, want)
	}
	if got, want := events[1].Summary, `Jane Doe\nDentist`; got != want {
		t.Errorf("personal Summary = %q, want %q (other feeds must be untouched)", got, want)
	}
}

func TestNewRule_Validation(t *testing.T) {
	cases := []struct {
		label   string
		match   string
		replace string
		exclude bool
		wantErr string
	}{
		{label: "empty match", match: "", wantErr: "must not be empty"},
		{label: "invalid regex", match: "(unclosed", wantErr: "invalid match"},
		{label: "replace and exclude together", match: "x", replace: "y", exclude: true, wantErr: "both replace and exclude"},
	}

	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			_, err := NewRule(tc.match, tc.replace, tc.exclude)
			if err == nil {
				t.Fatal("expected an error")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error = %q, want it to contain %q", err, tc.wantErr)
			}
		})
	}
}

func TestFeedsFromURLs(t *testing.T) {
	feeds := FeedsFromURLs([]string{"https://a.example/x.ics", "https://b.example/y.ics"})
	if len(feeds) != 2 {
		t.Fatalf("got %d feeds, want 2", len(feeds))
	}
	if feeds[0].URL != "https://a.example/x.ics" || feeds[1].URL != "https://b.example/y.ics" {
		t.Errorf("feeds = %+v", feeds)
	}
	if feeds[0].Rules != nil {
		t.Errorf("Rules = %v, want nil", feeds[0].Rules)
	}
}
