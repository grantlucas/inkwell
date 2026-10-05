package weatherview

import (
	"image"
	"math"
	"testing"

	"github.com/grantlucas/inkwell/internal/inkwell/testutil"
	"github.com/grantlucas/inkwell/internal/inkwell/weather"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
)

// sharedRange is the temperature scale every combined-chart test draws
// against, standing in for the one a screen computes across all its days.
var sharedRange = TempRange{Min: 0, Max: 30}

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

// warmHourly is a day at the top of sharedRange, so the temperature line
// runs along the top two rows of the plot and leaves every bar shorter
// than the plot untouched. Tests about the bars use it to keep the line
// out of the way.
func warmHourly(probs map[int]float64) []weather.HourlyPoint {
	return flatHourly(sharedRange.Max, probs)
}

// countIndex reports how many pixels in frame carry the given palette index.
func countIndex(frame *image.Paletted, idx uint8) int {
	n := 0
	for _, px := range frame.Pix {
		if px == idx {
			n++
		}
	}
	return n
}

// slotX is the left edge of an hour's column.
func slotX(bounds image.Rectangle, hour int) int {
	step := float64(bounds.Dx()) / float64(precipHours)
	return bounds.Min.X + int(float64(hour-precipStartHour)*step)
}

// slotCentreX is the column the temperature line passes through for an
// hour: the centre of that hour's bar.
func slotCentreX(bounds image.Rectangle, hour int) int {
	step := float64(bounds.Dx()) / float64(precipHours)
	barW := max(int(step)-1, 2)
	return slotX(bounds, hour) + barW/2
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

// rowRuns reports how many inked pixels and how many separate inked runs
// a single row of the frame holds.
func rowRuns(frame *image.Paletted, bounds image.Rectangle, y int) (inked, runs int) {
	prev := false
	for x := bounds.Min.X; x < bounds.Max.X; x++ {
		on := frame.ColorIndexAt(x, y) != widget.PaperWhite
		if on {
			inked++
			if !prev {
				runs++
			}
		}
		prev = on
	}
	return inked, runs
}

// findBaseline returns the y of the full-width PaperBlack rule, or -1.
func findBaseline(frame *image.Paletted, bounds image.Rectangle) int {
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		if inked, runs := rowRuns(frame, bounds, y); inked == bounds.Dx() && runs == 1 {
			return y
		}
	}
	return -1
}

// barHeightAt measures the contiguous run of ink standing on the
// baseline in the column just inside the given hour's slot.
func barHeightAt(t *testing.T, frame *image.Paletted, bounds image.Rectangle, hour int) int {
	t.Helper()
	baseline := findBaseline(frame, bounds)
	if baseline == -1 {
		t.Fatal("no baseline found")
	}
	x := slotX(bounds, hour) + 1
	h := 0
	for y := baseline - 1; y >= bounds.Min.Y && frame.ColorIndexAt(x, y) != widget.PaperWhite; y-- {
		h++
	}
	return h
}

// tallestBlackRun returns the longest contiguous vertical run of
// PaperBlack anywhere in bounds.
func tallestBlackRun(frame *image.Paletted, bounds image.Rectangle) int {
	best := 0
	for x := bounds.Min.X; x < bounds.Max.X; x++ {
		run := 0
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			if frame.ColorIndexAt(x, y) == widget.PaperBlack {
				run++
				best = max(best, run)
			} else {
				run = 0
			}
		}
	}
	return best
}

// inkClusters returns the centre x of each group of inked columns within
// the rows [y1,y2). Gaps narrower than maxGap are treated as part of the
// same group, so the two digits of "12" count as one label rather than
// two.
func inkClusters(frame *image.Paletted, bounds image.Rectangle, y1, y2, maxGap int) []int {
	inked := func(x int) bool {
		for y := y1; y < y2; y++ {
			if frame.ColorIndexAt(x, y) != widget.PaperWhite {
				return true
			}
		}
		return false
	}

	var centers []int
	start, gap := -1, 0
	for x := bounds.Min.X; x < bounds.Max.X; x++ {
		switch {
		case inked(x):
			if start == -1 {
				start = x
			}
			gap = 0
		case start != -1:
			gap++
			if gap >= maxGap {
				centers = append(centers, (start+x-gap)/2)
				start = -1
			}
		}
	}
	if start != -1 {
		centers = append(centers, (start+bounds.Max.X)/2)
	}
	return centers
}

