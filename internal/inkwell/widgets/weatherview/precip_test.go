package weatherview

import (
	"image"
	"testing"

	"github.com/grantlucas/inkwell/internal/inkwell/fonts"
	"github.com/grantlucas/inkwell/internal/inkwell/testutil"
	"github.com/grantlucas/inkwell/internal/inkwell/weather"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
)

// wetHourly builds a point for every hour of the day, with prob taken
// from probs keyed by hour. Hours absent from probs get 0.
func wetHourly(probs map[int]float64) []weather.HourlyPoint {
	var points []weather.HourlyPoint
	for h := range 24 {
		points = append(points, weather.HourlyPoint{
			Hour:              h,
			Temperature:       10 + float64(h)/2,
			PrecipitationProb: probs[h],
		})
	}
	return points
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

// A wet day draws its bars in PaperGray70 with a PaperBlack cap — the
// pairing CLAUDE.md pins as the one that lands dark-gray on Gray4 and
// solid black under the BW threshold, with the cap carrying the value
// at distance.
func TestRenderPrecipChart_BarsAreGray70WithBlackCap(t *testing.T) {
	frame := newTestFrame(312, 120)
	RenderPrecipChart(frame, image.Rect(0, 0, 312, 120), wetHourly(map[int]float64{12: 0.8}), PrecipChartOptions{})

	if got := countIndex(frame, widget.PaperGray70); got == 0 {
		t.Error("no PaperGray70 bar fill drawn")
	}
	if got := countIndex(frame, widget.PaperBlack); got == 0 {
		t.Error("no PaperBlack drawn (cap/baseline missing)")
	}
}

// The baseline is a solid PaperBlack rule across the whole cell width —
// it is what grounds the bars, so a run that stops short of either edge
// would read as a broken axis.
func TestRenderPrecipChart_BaselineSpansFullWidth(t *testing.T) {
	const w, h = 312, 120
	frame := newTestFrame(w, h)
	RenderPrecipChart(frame, image.Rect(0, 0, w, h), wetHourly(map[int]float64{12: 0.8}), PrecipChartOptions{})

	baselineY := -1
	for y := range h {
		full := true
		for x := range w {
			if frame.ColorIndexAt(x, y) != widget.PaperBlack {
				full = false
				break
			}
		}
		if full {
			baselineY = y
			break
		}
	}
	if baselineY == -1 {
		t.Fatal("no full-width PaperBlack baseline found")
	}
}

// The window is 06:00–21:00 inclusive, one hour wider than the live
// chart's 06–20, so an evening shower lands on the chart rather than off
// the end of it. Hours outside the window contribute nothing.
func TestRenderPrecipChart_HourWindow(t *testing.T) {
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
			RenderPrecipChart(frame, image.Rect(0, 0, 312, 120),
				wetHourly(map[int]float64{tc.hour: 0.9}), PrecipChartOptions{})

			got := countIndex(frame, widget.PaperGray70) > 0
			if got != tc.wantBar {
				t.Errorf("hour %d drew bar = %v, want %v", tc.hour, got, tc.wantBar)
			}
		})
	}
}

// A dry day draws nothing at all — no baseline, no stubs, no axis. A flat
// row of stubs reads as a broken widget from across the room; silence
// does not. The threshold is peak probability across the window below
// 15%. A caller that wants something in the space supplies DryText, which
// is drawn centred and is then the only ink on the cell.
func TestRenderPrecipChart_DryDay(t *testing.T) {
	cases := []struct {
		label    string
		peak     float64
		dryText  string
		wantInk  bool
		wantBars bool
	}{
		{"bone dry, no text", 0, "", false, false},
		{"bone dry, with text", 0, "NO RAIN TODAY", true, false},
		{"just under threshold", 0.14, "", false, false},
		{"just under threshold, with text", 0.14, "NO RAIN TODAY", true, false},
		{"exactly at threshold", 0.15, "", true, true},
		{"above threshold", 0.16, "", true, true},
	}

	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			frame := newTestFrame(312, 120)
			RenderPrecipChart(frame, image.Rect(0, 0, 312, 120),
				wetHourly(map[int]float64{12: tc.peak}),
				PrecipChartOptions{DryText: tc.dryText})

			if got := chartHasInk(frame); got != tc.wantInk {
				t.Errorf("hasInk = %v, want %v", got, tc.wantInk)
			}
			if got := countIndex(frame, widget.PaperGray70) > 0; got != tc.wantBars {
				t.Errorf("drew bars = %v, want %v", got, tc.wantBars)
			}
		})
	}
}

