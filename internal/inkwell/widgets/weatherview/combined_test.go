package weatherview

import (
	"image"
	"testing"

	"github.com/grantlucas/inkwell/internal/inkwell/testutil"
	"github.com/grantlucas/inkwell/internal/inkwell/weather"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
)

// sharedRange is the temperature scale every combined-chart test draws
// against, standing in for the one a screen computes across all its days.
var sharedRange = &TempRange{Min: 0, Max: 30}

// flatHourly builds a point for every hour with one temperature all day
// and precipitation taken from probs (absent hours get 0).
func flatHourly(temp float64, probs map[int]float64) []weather.HourlyPoint {
	var points []weather.HourlyPoint
	for h := range 24 {
		points = append(points, weather.HourlyPoint{
			Hour:              h,
			Temperature:       temp,
			PrecipitationProb: probs[h],
		})
	}
	return points
}

// slotCentreX is the column the temperature line passes through for an
// hour: the centre of that hour's bar.
func slotCentreX(bounds image.Rectangle, hour int) int {
	step := float64(bounds.Dx()) / float64(precipHours)
	barW := max(int(step)-1, 2)
	return bounds.Min.X + int(float64(hour-precipStartHour)*step) + barW/2
}

// inkRows returns the y of every non-white pixel in column x of
// [y1,y2).
func inkRows(frame *image.Paletted, x, y1, y2 int) []int {
	var ys []int
	for y := y1; y < y2; y++ {
		if frame.ColorIndexAt(x, y) != widget.PaperWhite {
			ys = append(ys, y)
		}
	}
	return ys
}

// Without a range the chart is byte-for-byte what it was before the
// temperature layer existed: opt-in per screen means extending the
// renderer changes no screen until that screen passes a range.
func TestRenderPrecipChart_NilRangeDrawsNoLine(t *testing.T) {
	const w, h = 312, 120
	bounds := image.Rect(0, 0, w, h)

	// A wet day whose only bar is at hour 12: any ink in the column of a
	// bar-less hour above the baseline would be the temperature line.
	frame := newTestFrame(w, h)
	RenderPrecipChart(frame, bounds, flatHourly(15, map[int]float64{12: 0.8}),
		PrecipChartOptions{LabelHeight: 16})

	baseline := findBaseline(frame, bounds)
	if ys := inkRows(frame, slotCentreX(bounds, 8), bounds.Min.Y, baseline); len(ys) != 0 {
		t.Errorf("column of a bar-less hour has ink at %v, want none without a range", ys)
	}
}

// On a dry day the chart is never blank once a range is supplied: the
// baseline, ticks and temperature line are all drawn, with no bars. The
// caller's DryText is not drawn over the line.
func TestRenderPrecipChart_DryDayWithRangeIsNeverBlank(t *testing.T) {
	const w, h = 312, 120
	bounds := image.Rect(0, 0, w, h)

	cases := []struct {
		label   string
		dryText string
	}{
		{"no dry text", ""},
		{"dry text is dropped", "NO RAIN TODAY"},
	}
	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			plain := newTestFrame(w, h)
			RenderPrecipChart(plain, bounds, flatHourly(15, nil),
				PrecipChartOptions{LabelHeight: 16, TempRange: sharedRange})
			withText := newTestFrame(w, h)
			RenderPrecipChart(withText, bounds, flatHourly(15, nil),
				PrecipChartOptions{LabelHeight: 16, TempRange: sharedRange, DryText: tc.dryText})

			baseline := findBaseline(plain, bounds)
			if baseline == -1 {
				t.Fatal("no baseline on a dry day with a range")
			}
			if got := countIndex(plain, widget.PaperGray70); got != 0 {
				t.Errorf("drew %d bar-fill pixels on a dry day, want none", got)
			}
			if ys := inkRows(plain, slotCentreX(bounds, 12), bounds.Min.Y, baseline); len(ys) == 0 {
				t.Error("no temperature line above the baseline on a dry day")
			}
			for i := range plain.Pix {
				if plain.Pix[i] != withText.Pix[i] {
					t.Fatal("DryText changed a dry day that has a temperature line")
				}
			}
		})
	}
}