// A wet day draws its bars in PaperGray70 with a PaperBlack cap — the
// pairing CLAUDE.md pins as the one that lands dark-gray on Gray4 and
// solid black under the BW threshold, with the cap carrying the value
// at distance.
func TestRenderCombinedChart_BarsAreGray70WithBlackCap(t *testing.T) {
	bounds := image.Rect(0, 0, 312, 120)
	frame := newTestFrame(312, 120)
	RenderCombinedChart(frame, bounds, warmHourly(map[int]float64{12: 0.8}), sharedRange, CombinedOptions{})

	x := slotX(bounds, 12) + 1
	ys := inkRows(frame, x, bounds.Min.Y+tempLineW, findBaseline(frame, bounds))
	if len(ys) == 0 {
		t.Fatal("no bar drawn")
	}
	if got := frame.ColorIndexAt(x, ys[0]); got != widget.PaperBlack {
		t.Errorf("bar top = index %d, want PaperBlack cap", got)
	}
	for _, y := range ys[1:] {
		if got := frame.ColorIndexAt(x, y); got != widget.PaperGray70 {
			t.Fatalf("bar body at y=%d = index %d, want PaperGray70", y, got)
		}
	}
}

// The baseline is a solid PaperBlack rule across the whole cell width —
// it is what grounds the bars, so a run that stops short of either edge
// would read as a broken axis.
func TestRenderCombinedChart_BaselineSpansFullWidth(t *testing.T) {
	bounds := image.Rect(0, 0, 312, 120)
	frame := newTestFrame(312, 120)
	RenderCombinedChart(frame, bounds, warmHourly(map[int]float64{12: 0.8}), sharedRange, CombinedOptions{})

	baseline := findBaseline(frame, bounds)
	if baseline == -1 {
		t.Fatal("no full-width baseline found")
	}
	for x := range 312 {
		if got := frame.ColorIndexAt(x, baseline); got != widget.PaperBlack {
			t.Fatalf("baseline at x=%d = index %d, want PaperBlack", x, got)
		}
	}
}

// The window is 06:00–21:00 inclusive, one hour wider than the live
// chart's 06–20, so an evening shower lands on the chart rather than off
// the end of it. Hours outside the window contribute nothing.
func TestRenderCombinedChart_HourWindow(t *testing.T) {
	cases := []struct {
		label   string
		hour    int
		wantBar bool
	}{
		{"hour before window", 5, false},
		{"first hour in window", 6, true},
		{"last hour in window", 21, true},
		{"hour after window", 22, false},
	}

	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			frame := newTestFrame(312, 120)
			RenderCombinedChart(frame, image.Rect(0, 0, 312, 120),
				warmHourly(map[int]float64{tc.hour: 0.9}), sharedRange, CombinedOptions{})

			got := countIndex(frame, widget.PaperGray70) > 0
			if got != tc.wantBar {
				t.Errorf("hour %d drew bar = %v, want %v", tc.hour, got, tc.wantBar)
			}
		})
	}
}

// A dry day — a peak below 15% across the window — draws no bars, but the
// chart is never blank: the baseline, ticks and temperature line still
// draw, so a dry day shows the shape of its temperature rather than a
// cell that reads as broken.
func TestRenderCombinedChart_DryDay(t *testing.T) {
	cases := []struct {
		label    string
		peak     float64
		wantBars bool
	}{
		{"bone dry", 0, false},
		{"just under threshold", 0.14, false},
		{"exactly at threshold", 0.15, true},
		{"above threshold", 0.16, true},
	}

	const w, h = 312, 120
	bounds := image.Rect(0, 0, w, h)
	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			frame := newTestFrame(w, h)
			RenderCombinedChart(frame, bounds, flatHourly(15, map[int]float64{12: tc.peak}),
				sharedRange, CombinedOptions{})

			if got := countIndex(frame, widget.PaperGray70) > 0; got != tc.wantBars {
				t.Errorf("drew bars = %v, want %v", got, tc.wantBars)
			}
			baseline := findBaseline(frame, bounds)
			if baseline == -1 {
				t.Fatal("no baseline")
			}
			if ys := inkRows(frame, slotCentreX(bounds, 8), bounds.Min.Y, baseline); len(ys) == 0 {
				t.Error("no temperature line above the baseline")
			}
		})
	}
}

