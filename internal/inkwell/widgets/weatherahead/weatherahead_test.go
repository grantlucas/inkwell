package weatherahead

import (
	"image"
	"math"
	"testing"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/testutil"
	"github.com/grantlucas/inkwell/internal/inkwell/weather"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daydata"
)

// testTime is a Monday mid-afternoon, so the rows start on Tuesday.
var testTime = time.Date(2026, 3, 16, 14, 30, 0, 0, time.UTC)

func fixedClock(t time.Time) func() time.Time { return func() time.Time { return t } }

// rightColumn is where the day-timeline screen places the widget: the
// right-hand third of the panel, under today-weather.
var rightColumn = image.Rect(534, 208, 800, 480)

// goldenBox is rightColumn's size at the origin, so a golden is the
// widget alone.
var goldenBox = image.Rect(0, 0, rightColumn.Dx(), rightColumn.Dy())

// day is one day's weather for a test forecast: its condition, high and
// low, and its chance of rain through the afternoon (zero is a dry day).
type day struct {
	cond      weather.Condition
	high, low float64
	rain      float64
}

// forecast builds a forecast starting today, testTime's date, one entry
// per day. Each day is coldest at 03:00 and warmest at 15:00, spanning
// its low to its high, so the temperature line has a shape.
func forecast(days ...day) []weather.DailyForecast {
	var out []weather.DailyForecast
	for i, d := range days {
		var hourly []weather.HourlyPoint
		for h := range 24 {
			prob := 0.0
			if h >= 11 && h <= 18 {
				prob = d.rain * (0.6 + 0.4*math.Sin(float64(h-11)/2))
			}
			hourly = append(hourly, weather.HourlyPoint{
				Hour:              h,
				Temperature:       d.low + (d.high-d.low)*(1-math.Cos(2*math.Pi*float64(h-3)/24))/2,
				PrecipitationProb: prob,
			})
		}
		out = append(out, weather.DailyForecast{
			Date:      time.Date(2026, 3, 16+i, 0, 0, 0, 0, time.UTC),
			High:      d.high,
			Low:       d.low,
			Condition: d.cond,
			Hourly:    hourly,
		})
	}
	return out
}

// today is a mild Monday; the rows start after it.
var today = day{weather.Clear, 14, 3, 0}

// mixedWeek is the four days after today in mixed weather. Thursday is
// the cold day, so the shared range shows: its line sits visibly lower
// than its neighbours'.
var mixedWeek = forecast(
	today,
	day{weather.PartlyCloudy, 15, 4, 0.3},
	day{weather.Rain, 12, 6, 0.9},
	day{weather.Snow, 1, -6, 0.5},
	day{weather.Clear, 18, 7, 0},
)

// config is the parsed config with days rows in unit.
func config(days int, unit string) Config {
	return Config{Config: daydata.Config{Weather: daydata.WeatherConfig{TempUnit: unit}}, Days: days}
}

// render draws the widget into a frame exactly its size.
func render(t *testing.T, w *Widget) *image.Paletted {
	t.Helper()
	frame := image.NewPaletted(w.Bounds(), widget.PaperPalette)
	if err := w.Render(frame); err != nil {
		t.Fatalf("Render: %v", err)
	}
	return frame
}

// Golden renders at the size the day-timeline screen gives the widget.
func TestWidget_Golden(t *testing.T) {
	tests := []struct {
		label    string
		forecast []weather.DailyForecast
		cfg      Config
		bounds   image.Rectangle
	}{
		{label: "four days in mixed weather", forecast: mixedWeek, cfg: config(4, "C")},
		// A dry day draws its temperature line with no bars.
		{label: "a dry day", cfg: config(4, "C"), forecast: forecast(
			today,
			day{weather.Cloudy, 9, 2, 0.4},
			day{weather.Clear, 16, 5, 0},
			day{weather.Rain, 11, 7, 0.8},
			day{weather.PartlyCloudy, 13, 4, 0.2},
		)},
		{label: "fahrenheit", forecast: mixedWeek, cfg: config(4, "F")},
		// The rows split the same box between fewer days.
		{label: "two days", forecast: mixedWeek, cfg: config(2, "C")},
		// A week ahead down a full-height column.
		{label: "a week", cfg: config(7, "C"), bounds: image.Rect(0, 0, 266, 480), forecast: forecast(
			today,
			day{weather.PartlyCloudy, 15, 4, 0.3},
			day{weather.Rain, 12, 6, 0.9},
			day{weather.Snow, 1, -6, 0.5},
			day{weather.Clear, 18, 7, 0},
			day{weather.Thunderstorm, 22, 14, 1},
			day{weather.Fog, 10, 8, 0.1},
			day{weather.Drizzle, 9, 5, 0.6},
		)},
		// The forecast stops short of the last rows, or never arrived.
		{label: "forecast stopping short", forecast: mixedWeek[:3], cfg: config(4, "C")},
		{label: "no forecast", forecast: nil, cfg: config(4, "C")},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			bounds := tt.bounds
			if bounds.Empty() {
				bounds = goldenBox
			}
			w := New(bounds, daydata.InMemory(nil, tt.forecast), fixedClock(testTime), tt.cfg)
			testutil.AssertGoldenPNG(t, render(t, w))
		})
	}
}