// Over bare paper the line is black; where it crosses a drawn bar it is
// white, decided from the pixel underneath, so it reads against the bar
// whether the bar lands dark gray (Gray4) or solid black (BW).
func TestRenderPrecipChart_LineColourFollowsPixelUnderneath(t *testing.T) {
	const w, h = 312, 120
	bounds := image.Rect(0, 0, w, h)

	cases := []struct {
		label    string
		probs    map[int]float64
		hour     int
		wantOver uint8
	}{
		{"over paper", map[int]float64{20: 0.9}, 8, widget.PaperBlack},
		// A 100% bar fills the plot, so a mid-range line must cross it.
		{"over a bar", map[int]float64{12: 1.0}, 12, widget.PaperWhite},
	}
	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			frame := newTestFrame(w, h)
			RenderPrecipChart(frame, bounds, flatHourly(15, tc.probs),
				PrecipChartOptions{LabelHeight: 16, TempRange: sharedRange})

			baseline := findBaseline(frame, bounds)
			x := slotCentreX(bounds, tc.hour)

			// Skip the 1 px cap row at the top of a full-height bar.
			var ys []int
			for y := bounds.Min.Y + 1; y < baseline; y++ {
				switch idx := frame.ColorIndexAt(x, y); {
				case tc.wantOver == widget.PaperWhite && idx == widget.PaperWhite:
					ys = append(ys, y)
				case tc.wantOver == widget.PaperBlack && idx == widget.PaperBlack:
					ys = append(ys, y)
				}
			}
			if len(ys) == 0 {
				t.Fatalf("no %d-index line pixel in the column of hour %d", tc.wantOver, tc.hour)
			}
			if tc.wantOver == widget.PaperWhite {
				// The bar fill is solid, so white anywhere inside it can
				// only be the line.
				if got := frame.ColorIndexAt(x, baseline-1); got == widget.PaperBlack {
					t.Errorf("bar column ends in black at the baseline, want fill or line-white")
				}
			}
		})
	}
}

// Every day on a screen shares one scale, so a cold day sits lower than a
// warm one, and the extremes of the range reach the edges of the plot.
func TestRenderPrecipChart_LineHeightFollowsSharedRange(t *testing.T) {
	const w, h = 312, 120
	bounds := image.Rect(0, 0, w, h)

	lineY := func(temp float64) (top, bottom, baseline int) {
		frame := newTestFrame(w, h)
		RenderPrecipChart(frame, bounds, flatHourly(temp, nil),
			PrecipChartOptions{LabelHeight: 16, TempRange: sharedRange})
		baseline = findBaseline(frame, bounds)
		ys := inkRows(frame, slotCentreX(bounds, 12), bounds.Min.Y, baseline)
		if len(ys) == 0 {
			t.Fatalf("no line at %v°", temp)
		}
		return ys[0], ys[len(ys)-1], baseline
	}

	coldTop, coldBottom, baseline := lineY(sharedRange.Min)
	midTop, _, _ := lineY(15)
	warmTop, _, _ := lineY(sharedRange.Max)

	if !(warmTop < midTop && midTop < coldTop) {
		t.Errorf("line tops warm=%d mid=%d cold=%d, want warm < mid < cold", warmTop, midTop, coldTop)
	}
	if warmTop != bounds.Min.Y {
		t.Errorf("warmest day top = %d, want the top of the plot (%d)", warmTop, bounds.Min.Y)
	}
	if coldBottom != baseline-1 {
		t.Errorf("coldest day bottom = %d, want one row above the baseline (%d)", coldBottom, baseline-1)
	}
}

// Whatever the caller passes, no ink may land outside the cell: a range
// that does not contain the temperatures and an inverted or collapsed
// range all stay inside bounds.
func TestRenderPrecipChart_RangeNeverDrawsOutsideBounds(t *testing.T) {
	cases := []struct {
		label  string
		bounds image.Rectangle
		rng    *TempRange
		temp   float64
		opts   PrecipChartOptions
	}{
		{"temperature far above the range", image.Rect(40, 40, 190, 120), sharedRange, 500, PrecipChartOptions{LabelHeight: 16}},
		{"temperature far below the range", image.Rect(40, 40, 190, 120), sharedRange, -500, PrecipChartOptions{LabelHeight: 16}},
		{"collapsed range", image.Rect(40, 40, 190, 120), &TempRange{Min: 10, Max: 10}, 10, PrecipChartOptions{LabelHeight: 16}},
		{"inverted range", image.Rect(40, 40, 190, 120), &TempRange{Min: 20, Max: 5}, 12, PrecipChartOptions{LabelHeight: 16}},
		{"label band swallows the cell", image.Rect(40, 40, 190, 50), sharedRange, 15, PrecipChartOptions{LabelHeight: 20}},
	}

	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			frame := newTestFrame(200, 200)
			opts := tc.opts
			opts.TempRange = tc.rng
			RenderPrecipChart(frame, tc.bounds, flatHourly(tc.temp, map[int]float64{12: 0.9}), opts)

			for y := range 200 {
				for x := range 200 {
					if frame.ColorIndexAt(x, y) != widget.PaperWhite && !image.Pt(x, y).In(tc.bounds) {
						t.Fatalf("ink at (%d,%d) is outside bounds %v", x, y, tc.bounds)
					}
				}
			}
		})
	}
}

