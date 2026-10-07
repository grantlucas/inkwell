// Package boldfive implements the bold-five screen: five days of calendar
// and weather as columns, with every element sized to be read from across
// the room rather than from arm's length.
package boldfive

import (
	"image"

	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daybadge"
)

const (
	// columns is fixed at five, not configurable. Every type size here
	// is derived from a 160 px column; a sixth column would shrink the
	// date numeral back below the size this screen exists to escape,
	// and a fourth would leave the grid to re-derive from scratch.
	columns = 5

	// chartH is the combined chart's band under the day badge:
	// precipitation bars with the temperature line drawn over them
	// rather than in a band of its own, which is what leaves the bars
	// enough height to read at distance.
	chartH = 40
	// chartPadX keeps the chart off the column dividers.
	chartPadX = 8
)

// badgeH is the day badge's height: the weekday and date numeral, then
// the condition icon and the high and low. Its last pixels are what let
// a column placed under a fuzzy_clock header band still fit three
// wrapped events and the "+N MORE" line beneath them.
var badgeH = daybadge.Column.Size().Y

// minHeight is the shortest widget this screen can be drawn into. Every
// renderer places its content at a fixed offset from its band's top —
// the date numeral's base lands at about +88, the low temperature's at
// about +148 — and the draw helpers clip to the *frame*, not to the
// widget's bounds. On a shared 800x480 frame that means a widget given
// less room than its bands need would paint over whichever widget sits
// below it, which is the hazard newPrecipLayout documents. Clamping the
// rects is not enough on its own, so a widget this short draws nothing.
var minHeight = badgeH + chartH

// columnLayout describes the vertical zones of one day column.
type columnLayout struct {
	Bounds image.Rectangle
	Badge  image.Rectangle
	Chart  image.Rectangle
	Events image.Rectangle
	IsLast bool
}

// computeColumns divides bounds into five equal-width day columns and
// assigns each the fixed badge / chart / events bands.
//
// Integer division leaves up to four pixels over; they go to the last
// column, which runs to bounds.Max.X. Spreading them would make the
// columns unequal widths and the date numerals would no longer line up
// across the panel, which is the one thing a five-column grid has to
// get right.
func computeColumns(bounds image.Rectangle) []columnLayout {
	colW := bounds.Dx() / columns
	cols := make([]columnLayout, columns)

	badgeBottom := min(bounds.Min.Y+badgeH, bounds.Max.Y)
	chartBottom := min(badgeBottom+chartH, bounds.Max.Y)

	for i := range columns {
		x0 := bounds.Min.X + i*colW
		x1 := x0 + colW
		if i == columns-1 {
			x1 = bounds.Max.X
		}
		cols[i] = columnLayout{
			Bounds: image.Rect(x0, bounds.Min.Y, x1, bounds.Max.Y),
			Badge:  image.Rect(x0, bounds.Min.Y, x1, badgeBottom),
			Chart:  image.Rect(x0+chartPadX, badgeBottom, x1-chartPadX, chartBottom),
			Events: image.Rect(x0, chartBottom, x1, bounds.Max.Y),
			IsLast: i == columns-1,
		}
	}
	return cols
}
