package todayhero

import (
	"context"
	"errors"
	"image"
	nethttp "net/http"
	"strings"
	"testing"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar"
	"github.com/grantlucas/inkwell/internal/inkwell/calendar/ical"
	"github.com/grantlucas/inkwell/internal/inkwell/testutil"
	"github.com/grantlucas/inkwell/internal/inkwell/weather"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daygrid"
)

// testTime is a Monday mid-afternoon: today's agenda still has events
// to come, and the now-marker falls inside the chart's 06:00-21:00
// window.
var testTime = time.Date(2026, 3, 16, 14, 30, 0, 0, time.UTC)

func fixedClock(t time.Time) func() time.Time { return func() time.Time { return t } }

func newTestFrame(w, h int) *image.Paletted {
	frame := image.NewPaletted(image.Rect(0, 0, w, h), widget.PaperPalette)
	daygrid.FillWhite(frame, frame.Bounds())
	return frame
}

func countIndexIn(frame *image.Paletted, r image.Rectangle, idx uint8) int {
	n := 0
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			if frame.ColorIndexAt(x, y) == idx {
				n++
			}
		}
	}
	return n
}

type stubCalSource struct {
	events           []ical.Event
	err              error
	gotStart, gotEnd time.Time
}

func (s *stubCalSource) Events(_ context.Context, start, end time.Time) ([]ical.Event, error) {
	s.gotStart, s.gotEnd = start, end
	if s.err != nil {
		return nil, s.err
	}
	return s.events, nil
}

type stubWeatherSource struct {
	forecast *weather.Forecast
	err      error
	gotDays  int
}

func (s *stubWeatherSource) Forecast(_ context.Context, _ weather.Location, days int) (*weather.Forecast, error) {
	s.gotDays = days
	return s.forecast, s.err
}

// stubHTTPClient records that the calendar source reached for it.
type stubHTTPClient struct{ called bool }

func (s *stubHTTPClient) Do(_ *nethttp.Request) (*nethttp.Response, error) {
	s.called = true
	return nil, context.DeadlineExceeded
}

// sampleForecast covers today plus the four rows, with rain today (so
// the hero chart draws bars and its marker) and a dry last day.
func sampleForecast() *weather.Forecast {
	var days []weather.DailyForecast
	for i := range totalDays {
		var hourly []weather.HourlyPoint
		for h := range 24 {
			prob := 0.0
			if i < totalDays-1 && h >= 12 && h <= 17 {
				prob = 0.4 + 0.1*float64(i)
			}
			hourly = append(hourly, weather.HourlyPoint{
				Hour:              h,
				Temperature:       8 + float64(i) + float64(h)/6,
				PrecipitationProb: prob,
			})
		}
		days = append(days, weather.DailyForecast{
			Date:      time.Date(2026, 3, 16+i, 0, 0, 0, 0, time.UTC),
			High:      14 + float64(i),
			Low:       3 + float64(i),
			Condition: weather.Condition(i % 4),
			Hourly:    hourly,
		})
	}
	return &weather.Forecast{Days: days}
}

// dryForecast is the same shape with no rain anywhere, for the
// "NO RAIN TODAY" path.
func dryForecast() *weather.Forecast {
	f := sampleForecast()
	for i := range f.Days {
		for h := range f.Days[i].Hourly {
			f.Days[i].Hourly[h].PrecipitationProb = 0
		}
	}
	return f
}

func ev(summary string, day, hour int) ical.Event {
	start := time.Date(2026, 3, day, hour, 0, 0, 0, time.UTC)
	return ical.Event{UID: summary, Summary: summary, Start: start, End: start.Add(time.Hour)}
}

func sampleEvents() []ical.Event {
	return []ical.Event{
		ev("Standup", 16, 9),        // finished by 14:30
		ev("Design review", 16, 16), // still to come
		ev("1:1", 16, 17),
		ev("Retro", 16, 18),
		ev("Grocery run", 16, 19),
		ev("Dentist", 17, 10),
		{
			UID: "trip", Summary: "Conference", AllDay: true,
			Start: time.Date(2026, 3, 18, 0, 0, 0, 0, time.UTC),
			End:   time.Date(2026, 3, 19, 0, 0, 0, 0, time.UTC),
		},
		ev("Standup", 19, 9), ev("Planning", 19, 11), ev("Review", 19, 14), ev("Demo", 19, 16),
	}
}

