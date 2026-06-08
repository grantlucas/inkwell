package weatherview

import (
	"image"
	"testing"

	"github.com/grantlucas/inkwell/internal/inkwell/weather"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
)

func sampleHourly() []weather.HourlyPoint {
	var points []weather.HourlyPoint
	for h := range 24 {
		points = append(points, weather.HourlyPoint{
			Hour:              h,
			Temperature:       10 + float64(h)/2,
			PrecipitationProb: float64(h) / 100.0,
		})
	}
	return points
}

// TestRenderHourlyChart_NowMarkerIsBlackStroke pins the "now" indicator
// shape: when IsToday is true and HighlightHour is within the rendered
// range, the chart draws a contiguous vertical run of PaperBlack pixels
// at the highlight column from the top of the chart down through the
// bar gutter. The previous PaperGray20 fill collapsed to white on both
// the BW threshold and Gray4 light-bucket paths — losing the marker on
// hardware. A solid stroke is the device-durable replacement; this test
// would catch any regression that swaps it back for a gray fill.
func TestRenderHourlyChart_NowMarkerIsBlackStroke(t *testing.T) {
	frame := newTestFrame(220, 80)
	opts := ChartOptions{
		TempUnit:      "C",
		GlobalTempMin: 5,
		GlobalTempMax: 24,
		HighlightHour: 12,
		IsToday:       true,
	}
	RenderHourlyChart(frame, image.Rect(0, 0, 220, 80), sampleHourly(), opts)

	// Scan every column for a contiguous PaperBlack run >= 25 px tall.
	// The chart area minus labels/axis is at least that tall at h=80,
	// so the marker is the only thing that produces a run that long.
	const minRun = 25
	found := false
	for x := range 220 {
		run := 0
		for y := range 80 {
			if frame.ColorIndexAt(x, y) == widget.PaperBlack {
				run++
				if run >= minRun {
					found = true
					break
				}
			} else {
				run = 0
			}
		}
		if found {
			break
		}
	}
	if !found {
		t.Errorf("no PaperBlack vertical run >= %d px found — now-marker missing", minRun)
	}
}

// And the inverse: when IsToday is false, no such tall PaperBlack column
// exists (the temp polyline draws single black pixels but those don't
// form a tall contiguous run).
func TestRenderHourlyChart_NoMarkerWhenNotToday(t *testing.T) {
	frame := newTestFrame(220, 80)
	opts := ChartOptions{
		TempUnit:      "C",
		GlobalTempMin: 5,
		GlobalTempMax: 24,
		HighlightHour: 12,
		IsToday:       false,
	}
	RenderHourlyChart(frame, image.Rect(0, 0, 220, 80), sampleHourly(), opts)

	for x := range 220 {
		run := 0
		for y := range 80 {
			if frame.ColorIndexAt(x, y) == widget.PaperBlack {
				run++
				if run >= 25 {
					t.Fatalf("col %d has black run %d — marker drew when IsToday=false", x, run)
				}
			} else {
				run = 0
			}
		}
	}
}

func TestRenderHourlyChart_Basic(t *testing.T) {
	frame := newTestFrame(120, 80)
	opts := ChartOptions{
		TempUnit:      "C",
		GlobalTempMin: 5,
		GlobalTempMax: 24,
		HighlightHour: 15,
		IsToday:       true,
	}

	RenderHourlyChart(frame, image.Rect(0, 0, 120, 80), sampleHourly(), opts)

	if !chartHasInk(frame) {
		t.Error("chart drew nothing")
	}
}

func TestRenderHourlyChart_Fahrenheit(t *testing.T) {
	frame := newTestFrame(120, 80)
	opts := ChartOptions{
		TempUnit:      "F",
		GlobalTempMin: 5,
		GlobalTempMax: 24,
		HighlightHour: 15,
	}

	RenderHourlyChart(frame, image.Rect(0, 0, 120, 80), sampleHourly(), opts)

	if !chartHasInk(frame) {
		t.Error("chart drew nothing in Fahrenheit mode")
	}
}