// Bar height is proportional to probability — that is the whole point of
// keeping the bar shape over a daily percentage: a rising Wednesday and a
// tapering Thursday must not draw the same picture.
func TestRenderCombinedChart_BarHeightTracksProbability(t *testing.T) {
	bounds := image.Rect(0, 0, 312, 120)
	frame := newTestFrame(312, 120)
	RenderCombinedChart(frame, bounds, warmHourly(map[int]float64{
		9: 0.25, 12: 0.5, 15: 1.0,
	}), sharedRange, CombinedOptions{})

	low := barHeightAt(t, frame, bounds, 9)
	mid := barHeightAt(t, frame, bounds, 12)
	high := barHeightAt(t, frame, bounds, 15)

	if !(low < mid && mid < high) {
		t.Errorf("bar heights not monotonic: 25%%=%d 50%%=%d 100%%=%d", low, mid, high)
	}
	// A 100% hour reaches the very top of the cell; anything shorter means
	// the scale is not anchored at 1.0 and the tallest bar of a wet day
	// would under-read. The warm line runs over its top two rows, and is
	// white there only because the bar's cap is underneath it.
	if want := findBaseline(frame, bounds) - bounds.Min.Y - tempLineW; high != want {
		t.Errorf("100%% bar stands %d px clear of the line, want %d", high, want)
	}
	if got := frame.ColorIndexAt(slotX(bounds, 15)+1, bounds.Min.Y); got != widget.PaperWhite {
		t.Errorf("top row at the 100%% bar = index %d, want the line inverted over the cap", got)
	}
}

// Within a wet day, an hour with a trace chance still draws a visible
// stub rather than rounding away to nothing — "1%" and "0%" are
// different claims, and the gap in the bar shape is what says which.
func TestRenderCombinedChart_TraceHourDrawsAStub(t *testing.T) {
	bounds := image.Rect(0, 0, 312, 120)
	frame := newTestFrame(312, 120)
	RenderCombinedChart(frame, bounds, warmHourly(map[int]float64{
		12: 0.9,   // carries the day past the dry threshold
		8:  0.001, // trace
		9:  0,     // none
	}), sharedRange, CombinedOptions{})

	trace := barHeightAt(t, frame, bounds, 8)
	none := barHeightAt(t, frame, bounds, 9)
	if trace <= none {
		t.Errorf("trace hour drew %d px, none-hour drew %d px; want the trace taller", trace, none)
	}
}

// The axis carries three 24-hour marks — 6, 12 and 18. The live chart's
// "6 9 12 3 8" mixes morning and afternoon on one axis and has to be
// worked out; these match the 15:04 format the rest of the panel is set
// in, and each sits under its own hour slot, below the ticks and inside
// the cell. The chart sizes that band itself, so the caller only gives it
// a rect.
func TestRenderCombinedChart_HourLabelsAt6_12_18(t *testing.T) {
	bounds := image.Rect(0, 0, 312, 120)
	frame := newTestFrame(312, 120)
	RenderCombinedChart(frame, bounds, warmHourly(map[int]float64{12: 0.8}), sharedRange, CombinedOptions{})

	bandTop := findBaseline(frame, bounds) + 1 + precipTickH
	centers := inkClusters(frame, bounds, bandTop, bounds.Max.Y, 4)
	if len(centers) != 3 {
		t.Fatalf("got %d label clusters at %v, want 3 (6/12/18)", len(centers), centers)
	}

	step := float64(bounds.Dx()) / float64(precipHours)
	for i, hour := range []int{6, 12, 18} {
		want := slotX(bounds, hour) + int(step)/2
		if diff := centers[i] - want; diff < -6 || diff > 6 {
			t.Errorf("label %d centered at x=%d, want ~%d", hour, centers[i], want)
		}
	}
}

// Base ticks ground the axis, one per hour slot, dropping below the
// baseline so the solid rule does not swallow them. Single pixels survive
// only as on/off, so they are PaperBlack.
func TestRenderCombinedChart_BaseTicks(t *testing.T) {
	bounds := image.Rect(0, 0, 312, 120)
	frame := newTestFrame(312, 120)
	RenderCombinedChart(frame, bounds, warmHourly(map[int]float64{12: 0.8}), sharedRange, CombinedOptions{})

	baseline := findBaseline(frame, bounds)
	if baseline == -1 {
		t.Fatal("no baseline found")
	}
	_, runs := rowRuns(frame, bounds, baseline+1)
	if runs != precipHours {
		t.Errorf("got %d ticks below the baseline, want %d (one per hour)", runs, precipHours)
	}
}

