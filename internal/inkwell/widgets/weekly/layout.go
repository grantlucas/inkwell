// Package weekly implements a rolling multi-day calendar+weather dashboard
// widget for e-ink displays, showing up to seven day columns starting today.
package weekly

import "image"

const (
	dayHeaderH = 56
	charWidth  = 8
	lineHeight = 18
)

// columnLayout describes the layout for one day column.
type columnLayout struct {
	Bounds  image.Rectangle
	Header  image.Rectangle
	Weather image.Rectangle
	Events  image.Rectangle
	IsLast  bool
}

// computeColumns divides bounds into days equal-width columns and assigns
// vertical zones for header, weather, and events within each. Fewer days
// means wider columns, which is the point: the event text budget in
// renderEvents is derived from the column width.
func computeColumns(bounds image.Rectangle, weatherH, days int) []columnLayout {
	w := bounds.Dx()
	colW := w / days
	cols := make([]columnLayout, days)

	for i := range days {
		x0 := bounds.Min.X + i*colW
		x1 := x0 + colW
		if i == days-1 {
			x1 = bounds.Max.X
		}

		headerBottom := bounds.Min.Y + dayHeaderH
		weatherBottom := min(headerBottom+weatherH, bounds.Max.Y)

		cols[i] = columnLayout{
			Bounds:  image.Rect(x0, bounds.Min.Y, x1, bounds.Max.Y),
			Header:  image.Rect(x0, bounds.Min.Y, x1, headerBottom),
			Weather: image.Rect(x0, headerBottom, x1, weatherBottom),
			Events:  image.Rect(x0, weatherBottom, x1, bounds.Max.Y),
			IsLast:  i == days-1,
		}
	}
	return cols
}
