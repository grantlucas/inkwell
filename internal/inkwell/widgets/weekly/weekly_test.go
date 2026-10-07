package weekly

import (
	"image"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar"
	"github.com/grantlucas/inkwell/internal/inkwell/calendar/ical"
	"github.com/grantlucas/inkwell/internal/inkwell/testutil"
	"github.com/grantlucas/inkwell/internal/inkwell/testutil/fakehttp"
	"github.com/grantlucas/inkwell/internal/inkwell/weather"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daydata"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/weatherview"
)

func fixedClock(t time.Time) func() time.Time {
	return func() time.Time { return t }
}

var testTime = time.Date(2026, 4, 27, 14, 30, 0, 0, time.UTC) // Monday

func sampleEvents() []ical.Event {
	return []ical.Event{
		{
			UID:     "1",
			Summary: "Standup",
			Start:   time.Date(2026, 4, 27, 9, 0, 0, 0, time.UTC),
			End:     time.Date(2026, 4, 27, 9, 30, 0, 0, time.UTC),
		},
		{
			UID:     "2",
			Summary: "Game Night",
			Start:   time.Date(2026, 4, 29, 19, 0, 0, 0, time.UTC),
			End:     time.Date(2026, 4, 29, 22, 0, 0, 0, time.UTC),
		},
		{
			UID:      "3",
			Summary:  "Lunch",
			Location: "Cafe",
			Start:    time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC),
			End:      time.Date(2026, 4, 28, 13, 0, 0, 0, time.UTC),
		},
	}
}

func sampleForecast() []weather.DailyForecast {
	var days []weather.DailyForecast
	for i := range 7 {
		day := time.Date(2026, 4, 27+i, 0, 0, 0, 0, time.UTC)
		var hourly []weather.HourlyPoint
		for h := range 24 {
			hourly = append(hourly, weather.HourlyPoint{
				Hour:              h,
				Temperature:       10 + float64(i) + float64(h)/4,
				PrecipitationProb: 0.1 * float64(i),
			})
		}
		days = append(days, weather.DailyForecast{
			Date:      day,
			High:      20 + float64(i),
			Low:       8 + float64(i),
			Condition: weather.Condition(i % 4),
			Hourly:    hourly,
		})
	}
	return days
}

// drawConfig is the config the goldens draw with: the full week, weather
// shown with its label, Celsius.
func drawConfig() Config {
	return Config{
		Config: daydata.Config{
			MaxEvents: 5,
			Weather:   daydata.WeatherConfig{TempUnit: "C", Latitude: 45.4, Longitude: -75.7},
		},
		WeekStart:        time.Monday,
		Days:             defaultDays,
		ShowWeather:      true,
		ShowWeatherLabel: true,
		HighlightHour:    15,
	}
}

func render(t *testing.T, w *Widget) *image.Paletted {
	t.Helper()
	frame := image.NewPaletted(image.Rect(0, 0, 800, 480), widget.PaperPalette)
	if err := w.Render(frame); err != nil {
		t.Fatalf("Render: %v", err)
	}
	return frame
}

func TestWidget_Bounds(t *testing.T) {
	bounds := image.Rect(0, 52, 800, 480)
	w := New(bounds, daydata.InMemory(nil, nil), fixedClock(testTime), drawConfig())
	if got := w.Bounds(); got != bounds {
		t.Errorf("Bounds() = %v, want %v", got, bounds)
	}
}

