package weekly

import (
	"context"
	"image"
	nethttp "net/http"
	"strings"
	"testing"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar/ical"
	"github.com/grantlucas/inkwell/internal/inkwell/weather"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
)

func fixedClock(t time.Time) func() time.Time {
	return func() time.Time { return t }
}

// stubCalSource implements calendar.Source for testing.
type stubCalSource struct {
	events []ical.Event
	err    error
}

func (s *stubCalSource) Events(_ context.Context, start, end time.Time) ([]ical.Event, error) {
	if s.err != nil {
		return s.events, s.err
	}
	var filtered []ical.Event
	for _, e := range s.events {
		if e.Start.Before(end) && e.End.After(start) {
			filtered = append(filtered, e)
		}
	}
	return filtered, nil
}

// stubWeatherSource implements weather.Source for testing.
type stubWeatherSource struct {
	forecast *weather.Forecast
	err      error
}

func (s *stubWeatherSource) Forecast(_ context.Context, _ weather.Location, _ int) (*weather.Forecast, error) {
	return s.forecast, s.err
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

func sampleForecast() *weather.Forecast {
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
	return &weather.Forecast{
		Location: weather.Location{Latitude: 45.4, Longitude: -75.7},
		Days:     days,
	}
}

func TestWidget_Bounds(t *testing.T) {
	bounds := image.Rect(0, 52, 800, 480)
	w := New(bounds, &stubCalSource{}, nil, fixedClock(testTime), Config{
		MaxEvents: 5,
		WeekStart: time.Monday,
		TempUnit:  "C",
	})
	if got := w.Bounds(); got != bounds {
		t.Errorf("Bounds() = %v, want %v", got, bounds)
	}
}

func TestWidget_RenderNoWeather(t *testing.T) {
	bounds := image.Rect(0, 52, 800, 480)
	cal := &stubCalSource{events: sampleEvents()}
	w := New(bounds, cal, nil, fixedClock(testTime), Config{
		MaxEvents: 5,
		WeekStart: time.Monday,
		TempUnit:  "C",
	})

	frame := image.NewPaletted(image.Rect(0, 0, 800, 480), widget.PaperPalette)
	if err := w.Render(frame); err != nil {
		t.Fatalf("Render: %v", err)
	}

	hasBlack := false
	for _, px := range frame.Pix {
		if px != 0 {
			hasBlack = true
			break
		}
	}
	if !hasBlack {
		t.Error("render produced blank frame")
	}
}

func TestWidget_RenderWithWeather(t *testing.T) {
	bounds := image.Rect(0, 52, 800, 480)
	cal := &stubCalSource{events: sampleEvents()}
	ws := &stubWeatherSource{forecast: sampleForecast()}
	w := New(bounds, cal, ws, fixedClock(testTime), Config{
		MaxEvents:        5,
		WeekStart:        time.Monday,
		TempUnit:         "C",
		ShowWeather:      true,
		ShowWeatherLabel: true,
		HighlightHour:    15,
		Latitude:         45.4,
		Longitude:        -75.7,
	})

	frame := image.NewPaletted(image.Rect(0, 0, 800, 480), widget.PaperPalette)
	if err := w.Render(frame); err != nil {
		t.Fatalf("Render: %v", err)
	}

	hasBlack := false
	for _, px := range frame.Pix {
		if px != 0 {
			hasBlack = true
			break
		}
	}
	if !hasBlack {
		t.Error("render with weather produced blank frame")
	}
}

func TestWidget_RenderWeatherError(t *testing.T) {
	bounds := image.Rect(0, 52, 800, 480)
	cal := &stubCalSource{events: sampleEvents()}
	ws := &stubWeatherSource{err: context.DeadlineExceeded}
	w := New(bounds, cal, ws, fixedClock(testTime), Config{
		MaxEvents:   5,
		WeekStart:   time.Monday,
		TempUnit:    "C",
		ShowWeather: true,
		Latitude:    45.4,
		Longitude:   -75.7,
	})

	frame := image.NewPaletted(image.Rect(0, 0, 800, 480), widget.PaperPalette)
	if err := w.Render(frame); err != nil {
		t.Fatalf("Render: %v", err)
	}
}

// Stale-on-error: when the weather source returns (forecast, err) — e.g.
// a CachedSource serving cached data because the live fetch failed — the
// widget should render the chart with the stale data anyway. Otherwise
// the dashboard would blank the chart any time the upstream blips, even
// though we have perfectly usable data on hand.
func TestWidget_RenderWeatherStaleOnError(t *testing.T) {
	bounds := image.Rect(0, 52, 800, 480)
	cal := &stubCalSource{events: sampleEvents()}
	ws := &stubWeatherSource{forecast: sampleForecast(), err: context.DeadlineExceeded}
	w := New(bounds, cal, ws, fixedClock(testTime), Config{
		MaxEvents:   5,
		WeekStart:   time.Monday,
		TempUnit:    "C",
		ShowWeather: true,
		Latitude:    45.4,
		Longitude:   -75.7,
	})

	frame := image.NewPaletted(image.Rect(0, 0, 800, 480), widget.PaperPalette)
	if err := w.Render(frame); err != nil {
		t.Fatalf("Render: %v", err)
	}
	// A rendered chart should have the temperature polyline drawn in
	// PaperBlack — if the stale forecast had been discarded the weather
	// row would be all white.
	chartRow := bounds.Min.Y + 56 + 80 // header + roughly mid-chart
	hasBlack := false
	for x := bounds.Min.X; x < bounds.Max.X; x++ {
		if frame.ColorIndexAt(x, chartRow) == widget.PaperBlack {
			hasBlack = true
			break
		}
	}
	if !hasBlack {
		t.Error("stale forecast was discarded — chart row has no ink")
	}
}

func TestWidget_RenderCalendarError(t *testing.T) {
	bounds := image.Rect(0, 52, 800, 480)
	cal := &stubCalSource{err: context.DeadlineExceeded}
	w := New(bounds, cal, nil, fixedClock(testTime), Config{
		MaxEvents: 5,
		WeekStart: time.Monday,
		TempUnit:  "C",
	})

	frame := image.NewPaletted(image.Rect(0, 0, 800, 480), widget.PaperPalette)
	if err := w.Render(frame); err != nil {
		t.Fatalf("Render: %v", err)
	}
}

func TestWidget_SundayWeekStart(t *testing.T) {
	bounds := image.Rect(0, 0, 800, 480)
	cal := &stubCalSource{}
	w := New(bounds, cal, nil, fixedClock(testTime), Config{
		MaxEvents: 5,
		WeekStart: time.Sunday,
		TempUnit:  "C",
	})

	frame := image.NewPaletted(image.Rect(0, 0, 800, 480), widget.PaperPalette)
	if err := w.Render(frame); err != nil {
		t.Fatalf("Render: %v", err)
	}
}

func TestWidget_ShowLocation(t *testing.T) {
	bounds := image.Rect(0, 0, 800, 480)
	cal := &stubCalSource{events: sampleEvents()}
	w := New(bounds, cal, nil, fixedClock(testTime), Config{
		MaxEvents:    5,
		WeekStart:    time.Monday,
		TempUnit:     "C",
		ShowLocation: true,
	})

	frame := image.NewPaletted(image.Rect(0, 0, 800, 480), widget.PaperPalette)
	if err := w.Render(frame); err != nil {
		t.Fatalf("Render: %v", err)
	}
}

func TestWidget_FahrenheitUnit(t *testing.T) {
	bounds := image.Rect(0, 0, 800, 480)
	ws := &stubWeatherSource{forecast: sampleForecast()}
	w := New(bounds, &stubCalSource{}, ws, fixedClock(testTime), Config{
		MaxEvents:   5,
		WeekStart:   time.Monday,
		TempUnit:    "F",
		ShowWeather: true,
	})

	frame := image.NewPaletted(image.Rect(0, 0, 800, 480), widget.PaperPalette)
	if err := w.Render(frame); err != nil {
		t.Fatalf("Render: %v", err)
	}
}

func TestFindForecast_Found(t *testing.T) {
	days := sampleForecast().Days
	result := findForecast(days, time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC))
	if result.Date.IsZero() {
		t.Error("expected to find forecast for Apr 29")
	}
}

