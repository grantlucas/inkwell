package rowagenda

import (
	"context"
	"errors"
	"image"
	nethttp "net/http"
	"slices"
	"strings"
	"sync"
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

// sampleForecast covers today plus the four rows, with rain today (so
// the hero chart draws bars and its marker) and a dry last day.
func sampleForecast() *weather.Forecast {
	var days []weather.DailyForecast
	for i := range rows {
		var hourly []weather.HourlyPoint
		for h := range 24 {
			prob := 0.0
			if i < rows-1 && h >= 12 && h <= 17 {
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

// sampleEvents is a busy week that still fits: five events today, four
// on Thursday and an empty Friday, with every event getting a line.
func sampleEvents() []ical.Event {
	return []ical.Event{
		ev("Standup", 16, 9), // finished by 14:30
		ev("Platform architecture review with infra", 16, 16), // still to come
		ev("1:1", 16, 17),
		ev("Retro", 16, 18),
		ev("Grocery run", 16, 19),
		ev("Dentist - Maeve", 17, 10),
		{
			UID: "trip", Summary: "Conference", AllDay: true,
			Start: time.Date(2026, 3, 18, 0, 0, 0, 0, time.UTC),
			End:   time.Date(2026, 3, 19, 0, 0, 0, 0, time.UTC),
		},
		ev("Standup", 19, 9), ev("Quarterly planning", 19, 11), ev("Review", 19, 14), ev("Demo", 19, 16),
	}
}

// quietEvents is a week with almost nothing in it, so every row sits
// at the minimum height plus an even share of the spare room.
func quietEvents() []ical.Event {
	return []ical.Event{
		ev("Sabres @ Stoney Creek", 16, 10),
		ev("Practice Green", 19, 18),
	}
}

// overflowingEvents is more than five rows can hold: Monday, Wednesday
// and Friday are packed and lose lines, while the quiet Tuesday and
// Thursday keep every event.
func overflowingEvents() []ical.Event {
	var out []ical.Event
	for h := range 9 {
		out = append(out, ev("Back-to-back meeting number "+string(rune('A'+h)), 16, 8+h))
	}
	out = append(out, ev("Dentist", 17, 10), ev("Book club", 17, 19))
	for h := range 7 {
		out = append(out, ev("Workshop session "+string(rune('A'+h)), 18, 9+h))
	}
	out = append(out, ev("Swim lessons", 19, 17))
	for h := range 6 {
		out = append(out, ev("Interview loop "+string(rune('A'+h)), 20, 9+h))
	}
	return out
}

// withoutToday drops today's events, leaving today's row empty.
func withoutToday(events []ical.Event) []ical.Event {
	var out []ical.Event
	for _, e := range events {
		if e.Start.Day() != 16 {
			out = append(out, e)
		}
	}
	return out
}

func newWidget(cal calendar.Source, ws weather.Source, clock time.Time) *Widget {
	return New(image.Rect(0, 0, 800, 480), cal, ws, fixedClock(clock), Config{
		Weather: daygrid.WeatherConfig{TempUnit: "C"},
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
	if ws.gotDays != rows {
		t.Errorf("forecast days = %d, want %d", ws.gotDays, rows)
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
				Config{Weather: daygrid.WeatherConfig{TempUnit: "C"}})
			if err := w.Render(frame); err != nil {
				t.Fatalf("Render: %v", err)
			}
			if got := countIndexIn(frame, frame.Bounds(), widget.PaperBlack); got != 0 {
				t.Errorf("drew %d px into bounds too small to draw into", got)
			}
		})
	}
}

// typedDeps is what the app hands every widget: one transport behind both
// the calendar fetch and the shared weather provider.
func typedDeps(client *recordingTransport) widget.Deps {
	return widget.Deps{
		Now:        fixedClock(testTime),
		HTTPClient: client,
		Weather: weather.NewProvider(client, time.Hour, fixedClock(testTime), weather.Settings{
			Location: weather.Location{Latitude: 43.25, Longitude: -79.87},
			TempUnit: "C",
			Model:    weather.ModelGEM,
		}),
	}
}

func TestFactory(t *testing.T) {
	cfg := map[string]any{"feeds": []any{"https://example.com/a.ics"}}
	w, err := Factory(image.Rect(0, 0, 800, 480), cfg, typedDeps(&recordingTransport{}))
	if err != nil {
		t.Fatalf("Factory: %v", err)
	}
	if got := w.Bounds(); got != image.Rect(0, 0, 800, 480) {
		t.Errorf("Bounds = %v", got)
	}
}

func TestFactory_InvalidConfig(t *testing.T) {
	_, err := Factory(image.Rect(0, 0, 800, 480), map[string]any{}, typedDeps(&recordingTransport{}))
	if err == nil {
		t.Fatal("expected an error for missing feeds")
	}
	if !strings.Contains(err.Error(), "feeds is required") {
		t.Errorf("error = %q", err)
	}
}

// A widget built without its dependencies fails instead of falling back
// to a default HTTP client the app never chose.
func TestFactory_MissingDeps(t *testing.T) {
	_, err := Factory(image.Rect(0, 0, 800, 480), map[string]any{
		"feeds": []any{"https://example.com/a.ics"},
	}, widget.Deps{Now: fixedClock(testTime)})
	if err == nil || !strings.Contains(err.Error(), "row-agenda: no HTTP client") {
		t.Errorf("error = %v, want a missing HTTP client error", err)
	}
}

// With no clock injected the widget falls back to the wall clock rather
// than a zero time, which would render the epoch.
func TestFactory_DefaultsTheClock(t *testing.T) {
	deps := typedDeps(&recordingTransport{})
	deps.Now = nil
	w, err := Factory(image.Rect(0, 0, 800, 480), map[string]any{
		"feeds": []any{"https://example.com/a.ics"},
	}, deps)
	if err != nil {
		t.Fatalf("Factory: %v", err)
	}
	if got := w.(*Widget).now().Year(); got < 2024 {
		t.Errorf("clock year = %d, want the real wall clock", got)
	}
}

// The calendar feed goes out through the injected client, and the forecast
// through the shared provider at its default location, so a dashboard sets
// its location once at the top level.
func TestFactory_FetchesThroughTypedDeps(t *testing.T) {
	client := &recordingTransport{}
	w, err := Factory(image.Rect(0, 0, 800, 480), map[string]any{
		"feeds": []any{"https://example.com/a.ics"},
	}, typedDeps(client))
	if err != nil {
		t.Fatalf("Factory: %v", err)
	}
	renderToFrame(t, w.(*Widget))

	if !client.requested("https://example.com/a.ics") {
		t.Errorf("calendar feed not fetched through the injected client; requests: %v", client.urls())
	}
	if !client.requested("api.open-meteo.com/v1/gem", "latitude=43.2500") {
		t.Errorf("forecast not fetched through the shared provider; requests: %v", client.urls())
	}
}

// recordingTransport records every URL it is asked for and answers none,
// so a test can see what a widget fetched without a network.
type recordingTransport struct {
	mu   sync.Mutex
	seen []string
}

func (r *recordingTransport) Do(req *nethttp.Request) (*nethttp.Response, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.seen = append(r.seen, req.URL.String())
	return nil, context.DeadlineExceeded
}

func (r *recordingTransport) urls() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.seen)
}

// requested reports whether any one request carried every fragment.
func (r *recordingTransport) requested(fragments ...string) bool {
	return slices.ContainsFunc(r.urls(), func(u string) bool {
		return !slices.ContainsFunc(fragments, func(f string) bool { return !strings.Contains(u, f) })
	})
}

// Today is shown by position — it is the first row — and never by a
// fill. The date gutter is the same plain date on every row, so no
// large black area lands in the same place every refresh and burns into
// the panel.
func TestWidget_NoFilledDateGutter(t *testing.T) {
	frame := renderToFrame(t, newWidget(&stubCalSource{events: sampleEvents()},
		&stubWeatherSource{forecast: sampleForecast()}, testTime))

	const band = 48
	for y := 0; y < 480; y += band {
		r := image.Rect(0, y, gutterW, y+band)
		if black := countIndexIn(frame, r, widget.PaperBlack); black > r.Dx()*r.Dy()/4 {
			t.Errorf("the gutter at y=%d is %d/%d black — a filled block, not a plain date", y, black, r.Dx()*r.Dy())
		}
	}
	if countIndexIn(frame, image.Rect(0, 0, gutterW, 480), widget.PaperWhite) == 0 {
		t.Error("the gutter has no paper at all")
	}
}

func TestWidget_Golden(t *testing.T) {
	tests := []struct {
		label string
		cal   *stubCalSource
		ws    *stubWeatherSource
		cfg   func(*Config)
	}{
		{
			// Every event gets a line: today's row and Thursday's grow,
			// the rest share the spare room, Friday says it is empty.
			label: "a busy week that fits",
			cal:   &stubCalSource{events: sampleEvents()},
			ws:    &stubWeatherSource{forecast: sampleForecast()},
		},
		{
			label: "a quiet week",
			cal:   &stubCalSource{events: quietEvents()},
			ws:    &stubWeatherSource{forecast: sampleForecast()},
		},
		{
			// The packed rows trim toward each other and end in
			// "+N MORE"; the quiet rows keep everything.
			label: "a week that overflows",
			cal:   &stubCalSource{events: overflowingEvents()},
			ws:    &stubWeatherSource{forecast: sampleForecast()},
		},
		{
			// Today's own row is the empty one.
			label: "an empty day",
			cal:   &stubCalSource{events: withoutToday(sampleEvents())},
			ws:    &stubWeatherSource{forecast: sampleForecast()},
		},
		{
			label: "no events at all",
			cal:   &stubCalSource{},
			ws:    &stubWeatherSource{forecast: sampleForecast()},
		},
		{
			label: "a dry week",
			cal:   &stubCalSource{events: sampleEvents()},
			ws:    &stubWeatherSource{forecast: dryForecast()},
		},
		{
			label: "no weather at all",
			cal:   &stubCalSource{events: sampleEvents()},
			ws:    &stubWeatherSource{},
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
			cfg := Config{Weather: daygrid.WeatherConfig{TempUnit: "C"}}
			if tt.cfg != nil {
				tt.cfg(&cfg)
			}
			w := New(image.Rect(0, 0, 800, 480), tt.cal, tt.ws, fixedClock(testTime), cfg)
			testutil.AssertGoldenPNG(t, renderToFrame(t, w))
		})
	}
}