// The now-marker is a 2 px solid PaperBlack stroke at the current hour. A
// solid stroke is the only treatment that survives all three render paths
// unchanged — a soft fill collapses to white in Gray4's light bucket and
// snaps away under the BW threshold. One pixel disappears at a metre;
// three starts competing with the bars it sits behind.
func TestRenderCombinedChart_NowMarker(t *testing.T) {
	bounds := image.Rect(0, 0, 312, 120)

	cases := []struct {
		label     string
		show      bool
		nowHour   int
		wantWidth int
	}{
		{"marker off", false, 18, 0},
		{"marker on, hour in window", true, 18, 2},
		{"marker on, hour before window", true, 3, 0},
		{"marker on, hour after window", true, 23, 0},
	}

	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			frame := newTestFrame(312, 120)
			RenderCombinedChart(frame, bounds, warmHourly(map[int]float64{12: 0.8}), sharedRange,
				CombinedOptions{ShowNowMarker: tc.show, NowHour: tc.nowHour})

			// The plot is ~100 px tall; only the marker produces a black
			// run anywhere near that. The 80% bar's cap is 1 px.
			width := 0
			for x := range 312 {
				if tallestBlackRun(frame, image.Rect(x, 0, x+1, 120)) >= 90 {
					width++
				}
			}
			if width != tc.wantWidth {
				t.Errorf("marker is %d px wide, want %d", width, tc.wantWidth)
			}
		})
	}
}

// Over bare paper the line is black: a bar-less hour's column carries
// PaperBlack above the baseline.
func TestRenderCombinedChart_LineIsBlackOverPaper(t *testing.T) {
	bounds := image.Rect(0, 0, 312, 120)
	frame := newTestFrame(312, 120)
	RenderCombinedChart(frame, bounds, flatHourly(15, map[int]float64{20: 0.9}), sharedRange, CombinedOptions{})

	x := slotCentreX(bounds, 8)
	ys := inkRows(frame, x, bounds.Min.Y, findBaseline(frame, bounds))
	if len(ys) != tempLineW {
		t.Fatalf("line over paper is %d px thick, want %d", len(ys), tempLineW)
	}
	for _, y := range ys {
		if got := frame.ColorIndexAt(x, y); got != widget.PaperBlack {
			t.Fatalf("line pixel over paper at y=%d is index %d, want PaperBlack", y, got)
		}
	}
}

// Where the line crosses a drawn bar it is white, decided from the pixel
// underneath, so it reads against the bar whether the bar lands dark gray
// (Gray4) or solid black (BW). A 100% bar fills the plot and its fill is
// solid, so white anywhere inside it below the cap can only be the line.
func TestRenderCombinedChart_LineIsWhiteOverBar(t *testing.T) {
	bounds := image.Rect(0, 0, 312, 120)
	frame := newTestFrame(312, 120)
	RenderCombinedChart(frame, bounds, flatHourly(15, map[int]float64{12: 1.0}), sharedRange, CombinedOptions{})

	x := slotCentreX(bounds, 12)
	for y := bounds.Min.Y + 1; y < findBaseline(frame, bounds); y++ {
		if frame.ColorIndexAt(x, y) == widget.PaperWhite {
			return
		}
	}
	t.Error("no white line pixel inside the bar")
}

// Every day on a screen shares one scale, so a cold day sits lower than a
// warm one, and the extremes of the range reach the edges of the plot.
func TestRenderCombinedChart_LineHeightFollowsSharedRange(t *testing.T) {
	bounds := image.Rect(0, 0, 312, 120)

	lineY := func(temp float64) (top, bottom, baseline int) {
		frame := newTestFrame(312, 120)
		RenderCombinedChart(frame, bounds, flatHourly(temp, nil), sharedRange, CombinedOptions{})
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

// A plot too short to give the line any travel draws no line at all,
// rather than a stub that sits on the baseline.
func TestRenderCombinedChart_PlotTooShortForLineDrawsNoLine(t *testing.T) {
	// Two rows above the baseline once the chart has taken its label
	// band and ticks: one short of what a 2 px line needs.
	h := precipLabelH + precipTickH + 1 + tempLineW
	bounds := image.Rect(0, 0, 150, h)
	frame := newTestFrame(150, h)
	RenderCombinedChart(frame, bounds, flatHourly(15, nil), sharedRange, CombinedOptions{})

	baseline := findBaseline(frame, bounds)
	if baseline != tempLineW {
		t.Fatalf("baseline at y=%d, want %d", baseline, tempLineW)
	}
	for y := range baseline {
		if inked, _ := rowRuns(frame, bounds, y); inked != 0 {
			t.Fatalf("a line was drawn into a plot with no room for it (row %d)", y)
		}
	}
}

// Absent data is not a dry day, and a cell too small to carry a chart is
// not one either: in both the chart draws nothing, rather than a
// baseline conjured out of nothing or a partial render that reads as a
// broken widget.
func TestRenderCombinedChart_Guards(t *testing.T) {
	cases := []struct {
		label  string
		w, h   int
		hourly []weather.HourlyPoint
	}{
		{"nil hourly", 312, 120, nil},
		{"empty hourly", 312, 120, []weather.HourlyPoint{}},
		{"no hours inside the window", 312, 120, []weather.HourlyPoint{
			{Hour: 0, PrecipitationProb: 0.9},
			{Hour: 23, PrecipitationProb: 0.9},
		}},
		{"cell too narrow", 8, 120, warmHourly(map[int]float64{12: 0.9})},
		{"cell too short", 312, 8, warmHourly(map[int]float64{12: 0.9})},
		{"label band fills the cell", 312, precipLabelH + precipTickH + 1, warmHourly(map[int]float64{12: 0.9})},
	}

	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			frame := newTestFrame(tc.w, tc.h)
			RenderCombinedChart(frame, image.Rect(0, 0, tc.w, tc.h), tc.hourly, sharedRange, CombinedOptions{})
			if chartHasInk(frame) {
				t.Error("drew ink with nothing renderable")
			}
		})
	}
}