func TestFindForecast_NotFound(t *testing.T) {
	days := sampleForecast().Days
	result := findForecast(days, time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC))
	if !result.Date.IsZero() {
		t.Error("expected zero value for missing date")
	}
}

func TestFindForecast_Empty(t *testing.T) {
	result := findForecast(nil, time.Date(2026, 4, 27, 0, 0, 0, 0, time.UTC))
	if !result.Date.IsZero() {
		t.Error("expected zero value for nil days")
	}
}

// --- Factory / parseConfig tests ---

func minimalConfig() map[string]any {
	return map[string]any{
		"feeds": []any{"https://example.com/cal.ics"},
	}
}

func TestFactory_Minimal(t *testing.T) {
	deps := widget.Deps{Now: fixedClock(testTime)}
	w, err := Factory(image.Rect(0, 0, 800, 480), minimalConfig(), deps)
	if err != nil {
		t.Fatalf("Factory: %v", err)
	}
	if w == nil {
		t.Fatal("Factory returned nil widget")
	}
}

func TestFactory_NilNow(t *testing.T) {
	deps := widget.Deps{}
	w, err := Factory(image.Rect(0, 0, 800, 480), minimalConfig(), deps)
	if err != nil {
		t.Fatalf("Factory: %v", err)
	}
	if w == nil {
		t.Fatal("Factory returned nil widget")
	}
}

