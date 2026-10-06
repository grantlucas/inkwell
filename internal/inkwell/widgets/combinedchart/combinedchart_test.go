package combinedchart_test

import (
	"image"
	"testing"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar"
	"github.com/grantlucas/inkwell/internal/inkwell/testutil"
	"github.com/grantlucas/inkwell/internal/inkwell/testutil/fakehttp"
	"github.com/grantlucas/inkwell/internal/inkwell/weather"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/combinedchart"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daydata"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/weatherview"
)

// testTime is a Monday mid-afternoon, so today's now marker falls inside
// the chart's 06:00-21:00 window.
var testTime = time.Date(2026, 3, 16, 14, 30, 0, 0, time.UTC)

func fixedClock(t time.Time) func() time.Time { return func() time.Time { return t } }

// day is a forecast for March 16+i whose hours run from low before dawn
// to high in the afternoon, with rain from noon when wet.
func day(i int, low, high float64, wet bool) weather.DailyForecast {
	var hourly []weather.HourlyPoint
	for h := range 24 {
		t := low
		if h >= 6 && h <= 15 {
			t = low + (high-low)*float64(h-6)/9
		} else if h > 15 {
			t = high - (high-low)*float64(h-15)/9
		}
		prob := 0.0
		if wet && h >= 12 && h <= 17 {
			prob = 0.7
		}
		hourly = append(hourly, weather.HourlyPoint{Hour: h, Temperature: t, PrecipitationProb: prob})
	}
	return weather.DailyForecast{
		Date: time.Date(2026, 3, 16+i, 0, 0, 0, 0, time.UTC), High: high, Low: low,
		Condition: weather.Rain, Hourly: hourly,
	}
}

// week is five days whose coldest is Wednesday's -4° and warmest
// Friday's 24°, so a chart ranging over all five plots on -4..24, and one
// ranging over today and tomorrow only on 2..14.
func week() []weather.DailyForecast {
	return []weather.DailyForecast{
		day(0, 4, 14, true),
		day(1, 2, 9, false),
		day(2, -4, 3, true),
		day(3, 0, 11, false),
		day(4, 10, 24, true),
	}
}

func chartConfig(d, rangeDays int) combinedchart.Config {
	return combinedchart.Config{Config: daydata.Config{Day: d}, RangeDays: rangeDays}
}

func render(t *testing.T, w *combinedchart.Widget) *image.Paletted {
	t.Helper()
	frame := image.NewPaletted(image.Rect(0, 0, 400, 200), widget.PaperPalette)
	if err := w.Render(frame); err != nil {
		t.Fatalf("Render: %v", err)
	}
	return frame
}

// The widget draws its one day's combined chart into its bounds, plotted
// on the shared temperature range across the first range_days days, with
// the now marker on today's chart only. Each row's expected range is
// worked out from week's lows and highs.
func TestWidget_DrawsItsDaysChart(t *testing.T) {
	bounds := image.Rect(20, 30, 260, 110)
	tests := []struct {
		label  string
		cfg    combinedchart.Config
		hourly []weather.HourlyPoint
		rng    weatherview.TempRange
		marker bool
	}{
		{"today on the week's range, with the now marker", chartConfig(0, 5), week()[0].Hourly, weatherview.TempRange{Min: -4, Max: 24}, true},
		{"a later day on the week's range", chartConfig(2, 5), week()[2].Hourly, weatherview.TempRange{Min: -4, Max: 24}, false},
		{"a shorter range leaves the later days out", chartConfig(1, 2), week()[1].Hourly, weatherview.TempRange{Min: 2, Max: 14}, false},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			w := combinedchart.New(bounds, daydata.InMemory(nil, week()), fixedClock(testTime), tt.cfg)
			got := render(t, w)

			want := image.NewPaletted(got.Bounds(), widget.PaperPalette)
			weatherview.RenderCombinedChart(want, bounds, tt.hourly, tt.rng, weatherview.CombinedOptions{
				NowHour: testTime.Hour(), ShowNowMarker: tt.marker,
			})
			assertSame(t, got, want)
		})
	}
}

// A day the forecast doesn't reach has no chart: a line at some default
// temperature would state a forecast nobody made. The widget still
// clears its bounds.
func TestWidget_NoForecastDrawsNothing(t *testing.T) {
	bounds := image.Rect(20, 30, 260, 110)
	frame := image.NewPaletted(image.Rect(0, 0, 400, 200), widget.PaperPalette)
	for y := range 200 {
		for x := range 400 {
			frame.SetColorIndex(x, y, widget.PaperBlack)
		}
	}
	w := combinedchart.New(bounds, daydata.InMemory(nil, week()[:2]), fixedClock(testTime), chartConfig(3, 5))
	if err := w.Render(frame); err != nil {
		t.Fatalf("Render: %v", err)
	}
	for y := range 200 {
		for x := range 400 {
			in := image.Pt(x, y).In(bounds)
			if got := frame.ColorIndexAt(x, y); in && got != widget.PaperWhite {
				t.Fatalf("ink at (%d,%d) for a day with no forecast", x, y)
			} else if !in && got != widget.PaperBlack {
				t.Fatalf("drew outside its bounds at (%d,%d)", x, y)
			}
		}
	}
	if w.Bounds() != bounds {
		t.Errorf("Bounds = %v, want %v", w.Bounds(), bounds)
	}
}