// chartHasInk reports whether any pixel in frame is non-white. The chart now
// draws in soft grays rather than pure black, so asserting on PaperBlack
// alone would miss every line and bar.
func chartHasInk(frame *image.Paletted) bool {
	for _, px := range frame.Pix {
		if px != widget.PaperWhite {
			return true
		}
	}
	return false
}

func TestRenderHourlyChart_EmptyHourly(t *testing.T) {
	frame := newTestFrame(120, 80)
	opts := ChartOptions{TempUnit: "C", GlobalTempMin: 5, GlobalTempMax: 24}
	RenderHourlyChart(frame, image.Rect(0, 0, 120, 80), nil, opts)
}

func TestRenderHourlyChart_TooSmallBounds(t *testing.T) {
	frame := newTestFrame(5, 5)
	opts := ChartOptions{TempUnit: "C", GlobalTempMin: 5, GlobalTempMax: 24}
	RenderHourlyChart(frame, image.Rect(0, 0, 5, 5), sampleHourly(), opts)
}

func TestRenderHourlyChart_NoMatchingHours(t *testing.T) {
	frame := newTestFrame(120, 80)
	hourly := []weather.HourlyPoint{
		{Hour: 0, Temperature: 10, PrecipitationProb: 0},
		{Hour: 1, Temperature: 11, PrecipitationProb: 0},
	}
	opts := ChartOptions{TempUnit: "C", GlobalTempMin: 5, GlobalTempMax: 24}
	RenderHourlyChart(frame, image.Rect(0, 0, 120, 80), hourly, opts)
}

func TestRenderHourlyChart_EqualTempRange(t *testing.T) {
	frame := newTestFrame(120, 80)
	hourly := []weather.HourlyPoint{
		{Hour: 10, Temperature: 15, PrecipitationProb: 0.5},
	}
	opts := ChartOptions{
		TempUnit:      "C",
		GlobalTempMin: 15,
		GlobalTempMax: 15,
	}
	RenderHourlyChart(frame, image.Rect(0, 0, 120, 80), hourly, opts)
}

func TestRenderHourlyChart_HighPrecip(t *testing.T) {
	frame := newTestFrame(120, 80)
	hourly := []weather.HourlyPoint{
		{Hour: 12, Temperature: 15, PrecipitationProb: 0.95},
		{Hour: 13, Temperature: 16, PrecipitationProb: 0.01},
	}
	opts := ChartOptions{
		TempUnit:      "C",
		GlobalTempMin: 5,
		GlobalTempMax: 24,
	}
	RenderHourlyChart(frame, image.Rect(0, 0, 120, 80), hourly, opts)
}

func TestFilterHours(t *testing.T) {
	hourly := sampleHourly()
	filtered := filterHours(hourly, 6, 20)
	if len(filtered) != 15 {
		t.Errorf("got %d filtered, want 15", len(filtered))
	}
	if filtered[0].Hour != 6 {
		t.Errorf("first hour = %d, want 6", filtered[0].Hour)
	}
	if filtered[len(filtered)-1].Hour != 20 {
		t.Errorf("last hour = %d, want 20", filtered[len(filtered)-1].Hour)
	}
}

func TestFilterHours_Empty(t *testing.T) {
	filtered := filterHours(nil, 6, 20)
	if len(filtered) != 0 {
		t.Errorf("got %d, want 0", len(filtered))
	}
}

func TestRenderHourlyChart_VerySmallHeight(t *testing.T) {
	frame := newTestFrame(120, 22)
	opts := ChartOptions{TempUnit: "C", GlobalTempMin: 5, GlobalTempMax: 24}
	RenderHourlyChart(frame, image.Rect(0, 0, 120, 22), sampleHourly(), opts)
}

func TestRenderHourlyChart_ZeroPrecip(t *testing.T) {
	frame := newTestFrame(120, 80)
	hourly := []weather.HourlyPoint{
		{Hour: 10, Temperature: 15, PrecipitationProb: 0},
		{Hour: 12, Temperature: 18, PrecipitationProb: 0},
	}
	opts := ChartOptions{TempUnit: "C", GlobalTempMin: 5, GlobalTempMax: 24}
	RenderHourlyChart(frame, image.Rect(0, 0, 120, 80), hourly, opts)
}
