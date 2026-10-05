package boldfive

import (
	"context"
	"errors"
	"image"
	"math"
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
		// Each day warms from its low before dawn to its high mid
		// afternoon, and the days differ, so the combined chart's line
		// has a shape and the shared scale has something to compare.
		high := []float64{14, 9, 17, 4, 12}[i]
		low := high - []float64{8, 5, 9, 6, 4}[i]
		var hourly []weather.HourlyPoint
		for h := range 24 {
			prob := 0.0
			if i > 0 && i < columns-1 && h >= 12 && h <= 16 {
				prob = 0.3 + 0.15*float64(i)
			}
			warmth := math.Exp(-math.Pow(float64(h-15), 2) / 40)
			hourly = append(hourly, weather.HourlyPoint{
				Hour:              h,
				Temperature:       low + (high-low)*warmth,
				PrecipitationProb: prob,
			})
		}
		days = append(days, weather.DailyForecast{
			Date:      time.Date(2026, 3, 16+i, 0, 0, 0, 0, time.UTC),
			High:      high,
			Low:       low,
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

// dryForecast is sampleForecast with the rain taken out, so every column
// is a dry day.
func dryForecast() *weather.Forecast {
	f := sampleForecast()
	for i := range f.Days {
		for h := range f.Days[i].Hourly {
			f.Days[i].Hourly[h].PrecipitationProb = 0
		}
	}
	return f
}

// rainyForecast is sampleForecast with rain on every day, building
// through the afternoon and easing off in the evening.
func rainyForecast() *weather.Forecast {
	f := sampleForecast()
	for i := range f.Days {
		for h := range f.Days[i].Hourly {
			if h >= 10 && h <= 19 {
				f.Days[i].Hourly[h].PrecipitationProb = 0.3 + 0.07*float64(min(h-10, 19-h)) + 0.05*float64(i)
			}
		}
	}
	return f
}

// columnChart is the chart cell of one column, in frame coordinates.
func columnChart(col columnLayout) image.Rectangle {
	return image.Rect(col.Weather.Min.X+chartPadX, col.Weather.Min.Y+chartTop, col.Weather.Max.X-chartPadX, col.Weather.Max.Y)
}

// Every column carries the combined chart, so a dry day still draws the
// temperature line and no column's chart band is left empty. Five
// columns that read unevenly look like a rendering fault.
func TestWidget_DryDayStillDrawsAChart(t *testing.T) {
	frame := renderToFrame(t, newWidget(t, &stubCalSource{}, &stubWeatherSource{forecast: dryForecast()}))

	for i, col := range computeColumns(image.Rect(0, 0, 800, 480)) {
		chart := columnChart(col)
		// Above the baseline and its ticks, so the axis alone does not
		// count: only the temperature line can ink the plot of a dry day.
		plot := image.Rect(chart.Min.X, chart.Min.Y, chart.Max.X, chart.Max.Y-chartLabelH-4)
		if countIndexIn(frame, plot, widget.PaperBlack) == 0 {
			t.Errorf("column %d: a dry day drew nothing in its chart plot", i)
		}
	}
}

// topInkRow is the first row in r carrying PaperBlack in column x, or -1.
func topInkRow(frame *image.Paletted, x int, r image.Rectangle) int {
	for y := r.Min.Y; y < r.Max.Y; y++ {
		if frame.ColorIndexAt(x, y) == widget.PaperBlack {
			return y
		}
	}
	return -1
}

// One temperature scale is shared by all five columns, so a cold day's
// line sits visibly lower than a warm one's. Per-column scales would
// stretch every day to fill its own chart and draw both lines at the
// same height.
func TestWidget_ColumnsShareOneTemperatureScale(t *testing.T) {
	f := dryForecast()
	for i := range f.Days {
		temp := 0.0
		if i == 1 {
			temp = 25
		}
		f.Days[i].High, f.Days[i].Low = temp, temp
		for h := range f.Days[i].Hourly {
			f.Days[i].Hourly[h].Temperature = temp
		}
	}
	frame := renderToFrame(t, newWidget(t, &stubCalSource{}, &stubWeatherSource{forecast: f}))

	cols := computeColumns(image.Rect(0, 0, 800, 480))
	// Hour 9 is a quiet column of the chart: no now-marker, no label.
	lineY := func(col columnLayout) int {
		chart := columnChart(col)
		x := chart.Min.X + chart.Dx()*(9-6)/16 + 2
		return topInkRow(frame, x, chart)
	}
	cold, warm := lineY(cols[2]), lineY(cols[1])
	if cold < 0 || warm < 0 {
		t.Fatalf("no line found: cold=%d warm=%d", cold, warm)
	}
	if warm >= cold {
		t.Errorf("warm day's line at y=%d is not above the cold day's at y=%d", warm, cold)
	}
}

// belowHeader is where the example config places bold-five: under a
// fuzzy_clock header band and the rule beneath it.
var belowHeader = image.Rect(0, 48, 800, 480)

// paintNeighbours inks every frame row outside bounds solid black,
// standing in for the widgets the compositor put above and below.
func paintNeighbours(frame *image.Paletted, bounds image.Rectangle) {
	for y := range frame.Bounds().Dy() {
		if y >= bounds.Min.Y && y < bounds.Max.Y {
			continue
		}
		for x := range frame.Bounds().Dx() {
			frame.SetColorIndex(x, y, widget.PaperBlack)
		}
	}
}

// busiestDay is a Monday packed with wrapped titles, more than the
// column can hold.
func busiestDay() []ical.Event {
	var out []ical.Event
	for i, title := range []string{
		"Platform architecture review", "Quarterly planning session",
		"Dentist appointment downtown", "Parent council meeting", "Swim lessons",
	} {
		e := ev(title, 16, 8+2*i)
		e.UID = title
		out = append(out, e)
	}
	return out
}

// Placed under a header band, the widget keeps every pixel inside its
// own bounds: the clock above and anything below are left untouched.
func TestWidget_StaysInsideBoundsBelowAHeaderBand(t *testing.T) {
	tests := []struct {
		label  string
		bounds image.Rectangle
	}{
		{"below a header band", belowHeader},
		{"above another widget", image.Rect(0, 0, 800, 440)},
		{"between two widgets", image.Rect(0, 48, 800, 440)},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			frame := image.NewPaletted(image.Rect(0, 0, 800, 480), widget.PaperPalette)
			paintNeighbours(frame, tt.bounds)
			w := New(tt.bounds, &stubCalSource{events: append(sampleEvents(), busiestDay()...)},
				&stubWeatherSource{forecast: sampleForecast()}, fixedClock(testTime),
				Config{MaxEvents: 3, Weather: daygrid.WeatherConfig{TempUnit: "C"}})
			if err := w.Render(frame); err != nil {
				t.Fatalf("Render: %v", err)
			}
			for y := range 480 {
				if y >= tt.bounds.Min.Y && y < tt.bounds.Max.Y {
					continue
				}
				for x := range 800 {
					if frame.ColorIndexAt(x, y) != widget.PaperBlack {
						t.Fatalf("painted over a neighbouring widget at (%d,%d)", x, y)
					}
				}
			}
		})
	}
}

// Under the header band the example config uses, a column still fits
// three events whose titles wrap, plus the line counting the rest.
func TestWidget_BelowHeaderFitsThreeWrappedEventsAndTheCount(t *testing.T) {
	frame := image.NewPaletted(image.Rect(0, 0, 800, 480), widget.PaperPalette)
	w := New(belowHeader, &stubCalSource{events: busiestDay()}, &stubWeatherSource{forecast: sampleForecast()},
		fixedClock(testTime), Config{MaxEvents: 3, Weather: daygrid.WeatherConfig{TempUnit: "C"}})
	if err := w.Render(frame); err != nil {
		t.Fatalf("Render: %v", err)
	}

	today := computeColumns(belowHeader)[0].Events
	lineH := daygrid.BodyLineH()
	moreY := today.Min.Y + eventsTopPad + daygrid.BodyAscent() + 3*(3*lineH+eventsGap)
	if !moreLineAt(frame, today, moreY, "+2 MORE") {
		t.Errorf("no \"+2 MORE\" line under three events at baseline %d", moreY)
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
		// bounds defaults to the whole panel.
		bounds image.Rectangle
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
			cfg:   func(c *Config) { c.Weather.TempUnit = "F" },
		},
		// The example config's layout: under a fuzzy_clock header band,
		// three events a column.
		{
			label:  "below header dry day",
			cal:    &stubCalSource{events: sampleEvents()},
			ws:     &stubWeatherSource{forecast: dryForecast()},
			cfg:    func(c *Config) { c.MaxEvents = 3 },
			bounds: belowHeader,
		},
		{
			label:  "below header rainy day",
			cal:    &stubCalSource{events: sampleEvents()},
			ws:     &stubWeatherSource{forecast: rainyForecast()},
			cfg:    func(c *Config) { c.MaxEvents = 3 },
			bounds: belowHeader,
		},
		{
			label:  "below header busiest day",
			cal:    &stubCalSource{events: append(busiestDay(), sampleEvents()[5:]...)},
			ws:     &stubWeatherSource{forecast: sampleForecast()},
			cfg:    func(c *Config) { c.MaxEvents = 3 },
			bounds: belowHeader,
		},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			cfg := Config{MaxEvents: defaultMaxEvents, Weather: daygrid.WeatherConfig{TempUnit: "C"}}
			if tt.cfg != nil {
				tt.cfg(&cfg)
			}
			bounds := tt.bounds
			if bounds.Empty() {
				bounds = image.Rect(0, 0, 800, 480)
			}
			w := New(bounds, tt.cal, tt.ws, fixedClock(testTime), cfg)
			testutil.AssertGoldenPNG(t, renderToFrame(t, w))
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
	if err == nil || !strings.Contains(err.Error(), "bold-five: no HTTP client") {
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

// Every renderer places content at a fixed offset from its band's top,
// and the draw helpers clip to the frame rather than to the widget's
// bounds. A widget given less room than the bands need would therefore
// paint over its neighbour on the shared frame — so it draws nothing
// instead. A blank region is a misconfiguration you can see; ink on top
// of another widget looks like a fault somewhere else entirely.
func TestWidget_TooShortDrawsNothing(t *testing.T) {
	frame := image.NewPaletted(image.Rect(0, 0, 800, 480), widget.PaperPalette)

	// A neighbour already on the frame, below where this widget sits.
	for y := 200; y < 480; y++ {
		for x := range 800 {
			frame.SetColorIndex(x, y, widget.PaperBlack)
		}
	}

	w := New(image.Rect(0, 0, 800, 150), &stubCalSource{events: sampleEvents()},
		&stubWeatherSource{forecast: sampleForecast()}, fixedClock(testTime),
		Config{MaxEvents: defaultMaxEvents, Weather: daygrid.WeatherConfig{TempUnit: "C"}})
	if err := w.Render(frame); err != nil {
		t.Fatalf("Render: %v", err)
	}

	// Its own region is blank, and the neighbour below is untouched.
	if got := countIndexIn(frame, image.Rect(0, 0, 800, 150), widget.PaperBlack); got != 0 {
		t.Errorf("drew %d px into a widget too short to draw into", got)
	}
	for y := 200; y < 480; y++ {
		for x := range 800 {
			if frame.ColorIndexAt(x, y) != widget.PaperBlack {
				t.Fatalf("painted over the neighbouring widget at (%d,%d)", x, y)
			}
		}
	}
}