func TestFactory_WithHTTPClient(t *testing.T) {
	deps := widget.Deps{
		Now:         fixedClock(testTime),
		DataSources: map[string]any{"http_client": &stubHTTPClient{}},
	}
	w, err := Factory(image.Rect(0, 0, 800, 480), minimalConfig(), deps)
	if err != nil {
		t.Fatalf("Factory: %v", err)
	}
	if w == nil {
		t.Fatal("Factory returned nil widget")
	}
}

func TestFactory_WithWeatherSource(t *testing.T) {
	deps := widget.Deps{
		Now: fixedClock(testTime),
		DataSources: map[string]any{
			"weather_source": &stubWeatherSource{forecast: sampleForecast()},
		},
	}
	cfg := minimalConfig()
	cfg["show_weather"] = true
	w, err := Factory(image.Rect(0, 0, 800, 480), cfg, deps)
	if err != nil {
		t.Fatalf("Factory: %v", err)
	}
	if w == nil {
		t.Fatal("Factory returned nil widget")
	}
}

func TestFactory_WeatherDisabled(t *testing.T) {
	deps := widget.Deps{Now: fixedClock(testTime)}
	cfg := minimalConfig()
	cfg["show_weather"] = false
	_, err := Factory(image.Rect(0, 0, 800, 480), cfg, deps)
	if err != nil {
		t.Fatalf("Factory: %v", err)
	}
}

func TestParseConfig_MissingFeeds(t *testing.T) {
	_, err := parseConfig(map[string]any{})
	if err == nil {
		t.Fatal("expected error for missing feeds")
	}
}

func TestParseConfig_FeedsNotList(t *testing.T) {
	_, err := parseConfig(map[string]any{"feeds": "not-a-list"})
	if err == nil {
		t.Fatal("expected error for non-list feeds")
	}
}

func TestParseConfig_FeedsEmpty(t *testing.T) {
	_, err := parseConfig(map[string]any{"feeds": []any{}})
	if err == nil {
		t.Fatal("expected error for empty feeds")
	}
}

func TestParseConfig_FeedsNonString(t *testing.T) {
	_, err := parseConfig(map[string]any{"feeds": []any{123}})
	if err == nil {
		t.Fatal("expected error for non-string feed")
	}
}

func TestParseConfig_RefreshInvalid(t *testing.T) {
	cfg := minimalConfig()

	cfg["refresh"] = 42
	if _, err := parseConfig(cfg); err == nil {
		t.Error("expected error for non-string refresh")
	}

	cfg["refresh"] = "not-a-duration"
	if _, err := parseConfig(cfg); err == nil {
		t.Error("expected error for unparseable refresh")
	}

	cfg["refresh"] = "30s"
	if _, err := parseConfig(cfg); err == nil {
		t.Error("expected error for refresh < 1m")
	}
}

func TestParseConfig_RefreshValid(t *testing.T) {
	cfg := minimalConfig()
	cfg["refresh"] = "30m"
	c, err := parseConfig(cfg)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	if c.Refresh != 30*time.Minute {
		t.Errorf("Refresh = %v, want 30m", c.Refresh)
	}
}

func TestParseConfig_WeekStart(t *testing.T) {
	cfg := minimalConfig()

	cfg["week_start"] = "sunday"
	c, err := parseConfig(cfg)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	if c.WeekStart != time.Sunday {
		t.Errorf("WeekStart = %v, want Sunday", c.WeekStart)
	}

	cfg["week_start"] = "monday"
	c, err = parseConfig(cfg)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	if c.WeekStart != time.Monday {
		t.Errorf("WeekStart = %v, want Monday", c.WeekStart)
	}

	cfg["week_start"] = "friday"
	if _, err := parseConfig(cfg); err == nil {
		t.Error("expected error for invalid week_start")
	}

	cfg["week_start"] = 42
	if _, err := parseConfig(cfg); err == nil {
		t.Error("expected error for non-string week_start")
	}
}