// Golden renders at the sizes the full-screen widgets give their charts.
func TestWidget_Golden(t *testing.T) {
	tests := []struct {
		label  string
		cfg    combinedchart.Config
		bounds image.Rectangle
	}{
		{"today in a bold-five column", chartConfig(0, 5), image.Rect(0, 0, 144, 40)},
		{"a dry day in a row-agenda badge", chartConfig(1, 5), image.Rect(0, 0, 106, 68)},
		{"the coldest day in today-hero's hero", chartConfig(2, 5), image.Rect(0, 0, 312, 68)},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			w := combinedchart.New(tt.bounds, daydata.InMemory(nil, week()), fixedClock(testTime), tt.cfg)
			frame := image.NewPaletted(tt.bounds, widget.PaperPalette)
			if err := w.Render(frame); err != nil {
				t.Fatalf("Render: %v", err)
			}
			testutil.AssertGoldenPNG(t, frame)
		})
	}
}

// The widget shows only the weather, so it takes the weather settings,
// day and range_days: how many days from today its temperature range
// spans, five unless told otherwise. Charts placed separately share one
// range by asking for the same span.
func TestFactory(t *testing.T) {
	deps := widget.Deps{
		Now:      fixedClock(testTime),
		Calendar: calendar.NewProvider(fakehttp.New(), fixedClock(testTime)),
		Weather:  weather.NewProvider(fakehttp.New(), time.Hour, fixedClock(testTime), weather.Settings{}),
	}
	bounds := image.Rect(0, 0, 144, 40)

	tests := []struct {
		label     string
		config    map[string]any
		wantDay   int
		wantRange int
		wantErr   string
	}{
		{label: "today on five days by default", config: nil, wantDay: 0, wantRange: 5},
		{label: "a later day", config: map[string]any{"day": 4}, wantDay: 4, wantRange: 5},
		{label: "its own range", config: map[string]any{"day": 6, "range_days": 7}, wantDay: 6, wantRange: 7},
		{label: "a range of today alone", config: map[string]any{"range_days": 1}, wantDay: 0, wantRange: 1},
		{
			label: "a range that doesn't reach its day", config: map[string]any{"day": 5},
			wantErr: "combined-chart: range_days must be at least 6 to reach day 5, got 5",
		},
		{
			label: "a range past a week", config: map[string]any{"range_days": 8},
			wantErr: "combined-chart: range_days must be in [1, 7], got 8",
		},
		{
			label: "an empty range", config: map[string]any{"range_days": 0},
			wantErr: "combined-chart: range_days must be in [1, 7], got 0",
		},
		{
			label: "range_days not an int", config: map[string]any{"range_days": "5"},
			wantErr: "combined-chart: range_days must be an integer, got string",
		},
		{
			label: "feeds", config: map[string]any{"feeds": []any{"https://example.com/a.ics"}},
			wantErr: "combined-chart: feeds is not supported: combined-chart shows only the weather, so it reads no calendar",
		},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			w, err := combinedchart.Factory(bounds, tt.config, deps)
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("err = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Factory: %v", err)
			}
			if w.Bounds() != bounds {
				t.Errorf("Bounds = %v, want %v", w.Bounds(), bounds)
			}
			cfg := w.(*combinedchart.Widget).Config
			if cfg.Day != tt.wantDay || cfg.RangeDays != tt.wantRange {
				t.Errorf("Day, RangeDays = %d, %d; want %d, %d", cfg.Day, cfg.RangeDays, tt.wantDay, tt.wantRange)
			}
		})
	}
}

// A dashboard wired without a clock is a fault to report at load, not a
// widget that quietly reads the wall clock.
func TestFactory_NeedsAClock(t *testing.T) {
	deps := widget.Deps{
		Calendar: calendar.NewProvider(fakehttp.New(), fixedClock(testTime)),
		Weather:  weather.NewProvider(fakehttp.New(), time.Hour, fixedClock(testTime), weather.Settings{}),
	}
	if _, err := combinedchart.Factory(image.Rect(0, 0, 144, 40), nil, deps); err == nil || err.Error() != "combined-chart: no clock" {
		t.Fatalf("err = %v, want %q", err, "combined-chart: no clock")
	}
}

func assertSame(t *testing.T, got, want *image.Paletted) {
	t.Helper()
	b := got.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if got.ColorIndexAt(x, y) != want.ColorIndexAt(x, y) {
				t.Fatalf("frame differs from the expected chart at (%d,%d)", x, y)
			}
		}
	}
}
