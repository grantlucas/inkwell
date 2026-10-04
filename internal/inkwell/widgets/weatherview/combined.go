package weatherview

import (
	"image"

	"github.com/grantlucas/inkwell/internal/inkwell/weather"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
)

// TempRange is the temperature scale a combined chart plots against, in
// degrees Celsius. A screen computes one across every day it shows and
// hands the same range to each chart, so a cold day sits visibly lower
// than a warm one. GlobalTempRange is the usual source.
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

// span returns the range's extent, widening a collapsed or inverted range
// to one degree so every temperature maps somewhere rather than dividing
// by zero.
func (r TempRange) span() float64 {
	return max(r.Max-r.Min, 1)
}

// y maps a temperature to the top row of the line within a plot that
// starts at top and runs height rows. Temperatures outside the range
// clamp to the edge of the plot rather than leaving it. The line is
// tempLineW thick, so the warmest day's line starts on top and the
// coldest day's ends on the plot's last row.
func (r TempRange) y(temp float64, top, height int) int {
	norm := min(max((temp-r.Min)/r.span(), 0), 1)
	travel := height - tempLineW
	return top + travel - int(norm*float64(travel)+0.5)
}

// drawTempLine draws the temperature line across the plot above the
// baseline, one vertex per hour at the centre of that hour's bar.
//
// Each pixel is black over bare paper and white over anything already
// drawn — a bar's fill or cap, or the now marker. That is decided from the
// pixel underneath, not from where the bars are meant to be, so the same
// rule holds on the Gray4 and BW paths: a black line over a bar that
// lands dark gray or solid black would vanish, and a white one over paper
// would not exist at all. The colour is read before any of the line is
// written, so a pixel the line has just blackened does not flip its
// neighbour to white.
func drawTempLine(frame *image.Paletted, l precipLayout, points []weather.HourlyPoint, rng TempRange) {
	if l.barMaxH < tempMinPlotH {
		return
	}

	pts := make([]image.Point, len(points))
	for i, hp := range points {
		pts[i] = image.Pt(l.slotX(hp.Hour)+l.barW/2, rng.y(hp.Temperature, l.bounds.Min.Y, l.barMaxH))
	}

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
