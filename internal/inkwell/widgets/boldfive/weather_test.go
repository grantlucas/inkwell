package boldfive

import (
	"image"
	"testing"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/weather"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/drawkit"
)

// The day badge above the chart (the date, icon and readings) is the
// day badge's, tested there. These cover the chart the column draws
// under it.

func newTestFrame(w, h int) *image.Paletted {
	frame := image.NewPaletted(image.Rect(0, 0, w, h), widget.PaperPalette)
	drawkit.FillWhite(frame, frame.Bounds())
	return frame
}

// countIndex reports how many pixels carry the given palette index.
func countIndex(frame *image.Paletted, idx uint8) int {
	n := 0
	for _, px := range frame.Pix {
		if px == idx {
			n++
		}
	}
	return n
}

// chartRect is the first column's chart band on a full panel.
func chartRect() image.Rectangle {
	return computeColumns(image.Rect(0, 0, 800, 480))[0].Chart
}

// countIndexIn counts pixels carrying idx within r.
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

// wetDay builds a forecast day with rain concentrated in the afternoon.
func wetDay(high, low float64) *weather.DailyForecast {
	var hourly []weather.HourlyPoint
	for h := range 24 {
		prob := 0.0
		if h >= 13 && h <= 17 {
			prob = 0.8
		}
		hourly = append(hourly, weather.HourlyPoint{
			Hour:              h,
			Temperature:       high - 4,
			PrecipitationProb: prob,
		})
	}
	return &weather.DailyForecast{
		Date:      time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC),
		High:      high,
		Low:       low,
		Condition: weather.Condition(0),
		Hourly:    hourly,
	}
}

// dryDay is the same shape with no rain anywhere.
func dryDay(high, low float64) *weather.DailyForecast {
	d := wetDay(high, low)
	for i := range d.Hourly {
		d.Hourly[i].PrecipitationProb = 0
	}
	return d
}

// The now-marker belongs on today's column only. On any other day "now"
// is not a point on that day's axis, so a marker there would assert
// something meaningless.
func TestRenderChart_NowMarkerOnlyOnToday(t *testing.T) {
	withMarker := newTestFrame(160, 480)
	renderChart(withMarker, chartRect(), wetDay(12, 4), chartOptions{ShowNowMarker: true, NowHour: 15})

	without := newTestFrame(160, 480)
	renderChart(without, chartRect(), wetDay(12, 4), chartOptions{ShowNowMarker: false, NowHour: 15})

	markerInk := countIndex(withMarker, widget.PaperBlack) - countIndex(without, widget.PaperBlack)
	if markerInk <= 0 {
		t.Errorf("marker added %d px of ink, want more than 0", markerInk)
	}
}

// A dry day draws no bars at all: a flat row of stubs reads as a broken
// widget from across the room. Its chart still carries the temperature
// line, which TestWidget_DryDayStillDrawsAChart pins.
func TestRenderChart_DryDayDrawsNoBars(t *testing.T) {
	wet := newTestFrame(160, 480)
	renderChart(wet, chartRect(), wetDay(12, 4), chartOptions{})

	dry := newTestFrame(160, 480)
	renderChart(dry, chartRect(), dryDay(12, 4), chartOptions{})

	if got := countIndexIn(dry, chartRect(), widget.PaperGray70); got != 0 {
		t.Errorf("dry day drew %d px of bar fill", got)
	}
	if countIndexIn(wet, chartRect(), widget.PaperGray70) == 0 {
		t.Error("wet day drew no bar fill")
	}
}

// The chart stays inside its band: nothing spills into the badge above
// or the agenda below. A day the forecast never reached draws nothing at
// all, since anything would state a temperature nobody forecast.
func TestRenderChart_StaysInsideTheBand(t *testing.T) {
	tests := []struct {
		label string
		day   *weather.DailyForecast
		ink   bool
	}{
		{"a wet day", wetDay(12, 4), true},
		{"no forecast", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			frame := newTestFrame(160, 480)
			rect := chartRect()
			renderChart(frame, rect, tt.day, chartOptions{ShowNowMarker: true, NowHour: 15})

			for y := range 480 {
				for x := range 160 {
					if !image.Pt(x, y).In(rect) && frame.ColorIndexAt(x, y) != widget.PaperWhite {
						t.Fatalf("ink at (%d,%d), outside the chart band %v", x, y, rect)
					}
				}
			}
			if got := countIndex(frame, widget.PaperWhite) < 160*480; got != tt.ink {
				t.Errorf("drew ink: %v, want %v", got, tt.ink)
			}
		})
	}
}
