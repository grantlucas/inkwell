package boldfive

import (
	"errors"
	"image"
	"testing"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/weather"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
)

func weatherRect() image.Rectangle {
	return image.Rect(0, headerH, 160, headerH+weatherH)
}

// chartRect is the precipitation cell inside the weather band.
func chartRect() image.Rectangle {
	r := weatherRect()
	return image.Rect(r.Min.X+chartPadX, r.Min.Y+chartTop, r.Max.X-chartPadX, r.Max.Y)
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
func wetDay(high, low float64) weather.DailyForecast {
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
	return weather.DailyForecast{
		Date:      time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC),
		High:      high,
		Low:       low,
		Condition: weather.Condition(0),
		Hourly:    hourly,
	}
}

// dryDay is the same shape with no rain anywhere.
func dryDay(high, low float64) weather.DailyForecast {
	d := wetDay(high, low)
	for i := range d.Hourly {
		d.Hourly[i].PrecipitationProb = 0
	}
	return d
}

// The now-marker belongs on today's column only. On any other day "now"
// is not a point on that day's axis, so a marker there would assert
// something meaningless.
func TestRenderWeatherBand_NowMarkerOnlyOnToday(t *testing.T) {
	rect := weatherRect()

	withMarker := newTestFrame(160, 480)
	renderWeatherBand(withMarker, rect, wetDay(12, 4), weatherOptions{
		TempUnit: "C", ShowNowMarker: true, NowHour: 15,
	})

	without := newTestFrame(160, 480)
	renderWeatherBand(without, rect, wetDay(12, 4), weatherOptions{
		TempUnit: "C", ShowNowMarker: false, NowHour: 15,
	})

	markerInk := countIndex(withMarker, widget.PaperBlack) - countIndex(without, widget.PaperBlack)
	if markerInk <= 0 {
		t.Errorf("marker added %d px of ink, want more than 0", markerInk)
	}
}

// Fahrenheit is a display concern: the forecast is always Celsius, and
// the conversion happens at the point of drawing.
func TestRenderWeatherBand_TempUnit(t *testing.T) {
	rect := weatherRect()
	celsius := newTestFrame(160, 480)
	renderWeatherBand(celsius, rect, wetDay(12, 4), weatherOptions{TempUnit: "C"})

	fahrenheit := newTestFrame(160, 480)
	renderWeatherBand(fahrenheit, rect, wetDay(12, 4), weatherOptions{TempUnit: "F"})

	// 12°C is 54°F — different digits, so different pixels.
	if countIndex(celsius, widget.PaperBlack) == countIndex(fahrenheit, widget.PaperBlack) {
		t.Error("C and F rendered the same amount of ink; the conversion may not be applied")
	}
}

// A dry day draws no bars at all. A flat row of stubs reads as a broken
// widget from across the room; silence reads as a dry day. There is no
// dry-day caption in this cell either — 144 px cannot carry one.
func TestRenderWeatherBand_DryDayDrawsNoBars(t *testing.T) {
	rect := weatherRect()
	wet := newTestFrame(160, 480)
	renderWeatherBand(wet, rect, wetDay(12, 4), weatherOptions{TempUnit: "C"})

	dry := newTestFrame(160, 480)
	renderWeatherBand(dry, rect, dryDay(12, 4), weatherOptions{TempUnit: "C"})

	// Counted inside the chart cell only: the condition icon is an
	// anti-aliased glyph and contributes gray pixels of its own, so a
	// whole-frame count would measure the icon rather than the bars.
	if got := countIndexIn(dry, chartRect(), widget.PaperGray70); got != 0 {
		t.Errorf("dry day drew %d px of bar fill", got)
	}
	if countIndexIn(wet, chartRect(), widget.PaperGray70) == 0 {
		t.Error("wet day drew no bar fill")
	}
}

// Everything must stay inside the band: the icon at the top, the chart
// at the bottom, and nothing spilling into the agenda below.
func TestRenderWeatherBand_StaysInsideTheBand(t *testing.T) {
	frame := newTestFrame(160, 480)
	rect := weatherRect()
	renderWeatherBand(frame, rect, wetDay(12, 4), weatherOptions{
		TempUnit: "C", ShowNowMarker: true, NowHour: 15,
	})

	for y := range 480 {
		if y >= rect.Min.Y && y < rect.Max.Y {
			continue
		}
		for x := range 160 {
			if frame.ColorIndexAt(x, y) != widget.PaperWhite {
				t.Fatalf("ink at (%d,%d), outside the weather band %v", x, y, rect)
			}
		}
	}
}

// A day the forecast never reached is a zero DailyForecast, and a zero
// DailyForecast is indistinguishable from a real reading — clear sky at
// 0°C is entirely plausible in a Hamilton January. Drawing it would
// state a temperature nobody forecast and leave an operator unable to
// tell an outage from the weather, so the whole band stays empty.
func TestRenderWeatherBand_MissingForecastDrawsNothing(t *testing.T) {
	frame := newTestFrame(160, 480)
	renderWeatherBand(frame, weatherRect(), weather.DailyForecast{}, weatherOptions{TempUnit: "C"})

	for y := range 480 {
		for x := range 160 {
			if frame.ColorIndexAt(x, y) != widget.PaperWhite {
				t.Fatalf("ink at (%d,%d) for a forecast that does not exist", x, y)
			}
		}
	}
}

// The guard keys off the forecast's date, not its values: a genuine
// forecast of 0°C on a clear day must still be drawn.
func TestRenderWeatherBand_RealZeroDegreesIsDrawn(t *testing.T) {
	day := dryDay(0, 0)
	frame := newTestFrame(160, 480)
	renderWeatherBand(frame, weatherRect(), day, weatherOptions{TempUnit: "C"})

	if countIndexIn(frame, weatherRect(), widget.PaperBlack) == 0 {
		t.Error("a real 0°C forecast drew nothing")
	}
}

// A condition glyph that will not load must not take the rest of the
// cell with it: the temperatures and the chart are still worth drawing.
func TestRenderWeatherBand_IconFailureStillDrawsTheRest(t *testing.T) {
	orig := drawIcon
	defer func() { drawIcon = orig }()
	drawIcon = func(*image.Paletted, int, int, int, weather.Condition) error {
		return errors.New("no glyph")
	}

	frame := newTestFrame(160, 480)
	renderWeatherBand(frame, weatherRect(), wetDay(12, 4), weatherOptions{TempUnit: "C"})

	if countIndexIn(frame, chartRect(), widget.PaperGray70) == 0 {
		t.Error("the chart was not drawn after the icon failed")
	}
}