func newWidget(cal calendar.Source, ws weather.Source, clock time.Time) *Widget {
	return New(image.Rect(0, 0, 800, 480), cal, ws, fixedClock(clock), Config{
		MaxEvents: defaultMaxEvents,
		Weather:   daygrid.WeatherConfig{TempUnit: "C"},
	})
}

func renderToFrame(t *testing.T, w *Widget) *image.Paletted {
	t.Helper()
	frame := image.NewPaletted(image.Rect(0, 0, 800, 480), widget.PaperPalette)
	if err := w.Render(frame); err != nil {
		t.Fatalf("Render: %v", err)
	}
	return frame
}

func TestWidget_Bounds(t *testing.T) {
	if got := newWidget(&stubCalSource{}, nil, testTime).Bounds(); got != image.Rect(0, 0, 800, 480) {
		t.Errorf("Bounds = %v", got)
	}
}

// The window covers today plus the four rows — the span the panel
// actually shows.
func TestWidget_RequestsFiveDays(t *testing.T) {
	cal := &stubCalSource{}
	ws := &stubWeatherSource{forecast: sampleForecast()}
	renderToFrame(t, newWidget(cal, ws, testTime))

	if want := time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC); !cal.gotStart.Equal(want) {
		t.Errorf("start = %v, want %v", cal.gotStart, want)
	}
	if got := cal.gotEnd.Sub(cal.gotStart); got != 5*24*time.Hour {
		t.Errorf("window = %v, want 120h", got)
	}
	if ws.gotDays != totalDays {
		t.Errorf("forecast days = %d, want %d", ws.gotDays, totalDays)
	}
}

// The identity block is inverted: a gray tint would vanish in Gray4's
// light bucket and snap to white under the BW threshold, so inversion
// is the only treatment that reads on both.
func TestWidget_IdentityBlockIsInverted(t *testing.T) {
	frame := renderToFrame(t, newWidget(&stubCalSource{}, &stubWeatherSource{forecast: sampleForecast()}, testTime))
	block := computeHero(image.Rect(0, 0, 800, 480)).Identity

	black := countIndexIn(frame, block, widget.PaperBlack)
	white := countIndexIn(frame, block, widget.PaperWhite)
	if black < black+white-black/2 {
		t.Errorf("identity block is %d black / %d white — it does not look inverted", black, white)
	}
	if white == 0 {
		t.Error("no white text on the inverted block")
	}
}

// Today's agenda shows what is left of the day. An event that finished
// two hours ago is history, and this is the one screen that spends real
// estate on today — spending it on the past would waste the whole idea.
func TestWidget_HeroAgendaShowsOnlyRemainingEvents(t *testing.T) {
	agenda := computeHero(image.Rect(0, 0, 800, 480)).Agenda

	// 08:00: everything is still to come.
	morning := renderToFrame(t, newWidget(&stubCalSource{events: sampleEvents()},
		&stubWeatherSource{forecast: sampleForecast()},
		time.Date(2026, 3, 16, 8, 0, 0, 0, time.UTC)))

	// 23:00: nothing is.
	night := renderToFrame(t, newWidget(&stubCalSource{events: sampleEvents()},
		&stubWeatherSource{forecast: sampleForecast()},
		time.Date(2026, 3, 16, 23, 0, 0, 0, time.UTC)))

	morningInk := countIndexIn(morning, agenda, widget.PaperBlack)
	nightInk := countIndexIn(night, agenda, widget.PaperBlack)
	if nightInk >= morningInk {
		t.Errorf("end-of-day agenda has %d px of ink against the morning's %d; it should have collapsed to the done marker",
			nightInk, morningInk)
	}
	if nightInk == 0 {
		t.Error("end of day drew nothing at all — the done marker is missing")
	}
}