// barHeightAt measures the tallest contiguous run of bar ink (fill or
// cap) ending at the baseline, scanning the column nearest the centre of
// the given hour's slot.
func barHeightAt(t *testing.T, frame *image.Paletted, bounds image.Rectangle, hour int) int {
	t.Helper()
	step := float64(bounds.Dx()) / float64(precipHours)
	x := bounds.Min.X + int(float64(hour-precipStartHour)*step) + 1

	h := 0
	for y := bounds.Max.Y - 1; y >= bounds.Min.Y; y-- {
		idx := frame.ColorIndexAt(x, y)
		if idx == widget.PaperGray70 || idx == widget.PaperBlack {
			h++
			continue
		}
		if h > 0 {
			break
		}
	}
	return h
}

// Bar height is proportional to probability — that is the whole point of
// keeping the bar shape over a daily percentage: a rising Wednesday and a
// tapering Thursday must not draw the same picture.
func TestRenderPrecipChart_BarHeightTracksProbability(t *testing.T) {
	bounds := image.Rect(0, 0, 312, 120)
	frame := newTestFrame(312, 120)
	RenderPrecipChart(frame, bounds, wetHourly(map[int]float64{
		9: 0.25, 12: 0.5, 15: 1.0,
	}), PrecipChartOptions{})

	low := barHeightAt(t, frame, bounds, 9)
	mid := barHeightAt(t, frame, bounds, 12)
	high := barHeightAt(t, frame, bounds, 15)

	if !(low < mid && mid < high) {
		t.Errorf("bar heights not monotonic: 25%%=%d 50%%=%d 100%%=%d", low, mid, high)
	}
	// A 100% hour reaches the very top of the cell; anything shorter means
	// the scale is not anchored at 1.0 and the tallest bar of a wet day
	// would under-read.
	step := float64(bounds.Dx()) / float64(precipHours)
	topX := bounds.Min.X + int(float64(15-precipStartHour)*step) + 1
	if got := frame.ColorIndexAt(topX, bounds.Min.Y); got != widget.PaperBlack {
		t.Errorf("top row at the 100%% bar = index %d, want PaperBlack cap", got)
	}
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

// The axis carries three 24-hour marks — 6, 12 and 18. The live chart's
// "6 9 12 3 8" mixes morning and afternoon on one axis and has to be
// worked out; these match the 15:04 format the rest of the panel is set
// in, and each sits under its own hour slot.
func TestRenderPrecipChart_HourLabelsAt6_12_18(t *testing.T) {
	const labelH = 16
	bounds := image.Rect(0, 0, 312, 120)
	frame := newTestFrame(312, 120)
	RenderPrecipChart(frame, bounds, wetHourly(map[int]float64{12: 0.8}),
		PrecipChartOptions{LabelHeight: labelH})

	bandTop := bounds.Max.Y - labelH
	centers := inkClusters(frame, bounds, bandTop, bounds.Max.Y, 4)
	if len(centers) != 3 {
		t.Fatalf("got %d label clusters at %v, want 3 (6/12/18)", len(centers), centers)
	}

	step := float64(bounds.Dx()) / float64(precipHours)
	for i, hour := range []int{6, 12, 18} {
		want := bounds.Min.X + int(float64(hour-precipStartHour)*step) + int(step)/2
		if diff := centers[i] - want; diff < -6 || diff > 6 {
			t.Errorf("label %d centered at x=%d, want ~%d", hour, centers[i], want)
		}
	}
}

// LabelHeight zero is the narrow-cell case: no axis labels, and the bars
// take the band back rather than leaving a white gutter.
func TestRenderPrecipChart_NoLabelBandWhenHeightZero(t *testing.T) {
	const w, h = 110, 80
	bounds := image.Rect(0, 0, w, h)

	withBand := newTestFrame(w, h)
	RenderPrecipChart(withBand, bounds, wetHourly(map[int]float64{12: 1.0}),
		PrecipChartOptions{LabelHeight: 16})
	without := newTestFrame(w, h)
	RenderPrecipChart(without, bounds, wetHourly(map[int]float64{12: 1.0}),
		PrecipChartOptions{})

	banded := barHeightAt(t, withBand, bounds, 12)
	full := barHeightAt(t, without, bounds, 12)
	if full <= banded {
		t.Errorf("bar height without label band = %d, want > %d (banded)", full, banded)
	}
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

// The now-marker is a 2 px solid PaperBlack stroke at the current hour. A
// solid stroke is the only treatment that survives all three render paths
// unchanged — a soft fill collapses to white in Gray4's light bucket and
// snaps away under the BW threshold.
func TestRenderPrecipChart_NowMarker(t *testing.T) {
	const w, h = 312, 120
	bounds := image.Rect(0, 0, w, h)

	cases := []struct {
		label      string
		show       bool
		nowHour    int
		wantMarker bool
	}{
		{"marker off", false, 18, false},
		{"marker on, hour in window", true, 18, true},
		{"marker on, hour before window", true, 3, false},
		{"marker on, hour after window", true, 23, false},
	}

	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			frame := newTestFrame(w, h)
			RenderPrecipChart(frame, bounds, wetHourly(map[int]float64{12: 0.8}),
				PrecipChartOptions{ShowNowMarker: tc.show, NowHour: tc.nowHour})

			// The plot is ~119 px tall; only the marker produces a black
			// run anywhere near that. The 80% bar's cap is 1 px.
			got := tallestBlackRun(frame, bounds) >= 100
			if got != tc.wantMarker {
				t.Errorf("marker present = %v, want %v", got, tc.wantMarker)
			}
		})
	}
}