// The weather band's height follows from whether a forecast arrived at
// all, not from whether it reaches any day shown. A 200 response with no
// daily data still keeps the band, so one odd cycle doesn't reflow the
// whole screen; only no forecast at all gives the height back to the
// events, exactly as show_weather off does.
func TestWidget_BandFollowsWhetherAForecastArrived(t *testing.T) {
	bounds := image.Rect(0, 52, 800, 480)
	hidden := drawConfig()
	hidden.ShowWeather = false
	weatherOff := render(t, New(bounds, daydata.InMemory(sampleEvents(), sampleForecast()), fixedClock(testTime), hidden))

	nextYear := sampleForecast()
	for i := range nextYear {
		nextYear[i].Date = nextYear[i].Date.AddDate(1, 0, 0)
	}

	tests := []struct {
		label    string
		forecast []weather.DailyForecast
		wantBand bool
	}{
		{label: "a forecast for the week", forecast: sampleForecast(), wantBand: true},
		{label: "a forecast carrying no days", forecast: []weather.DailyForecast{}, wantBand: true},
		{label: "a forecast reaching none of the days", forecast: nextYear, wantBand: true},
		{label: "no forecast at all", forecast: nil, wantBand: false},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			frame := render(t, New(bounds, daydata.InMemory(sampleEvents(), tt.forecast), fixedClock(testTime), drawConfig()))
			if gotBand := !slices.Equal(frame.Pix, weatherOff.Pix); gotBand != tt.wantBand {
				t.Errorf("weather band drawn = %v, want %v", gotBand, tt.wantBand)
			}
		})
	}
}

// Settings no golden covers still render.
func TestWidget_RendersOtherSettings(t *testing.T) {
	tests := []struct {
		label string
		cfg   func(*Config)
	}{
		{"sunday week start", func(c *Config) { c.WeekStart = time.Sunday }},
		{"fahrenheit", func(c *Config) { c.Weather.TempUnit = "F" }},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			cfg := drawConfig()
			tt.cfg(&cfg)
			frame := render(t, New(image.Rect(0, 0, 800, 480),
				daydata.InMemory(sampleEvents(), sampleForecast()), fixedClock(testTime), cfg))
			if !slices.ContainsFunc(frame.Pix, func(px uint8) bool { return px != 0 }) {
				t.Error("render produced a blank frame")
			}
		})
	}
}

// minimalConfig is the minimum valid config: feeds is the only required key.
func minimalConfig() map[string]any {
	return map[string]any{
		"feeds": []any{"https://example.com/cal.ics"},
	}
}

func withKey(k string, v any) map[string]any {
	c := minimalConfig()
	c[k] = v
	return c
}

func TestFactory(t *testing.T) {
	deps := widget.Deps{
		Now:      fixedClock(testTime),
		Calendar: calendar.NewProvider(fakehttp.New(), fixedClock(testTime)),
		Weather:  weather.NewProvider(fakehttp.New(), time.Hour, fixedClock(testTime), weather.Settings{TempUnit: "F"}),
	}
	tests := []struct {
		label   string
		config  map[string]any
		deps    widget.Deps
		wantErr string
	}{
		{label: "builds from typed deps", config: minimalConfig(), deps: deps},
		{
			// The weekly-calendar screen in inkwell.example.yaml.
			label: "the example config", deps: deps,
			config: map[string]any{
				"feeds":              []any{"https://example.com/my-calendar.ics"},
				"days":               7,
				"show_weather":       true,
				"show_weather_label": true,
				"max_events":         5,
			},
		},
		{
			// A config using every key weekly-calendar has ever taken,
			// the unused week_start and highlight_hour included.
			label: "every key it takes", deps: deps,
			config: map[string]any{
				"feeds": []any{map[string]any{
					"url":   "https://example.com/team.ics",
					"name":  "Hockey",
					"rules": []any{map[string]any{"match": "^Jane Doe\n"}},
				}},
				"refresh":            "30m",
				"days":               5,
				"max_events":         4,
				"show_location":      true,
				"show_weather":       true,
				"show_weather_label": false,
				"week_start":         "sunday",
				"highlight_hour":     12,
				"latitude":           40.7128,
				"longitude":          -74.006,
				"temp_unit":          "F",
				"weather_model":      "ecmwf",
			},
		},
		{label: "rejects its config", config: map[string]any{}, deps: deps, wantErr: "weekly-calendar: feeds is required"},
		{
			label: "rejects a misspelt key", config: withKey("max_event", 3), deps: deps,
			wantErr: `weekly-calendar: unsupported setting "max_event" (accepted: days, feeds, highlight_hour, latitude, longitude, max_events, refresh, show_location, show_weather, show_weather_label, temp_unit, weather_model, week_start)`,
		},
		{label: "rejects its own key", config: withKey("days", 8), deps: deps, wantErr: "weekly-calendar: days must be in [1, 7], got 8"},
		{label: "needs the calendar module", config: minimalConfig(), deps: widget.Deps{Weather: deps.Weather}, wantErr: "weekly-calendar: no calendar module"},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			w, err := Factory(image.Rect(0, 0, 800, 480), tt.config, tt.deps)
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("err = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Factory: %v", err)
			}
			wc := w.(*Widget)
			if wc.Bounds() != image.Rect(0, 0, 800, 480) {
				t.Errorf("Bounds = %v", wc.Bounds())
			}
		})
	}
}