func TestParseConfig_MaxEvents(t *testing.T) {
	cfg := minimalConfig()

	cfg["max_events"] = 3
	c, err := parseConfig(cfg)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	if c.MaxEvents != 3 {
		t.Errorf("MaxEvents = %d, want 3", c.MaxEvents)
	}

	cfg["max_events"] = 0
	if _, err := parseConfig(cfg); err == nil {
		t.Error("expected error for max_events = 0")
	}

	cfg["max_events"] = -1
	if _, err := parseConfig(cfg); err == nil {
		t.Error("expected error for negative max_events")
	}

	cfg["max_events"] = "five"
	if _, err := parseConfig(cfg); err == nil {
		t.Error("expected error for non-int max_events")
	}
}

func TestParseConfig_ShowLocation(t *testing.T) {
	cfg := minimalConfig()

	cfg["show_location"] = true
	c, err := parseConfig(cfg)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	if !c.ShowLocation {
		t.Error("expected ShowLocation = true")
	}

	cfg["show_location"] = "yes"
	if _, err := parseConfig(cfg); err == nil {
		t.Error("expected error for non-bool show_location")
	}
}

func TestParseConfig_LatLon(t *testing.T) {
	cfg := minimalConfig()
	cfg["latitude"] = 45.4
	cfg["longitude"] = -75.7
	c, err := parseConfig(cfg)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	if c.Latitude != 45.4 || c.Longitude != -75.7 {
		t.Errorf("got lat=%f lon=%f", c.Latitude, c.Longitude)
	}

	cfg["latitude"] = "north"
	if _, err := parseConfig(cfg); err == nil {
		t.Error("expected error for non-number latitude")
	}

	cfg["latitude"] = 45.4
	cfg["longitude"] = "west"
	if _, err := parseConfig(cfg); err == nil {
		t.Error("expected error for non-number longitude")
	}
}

func TestParseConfig_LatLonRange(t *testing.T) {
	cases := []struct {
		label   string
		lat     float64
		lon     float64
		wantErr bool
	}{
		{"lat lower boundary", -90, 0, false},
		{"lat upper boundary", 90, 0, false},
		{"lat below range", -90.001, 0, true},
		{"lat above range", 90.001, 0, true},
		{"lat far out of range", 500, 0, true},
		{"lon lower boundary", 0, -180, false},
		{"lon upper boundary", 0, 180, false},
		{"lon below range", 0, -180.001, true},
		{"lon above range", 0, 180.001, true},
	}
	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			cfg := minimalConfig()
			cfg["latitude"] = tc.lat
			cfg["longitude"] = tc.lon
			_, err := parseConfig(cfg)
			if tc.wantErr && err == nil {
				t.Errorf("lat=%f lon=%f: want error, got nil", tc.lat, tc.lon)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("lat=%f lon=%f: unexpected error: %v", tc.lat, tc.lon, err)
			}
		})
	}
}

func TestParseConfig_ShowWeather(t *testing.T) {
	cfg := minimalConfig()
	cfg["show_weather"] = false
	c, err := parseConfig(cfg)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	if c.ShowWeather {
		t.Error("expected ShowWeather = false")
	}

	cfg["show_weather"] = "yes"
	if _, err := parseConfig(cfg); err == nil {
		t.Error("expected error for non-bool show_weather")
	}
}

func TestParseConfig_ShowWeatherLabel(t *testing.T) {
	cfg := minimalConfig()
	cfg["show_weather_label"] = false
	c, err := parseConfig(cfg)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	if c.ShowWeatherLabel {
		t.Error("expected ShowWeatherLabel = false")
	}

	cfg["show_weather_label"] = 1
	if _, err := parseConfig(cfg); err == nil {
		t.Error("expected error for non-bool show_weather_label")
	}
}

func TestParseConfig_TempUnit(t *testing.T) {
	cfg := minimalConfig()
	cfg["temp_unit"] = "F"
	c, err := parseConfig(cfg)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	if c.TempUnit != "F" {
		t.Errorf("TempUnit = %q, want F", c.TempUnit)
	}

	cfg["temp_unit"] = "K"
	if _, err := parseConfig(cfg); err == nil {
		t.Error("expected error for invalid temp_unit")
	}

	cfg["temp_unit"] = 42
	if _, err := parseConfig(cfg); err == nil {
		t.Error("expected error for non-string temp_unit")
	}
}