// A plot too short to give the line any travel draws no line at all,
// rather than a stub that sits on the baseline: the frame is identical to
// the precipitation-only chart.
func TestRenderPrecipChart_PlotTooShortForLineDrawsNoLine(t *testing.T) {
	// A 7 px label band leaves 2 rows above the baseline, one short of
	// what a 2 px line needs.
	bounds := image.Rect(0, 0, 150, 12)
	hourly := flatHourly(15, map[int]float64{12: 0.9})

	without := newTestFrame(150, 12)
	RenderPrecipChart(without, bounds, hourly, PrecipChartOptions{LabelHeight: 7})
	with := newTestFrame(150, 12)
	RenderPrecipChart(with, bounds, hourly, PrecipChartOptions{LabelHeight: 7, TempRange: sharedRange})

	for i := range without.Pix {
		if without.Pix[i] != with.Pix[i] {
			t.Fatal("a line was drawn into a plot with no room for it")
		}
	}
}

// A day with no hourly points in the window is absent data, not a dry
// day, so the range does not conjure a baseline out of nothing.
func TestRenderPrecipChart_RangeWithNoDataDrawsNothing(t *testing.T) {
	frame := newTestFrame(312, 120)
	RenderPrecipChart(frame, image.Rect(0, 0, 312, 120), nil,
		PrecipChartOptions{LabelHeight: 16, TempRange: sharedRange})
	if chartHasInk(frame) {
		t.Error("drew ink for absent data")
	}
}

// Goldens for the combined chart. The temperatures climb through the day
// so the line crosses the bars, and the shared range is 0–30°. Regenerate
// with `go test ./... -update`.
func TestRenderPrecipChart_GoldenCombined(t *testing.T) {
	climb := func(lo, hi float64, probs map[int]float64) []weather.HourlyPoint {
		points := flatHourly(lo, probs)
		for i := range points {
			frac := float64(points[i].Hour) / 23
			points[i].Temperature = lo + (hi-lo)*frac
		}
		return points
	}
	// Rain only in the evening, after the line has climbed above it.
	evening := map[int]float64{18: 0.3, 19: 0.4, 20: 0.35, 21: 0.3}
	// Rain across the middle of the day, under and across the line.
	midday := map[int]float64{
		9: 0.35, 10: 0.5, 11: 0.65, 12: 0.8, 13: 0.95, 14: 0.9, 15: 0.7, 16: 0.55, 17: 0.4,
	}

	cases := []struct {
		label  string
		w, h   int
		hourly []weather.HourlyPoint
		opts   PrecipChartOptions
	}{
		{"hero_over_paper", 312, 120, climb(8, 24, evening), PrecipChartOptions{LabelHeight: 16}},
		{"hero_crossing_bars", 312, 120, climb(5, 25, midday), PrecipChartOptions{LabelHeight: 16}},
		{"hero_dry", 312, 120, climb(8, 22, nil), PrecipChartOptions{LabelHeight: 16}},
		{"hero_low_end", 312, 120, climb(0, 3, midday), PrecipChartOptions{LabelHeight: 16}},
		{"hero_high_end", 312, 120, climb(27, 30, midday), PrecipChartOptions{LabelHeight: 16}},
		{"hero_marker_on", 312, 120, climb(5, 25, midday), PrecipChartOptions{
			LabelHeight: 16, ShowNowMarker: true, NowHour: 13}},
		{"hero_marker_off", 312, 120, climb(5, 25, midday), PrecipChartOptions{
			LabelHeight: 16, ShowNowMarker: false, NowHour: 13}},
		{"badge_crossing_bars", 110, 72, climb(5, 25, midday), PrecipChartOptions{LabelHeight: 16}},
		{"badge_dry", 110, 72, climb(8, 22, nil), PrecipChartOptions{}},
		{"badge_marker_on", 110, 72, climb(5, 25, midday), PrecipChartOptions{
			LabelHeight: 16, ShowNowMarker: true, NowHour: 13}},
	}

	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			frame := newTestFrame(tc.w, tc.h)
			opts := tc.opts
			opts.TempRange = sharedRange
			RenderPrecipChart(frame, image.Rect(0, 0, tc.w, tc.h), tc.hourly, opts)
			testutil.AssertGoldenPNG(t, frame)
		})
	}
}