// A fetch failure on either side must leave a usable panel.
func TestWidget_RendersDespiteFetchFailures(t *testing.T) {
	tests := []struct {
		label string
		cal   *stubCalSource
		ws    *stubWeatherSource
	}{
		{"calendar fails", &stubCalSource{err: errors.New("boom")}, &stubWeatherSource{forecast: sampleForecast()}},
		{"weather fails", &stubCalSource{events: sampleEvents()}, &stubWeatherSource{err: errors.New("boom")}},
		{"both fail", &stubCalSource{err: errors.New("boom")}, &stubWeatherSource{err: errors.New("boom")}},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			frame := renderToFrame(t, newWidget(tt.cal, tt.ws, testTime))
			if countIndexIn(frame, frame.Bounds(), widget.PaperBlack) == 0 {
				t.Error("nothing rendered at all")
			}
		})
	}
}

func TestWidget_NoWeatherSource(t *testing.T) {
	frame := renderToFrame(t, newWidget(&stubCalSource{events: sampleEvents()}, nil, testTime))
	if countIndexIn(frame, frame.Bounds(), widget.PaperBlack) == 0 {
		t.Error("nothing rendered")
	}
}

// Every element is placed at a fixed offset from its band and the draw
// helpers clip to the frame, not to the widget's bounds, so a widget
// given less room than the layout needs would paint over its neighbour.
func TestWidget_TooSmallDrawsNothing(t *testing.T) {
	tests := []struct {
		label  string
		bounds image.Rectangle
	}{
		{"too short", image.Rect(0, 0, 800, 200)},
		{"too narrow", image.Rect(0, 0, 400, 480)},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			frame := image.NewPaletted(image.Rect(0, 0, 800, 480), widget.PaperPalette)
			// A neighbour already on the frame, outside these bounds.
			neighbour := image.Rect(0, 481-1, 800, 480)
			_ = neighbour
			daygrid.FillRect(frame, image.Rect(0, 0, 800, 480), widget.PaperWhite)

			w := New(tt.bounds, &stubCalSource{events: sampleEvents()},
				&stubWeatherSource{forecast: sampleForecast()}, fixedClock(testTime),
				Config{MaxEvents: defaultMaxEvents, Weather: daygrid.WeatherConfig{TempUnit: "C"}})
			if err := w.Render(frame); err != nil {
				t.Fatalf("Render: %v", err)
			}
			if got := countIndexIn(frame, frame.Bounds(), widget.PaperBlack); got != 0 {
				t.Errorf("drew %d px into bounds too small to draw into", got)
			}
		})
	}
}

func TestWidget_Golden(t *testing.T) {
	tests := []struct {
		label string
		cal   *stubCalSource
		ws    *stubWeatherSource
		clock time.Time
		cfg   func(*Config)
	}{
		{
			// Mid-afternoon: today still has events, the chart has its
			// bars and marker, and a day row overflows to "+N more".
			label: "mid-afternoon with events remaining",
			cal:   &stubCalSource{events: sampleEvents()},
			ws:    &stubWeatherSource{forecast: sampleForecast()},
		},
		{
			label: "end of day with nothing left",
			cal:   &stubCalSource{events: sampleEvents()},
			ws:    &stubWeatherSource{forecast: sampleForecast()},
			clock: time.Date(2026, 3, 16, 23, 0, 0, 0, time.UTC),
		},
		{
			label: "a dry today",
			cal:   &stubCalSource{events: sampleEvents()},
			ws:    &stubWeatherSource{forecast: dryForecast()},
		},
		{
			label: "no weather at all",
			cal:   &stubCalSource{events: sampleEvents()},
			ws:    &stubWeatherSource{},
		},
		{
			label: "no events at all",
			cal:   &stubCalSource{},
			ws:    &stubWeatherSource{forecast: sampleForecast()},
		},
		{
			label: "fahrenheit with locations",
			cal:   &stubCalSource{events: sampleEvents()},
			ws:    &stubWeatherSource{forecast: sampleForecast()},
			cfg: func(c *Config) {
				c.Weather.TempUnit = "F"
				c.ShowLocation = true
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			cfg := Config{MaxEvents: defaultMaxEvents, Weather: daygrid.WeatherConfig{TempUnit: "C"}}
			if tt.cfg != nil {
				tt.cfg(&cfg)
			}
			clock := tt.clock
			if clock.IsZero() {
				clock = testTime
			}
			w := New(image.Rect(0, 0, 800, 480), tt.cal, tt.ws, fixedClock(clock), cfg)
			testutil.AssertGoldenPNG(t, renderToFrame(t, w))
		})
	}
}

