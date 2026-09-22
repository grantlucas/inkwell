package boldfive

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
	"github.com/grantlucas/inkwell/internal/inkwell/fonts"
	"github.com/grantlucas/inkwell/internal/inkwell/testutil"
	"github.com/grantlucas/inkwell/internal/inkwell/weather"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
)

// testTime is a Monday mid-afternoon, so today's column has both
// finished and upcoming events and the now-marker falls inside the
// chart's 06:00-21:00 window.
var testTime = time.Date(2026, 3, 16, 14, 30, 0, 0, time.UTC)

func fixedClock(t time.Time) func() time.Time { return func() time.Time { return t } }

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

// sampleForecast covers all five columns, with rain on the middle days
// and a dry last day so the dry path is exercised by the full render.
func sampleForecast() *weather.Forecast {
	var days []weather.DailyForecast
	for i := range columns {
		var hourly []weather.HourlyPoint
		for h := range 24 {
			prob := 0.0
			if i > 0 && i < columns-1 && h >= 12 && h <= 16 {
				prob = 0.3 + 0.15*float64(i)
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

func ev(summary string, day, hour int) ical.Event {
	start := time.Date(2026, 3, day, hour, 0, 0, 0, time.UTC)
	return ical.Event{UID: summary, Summary: summary, Start: start, End: start.Add(time.Hour)}
}

// sampleEvents gives each column a different shape: Monday packed past
// the cap, Tuesday sparse, Wednesday all-day only, Thursday empty and
// Friday a single long title that has to wrap.
func sampleEvents() []ical.Event {
	return []ical.Event{
		ev("Standup", 16, 9),
		ev("Design review", 16, 11),
		ev("Lunch", 16, 12),
		ev("1:1", 16, 15),
		ev("Retro", 16, 16),
		ev("Dentist", 17, 10),
		{
			UID: "trip", Summary: "Conference", AllDay: true,
			Start: time.Date(2026, 3, 18, 0, 0, 0, 0, time.UTC),
			End:   time.Date(2026, 3, 19, 0, 0, 0, 0, time.UTC),
		},
		ev("Platform architecture review", 20, 14),
	}
}

func newWidget(t *testing.T, cal calendar.Source, ws weather.Source) *Widget {
	t.Helper()
	return New(image.Rect(0, 0, 800, 480), cal, ws, fixedClock(testTime), Config{
		MaxEvents: defaultMaxEvents,
		TempUnit:  "C",
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
	w := newWidget(t, &stubCalSource{}, nil)
	if got := w.Bounds(); got != image.Rect(0, 0, 800, 480) {
		t.Errorf("Bounds = %v", got)
	}
}

// The window asked of the calendar is five days from local midnight —
// the span the five columns actually cover.
func TestWidget_RequestsFiveDaysFromToday(t *testing.T) {
	cal := &stubCalSource{}
	ws := &stubWeatherSource{forecast: sampleForecast()}
	renderToFrame(t, newWidget(t, cal, ws))

	wantStart := time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC)
	if !cal.gotStart.Equal(wantStart) {
		t.Errorf("start = %v, want %v", cal.gotStart, wantStart)
	}
	if got := cal.gotEnd.Sub(cal.gotStart); got != 5*24*time.Hour {
		t.Errorf("window = %v, want 120h", got)
	}
	if ws.gotDays != columns {
		t.Errorf("forecast days = %d, want %d", ws.gotDays, columns)
	}
}

// A fetch failure on either side must leave a usable panel rather than
// a blank one.
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
			frame := renderToFrame(t, newWidget(t, tt.cal, tt.ws))
			// The headers do not depend on either fetch, so the panel
			// still carries its five date numerals.
			if countIndex(frame, widget.PaperBlack) == 0 {
				t.Error("nothing rendered at all")
			}
		})
	}
}

// With no weather source configured at all the calendar half must still
// render — the widget is a calendar first.
func TestWidget_NoWeatherSource(t *testing.T) {
	frame := renderToFrame(t, newWidget(t, &stubCalSource{events: sampleEvents()}, nil))
	if countIndex(frame, widget.PaperBlack) == 0 {
		t.Error("nothing rendered")
	}
}

// Four dividers for five columns, and none after the last one.
func TestWidget_DrawsColumnDividers(t *testing.T) {
	frame := renderToFrame(t, newWidget(t, &stubCalSource{}, &stubWeatherSource{forecast: sampleForecast()}))

	for i, col := range computeColumns(image.Rect(0, 0, 800, 480)) {
		x := col.Bounds.Max.X - 1
		inked := 0
		for y := range 480 {
			if frame.ColorIndexAt(x, y) == widget.PaperBlack {
				inked++
			}
		}
		if col.IsLast {
			if inked == 480 {
				t.Errorf("column %d: a full-height rule was drawn after the last column", i)
			}
			continue
		}
		if inked != 480 {
			t.Errorf("column %d: divider is %d/480 px tall", i, inked)
		}
	}
}

// Today is the leftmost column and gets no highlight, so the five
// header bands must be structurally alike — none of them inverted.
func TestWidget_NoColumnIsHighlighted(t *testing.T) {
	frame := renderToFrame(t, newWidget(t, &stubCalSource{events: sampleEvents()}, &stubWeatherSource{forecast: sampleForecast()}))

	for i, col := range computeColumns(image.Rect(0, 0, 800, 480)) {
		black := countIndexIn(frame, col.Header, widget.PaperBlack)
		if area := col.Header.Dx() * col.Header.Dy(); black > area/2 {
			t.Errorf("column %d header is %d/%d black — it looks inverted", i, black, area)
		}
	}
}

// Golden renders of the whole panel. These are the regression net for
// the geometry: every constant in this package shows up in them.
func TestWidget_Golden(t *testing.T) {
	tests := []struct {
		label string
		cal   *stubCalSource
		ws    *stubWeatherSource
		cfg   func(*Config)
	}{
		{
			label: "full week",
			cal:   &stubCalSource{events: sampleEvents()},
			ws:    &stubWeatherSource{forecast: sampleForecast()},
		},
		{
			label: "no events at all",
			cal:   &stubCalSource{},
			ws:    &stubWeatherSource{forecast: sampleForecast()},
		},
		{
			label: "no weather at all",
			cal:   &stubCalSource{events: sampleEvents()},
			ws:    &stubWeatherSource{},
		},
		{
			label: "locations shown",
			cal: &stubCalSource{events: []ical.Event{
				{
					UID: "l", Summary: "Lunch", Location: "Cafe",
					Start: time.Date(2026, 3, 16, 12, 0, 0, 0, time.UTC),
					End:   time.Date(2026, 3, 16, 13, 0, 0, 0, time.UTC),
				},
			}},
			ws:  &stubWeatherSource{forecast: sampleForecast()},
			cfg: func(c *Config) { c.ShowLocation = true },
		},
		{
			label: "fahrenheit",
			cal:   &stubCalSource{events: sampleEvents()},
			ws:    &stubWeatherSource{forecast: sampleForecast()},
			cfg:   func(c *Config) { c.TempUnit = "F" },
		},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			cfg := Config{MaxEvents: defaultMaxEvents, TempUnit: "C"}
			if tt.cfg != nil {
				tt.cfg(&cfg)
			}
			w := New(image.Rect(0, 0, 800, 480), tt.cal, tt.ws, fixedClock(testTime), cfg)
			testutil.AssertGoldenPNG(t, renderToFrame(t, w))
		})
	}
}