// The marker is 2 px wide — one pixel disappears at a metre, three starts
// competing with the bars it is meant to sit behind.
func TestRenderPrecipChart_NowMarkerIsTwoPixelsWide(t *testing.T) {
	const w, h = 312, 120
	bounds := image.Rect(0, 0, w, h)
	frame := newTestFrame(w, h)
	RenderPrecipChart(frame, bounds, wetHourly(map[int]float64{12: 0.8}),
		PrecipChartOptions{ShowNowMarker: true, NowHour: 18})

	wide := 0
	for x := range w {
		run := 0
		for y := range h {
			if frame.ColorIndexAt(x, y) == widget.PaperBlack {
				run++
			} else {
				run = 0
			}
			if run >= 100 {
				wide++
				break
			}
		}
	}
	if wide != 2 {
		t.Errorf("marker is %d px wide, want 2", wide)
	}
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

// The 50% guide is off by default — only today-hero asks for it, and
// bold-five and row-agenda explicitly do not want a horizontal line
// across their cells. When on, it is dashed so it reads as a reference
// rather than competing with the solid baseline.
func TestRenderPrecipChart_GuideLine(t *testing.T) {
	const w, h = 312, 120
	bounds := image.Rect(0, 0, w, h)
	// A 20% bar is well clear of the 50% line, so anything found on that
	// row is the guide and not bar ink.
	hourly := wetHourly(map[int]float64{12: 0.2})

	// The 50% row sits halfway between the top of the cell and the
	// baseline, wherever the baseline lands.
	guideRow := func(frame *image.Paletted) int {
		baselineY := findBaseline(frame, bounds)
		if baselineY == -1 {
			t.Fatal("no baseline found")
		}
		return baselineY - (baselineY-bounds.Min.Y)/2
	}

	t.Run("off by default", func(t *testing.T) {
		frame := newTestFrame(w, h)
		RenderPrecipChart(frame, bounds, hourly, PrecipChartOptions{})
		if inked, _ := rowRuns(frame, bounds, guideRow(frame)); inked != 0 {
			t.Errorf("%d inked px on the 50%% row with guide off, want 0", inked)
		}
	})

	t.Run("dashed when on", func(t *testing.T) {
		frame := newTestFrame(w, h)
		RenderPrecipChart(frame, bounds, hourly, PrecipChartOptions{ShowGuide: true})
		inked, runs := rowRuns(frame, bounds, guideRow(frame))
		if inked == 0 {
			t.Fatal("no guide line drawn")
		}
		if inked == w {
			t.Error("guide line is solid across the full width, want dashed")
		}
		if runs < 10 {
			t.Errorf("guide line has %d runs, want a dash pattern (>=10)", runs)
		}
	})
}

// Base ticks ground the axis, one per hour slot, dropping below the
// baseline so the solid rule does not swallow them. Single pixels survive
// only as on/off, so they are PaperBlack.
func TestRenderPrecipChart_BaseTicks(t *testing.T) {
	const w, h = 312, 120
	bounds := image.Rect(0, 0, w, h)
	frame := newTestFrame(w, h)
	RenderPrecipChart(frame, bounds, wetHourly(map[int]float64{12: 0.8}), PrecipChartOptions{})

	baselineY := findBaseline(frame, bounds)
	if baselineY == -1 {
		t.Fatal("no baseline found")
	}
	if baselineY >= h-1 {
		t.Fatalf("baseline at y=%d leaves no room for ticks below it", baselineY)
	}

	_, runs := rowRuns(frame, bounds, baselineY+1)
	if runs != precipHours {
		t.Errorf("got %d ticks below the baseline, want %d (one per hour)", runs, precipHours)
	}
}

// Absent data is not a dry day. With no hourly points in the window the
// chart draws nothing — including no DryText, because "NO RAIN TODAY" on
// a failed forecast fetch is a claim the widget cannot make.
func TestRenderPrecipChart_Guards(t *testing.T) {
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
		{"cell too narrow", 8, 120, wetHourly(map[int]float64{12: 0.9})},
		{"cell too short", 312, 8, wetHourly(map[int]float64{12: 0.9})},
	}

	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			frame := newTestFrame(tc.w, tc.h)
			RenderPrecipChart(frame, image.Rect(0, 0, tc.w, tc.h), tc.hourly,
				PrecipChartOptions{DryText: "NO RAIN TODAY", LabelHeight: 16})
			if chartHasInk(frame) {
				t.Error("drew ink with nothing renderable")
			}
		})
	}
}

