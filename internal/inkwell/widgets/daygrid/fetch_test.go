package daygrid

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar/ical"
	"github.com/grantlucas/inkwell/internal/inkwell/weather"
)

// slowCal blocks for delay, or until the context is done, before
// answering — so a test can starve the fetch that runs beside it.
type slowCal struct {
	delay  time.Duration
	events []ical.Event
	err    error
}

func (s *slowCal) Events(ctx context.Context, _, _ time.Time) ([]ical.Event, error) {
	select {
	case <-time.After(s.delay):
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return s.events, s.err
}

type stubWeather struct {
	forecast *weather.Forecast
	err      error
	gotDays  int
	// sawExpired records whether the context was already done when the
	// call arrived, which is the failure this package exists to stop.
	sawExpired bool
}

func (s *stubWeather) Forecast(ctx context.Context, _ weather.Location, days int) (*weather.Forecast, error) {
	s.gotDays = days
	if ctx.Err() != nil {
		s.sawExpired = true
	}
	return s.forecast, s.err
}

func testDays() []Day { return Days(time.Date(2026, 3, 16, 9, 0, 0, 0, time.UTC), 5) }

func oneDayForecast() *weather.Forecast {
	return &weather.Forecast{Days: []weather.DailyForecast{{
		Date: time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC), High: 14,
	}}}
}

// The reason this function exists. Fetched in sequence, a calendar that
// ate the whole budget handed the forecast an already-expired context
// and that render drew no weather at all. Run together, neither can
// starve the other.
func TestFetch_SlowCalendarDoesNotStarveTheForecast(t *testing.T) {
	// The calendar outlasts the whole budget, which is what makes this
	// a starvation test rather than a slowness test: run in sequence,
	// the forecast is not reached until the deadline has already
	// passed. Run together, it is called immediately.
	const budget = 150 * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), budget)
	defer cancel()

	cal := &slowCal{delay: time.Hour}
	ws := &stubWeather{forecast: oneDayForecast()}

	forecast := Fetch(ctx, "test-widget", cal, ws, testDays(), weather.Location{}).Days()

	if ws.sawExpired {
		t.Error("the forecast was called with an already-expired context")
	}
	if len(forecast) != 1 {
		t.Errorf("got %d forecast days, want 1 — the forecast was starved", len(forecast))
	}
}

// The shared deadline is the point: running the two together must not
// widen the total bound, it must stop them queueing behind each other.
func TestFetch_StaysWithinTheSharedDeadline(t *testing.T) {
	const budget = 200 * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), budget)
	defer cancel()

	// Both sides would outlast the budget on their own.
	cal := &slowCal{delay: time.Hour}
	ws := &stubWeather{forecast: oneDayForecast()}

	start := time.Now()
	Fetch(ctx, "test-widget", cal, ws, testDays(), weather.Location{})
	elapsed := time.Since(start)

	// Sequential fetches of two hour-long calls would take two hours;
	// what matters is that the whole thing is bounded by the one
	// budget, with enough slack for a loaded machine.
	if elapsed > budget*5 {
		t.Errorf("took %v against a %v budget", elapsed, budget)
	}
}

// A failure on either side must leave the other's data usable: the
// panel is a calendar first and a forecast second, and neither should
// blank the other.
func TestFetch_OneSideFailingLeavesTheOther(t *testing.T) {
	events := []ical.Event{{UID: "a", Summary: "Standup"}}
	boom := func() error { return errors.New("boom") }

	tests := []struct {
		label        string
		cal          *slowCal
		ws           *stubWeather
		wantEvents   int
		wantForecast int
	}{
		{
			label:      "calendar fails",
			cal:        &slowCal{err: boom()},
			ws:         &stubWeather{forecast: oneDayForecast()},
			wantEvents: 0, wantForecast: 1,
		},
		{
			label:      "weather fails",
			cal:        &slowCal{events: events},
			ws:         &stubWeather{err: boom()},
			wantEvents: 1, wantForecast: 0,
		},
		{
			label:      "both fail",
			cal:        &slowCal{err: boom()},
			ws:         &stubWeather{err: boom()},
			wantEvents: 0, wantForecast: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			res := Fetch(context.Background(), "test-widget", tt.cal, tt.ws, testDays(), weather.Location{})
			if len(res.Events) != tt.wantEvents {
				t.Errorf("got %d events, want %d", len(res.Events), tt.wantEvents)
			}
			if len(res.Days()) != tt.wantForecast {
				t.Errorf("got %d forecast days, want %d", len(res.Days()), tt.wantForecast)
			}
		})
	}
}

// "No forecast arrived" and "a forecast arrived carrying no days" are
// different states, and weekly-calendar gives its weather band's height
// to the first. A 200 response with no daily data is the second, so
// flattening the two would collapse the band for that cycle.
func TestFetch_DistinguishesAnEmptyForecastFromNone(t *testing.T) {
	tests := []struct {
		label       string
		ws          weather.Source
		wantPresent bool
	}{
		{"a forecast carrying no days", &stubWeather{forecast: &weather.Forecast{}}, true},
		{"a forecast with days", &stubWeather{forecast: oneDayForecast()}, true},
		{"the fetch failed", &stubWeather{err: errors.New("boom")}, false},
		{"no source configured", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			res := Fetch(context.Background(), "test-widget", &slowCal{}, tt.ws, testDays(), weather.Location{})
			if res.HasForecast() != tt.wantPresent {
				t.Errorf("HasForecast = %v, want %v", res.HasForecast(), tt.wantPresent)
			}
		})
	}
}

// A screen configured without weather is not a failure — the forecast
// is simply skipped, and the calendar half still runs.
func TestFetch_NilWeatherSource(t *testing.T) {
	events := []ical.Event{{UID: "a", Summary: "Standup"}}
	res := Fetch(context.Background(), "test-widget",
		&slowCal{events: events}, nil, testDays(), weather.Location{})

	if len(res.Events) != 1 {
		t.Errorf("got %d events, want 1", len(res.Events))
	}
	if res.Days() != nil {
		t.Errorf("got %v, want no forecast", res.Days())
	}
}

// The forecast is asked for exactly as many days as the grid shows.
func TestFetch_AsksForTheGridsDayCount(t *testing.T) {
	ws := &stubWeather{forecast: oneDayForecast()}
	days := testDays()
	Fetch(context.Background(), "test-widget", &slowCal{}, ws, days, weather.Location{})
	if ws.gotDays != len(days) {
		t.Errorf("asked for %d days, want %d", ws.gotDays, len(days))
	}
}

func TestFetchContext(t *testing.T) {
	ctx, cancel := FetchContext()
	defer cancel()

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("no deadline on the render-scope context")
	}
	if d := time.Until(deadline); d > FetchTimeout || d < FetchTimeout-time.Second {
		t.Errorf("deadline in %v, want about %v", d, FetchTimeout)
	}
}
