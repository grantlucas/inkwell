package weatherview

import (
	"image"
	"math"
	"strconv"

	"github.com/grantlucas/inkwell/internal/inkwell/weather"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
)

const (
	precipStartHour = 6
	precipEndHour   = 21
	precipHours     = precipEndHour - precipStartHour + 1

	// dryThreshold is the peak probability across the window below which
	// the day is drawn as nothing at all. Bars under ~15% are stubs a
	// pixel or two tall, and a flat row of those reads as a broken
	// widget from across the room where silence reads as a dry day.
	dryThreshold = 0.15

	// precipTickH is the band reserved below the baseline for the base
	// ticks. The live chart can put its ticks on the base row because its
	// axis rule sits at the top of the bar gutter; here the baseline is
	// under the bars, so the ticks have to drop below it to be seen.
	precipTickH = 2

	// precipMinW/H are the smallest bounds worth drawing into. Below
	// these the bars are a pixel or two and the axis has nowhere to go.
	precipMinW = 10
	precipMinH = 10

	// precipTraceH is the stub height a trace chance floors to, so a 1%
	// hour is visibly different from a 0% one.
	precipTraceH = 2

	// precipMarkerW is the now-marker's width. One pixel disappears at a
	// metre; three starts competing with the bars it sits behind.
	precipMarkerW = 2

	// precipLabelGap is the paper between the ticks and the top of the
	// hour labels.
	precipLabelGap = 2
)

// precipLabelH is the band the hour labels take at the bottom of the
// cell: the gap under the ticks plus the label face's full height, so a
// glyph's descent stays inside the cell. The chart sizes it from its own
// face, so no caller has to know which font tier that is.
var precipLabelH = precipLabelGap + defaultFace.Metrics().Ascent.Ceil() + defaultFace.Metrics().Descent.Ceil()

// precipLabelHours are the marks on the hour axis. Three 24-hour labels
// rather than the live chart's "6 9 12 3 8", which mixes morning and
// afternoon on one axis and has to be worked out at exactly the moment
// you are trying not to.
var precipLabelHours = []int{6, 12, 18}

// precipLayout is the resolved geometry of one chart cell: everything the
// drawing steps need to place a bar, a tick or a label.
type precipLayout struct {
	bounds    image.Rectangle
	baselineY int
	barMaxH   int
	step      float64
	barW      int
}

// newPrecipLayout resolves the geometry, reporting false when the cell
// cannot carry a chart at all. A label band that eats the cell would put
// baselineY above bounds.Min.Y, and every bar, tick and label would then
// be drawn outside the rect the caller gave us. The draw helpers clip to
// the frame rather than to bounds, so on a real panel — where every
// widget shares one 800x480 frame — that means painting over whichever
// widget sits above this one.
func newPrecipLayout(bounds image.Rectangle) (precipLayout, bool) {
	baselineY := bounds.Max.Y - precipLabelH - precipTickH - 1
	barMaxH := baselineY - bounds.Min.Y
	if barMaxH <= 0 {
		return precipLayout{}, false
	}

	step := float64(bounds.Dx()) / float64(precipHours)
	return precipLayout{
		bounds:    bounds,
		baselineY: baselineY,
		barMaxH:   barMaxH,
		step:      step,
		// One pixel of gutter between bars rather than the live chart's
		// two: the whole point of this mode is bars that read at
		// distance, and at a 110 px column width every pixel of bar is
		// worth having.
		barW: max(int(step)-1, 2),
	}, true
}

// slotX is the left edge of an hour's column.
func (l precipLayout) slotX(hour int) int {
	return l.bounds.Min.X + int(float64(hour-precipStartHour)*l.step)
}

// barHeight scales a probability to pixels, flooring a trace chance at a
// visible stub — "1%" and "0%" are different claims, and the gap in the
// bar shape is what says which.
//
// The probability is clamped to [0,1] rather than trusted: it reaches us
// as whatever the forecast API returned divided by 100, so a malformed
// response would otherwise scale a bar clean off the top of the cell and
// over the widget above. The stub is capped at the plot height for the
// same reason — a 1 px plot cannot carry a 2 px stub.
func (l precipLayout) barHeight(prob float64) int {
	h := int(math.Round(min(max(prob, 0), 1) * float64(l.barMaxH)))
	if h < 1 && prob > 0 {
		return min(precipTraceH, l.barMaxH)
	}
	return h
}

// drawPrecipNowMarker draws a 2-px solid PaperBlack vertical stroke at
// the given hour. Drawn before the bars so they overlay it — the reader
// still sees one clear vertical mark that intersects the data at the
// right hour. Solid because a soft fill is invisible on the device:
// PaperGray20 collapses into Gray4's light bucket and BW thresholds
// without dither, so any subtle treatment disappears.
func drawPrecipNowMarker(frame *image.Paletted, l precipLayout, hour int) {
	if hour < precipStartHour || hour > precipEndHour {
		return
	}
	cx := l.slotX(hour) + l.barW/2
	fillRect(frame, image.Rect(cx, l.bounds.Min.Y, cx+precipMarkerW, l.baselineY), widget.PaperBlack)
}

// drawPrecipBars draws the PaperGray70 fill and PaperBlack cap for each
// hour. Per CLAUDE.md that pairing lands dark-gray on Gray4 and solid
// black under the BW threshold, and the cap is what carries the value at
// distance.
func drawPrecipBars(frame *image.Paletted, l precipLayout, points []weather.HourlyPoint) {
	for _, hp := range points {
		barH := l.barHeight(hp.PrecipitationProb)
		if barH <= 0 {
			continue
		}
		bx := l.slotX(hp.Hour)
		r := image.Rect(bx, l.baselineY-barH, bx+l.barW, l.baselineY)
		fillRect(frame, r, widget.PaperGray70)
		drawHLine(frame, r.Min.X, r.Max.X, r.Min.Y, widget.PaperBlack)
	}
}

// drawPrecipAxis draws the base ticks and the hour marks under them.
func drawPrecipAxis(frame *image.Paletted, l precipLayout) {
	// One tick per hour slot, whether or not that hour drew a bar, so the
	// axis still reads as an axis on a mostly-dry day. Single pixels
	// survive only as on/off, so they stay PaperBlack.
	for hour := precipStartHour; hour <= precipEndHour; hour++ {
		tx := l.slotX(hour)
		drawVLine(frame, tx, l.baselineY+1, l.baselineY+1+precipTickH, widget.PaperBlack)
	}

	y := l.baselineY + precipTickH + defaultFace.Metrics().Ascent.Ceil() + precipLabelGap
	for _, hour := range precipLabelHours {
		// Hour labels in solid PaperBlack: the glyphs are 1-bit bitmap
		// masks, so black paints pixels that read on both the Gray4 and
		// BW paths.
		x := l.slotX(hour)
		drawTextCenteredWithFace(frame, x, x+int(l.step), y, strconv.Itoa(hour), defaultFace)
	}
}

// peakProb returns the highest precipitation probability in points, or 0
// for an empty slice.
func peakProb(points []weather.HourlyPoint) float64 {
	peak := 0.0
	for _, hp := range points {
		peak = math.Max(peak, hp.PrecipitationProb)
	}
	return peak
}
