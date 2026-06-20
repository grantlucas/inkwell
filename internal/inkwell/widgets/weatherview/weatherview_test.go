package weatherview

import (
	"image"
	"testing"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/weather"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
)

func sampleDay() weather.DailyForecast {
	hourly := make([]weather.HourlyPoint, 24)
	for h := range 24 {
		hourly[h] = weather.HourlyPoint{
			Hour:              h,
			Temperature:       10 + float64(h)/2,
			PrecipitationProb: float64(h) / 100.0,
		}
	}
	return weather.DailyForecast{
		Date:      time.Date(2026, 4, 28, 0, 0, 0, 0, time.UTC),
		High:      22,
		Low:       10,
		Condition: weather.PartlyCloudy,
		Hourly:    hourly,
	}
}

func TestRenderDayWeather_Basic(t *testing.T) {
	frame := newTestFrame(114, 160)
	day := sampleDay()
	opts := Options{
		TempUnit:      "C",
		ShowLabel:     true,
		GlobalTempMin: 5,
		GlobalTempMax: 24,
		HighlightHour: 15,
	}

	RenderDayWeather(frame, image.Rect(0, 0, 114, 160), day, opts)

	blackCount := 0
	for y := range 160 {
		for x := range 114 {
			if frame.ColorIndexAt(x, y) == widget.PaperBlack {
				blackCount++
			}
		}
	}
	if blackCount < 50 {
		t.Errorf("too few black pixels (%d), rendering may have failed", blackCount)
	}
}

// rightmostInkInCondRow returns the largest x with a black pixel in the
// condition-row band (rows above the divider). The divider itself spans the
// full width, so it is excluded by scanning y < condRowH.
func rightmostInkInCondRow(frame *image.Paletted, w int) int {
	iconSize := min(24, w/3)
	condRowH := max(iconSize+4, 2*lineHeight+4)
	rightmost := -1
	for y := range condRowH {
		for x := range w {
			if frame.ColorIndexAt(x, y) == widget.PaperBlack && x > rightmost {
				rightmost = x
			}
		}
	}
	return rightmost
}

// The condition label and temps must hug the cell's right edge so the icon
// (left) and text (right) bookend the cell rather than clumping on the left.
func TestRenderDayWeather_ConditionRowRightAligned(t *testing.T) {
	const w, h = 114, 160
	frame := newTestFrame(w, h)
	day := sampleDay()
	day.Condition = weather.Clear // short label leaves an obvious left-pack gap pre-change
	opts := Options{
		TempUnit:      "C",
		ShowLabel:     true,
		GlobalTempMin: 5,
		GlobalTempMax: 24,
	}
	RenderDayWeather(frame, image.Rect(0, 0, w, h), day, opts)

	const rightMargin = 4
	rightEdge := w - rightMargin
	rightmost := rightmostInkInCondRow(frame, w)
	if rightmost < rightEdge-6 {
		t.Errorf("condition row not right-aligned: rightmost ink x=%d, want within 6px of right edge %d", rightmost, rightEdge)
	}
	if rightmost > rightEdge {
		t.Errorf("condition row overran right edge: rightmost ink x=%d > %d", rightmost, rightEdge)
	}
}

// maxInteriorWhiteGapInCondRow returns the widest run of fully-white columns
// strictly between the leftmost and rightmost inked columns of the condition
// row band. Right-aligning the temps opens a wide gap between the left icon
// and the right-hugging text; left-packing leaves only narrow inter-glyph
// spacing.
func maxInteriorWhiteGapInCondRow(frame *image.Paletted, w int) int {
	iconSize := min(24, w/3)
	condRowH := max(iconSize+4, 2*lineHeight+4)
	inked := make([]bool, w)
	first, last := -1, -1
	for x := range w {
		for y := range condRowH {
			if frame.ColorIndexAt(x, y) == widget.PaperBlack {
				inked[x] = true
				if first < 0 {
					first = x
				}
				last = x
				break
			}
		}
	}
	if first < 0 {
		return 0
	}
	maxGap, run := 0, 0
	for x := first; x <= last; x++ {
		if inked[x] {
			run = 0
			continue
		}
		run++
		if run > maxGap {
			maxGap = run
		}
	}
	return maxGap
}

// The icon stays flush-left while the temps hug the right edge, so a clear gap
// opens between them — they bookend the cell rather than clumping on the left.
func TestRenderDayWeather_IconAndTempsBookend(t *testing.T) {
	const w, h = 114, 160
	frame := newTestFrame(w, h)
	day := sampleDay()
	opts := Options{
		TempUnit:      "C",
		ShowLabel:     false, // isolate the temp row
		GlobalTempMin: 5,
		GlobalTempMax: 24,
	}
	RenderDayWeather(frame, image.Rect(0, 0, w, h), day, opts)

	if gap := maxInteriorWhiteGapInCondRow(frame, w); gap < 12 {
		t.Errorf("no bookend gap between icon and temps: widest interior white gap=%dpx, want >= 12px", gap)
	}
}

