package rowagenda

import (
	"image"
	"testing"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/weather"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/weatherview"
)

// The day badge beside the chart (the date, icon and readings) is the
// day badge's, tested there, including that its readings never reach
// the chart. These cover the chart the row draws.

// busyRow is the first row of a week whose first day carries n events
// and whose other days are empty, as the widget would lay it out.
func busyRow(n int) rowLayout {
	return planRows(panel, []int{n, 0, 0, 0, 0})[0]
}

// A day the forecast never covered draws no chart: a line would state a
// temperature nobody forecast.
func TestRenderChart_MissingForecast(t *testing.T) {
	frame := newTestFrame(800, 480)
	renderChart(frame, busyRow(0).Chart, nil, true, 14, weatherview.TempRange{Min: 0, Max: 25})
	if got := countIndexIn(frame, frame.Bounds(), widget.PaperBlack); got != 0 {
		t.Errorf("drew %d px for a day with no forecast", got)
	}
}

// Every row carries the combined chart, so a dry day still draws its
// temperature line rather than leaving the chart blank, and two days on
// one shared range sit at different heights when one is colder.
func TestRenderChart_SharedRange(t *testing.T) {
	chart := busyRow(0).Chart
	rng := weatherview.TempRange{Min: -10, Max: 30}

	dryDay := func(temp float64) *weather.DailyForecast {
		var hourly []weather.HourlyPoint
		for h := range 24 {
			hourly = append(hourly, weather.HourlyPoint{Hour: h, Temperature: temp})
		}
		return &weather.DailyForecast{
			Date: time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC),
			High: temp, Low: temp, Hourly: hourly,
		}
	}

	// topInk is the first row of the chart carrying ink above its
	// baseline, which on a dry day is the temperature line.
	topInk := func(frame *image.Paletted) int {
		for y := chart.Min.Y; y < chart.Max.Y; y++ {
			if countIndexIn(frame, image.Rect(chart.Min.X, y, chart.Max.X, y+1), widget.PaperBlack) > 0 {
				return y
			}
		}
		return -1
	}

	cold, warm := newTestFrame(800, 480), newTestFrame(800, 480)
	renderChart(cold, chart, dryDay(-5), false, 14, rng)
	renderChart(warm, chart, dryDay(25), false, 14, rng)

	coldY, warmY := topInk(cold), topInk(warm)
	if coldY < 0 || warmY < 0 {
		t.Fatal("a dry day drew a blank chart")
	}
	if warmY >= coldY {
		t.Errorf("the warm day's line (y=%d) is not above the cold day's (y=%d)", warmY, coldY)
	}
}