func TestParseConfig_HighlightHour(t *testing.T) {
	cfg := minimalConfig()
	cfg["highlight_hour"] = 12
	c, err := parseConfig(cfg)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	if c.HighlightHour != 12 {
		t.Errorf("HighlightHour = %d, want 12", c.HighlightHour)
	}

	cfg["highlight_hour"] = "noon"
	if _, err := parseConfig(cfg); err == nil {
		t.Error("expected error for non-int highlight_hour")
	}
}

func TestParseConfig_HighlightHourRange(t *testing.T) {
	cases := []struct {
		label   string
		hour    int
		wantErr bool
	}{
		{"lower boundary", 0, false},
		{"upper boundary", 23, false},
		{"below range", -1, true},
		{"above range", 24, true},
		{"far above range", 99, true},
	}
	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			cfg := minimalConfig()
			cfg["highlight_hour"] = tc.hour
			_, err := parseConfig(cfg)
			if tc.wantErr && err == nil {
				t.Errorf("hour=%d: want error, got nil", tc.hour)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("hour=%d: unexpected error: %v", tc.hour, err)
			}
		})
	}
}

func TestFactory_InvalidConfig(t *testing.T) {
	deps := widget.Deps{}
	_, err := Factory(image.Rect(0, 0, 800, 480), map[string]any{}, deps)
	if err == nil {
		t.Fatal("expected error for missing feeds")
	}
}

// stubHTTPClient satisfies calendar.HTTPClient.
type stubHTTPClient struct{}

func (s *stubHTTPClient) Do(_ *nethttp.Request) (*nethttp.Response, error) {
	return nil, context.DeadlineExceeded
}

func TestParseConfig_WeatherModel(t *testing.T) {
	tests := []struct {
		label   string
		value   any
		set     bool
		want    weather.Model
		wantErr bool
	}{
		{label: "default is gem", set: false, want: weather.ModelGEM},
		{label: "gfs", value: "gfs", set: true, want: weather.ModelGFS},
		{label: "ecmwf", value: "ecmwf", set: true, want: weather.ModelECMWF},
		{label: "gem", value: "gem", set: true, want: weather.ModelGEM},
		{label: "invalid string", value: "bogus", set: true, wantErr: true},
		{label: "wrong type", value: 123, set: true, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			cfg := minimalConfig()
			if tt.set {
				cfg["weather_model"] = tt.value
			}
			got, err := parseConfig(cfg)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseConfig weather_model=%v: want error", tt.value)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseConfig: unexpected error: %v", err)
			}
			if got.WeatherModel != tt.want {
				t.Errorf("WeatherModel = %q, want %q", got.WeatherModel, tt.want)
			}
		})
	}
}

// recordingHTTPClient captures the requested URL so a test can assert which
// Open-Meteo model endpoint the widget-built weather source queries. It
// returns an error after recording; the URL is set regardless.
type recordingHTTPClient struct{ lastURL string }

func (c *recordingHTTPClient) Do(req *nethttp.Request) (*nethttp.Response, error) {
	c.lastURL = req.URL.String()
	return nil, context.DeadlineExceeded
}

func TestFactory_BuildsWeatherSourceFromModel(t *testing.T) {
	tests := []struct {
		label    string
		model    any
		wantPath string
	}{
		{label: "default gem", model: nil, wantPath: "/v1/gem"},
		{label: "ecmwf", model: "ecmwf", wantPath: "/v1/ecmwf"},
		{label: "gfs", model: "gfs", wantPath: "/v1/forecast"},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			rec := &recordingHTTPClient{}
			deps := widget.Deps{
				Now:         fixedClock(testTime),
				DataSources: map[string]any{"http_client": rec},
			}
			cfg := minimalConfig()
			cfg["show_weather"] = true
			if tt.model != nil {
				cfg["weather_model"] = tt.model
			}
			w, err := Factory(image.Rect(0, 0, 800, 480), cfg, deps)
			if err != nil {
				t.Fatalf("Factory: %v", err)
			}
			ww := w.(*Widget)
			if ww.weather == nil {
				t.Fatal("Factory did not build a weather source")
			}
			_, _ = ww.weather.Forecast(context.Background(), weather.Location{}, 7)
			if !strings.Contains(rec.lastURL, tt.wantPath) {
				t.Errorf("requested URL = %q, want path %q", rec.lastURL, tt.wantPath)
			}
		})
	}
}