func TestRenderDayWeather_Fahrenheit(t *testing.T) {
	frame := newTestFrame(114, 160)
	day := sampleDay()
	opts := Options{
		TempUnit:      "F",
		ShowLabel:     true,
		GlobalTempMin: 5,
		GlobalTempMax: 24,
		HighlightHour: 15,
	}
	RenderDayWeather(frame, image.Rect(0, 0, 114, 160), day, opts)
}

func TestRenderDayWeather_NoLabel(t *testing.T) {
	frame := newTestFrame(114, 160)
	day := sampleDay()
	opts := Options{
		TempUnit:      "C",
		ShowLabel:     false,
		GlobalTempMin: 5,
		GlobalTempMax: 24,
	}
	RenderDayWeather(frame, image.Rect(0, 0, 114, 160), day, opts)
}

func TestRenderDayWeather_TooSmall(t *testing.T) {
	frame := newTestFrame(10, 10)
	day := sampleDay()
	opts := Options{TempUnit: "C"}
	RenderDayWeather(frame, image.Rect(0, 0, 10, 10), day, opts)
}

func TestRenderDayWeather_CustomIconSize(t *testing.T) {
	frame := newTestFrame(114, 160)
	day := sampleDay()
	opts := Options{
		TempUnit:      "C",
		ShowLabel:     true,
		GlobalTempMin: 5,
		GlobalTempMax: 24,
		IconSize:      32,
	}
	RenderDayWeather(frame, image.Rect(0, 0, 114, 160), day, opts)
}

// When DrawIcon fails the day cell still has to render — the rest of
// the row (label, temps, chart) is independent of the glyph. Pin that
// behavior by swapping in bad font data while RenderDayWeather runs.
func TestRenderDayWeather_DrawIconErrorRendersRest(t *testing.T) {
	orig := fontData
	fontData = []byte("not a font")
	defer func() { fontData = orig }()

	frame := newTestFrame(114, 160)
	day := sampleDay()
	opts := Options{
		TempUnit:      "C",
		ShowLabel:     true,
		GlobalTempMin: 5,
		GlobalTempMax: 24,
	}
	RenderDayWeather(frame, image.Rect(0, 0, 114, 160), day, opts)

	// The chart and temps should still produce non-white pixels even
	// though the icon glyph failed.
	hasInk := false
	for _, px := range frame.Pix {
		if px != widget.PaperWhite {
			hasInk = true
			break
		}
	}
	if !hasInk {
		t.Error("RenderDayWeather produced no pixels with bad font data — icon failure shouldn't blank the row")
	}
}

func TestRenderDayWeather_SmallChart(t *testing.T) {
	frame := newTestFrame(114, 50)
	day := sampleDay()
	opts := Options{
		TempUnit:      "C",
		ShowLabel:     true,
		GlobalTempMin: 5,
		GlobalTempMax: 24,
	}
	RenderDayWeather(frame, image.Rect(0, 0, 114, 50), day, opts)
}

func TestGlobalTempRange(t *testing.T) {
	days := []weather.DailyForecast{
		{High: 20, Low: 8, Hourly: []weather.HourlyPoint{
			{Temperature: 10}, {Temperature: 18},
		}},
		{High: 25, Low: 12, Hourly: []weather.HourlyPoint{
			{Temperature: 13}, {Temperature: 24},
		}},
	}

	minT, maxT := GlobalTempRange(days)
	if minT != 8 {
		t.Errorf("minTemp = %v, want 8", minT)
	}
	if maxT != 25 {
		t.Errorf("maxTemp = %v, want 25", maxT)
	}
}

func TestGlobalTempRange_Empty(t *testing.T) {
	minT, maxT := GlobalTempRange(nil)
	if minT != 0 {
		t.Errorf("minTemp = %v, want 0", minT)
	}
	if maxT != 25 {
		t.Errorf("maxTemp = %v, want 25", maxT)
	}
}

func TestGlobalTempRange_NoHourly(t *testing.T) {
	days := []weather.DailyForecast{
		{High: 20, Low: 8},
	}
	minT, maxT := GlobalTempRange(days)
	if minT != 8 {
		t.Errorf("minTemp = %v, want 8", minT)
	}
	if maxT != 20 {
		t.Errorf("maxTemp = %v, want 20", maxT)
	}
}