func TestFactory(t *testing.T) {
	w, err := Factory(image.Rect(0, 0, 800, 480), map[string]any{
		"feeds": []any{"https://example.com/a.ics"},
	}, widget.Deps{Now: fixedClock(testTime)})
	if err != nil {
		t.Fatalf("Factory: %v", err)
	}
	if got := w.Bounds(); got != image.Rect(0, 0, 800, 480) {
		t.Errorf("Bounds = %v", got)
	}
}

func TestFactory_InvalidConfig(t *testing.T) {
	_, err := Factory(image.Rect(0, 0, 800, 480), map[string]any{}, widget.Deps{})
	if err == nil {
		t.Fatal("expected an error for missing feeds")
	}
	if !strings.Contains(err.Error(), "feeds is required") {
		t.Errorf("error = %q", err)
	}
}

func TestFactory_DefaultsTheClock(t *testing.T) {
	w, err := Factory(image.Rect(0, 0, 800, 480), map[string]any{
		"feeds": []any{"https://example.com/a.ics"},
	}, widget.Deps{})
	if err != nil {
		t.Fatalf("Factory: %v", err)
	}
	if got := w.(*Widget).now().Year(); got < 2024 {
		t.Errorf("clock year = %d, want the real wall clock", got)
	}
}

func TestFactory_UsesInjectedWeatherSource(t *testing.T) {
	ws := &stubWeatherSource{forecast: sampleForecast()}
	w, err := Factory(image.Rect(0, 0, 800, 480), map[string]any{
		"feeds": []any{"https://example.com/a.ics"},
	}, widget.Deps{
		Now:         fixedClock(testTime),
		DataSources: map[string]any{"weather_source": weather.Source(ws)},
	})
	if err != nil {
		t.Fatalf("Factory: %v", err)
	}
	if w.(*Widget).weather != weather.Source(ws) {
		t.Error("injected weather_source was not used")
	}
}

func TestFactory_UsesSharedProvider(t *testing.T) {
	provider := weather.NewProvider(nil, time.Hour, time.Now, weather.Settings{
		Location: weather.Location{Latitude: 43.25, Longitude: -79.87},
		TempUnit: "C",
		Model:    weather.ModelGEM,
	})
	w, err := Factory(image.Rect(0, 0, 800, 480), map[string]any{
		"feeds": []any{"https://example.com/a.ics"},
	}, widget.Deps{
		Now:         fixedClock(testTime),
		DataSources: map[string]any{"weather": provider},
	})
	if err != nil {
		t.Fatalf("Factory: %v", err)
	}
	th := w.(*Widget)
	if th.weather == nil {
		t.Fatal("no weather source taken from the provider")
	}
	if th.config.Weather.Latitude != 43.25 {
		t.Errorf("Latitude = %v, want the provider default", th.config.Weather.Latitude)
	}
}

func TestFactory_UsesInjectedHTTPClient(t *testing.T) {
	client := &stubHTTPClient{}
	w, err := Factory(image.Rect(0, 0, 800, 480), map[string]any{
		"feeds": []any{"https://example.com/a.ics"},
	}, widget.Deps{
		Now:         fixedClock(testTime),
		DataSources: map[string]any{"http_client": calendar.HTTPClient(client)},
	})
	if err != nil {
		t.Fatalf("Factory: %v", err)
	}
	renderToFrame(t, w.(*Widget))
	if !client.called {
		t.Error("the injected HTTP client was never used")
	}
}
