// Package todayhero implements the today-hero screen: today at reading
// distance down the left of the panel, the rest of the week compressed
// into one-line rows down the right.
//
// The bet is that from across the room you only ever want today, and
// the rest of the week is a glance you take once you have walked up to
// it. That is why the panel is spent unevenly — today gets 42% of the
// width at a size that reads across a room, and four more days share
// the remainder.
package todayhero

import (
	"image"

	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daybadge"
)

const (
	// split is where the hero column ends and the day rows begin.
	// 338 of 800 is 42% — enough for a 60 px date numeral and a
	// combined chart wide enough to show a band of rain as a band
	// rather than a texture.
	split = 338

	// dividerW is the rule between the two halves. 3 px rather than 1
	// because it separates two different kinds of content, not two
	// instances of the same kind the way a column divider does.
	dividerW = 3

	// The hero column's bands under its day badge. The chart sizes its
	// own label band, so the 10 px of paper between it and the agenda
	// rule is the layout's to set.
	chartBot   = 270
	agendaRule = 280

	// dayRows is how many days follow today, one row each. Four rows
	// of 120 px fill the panel exactly.
	dayRows = 4
	dayRowH = 120
	// totalDays counts today plus the rows, which is the span the
	// calendar and forecast fetches have to cover.
	totalDays = 1 + dayRows
)

// heroLayout is the left column's zones.
type heroLayout struct {
	// Badge is today's day badge: the date, month and fuzzy clock above
	// a rule, then today's weather. It spans the column.
	Badge  image.Rectangle
	Chart  image.Rectangle
	Agenda image.Rectangle
}

// computeHero divides the left column into its three bands.
func computeHero(bounds image.Rectangle) heroLayout {
	x0, x1 := bounds.Min.X, bounds.Min.X+split
	top := bounds.Min.Y
	badge := image.Rectangle{Min: bounds.Min, Max: bounds.Min.Add(daybadge.Hero.Size())}
	return heroLayout{
		Badge:  badge,
		Chart:  image.Rect(x0+12, badge.Max.Y, x0+324, top+chartBot),
		Agenda: image.Rect(x0, top+agendaRule, x1, bounds.Max.Y),
	}
}

// computeDayRows divides the right column into one row per following
// day. Any remainder from an odd height goes to the last row, so the
// rows above it keep a common height and their date numerals line up.
func computeDayRows(bounds image.Rectangle) []image.Rectangle {
	x0 := bounds.Min.X + split + dividerW
	rows := make([]image.Rectangle, dayRows)
	for i := range dayRows {
		y0 := bounds.Min.Y + i*dayRowH
		y1 := y0 + dayRowH
		if i == dayRows-1 {
			y1 = bounds.Max.Y
		}
		rows[i] = image.Rect(x0, y0, bounds.Max.X, y1)
	}
	return rows
}

// minHeight and minWidth are the smallest bounds this screen can draw
// into. Every element is placed at a fixed offset from its band, and
// the draw helpers clip to the frame rather than to the widget's
// bounds, so a widget given less room would paint over whichever widget
// shares the frame with it.
const (
	minHeight = dayRows * dayRowH
	minWidth  = split + dividerW + 200
)
