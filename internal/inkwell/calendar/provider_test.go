package calendar

import (
	"context"
	"net/http"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/testutil/fakehttp"
)

// Two widgets showing the same feed share one fetch of it.
func TestProvider_WidgetsSharingAFeedFetchItOnce(t *testing.T) {
	const url = "https://example.com/cal.ics"
	tr := fakehttp.New()
	tr.Serve(url, recurringFeedICS)
	now := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
	p := NewProvider(tr, func() time.Time { return now })

	first := p.Source([]Feed{{URL: url}}, 15*time.Minute)
	second := p.Source([]Feed{{URL: url}}, 15*time.Minute)
	for _, src := range []Source{first, second} {
		got, err := src.Events(context.Background(), now, now.AddDate(0, 0, 7))
		if err != nil {
			t.Fatalf("Events: %v", err)
		}
		if len(got) != 1 {
			t.Errorf("got %d occurrences, want 1", len(got))
		}
	}

	if got := tr.Requests(url); got != 1 {
		t.Errorf("upstream requests = %d, want 1", got)
	}
}

// Widgets on a screen fetch concurrently. Those asking for one feed at once
// wait for a single fetch of it rather than each making their own.
func TestProvider_ConcurrentRequestsShareOneFetch(t *testing.T) {
	const url = "https://example.com/cal.ics"
	tr := fakehttp.New()
	tr.Serve(url, recurringFeedICS)
	now := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
	p := NewProvider(tr, func() time.Time { return now })

	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			_, _ = p.Source([]Feed{{URL: url}}, time.Hour).Events(context.Background(), now, now.AddDate(0, 0, 7))
		})
	}
	wg.Wait()

	if got := tr.Requests(url); got != 1 {
		t.Errorf("upstream requests = %d, want 1", got)
	}
}

// Two widgets may give one feed different rules. Each sees its own rules
// applied, whichever of them fetched the feed into the cache.
func TestProvider_EachWidgetSeesItsOwnRules(t *testing.T) {
	const url = "https://team.example/cal.ics"
	tr := fakehttp.New()
	tr.Serve(url, teamICS)
	now := time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC)
	p := NewProvider(tr, func() time.Time { return now })

	widgets := []struct {
		label string
		rules []Rule
		want  []string
	}{
		{label: "strips the player's name", rules: []Rule{mustRule(t, `^Jane Doe\n`, "", false)}, want: []string{"Ravens\nPractice\nEast Rink"}},
		{label: "drops practices", rules: []Rule{mustRule(t, `Practice`, "", true)}, want: []string{}},
		{label: "has no rules", want: []string{"Jane Doe\nRavens\nPractice\nEast Rink"}},
		{label: "strips the player's name again", rules: []Rule{mustRule(t, `^Jane Doe\n`, "", false)}, want: []string{"Ravens\nPractice\nEast Rink"}},
	}
	for _, w := range widgets {
		got, err := p.Source([]Feed{{URL: url, Rules: w.rules}}, time.Hour).Events(context.Background(), now, now.AddDate(0, 0, 1))
		if err != nil {
			t.Fatalf("%s: Events: %v", w.label, err)
		}
		if got := summaries(got); !slices.Equal(got, w.want) {
			t.Errorf("widget that %s sees %q, want %q", w.label, got, w.want)
		}
	}
	if got := tr.Requests(url); got != 1 {
		t.Errorf("upstream requests = %d, want 1", got)
	}
}