// The caller supplies the label face, which is what lets one renderer
// serve a 312 px hero cell and a 110 px row badge: the hero sets its axis
// larger without a second code path.
func TestRenderPrecipChart_UsesCallerLabelFace(t *testing.T) {
	const w, h = 312, 140
	const labelH = 24
	bounds := image.Rect(0, 0, w, h)
	hourly := wetHourly(map[int]float64{12: 0.8})

	big, err := fonts.Face(fonts.Bold, 16)
	if err != nil {
		t.Fatalf("load face: %v", err)
	}

	bandInk := func(opts PrecipChartOptions) int {
		frame := newTestFrame(w, h)
		RenderPrecipChart(frame, bounds, hourly, opts)
		n := 0
		for y := bounds.Max.Y - labelH; y < bounds.Max.Y; y++ {
			if inked, _ := rowRuns(frame, bounds, y); inked > 0 {
				n += inked
			}
		}
		return n
	}

	def := bandInk(PrecipChartOptions{LabelHeight: labelH})
	custom := bandInk(PrecipChartOptions{LabelHeight: labelH, LabelFace: big})
	if def == 0 {
		t.Fatal("default face drew no labels")
	}
	if custom <= def {
		t.Errorf("larger face drew %d px of label ink, want more than the default's %d", custom, def)
	}
}