// With show_weather off the screen fetches no forecast at all, rather
// than one it would throw away; with it on, the forecast comes through
// the shared provider.
func TestFactory_FetchesWeatherOnlyWhenShown(t *testing.T) {
	const feed = "https://example.com/cal.ics"
	const gem = "https://api.open-meteo.com/v1/gem"
	tests := []struct {
		label            string
		showWeather      bool
		forecastRequests int
	}{
		{label: "weather shown", showWeather: true, forecastRequests: 1},
		{label: "weather hidden", showWeather: false, forecastRequests: 0},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			tr := fakehttp.New()
			tr.Serve(feed, "BEGIN:VCALENDAR\r\nEND:VCALENDAR\r\n")
			tr.Handle(gem, fakehttp.OpenMeteo{Now: testTime, Site: time.UTC}.Reply)
			deps := widget.Deps{
				Now:      fixedClock(testTime),
				Calendar: calendar.NewProvider(tr, fixedClock(testTime)),
				Weather:  weather.NewProvider(tr, time.Hour, fixedClock(testTime), weather.Settings{Model: weather.ModelGEM, TempUnit: "C"}),
			}
			w, err := Factory(image.Rect(0, 52, 800, 480), withKey("show_weather", tt.showWeather), deps)
			if err != nil {
				t.Fatalf("Factory: %v", err)
			}
			render(t, w.(*Widget))

			if got := tr.Requests(gem); got != tt.forecastRequests {
				t.Errorf("forecast requests = %d, want %d", got, tt.forecastRequests)
			}
			if got := tr.Requests(feed); got != 1 {
				t.Errorf("feed requests = %d, want 1", got)
			}
		})
	}
}

// weekly-calendar's own keys, each accepted and each rejected with the
// widget's name. The keys every calendar widget shares are the shared
// parser's to test.
func TestParseConfig_OwnKeys(t *testing.T) {
	tests := []struct {
		label   string
		key     string
		value   any
		check   func(Config) bool
		wantErr string
	}{
		{label: "default days", check: func(c Config) bool { return c.Days == 7 }},
		{label: "default max_events", check: func(c Config) bool { return c.MaxEvents == 5 }},
		{label: "weather shown by default", check: func(c Config) bool { return c.ShowWeather && c.ShowWeatherLabel }},
		{label: "days", key: "days", value: 5, check: func(c Config) bool { return c.Days == 5 }},
		{label: "days lower boundary", key: "days", value: 1, check: func(c Config) bool { return c.Days == 1 }},
		{label: "days upper boundary", key: "days", value: 7, check: func(c Config) bool { return c.Days == 7 }},
		{label: "days zero", key: "days", value: 0, wantErr: "days must be in [1, 7]"},
		{label: "days negative", key: "days", value: -1, wantErr: "days must be in [1, 7]"},
		{label: "days above range", key: "days", value: 8, wantErr: "days must be in [1, 7]"},
		{label: "days not an integer", key: "days", value: "five", wantErr: "days must be an integer"},
		{label: "show_weather off", key: "show_weather", value: false, check: func(c Config) bool { return !c.ShowWeather }},
		{label: "show_weather not a bool", key: "show_weather", value: "yes", wantErr: "show_weather must be a bool"},
		{label: "show_weather_label off", key: "show_weather_label", value: false, check: func(c Config) bool { return !c.ShowWeatherLabel }},
		{label: "show_weather_label not a bool", key: "show_weather_label", value: 1, wantErr: "show_weather_label must be a bool"},
		{label: "week_start sunday", key: "week_start", value: "sunday", check: func(c Config) bool { return c.WeekStart == time.Sunday }},
		{label: "week_start monday", key: "week_start", value: "monday", check: func(c Config) bool { return c.WeekStart == time.Monday }},
		{label: "week_start unknown", key: "week_start", value: "friday", wantErr: "invalid week_start"},
		{label: "week_start not a string", key: "week_start", value: 42, wantErr: "week_start must be a string"},
		{label: "highlight_hour", key: "highlight_hour", value: 12, check: func(c Config) bool { return c.HighlightHour == 12 }},
		{label: "highlight_hour lower boundary", key: "highlight_hour", value: 0, check: func(c Config) bool { return c.HighlightHour == 0 }},
		{label: "highlight_hour upper boundary", key: "highlight_hour", value: 23, check: func(c Config) bool { return c.HighlightHour == 23 }},
		{label: "highlight_hour below range", key: "highlight_hour", value: -1, wantErr: "highlight_hour must be in [0, 23]"},
		{label: "highlight_hour above range", key: "highlight_hour", value: 24, wantErr: "highlight_hour must be in [0, 23]"},
		{label: "highlight_hour not an integer", key: "highlight_hour", value: "noon", wantErr: "highlight_hour must be an integer"},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			raw := minimalConfig()
			if tt.key != "" {
				raw[tt.key] = tt.value
			}
			got, err := parseConfig(raw, nil)
			if tt.wantErr != "" {
				if err == nil || !strings.HasPrefix(err.Error(), "weekly-calendar: ") || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want weekly-calendar: ...%s", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseConfig: %v", err)
			}
			if !tt.check(got) {
				t.Errorf("config = %+v", got)
			}
		})
	}
}

