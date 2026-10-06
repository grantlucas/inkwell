package weatherview

import (
	"image"
	"math"

	"github.com/grantlucas/inkwell/internal/inkwell/weather"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
)

// TempRange is the temperature scale a combined chart plots against, in
// degrees Celsius. A screen computes one across every day it shows and
// hands the same range to each chart, so a cold day sits visibly lower
// than a warm one. The day data module computes it with GlobalTempRange.
type TempRange struct {
	Min, Max float64
}

const (
	// tempLineW is the temperature line's thickness. A single pixel is
	// fine at arm's length and gone at a metre; two reads as a line on
	// both the Gray4 and BW paths without starting to compete with the
	// bars.
	tempLineW = 2

	// tempMinPlotH is the shortest plot that can carry the line: the
	// line is tempLineW thick, so it needs at least one row of travel
	// above that.
	tempMinPlotH = tempLineW + 1
)

// CombinedOptions are the combined chart's per-day knobs.
type CombinedOptions struct {
	// NowHour is the hour the now-marker is drawn at, when ShowNowMarker
	// is set and the hour falls inside the window.
	NowHour int
	// ShowNowMarker draws a 2 px solid PaperBlack stroke at NowHour.
	// Only today's chart sets it: on any other day "now" is not a point
	// on that day's axis.
	ShowNowMarker bool
}

// RenderCombinedChart draws a day's combined chart into bounds:
// precipitation probability as bars across 06:00–21:00, with the
// temperature line over them scaled to rng, the range shared by every
// day on the screen.
//
// The caller supplies only the rect. The chart takes its own band for the
// hour labels at the bottom, so one renderer serves a 312 px hero cell
// and a 106 px row badge without either knowing its font.
//
// A dry day is never blank: the baseline, ticks and line are drawn with
// no bars. Absent data — no hourly points in the window — draws nothing,
// and so does a cell too small to carry a legible chart, since a partial
// render reads as a broken widget rather than as no data.
func RenderCombinedChart(frame *image.Paletted, bounds image.Rectangle, hourly []weather.HourlyPoint, rng TempRange, opts CombinedOptions) {
	if bounds.Dx() < precipMinW || bounds.Dy() < precipMinH {
		return
	}
	filtered := filterHours(hourly, precipStartHour, precipEndHour)
	if len(filtered) == 0 {
		return
	}
	l, ok := newPrecipLayout(bounds)
	if !ok {
		return
	}

	// Baseline: a solid PaperBlack rule the bars sit on. Solid rather
	// than a gray hairline because a PaperGrayNN rule snaps to white
	// under the BW threshold and vanishes into Gray4's light bucket.
	drawHLine(frame, bounds.Min.X, bounds.Max.X, l.baselineY, widget.PaperBlack)

	if opts.ShowNowMarker {
		drawPrecipNowMarker(frame, l, opts.NowHour)
	}
	// Below the dry threshold the bars are stubs a pixel or two tall,
	// and a flat row of those reads as a broken widget from across the
	// room; the line alone says the day is dry.
	if peakProb(filtered) >= dryThreshold {
		drawPrecipBars(frame, l, filtered)
	}
	drawPrecipAxis(frame, l)
	drawTempLine(frame, l, filtered, rng)
}

// span returns the range's extent, widening a collapsed or inverted range
// to one degree so every temperature maps somewhere rather than dividing
// by zero.
func (r TempRange) span() float64 {
	return max(r.Max-r.Min, 1)
}

// y maps a temperature to the top row of the line within a plot that
// starts at top and runs height rows. Temperatures outside the range
// clamp to the edge of the plot rather than leaving it, and a NaN reads as
// the coldest value: converting NaN to an int is undefined, and a row far
// outside the plot would send the line walker off to find it. The line is
// tempLineW thick, so the warmest day's line starts on top and the
// coldest day's ends on the plot's last row.
func (r TempRange) y(temp float64, top, height int) int {
	norm := 0.0
	if !math.IsNaN(temp) {
		norm = min(max((temp-r.Min)/r.span(), 0), 1)
	}
	travel := height - tempLineW
	return top + travel - int(norm*float64(travel)+0.5)
}

// drawTempLine draws the temperature line across the plot above the
// baseline, one vertex per hour at the centre of that hour's bar, under
// the line-inversion rule DrawContrastLine applies.
func drawTempLine(frame *image.Paletted, l precipLayout, points []weather.HourlyPoint, rng TempRange) {
	if l.barMaxH < tempMinPlotH {
		return
	}

	pts := make([]image.Point, len(points))
	for i, hp := range points {
		pts[i] = image.Pt(l.slotX(hp.Hour)+l.barW/2, rng.y(hp.Temperature, l.bounds.Min.Y, l.barMaxH))
	}
	DrawContrastLine(frame, pts)
}

// DrawContrastLine draws a 2 px polyline through pts, each vertex being
// the line's top row at that x.
//
// Each pixel is black over bare paper and white over anything already
// drawn — a bar's fill or cap, or the now marker. That is decided from the
// pixel underneath, not from where the bars are meant to be, so the same
// rule holds on the Gray4 and BW paths: a black line over a bar that
// lands dark gray or solid black would vanish, and a white one over paper
// would not exist at all. The colour is read before any of the line is
// written, so a pixel the line has just blackened does not flip its
// neighbour to white.
//
// It is the combined chart's own rule, exported so other drawing code
// that lays a temperature line over its own fills reads the same way.
func DrawContrastLine(frame *image.Paletted, pts []image.Point) {
	var line []image.Point
	add := func(x, y int) {
		for dy := range tempLineW {
			line = append(line, image.Pt(x, y+dy))
		}
	}
	for i, p := range pts {
		if i == 0 {
			add(p.X, p.Y)
			continue
		}
		walkLine(pts[i-1].X, pts[i-1].Y, p.X, p.Y, add)
	}

	ink := make([]uint8, len(line))
	for i, p := range line {
		ink[i] = widget.PaperBlack
		if frame.ColorIndexAt(p.X, p.Y) != widget.PaperWhite {
			ink[i] = widget.PaperWhite
		}
	}
	for i, p := range line {
		setPixel(frame, p.X, p.Y, ink[i])
	}
}