func TestFactory(t *testing.T) {
	cfg := map[string]any{"feeds": []any{"https://example.com/a.ics"}}
	w, err := Factory(image.Rect(0, 0, 800, 480), cfg, widget.Deps{Now: fixedClock(testTime)})
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

// With no clock injected the widget falls back to the wall clock rather
// than a zero time, which would render the epoch.
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

// An injected weather_source wins over the shared provider, which is
// what lets a test drive the widget without a network.
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

// An injected HTTP client must reach the calendar source, or tests and
// custom transports would silently fall back to http.DefaultClient.
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

// stubHTTPClient records that the calendar source reached for it.
type stubHTTPClient struct{ called bool }

func (s *stubHTTPClient) Do(_ *nethttp.Request) (*nethttp.Response, error) {
	s.called = true
	return nil, context.DeadlineExceeded
}

// mustLoadFace runs at package init with valid embedded fonts. Pin its
// failure branch by swapping in data that will not parse.
func TestMustLoadFace_PanicsOnFontError(t *testing.T) {
	restore := fonts.SwapDataForTest([]byte("bad"), []byte("bad"))
	defer restore()

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected a panic")
		}
		msg, ok := r.(string)
		if !ok || !strings.Contains(msg, "boldfive: load smoke font") {
			t.Errorf("panic = %v, want a string naming the failed face", r)
		}
	}()
	_ = mustLoadFace(fonts.Regular, 16, "smoke")
}

// With no weather_source injected, the widget draws from the shared
// provider bound to its resolved model, so every weather widget on the
// dashboard deduplicates fetches through one cache.
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
	bw := w.(*Widget)
	if bw.weather == nil {
		t.Fatal("no weather source taken from the provider")
	}
	// The provider's defaults reach the widget, so a dashboard sets
	// location once at the top level.
	if bw.config.Latitude != 43.25 {
		t.Errorf("Latitude = %v, want the provider default", bw.config.Latitude)
	}
}
