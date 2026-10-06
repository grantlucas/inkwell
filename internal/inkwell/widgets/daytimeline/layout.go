package daytimeline

import (
	"image"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daygrid"
)

const (
	// gridPadY is the paper above and below the grid when no note sits
	// there, so the edge rules don't touch a neighbour's.
	gridPadY = 2
	// gutterW is the hour-label column: two digits and a little paper on
	// each side.
	gutterW = 29
	// ruleW is the solid rule between the labels and lane on its left
	// and the events on its right.
	ruleW = 1
	// eventsPadX is the paper between the rule and the blocks, and
	// between the blocks and the right edge.
	eventsPadX = 4

	// The widget will not draw into less than this. Below it the hour
	// rows are thinner than a line of text and the blocks narrower than
	// a few characters; the helpers clip to the frame, not the bounds, so
	// it draws nothing rather than spill onto a neighbour.
	minWidth  = 160
	minHeight = 160
)

// noteH is the band an "earlier" or "later" note takes: one body line
// and its padding.
func noteH() int { return daygrid.BodyLineH() + 4 }

// sections says which of the optional bands this render needs. The bands
// take height only when they have something to say, so an ordinary day
// gives the grid the whole height.
type sections struct {
	Earlier bool
	Later   bool
}

// layout is where each part of the widget goes. Across the grid, left to
// right: the hour labels, the weather lane, a rule, then the events.
type layout struct {
	// Earlier and Later are the note bands above and below the grid,
	// empty when nothing falls outside the window on that side.
	Earlier, Later image.Rectangle
	// Grid is the hour grid, every column of it.
	Grid image.Rectangle
	// Gutter is the hour-label column on the grid's left.
	Gutter image.Rectangle
	// Lane is the weather lane between the labels and the rule, sharing
	// the grid's rows. It has no width until the lane is drawn.
	Lane image.Rectangle
	// Events is the column the blocks are drawn in.
	Events image.Rectangle
}

// computeLayout splits the widget's bounds. The agenda is the whole of
// the bounds for now; the all-day strip comes off its top in a later
// change, and everything below is laid out inside whatever is left.
func computeLayout(bounds image.Rectangle, s sections) layout {
	agenda := bounds

	var l layout
	top, bottom := agenda.Min.Y+gridPadY, agenda.Max.Y-gridPadY
	if s.Earlier {
		l.Earlier = image.Rect(agenda.Min.X, agenda.Min.Y, agenda.Max.X, agenda.Min.Y+noteH())
		top = l.Earlier.Max.Y
	}
	if s.Later {
		l.Later = image.Rect(agenda.Min.X, agenda.Max.Y-noteH(), agenda.Max.X, agenda.Max.Y)
		bottom = l.Later.Min.Y
	}
	l.Grid = image.Rect(agenda.Min.X, top, agenda.Max.X, bottom)
	l.Gutter = image.Rect(l.Grid.Min.X, top, l.Grid.Min.X+gutterW, bottom)
	l.Lane = image.Rect(l.Gutter.Max.X, top, l.Gutter.Max.X, bottom)
	l.Events = image.Rect(l.Lane.Max.X+ruleW+eventsPadX, top, l.Grid.Max.X-eventsPadX, bottom)
	return l
}

// timeline maps today's clock onto the grid's rows: the window's start
// to its top row and the window's end to its bottom row.
type timeline struct {
	start, end  time.Time
	top, bottom int
}

// on is the window on the day starting at day, in day's zone. The edges
// are built from the date rather than by adding hours to midnight, so a
// day that changes the clocks still opens the window at the hour the
// wall clock reads; an end of 24 is the next midnight.
func (w Window) on(day time.Time) (start, end time.Time) {
	y, m, d := day.Date()
	return time.Date(y, m, d, w.StartHour, 0, 0, 0, day.Location()),
		time.Date(y, m, d, w.EndHour, 0, 0, 0, day.Location())
}

// newTimeline places win on the day starting at day over grid.
func newTimeline(day time.Time, win Window, grid image.Rectangle) timeline {
	start, end := win.on(day)
	return timeline{start: start, end: end, top: grid.Min.Y, bottom: grid.Max.Y - 1}
}

// y is the row t falls on, clamped to the window.
func (tl timeline) y(t time.Time) int {
	if t.Before(tl.start) {
		t = tl.start
	}
	if t.After(tl.end) {
		t = tl.end
	}
	elapsed, total := int64(t.Sub(tl.start)), int64(tl.end.Sub(tl.start))
	return tl.top + int(elapsed*int64(tl.bottom-tl.top)/total)
}
