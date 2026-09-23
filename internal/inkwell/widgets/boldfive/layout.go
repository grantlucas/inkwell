// Package boldfive implements the bold-five screen: the same five-column
// calendar + weather shape as weekly-calendar, with every element sized to
// be read from across the room rather than from arm's length.
//
// It is a separate widget type rather than a mode of weekly-calendar so the
// current view stays available as a control while this one is evaluated in
// the rotation.
package boldfive

import "image"

const (
	// columns is fixed at five, not configurable. Every type size here
	// is derived from a 160 px column; a sixth column would shrink the
	// date numeral back below the size this screen exists to escape,
	// and a fourth would leave the grid to re-derive from scratch.
	columns = 5

	// headerH covers the weekday abbreviation and the date numeral.
	// 112 is what the two scaled runs need: the weekday's 2x baseline
	// sits at 12+28 and the numeral's 3x baseline at 52+42, whose
	// descent lands exactly on the band's bottom edge.
	headerH = 112

	// weatherH covers the condition icon, the hi/lo pair and the
	// precipitation chart. It is the height the hourly temperature
	// curve used to occupy — dropping the curve is what buys the bars
	// enough height to read at distance.
	weatherH = 104
)

// minHeight is the shortest widget this screen can be drawn into. Every
// renderer places its content at a fixed offset from its band's top —
// the date numeral's descent lands at +112, the low temperature's at
// about +168 — and the draw helpers clip to the *frame*, not to the
// widget's bounds. On a shared 800x480 frame that means a widget given
// less room than its bands need would paint over whichever widget sits
// below it, which is the hazard newPrecipLayout documents. Clamping the
// rects is not enough on its own, so a widget this short draws nothing.
const minHeight = headerH + weatherH

// columnLayout describes the vertical zones of one day column.
type columnLayout struct {
	Bounds  image.Rectangle
	Header  image.Rectangle
	Weather image.Rectangle
	Events  image.Rectangle
	IsLast  bool
}

// computeColumns divides bounds into five equal-width day columns and
// assigns each the fixed header / weather / events bands.
//
// Integer division leaves up to four pixels over; they go to the last
// column, which runs to bounds.Max.X. Spreading them would make the
// columns unequal widths and the date numerals would no longer line up
// across the panel, which is the one thing a five-column grid has to
// get right.
func computeColumns(bounds image.Rectangle) []columnLayout {
	colW := bounds.Dx() / columns
	cols := make([]columnLayout, columns)

	headerBottom := min(bounds.Min.Y+headerH, bounds.Max.Y)
	weatherBottom := min(headerBottom+weatherH, bounds.Max.Y)

	for i := range columns {
		x0 := bounds.Min.X + i*colW
		x1 := x0 + colW
		if i == columns-1 {
			x1 = bounds.Max.X
		}
		cols[i] = columnLayout{
			Bounds:  image.Rect(x0, bounds.Min.Y, x1, bounds.Max.Y),
			Header:  image.Rect(x0, bounds.Min.Y, x1, headerBottom),
			Weather: image.Rect(x0, headerBottom, x1, weatherBottom),
			Events:  image.Rect(x0, weatherBottom, x1, bounds.Max.Y),
			IsLast:  i == columns-1,
		}
	}
	return cols
}