// The shared weather Provider answers every span from one forecast of
// ForecastHorizon days, and once the cache has outlived midnight the first of
// those days is gone. The longest span this widget accepts has to fit in what
// remains, or its last column loses its weather.
func TestMaxDaysFitTheForecastHorizon(t *testing.T) {
	if defaultDays > weather.ForecastHorizon-1 {
		t.Errorf("weekly-calendar accepts %d days, but a forecast of %d days covers only %d after midnight",
			defaultDays, weather.ForecastHorizon, weather.ForecastHorizon-1)
	}
}

// TestWidget_HighlightHourUsesDisplayZone pins that the weather column's
// "current hour" marker follows the configured zone too. Two viewers looking
// at the same instant from zones four hours apart must see the marker on
// different hours, so rendering the same frame for both means the highlight is
// still reading the clock's raw hour.
func TestWidget_HighlightHourUsesDisplayZone(t *testing.T) {
	toronto, err := time.LoadLocation("America/Toronto")
	if err != nil {
		t.Fatalf("load America/Toronto: %v", err)
	}

	// Mid-afternoon UTC, late morning in Toronto — same day in both zones, so
	// only the hour differs and the frames stay otherwise comparable.
	now := time.Date(2026, 4, 27, 18, 0, 0, 0, time.UTC)

	renderIn := func(loc *time.Location) *image.Paletted {
		t.Helper()
		w := New(image.Rect(0, 0, 800, 480), daydata.InMemory(nil, sampleForecast()),
			fixedClock(now.In(loc)), drawConfig())
		return render(t, w)
	}

	if slices.Equal(renderIn(time.UTC).Pix, renderIn(toronto).Pix) {
		t.Error("UTC and America/Toronto rendered identically; highlight hour ignores the display zone")
	}
}

func TestWidget_DaysNarrowsTheGrid(t *testing.T) {
	cfg := drawConfig()
	cfg.Days = 5
	frame := render(t, New(image.Rect(0, 52, 800, 480),
		daydata.InMemory(sampleEvents(), sampleForecast()), fixedClock(testTime), cfg))

	// Five columns tile 800 px, so the four dividers land at the 160 px
	// boundaries and nothing is drawn at the 7-day boundary of x=113.
	for _, x := range []int{159, 319, 479, 639} {
		if frame.ColorIndexAt(x, 300) != widget.PaperBlack {
			t.Errorf("no divider at x=%d", x)
		}
	}
	if frame.ColorIndexAt(113, 300) == widget.PaperBlack {
		t.Error("divider at x=113 — still laying out 7 columns")
	}
}

