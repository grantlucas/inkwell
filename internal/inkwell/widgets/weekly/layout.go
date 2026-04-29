// Package weekly implements a 7-day calendar+weather dashboard widget
// for e-ink displays.
package weekly

import "image"

const (
	dayHeaderH = 44
	charWidth  = 7
	lineHeight = 13
)

// columnLayout describes the layout for one day column.
type columnLayout struct {
	Bounds  image.Rectangle
	Header  image.Rectangle
	Weather image.Rectangle
	Events  image.Rectangle
	IsLast  bool
}

// computeColumns divides bounds into 7 equal-width columns and assigns
// vertical zones for header, weather, and events within each.
func computeColumns(bounds image.Rectangle, weatherH int) []columnLayout {
	w := bounds.Dx()
	colW := w / 7
	cols := make([]columnLayout, 7)

	for i := range 7 {
		x0 := bounds.Min.X + i*colW
		x1 := x0 + colW
		if i == 6 {
			x1 = bounds.Max.X
		}

		headerBottom := bounds.Min.Y + dayHeaderH
		weatherBottom := min(headerBottom+weatherH, bounds.Max.Y)

		cols[i] = columnLayout{
			Bounds:  image.Rect(x0, bounds.Min.Y, x1, bounds.Max.Y),
			Header:  image.Rect(x0, bounds.Min.Y, x1, headerBottom),
			Weather: image.Rect(x0, headerBottom, x1, weatherBottom),
			Events:  image.Rect(x0, weatherBottom, x1, bounds.Max.Y),
			IsLast:  i == 6,
		}
	}
	return cols
}
