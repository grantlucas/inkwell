// Package rowagenda implements the row-agenda screen: days as
// full-width rows, so event titles stop truncating.
//
// It is the axis swap. A day gets its height from its events instead of
// a fixed column width, and for text that is the trade that matters —
// width is what titles were starving for. Every row has one event
// column running the width of the agenda, so a title gets 35-odd
// characters whatever else the day holds.
//
// It is a separate widget type rather than a mode of weekly-calendar,
// so the current view stays available as a control in the rotation.
package rowagenda

import (
	"image"

	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daygrid"
)

const (
	// rows is fixed at five: the screen always shows the same span of
	// days, however crowded the week is.
	rows = 5

	// gutterW is the date block: numeral plus the stacked weekday and
	// month abbreviations.
	gutterW = 122

	// The weather badge sits between the gutter and the agenda rule.
	badgeW    = 222
	agendaX   = gutterW + badgeW // 344
	ruleInset = 4                // the rule sits just left of the agenda
)

// rowLayout is one day's zones.
//
// Gutter and Badge are minRowH tall and centred in the row, so the date
// and the chart sit level with each other on every row and every chart
// shares one height — a taller chart on a busier day would draw the
// same rain at a different size. Agenda runs the row's full height.
type rowLayout struct {
	Bounds image.Rectangle
	Gutter image.Rectangle
	Badge  image.Rectangle
	Agenda image.Rectangle
	// Lines is how many agenda lines the row draws: as many as its list
	// needs, or fewer when the week would not otherwise fit, in which
	// case the event list makes the last of them "+N MORE".
	Lines  int
	IsLast bool
}

// planRows divides bounds into five day rows, one per entry in counts,
// each as tall as the lines its list needs (the event list's count) and
// never shorter than minRowH.
//
// Room the rows do not need is shared between them evenly, so a quiet
// week still fills the panel. Any remainder from the division goes to
// the last row, so the rows above it keep a common share.
func planRows(bounds image.Rectangle, counts []int) []rowLayout {
	lines := fitLines(counts, bounds.Dy())

	heights := make([]int, rows)
	used := 0
	for i, n := range lines {
		heights[i] = rowHeight(n)
		used += heights[i]
	}
	spare := max(bounds.Dy()-used, 0)
	for i := range heights {
		heights[i] += spare / rows
	}
	heights[rows-1] += spare % rows

	out := make([]rowLayout, rows)
	x := bounds.Min.X
	y0 := bounds.Min.Y
	for i := range rows {
		y1 := y0 + heights[i]
		if i == rows-1 {
			y1 = bounds.Max.Y
		}
		blockY := y0 + (y1-y0-minRowH)/2
		out[i] = rowLayout{
			Bounds: image.Rect(x, y0, bounds.Max.X, y1),
			Gutter: image.Rect(x, blockY, x+gutterW, blockY+minRowH),
			Badge:  image.Rect(x+gutterW, blockY, x+agendaX, blockY+minRowH),
			Agenda: image.Rect(x+agendaX, y0, bounds.Max.X, y1),
			Lines:  lines[i],
			IsLast: i == rows-1,
		}
		y0 = y1
	}
	return out
}

// fitLines gives each row the lines its list needs, then, while the rows would
// be taller than height, takes one line at a time from the busiest row.
//
// Taking from the busiest row is what keeps quiet days whole: a day with
// two events never loses one so that a day with nine can show a tenth.
// A tie goes against the later day, so today and tomorrow — the rows
// read first — keep their detail longest.
//
// Trimming stops at the lines a minimum-height row holds anyway, since
// below that a row gets no shorter. Bounds too short for five minimum
// rows are refused by Render before they reach here; the stop keeps any
// other caller from looping forever.
func fitLines(counts []int, height int) []int {
	lines := make([]int, rows)
	total := 0
	for i := range rows {
		// Every row takes at least a line. The list already counts an
		// empty day's "NOTHING SCHEDULED"; this keeps a row that drew
		// nothing at all from planning zero lines.
		lines[i] = max(counts[i], 1)
		total += rowHeight(lines[i])
	}

	floor := (minRowH - 2*agendaPadY) / daygrid.BodyLineH()
	for total > height {
		busiest := 0
		for i, n := range lines {
			if n >= lines[busiest] {
				busiest = i
			}
		}
		if lines[busiest] <= floor {
			break
		}
		total -= rowHeight(lines[busiest])
		lines[busiest]--
		total += rowHeight(lines[busiest])
	}
	return lines
}

// rowHeight is the height a row needs for n agenda lines.
func rowHeight(n int) int {
	return max(minRowH, 2*agendaPadY+n*daygrid.BodyLineH())
}

// minRowH is the shortest a row may be: the height the badge's combined
// chart needs to stay readable, which is also three lines of agenda.
// With five rows at the minimum the panel has 100 px to spare, which is
// five more lines spread across the busy days.
const minRowH = 76

// minHeight and minWidth are the smallest bounds this screen can draw
// into. Every element is placed at a fixed offset from its row, and the
// draw helpers clip to the frame rather than to the widget's bounds, so
// a widget given less room would paint over whichever widget shares the
// frame with it.
const (
	minHeight = rows * minRowH
	minWidth  = agendaX + 120
)
