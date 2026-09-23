// Package rowagenda implements the row-agenda screen: days as
// full-width rows, so event titles stop truncating.
//
// It is the axis swap. A day gets 96 px of height instead of 160 px of
// width, and for text that is the trade that matters — width is what
// titles were starving for. Titles get 40-odd characters instead of 13,
// which makes this the only one of the three new screens where nothing
// truncates on a realistic week.
//
// It is a separate widget type rather than a mode of weekly-calendar,
// so the current view stays available as a control in the rotation.
package rowagenda

import "image"

const (
	// rows is fixed at five: five 96 px rows fill the panel exactly,
	// and the date numeral's 3x height is what sets that 96.
	rows = 5
	rowH = 96

	// gutterW is the date block: numeral plus the stacked weekday and
	// month abbreviations.
	gutterW = 122

	// The weather badge sits between the gutter and the agenda rule.
	badgeW    = 222
	agendaX   = gutterW + badgeW // 344
	ruleInset = 4                // the rule sits just left of the agenda
)

// rowLayout is one day's zones.
type rowLayout struct {
	Bounds image.Rectangle
	Gutter image.Rectangle
	Badge  image.Rectangle
	Agenda image.Rectangle
	IsLast bool
}

// computeRows divides bounds into five day rows and gives each its
// gutter, weather badge and agenda.
//
// Any remainder from an odd height goes to the last row, so the rows
// above it keep a common height and their date numerals line up — which
// is the one thing a stack of rows has to get right.
func computeRows(bounds image.Rectangle) []rowLayout {
	out := make([]rowLayout, rows)
	for i := range rows {
		y0 := bounds.Min.Y + i*rowH
		y1 := y0 + rowH
		if i == rows-1 {
			y1 = bounds.Max.Y
		}
		x := bounds.Min.X
		out[i] = rowLayout{
			Bounds: image.Rect(x, y0, bounds.Max.X, y1),
			Gutter: image.Rect(x, y0, x+gutterW, y1),
			Badge:  image.Rect(x+gutterW, y0, x+agendaX, y1),
			Agenda: image.Rect(x+agendaX, y0, bounds.Max.X, y1),
			IsLast: i == rows-1,
		}
	}
	return out
}

// minHeight and minWidth are the smallest bounds this screen can draw
// into. Every element is placed at a fixed offset from its row, and the
// draw helpers clip to the frame rather than to the widget's bounds, so
// a widget given less room would paint over whichever widget shares the
// frame with it.
const (
	minHeight = rows * rowH
	minWidth  = agendaX + 120
)