// Golden renders across the shapes the three new screens ask for. The
// hero cell is today-hero's 312 px column, the badge is row-agenda's
// 110 px one. Regenerate with `go test ./... -update`.
func TestRenderPrecipChart_Golden(t *testing.T) {
	// A day that rises through the morning, peaks mid-afternoon and
	// tapers — the shape a daily percentage cannot express.
	wet := wetHourly(map[int]float64{
		6: 0.05, 7: 0.1, 8: 0.2, 9: 0.35, 10: 0.5, 11: 0.65,
		12: 0.8, 13: 0.95, 14: 0.9, 15: 0.7, 16: 0.55, 17: 0.4,
		18: 0.3, 19: 0.2, 20: 0.15, 21: 0.6,
	})
	dry := wetHourly(map[int]float64{9: 0.05, 14: 0.1})

	cases := []struct {
		label  string
		w, h   int
		hourly []weather.HourlyPoint
		opts   PrecipChartOptions
	}{
		{"hero", 312, 120, wet, PrecipChartOptions{LabelHeight: 16}},
		{"hero_marker_on", 312, 120, wet, PrecipChartOptions{
			LabelHeight: 16, ShowNowMarker: true, NowHour: 15}},
		{"hero_marker_off", 312, 120, wet, PrecipChartOptions{
			LabelHeight: 16, ShowNowMarker: false, NowHour: 15}},
		{"hero_guide_on", 312, 120, wet, PrecipChartOptions{
			LabelHeight: 16, ShowGuide: true}},
		{"hero_guide_off", 312, 120, wet, PrecipChartOptions{
			LabelHeight: 16, ShowGuide: false}},
		{"hero_dry_with_text", 312, 120, dry, PrecipChartOptions{
			LabelHeight: 16, DryText: "NO RAIN TODAY"}},
		{"hero_dry_no_text", 312, 120, dry, PrecipChartOptions{LabelHeight: 16}},
		{"badge", 110, 72, wet, PrecipChartOptions{LabelHeight: 16}},
		{"badge_no_labels", 110, 72, wet, PrecipChartOptions{}},
		{"badge_marker_on", 110, 72, wet, PrecipChartOptions{
			LabelHeight: 16, ShowNowMarker: true, NowHour: 15}},
		{"badge_dry_no_text", 110, 72, dry, PrecipChartOptions{LabelHeight: 16}},
	}

	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			frame := newTestFrame(tc.w, tc.h)
			RenderPrecipChart(frame, image.Rect(0, 0, tc.w, tc.h), tc.hourly, tc.opts)
			testutil.AssertGoldenPNG(t, frame)
		})
	}
}

// Within a wet day, an hour with a trace chance still draws a visible
// stub rather than rounding away to nothing — "1%" and "0%" are
// different claims, and the gap in the bar shape is what says which.
func TestRenderPrecipChart_TraceHourDrawsAStub(t *testing.T) {
	const w, h = 312, 120
	bounds := image.Rect(0, 0, w, h)
	frame := newTestFrame(w, h)
	RenderPrecipChart(frame, bounds, wetHourly(map[int]float64{
		12: 0.9,   // carries the day past the dry threshold
		8:  0.001, // trace
		9:  0,     // none
	}), PrecipChartOptions{})

	trace := barHeightAt(t, frame, bounds, 8)
	none := barHeightAt(t, frame, bounds, 9)
	if trace <= none {
		t.Errorf("trace hour drew %d px, none-hour drew %d px; want the trace taller", trace, none)
	}
}

// Nothing the caller passes may put ink outside the rect it asked for.
// The draw helpers clip to the frame rather than to bounds, and on a real
// panel every widget shares one 800x480 frame, so a chart that overruns
// its cell paints over its neighbour rather than being harmlessly
// cropped.
func TestRenderPrecipChart_NeverDrawsOutsideBounds(t *testing.T) {
	cases := []struct {
		label string
		opts  PrecipChartOptions
		prob  float64
	}{
		{"label band taller than the cell", PrecipChartOptions{LabelHeight: 10}, 0.9},
		{"label band exactly fills the cell", PrecipChartOptions{LabelHeight: 7}, 0.9},
		{"negative label height", PrecipChartOptions{LabelHeight: -20}, 0.9},
		{"probability above 1.0", PrecipChartOptions{}, 3.0},
		{"negative probability", PrecipChartOptions{}, -1.0},
		{"trace chance in a very short plot", PrecipChartOptions{LabelHeight: 6}, 0.001},
		{"healthy cell", PrecipChartOptions{LabelHeight: 16}, 0.9},
	}

	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			// A cell deliberately smaller than the frame, so anything
			// drawn outside it is visible rather than clipped away.
			frame := newTestFrame(200, 200)
			bounds := image.Rect(40, 40, 190, 50)
			RenderPrecipChart(frame, bounds,
				wetHourly(map[int]float64{12: tc.prob}), tc.opts)

			for y := range 200 {
				for x := range 200 {
					if frame.ColorIndexAt(x, y) == widget.PaperWhite {
						continue
					}
					if !image.Pt(x, y).In(bounds) {
						t.Fatalf("ink at (%d,%d) is outside bounds %v", x, y, bounds)
					}
				}
			}
		})
	}
}