// Nothing the caller passes may put ink outside the rect it asked for.
// The draw helpers clip to the frame rather than to bounds, and on a real
// panel every widget shares one 800x480 frame, so a chart that overruns
// its cell paints over its neighbour rather than being harmlessly
// cropped.
func TestRenderCombinedChart_NeverDrawsOutsideBounds(t *testing.T) {
	healthy := image.Rect(40, 40, 190, 120)
	// One row of plot: room for a stub but not for the line.
	short := image.Rect(40, 40, 190, 40+precipLabelH+precipTickH+2)
	// The label band and ticks swallow the cell.
	swallowed := image.Rect(40, 40, 190, 50)

	cases := []struct {
		label  string
		bounds image.Rectangle
		rng    TempRange
		temp   float64
		prob   float64
	}{
		{"healthy cell", healthy, sharedRange, 15, 0.9},
		{"probability above 1.0", healthy, sharedRange, 15, 3.0},
		{"negative probability", healthy, sharedRange, 15, -1.0},
		{"trace chance in a very short plot", short, sharedRange, 15, 0.001},
		{"label band swallows the cell", swallowed, sharedRange, 15, 0.9},
		{"temperature far above the range", healthy, sharedRange, 500, 0.9},
		{"temperature far below the range", healthy, sharedRange, -500, 0.9},
		{"NaN temperature", healthy, sharedRange, math.NaN(), 0.9},
		{"collapsed range", healthy, TempRange{Min: 10, Max: 10}, 10, 0.9},
		{"inverted range", healthy, TempRange{Min: 20, Max: 5}, 12, 0.9},
	}

	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			// A cell deliberately smaller than the frame, so anything
			// drawn outside it is visible rather than clipped away.
			frame := newTestFrame(200, 200)
			RenderCombinedChart(frame, tc.bounds, flatHourly(tc.temp, map[int]float64{12: tc.prob}), tc.rng, CombinedOptions{})

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

// Goldens for the combined chart. The temperatures climb through the day
// so the line crosses the bars, and the shared range is 0–30°. Each chart
// is drawn into the top of a frame two rows taller than its cell: the
// goldens predate the chart sizing its own label band and were drawn with
// a 16 px one, so the extra two rows are the paper that band left under
// the labels. Regenerate with `go test ./... -update`.
func TestRenderCombinedChart_Golden(t *testing.T) {
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
	const bandSlack = 2

	cases := []struct {
		label  string
		w, h   int
		hourly []weather.HourlyPoint
		opts   CombinedOptions
	}{
		{"hero_over_paper", 312, 120, climb(8, 24, evening), CombinedOptions{}},
		{"hero_crossing_bars", 312, 120, climb(5, 25, midday), CombinedOptions{}},
		{"hero_dry", 312, 120, climb(8, 22, nil), CombinedOptions{}},
		{"hero_low_end", 312, 120, climb(0, 3, midday), CombinedOptions{}},
		{"hero_high_end", 312, 120, climb(27, 30, midday), CombinedOptions{}},
		{"hero_marker_on", 312, 120, climb(5, 25, midday), CombinedOptions{ShowNowMarker: true, NowHour: 13}},
		{"hero_marker_off", 312, 120, climb(5, 25, midday), CombinedOptions{ShowNowMarker: false, NowHour: 13}},
		{"badge_crossing_bars", 110, 72, climb(5, 25, midday), CombinedOptions{}},
		{"badge_marker_on", 110, 72, climb(5, 25, midday), CombinedOptions{ShowNowMarker: true, NowHour: 13}},
	}

	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			frame := newTestFrame(tc.w, tc.h)
			RenderCombinedChart(frame, image.Rect(0, 0, tc.w, tc.h-bandSlack), tc.hourly, sharedRange, tc.opts)
			testutil.AssertGoldenPNG(t, frame)
		})
	}
}