func TestWidget_UnsetDaysRendersFullWeek(t *testing.T) {
	// New is exported and callers build Config literals by hand, so a zero
	// Days must fall back to the full week rather than drawing no columns.
	cfg := drawConfig()
	cfg.Days = 0
	frame := render(t, New(image.Rect(0, 52, 800, 480),
		daydata.InMemory(sampleEvents(), nil), fixedClock(testTime), cfg))

	if frame.ColorIndexAt(113, 300) != widget.PaperBlack {
		t.Error("no divider at x=113 — not laying out 7 columns")
	}
}

// rangeOver serves src's days on a fixed temperature range.
type rangeOver struct {
	daydata.Source
	rng weatherview.TempRange
}

func (r rangeOver) Days(now time.Time, n int) daydata.Data {
	d := r.Source.Days(now, n)
	d.TempRange = r.rng
	return d
}

// TestWidget_Golden is the regression net for refactors of the shared
// calendar + weather scaffolding (issue #92). The extraction is only
// correct if these do not move: every constant, every wrap decision and
// every day-bucketing rule shows up in the pixels.
func TestWidget_Golden(t *testing.T) {
	bounds := image.Rect(0, 52, 800, 480)

	tests := []struct {
		label    string
		events   []ical.Event
		forecast []weather.DailyForecast
		cfg      func(*Config)
		clock    time.Time // zero means testTime in UTC
		// weekRange draws on the range of all seven forecast days
		// rather than of the days shown.
		weekRange bool
	}{
		{
			label:    "full week with weather",
			events:   sampleEvents(),
			forecast: sampleForecast(),
		},
		{
			label:  "no weather source",
			events: sampleEvents(),
			cfg:    func(c *Config) { c.ShowWeather = false },
		},
		{
			label:    "no events",
			forecast: sampleForecast(),
		},
		{
			// This golden was drawn when the test's forecast stub
			// served all seven days to a five-day screen, and the range
			// was taken across every day served. The provider answers
			// with only the days asked for, so in production the range
			// spans the days shown either way; weekRange keeps the
			// golden on the scale it was drawn with.
			label:     "five day columns",
			events:    sampleEvents(),
			forecast:  sampleForecast(),
			cfg:       func(c *Config) { c.Days = 5 },
			weekRange: true,
		},
		{
			label:    "locations shown",
			events:   sampleEvents(),
			forecast: sampleForecast(),
			cfg:      func(c *Config) { c.ShowLocation = true },
		},
		{
			// The all-day bucketing case the extraction must not lose:
			// a VALUE=DATE event is anchored to UTC midnight while the
			// columns are built in the viewer's zone.
			//
			// The clock is Toronto, not UTC, and that is the whole
			// point. In UTC, date-bucketing and plain instant-overlap
			// produce identical pixels, so the golden would pass
			// against the bug it exists to catch. At UTC-4 the
			// Wednesday column starts hours after the event's UTC
			// midnight, so an instant comparison drags the Wednesday
			// event into Tuesday and the pixels move.
			label: "all-day event in a negative-UTC zone",
			clock: testTime.In(mustLoad(t, "America/Toronto")),
			events: []ical.Event{{
				UID: "trip", Summary: "Conference", AllDay: true,
				Start: time.Date(2026, 4, 29, 0, 0, 0, 0, time.UTC),
				End:   time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC),
			}},
			forecast: sampleForecast(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			cfg := drawConfig()
			if tt.cfg != nil {
				tt.cfg(&cfg)
			}
			clock := tt.clock
			if clock.IsZero() {
				clock = testTime
			}
			days := daydata.InMemory(tt.events, tt.forecast)
			if tt.weekRange {
				days = rangeOver{Source: days, rng: weatherview.GlobalTempRange(tt.forecast)}
			}
			testutil.AssertGoldenPNG(t, render(t, New(bounds, days, fixedClock(clock), cfg)))
		})
	}
}