// A feed that fails to fetch serves its last good copy alongside the
// error, so a network blip doesn't blank the calendar. A failure is not a
// fresh copy: the next request tries again.
func TestProvider_FailedFetch(t *testing.T) {
	const (
		team     = "https://team.example/cal.ics"
		personal = "https://personal.example/cal.ics"
	)
	day := time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC)
	down := fakehttp.Reply{Status: http.StatusServiceUnavailable}
	up := fakehttp.Reply{Body: teamICS}

	steps := []struct {
		label        string
		at           time.Duration
		team         fakehttp.Reply
		want         []string
		wantErr      bool
		wantRequests int // upstream requests for the team feed so far
	}{
		{label: "failing before any good copy serves only the other feed", team: down, want: []string{`Jane Doe\nDentist`}, wantErr: true, wantRequests: 1},
		{label: "the next request retries", team: up, want: []string{"Jane Doe\nRavens\nPractice\nEast Rink", `Jane Doe\nDentist`}, wantRequests: 2},
		{label: "failing after a good copy serves that copy", at: time.Hour, team: down, want: []string{"Jane Doe\nRavens\nPractice\nEast Rink", `Jane Doe\nDentist`}, wantErr: true, wantRequests: 3},
		{label: "and keeps retrying", at: time.Hour, team: down, want: []string{"Jane Doe\nRavens\nPractice\nEast Rink", `Jane Doe\nDentist`}, wantErr: true, wantRequests: 4},
	}

	tr := fakehttp.New()
	tr.Serve(personal, personalICS)
	now := day
	p := NewProvider(tr, func() time.Time { return now })
	src := p.Source([]Feed{{URL: team}, {URL: personal}}, 15*time.Minute)
	for _, s := range steps {
		now = day.Add(s.at)
		tr.Set(team, s.team)
		got, err := src.Events(context.Background(), day, day.AddDate(0, 0, 1))
		if (err != nil) != s.wantErr {
			t.Errorf("%s: err = %v, want error %v", s.label, err, s.wantErr)
		}
		if got := summaries(got); !slices.Equal(got, s.want) {
			t.Errorf("%s: got %q, want %q", s.label, got, s.want)
		}
		if got := tr.Requests(team); got != s.wantRequests {
			t.Errorf("%s: team requests = %d, want %d", s.label, got, s.wantRequests)
		}
	}
}

func summaries(events []Event) []string {
	out := make([]string, 0, len(events))
	for _, e := range events {
		out = append(out, e.Summary)
	}
	return out
}

// Each widget's request refetches only when the cached copy is at least as
// old as that widget's own refresh setting. A widget that asks for fresher
// data than another refetches for itself; the other is then served the
// fresher copy rather than a copy as stale as it would have tolerated.
func TestProvider_EachWidgetsRefreshIsHonoured(t *testing.T) {
	const url = "https://example.com/cal.ics"
	start := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)

	steps := []struct {
		label        string
		at           time.Duration // since the first fetch
		refresh      time.Duration // the requesting widget's setting
		wantRequests int           // upstream requests so far
	}{
		{label: "first request fetches", at: 0, refresh: time.Hour, wantRequests: 1},
		{label: "hourly widget at 10m is served the cache", at: 10 * time.Minute, refresh: time.Hour, wantRequests: 1},
		{label: "five-minute widget at 10m refetches", at: 10 * time.Minute, refresh: 5 * time.Minute, wantRequests: 2},
		{label: "five-minute widget at 14m is served the cache", at: 14 * time.Minute, refresh: 5 * time.Minute, wantRequests: 2},
		{label: "five-minute widget at exactly 15m refetches", at: 15 * time.Minute, refresh: 5 * time.Minute, wantRequests: 3},
		{label: "hourly widget at 70m is served the five-minute widget's copy", at: 70 * time.Minute, refresh: time.Hour, wantRequests: 3},
		{label: "hourly widget at 75m refetches", at: 75 * time.Minute, refresh: time.Hour, wantRequests: 4},
	}

	tr := fakehttp.New()
	tr.Serve(url, recurringFeedICS)
	now := start
	p := NewProvider(tr, func() time.Time { return now })
	for _, s := range steps {
		now = start.Add(s.at)
		if _, err := p.Source([]Feed{{URL: url}}, s.refresh).Events(context.Background(), start, start.AddDate(0, 0, 7)); err != nil {
			t.Fatalf("%s: Events: %v", s.label, err)
		}
		if got := tr.Requests(url); got != s.wantRequests {
			t.Errorf("%s: upstream requests = %d, want %d", s.label, got, s.wantRequests)
		}
	}
}
