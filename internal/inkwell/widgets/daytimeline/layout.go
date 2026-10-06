package daytimeline

import (
	"image"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/widgets/drawkit"
)

const (
	// gridPadY is the paper between the grid's solid edge rules and
	// whatever sits beyond them in the widget's bounds, so a rule doesn't
	// touch the all-day strip's text or a neighbour's rule.
	gridPadY = 2
	// gutterW is the hour-label column: two digits and a little paper on
	// each side.
	gutterW = 29
	// laneW is the weather lane between the labels and the rule. Wide
	// enough that an hour's bar shows its chance to a few percent and the
	// temperature line has room to swing, narrow enough to leave the
	// events most of the width.
	laneW = 80
	// ruleW is the solid rule between the labels and lane on its left
	// and the events on its right.
	ruleW = 1
	// eventsPadX is the paper between the rule and the blocks, and
	// between the blocks and the right edge.
	eventsPadX = 4
	// minEventsW is the narrowest event column worth drawing: room for a
	// label's time, a few characters of title and a continuation mark.
	minEventsW = 120

	// The widget will not draw into less than this. Below it the hour
	// rows are thinner than a line of text and the blocks narrower than
	// a few characters; the helpers clip to the frame, not the bounds, so
	// it draws nothing rather than spill onto a neighbour.
	minWidth  = gutterW + laneW + ruleW + 2*eventsPadX + minEventsW
	minHeight = 160
)

// notePadY is the paper above and below the text of a band over or
// under the grid.
const notePadY = 2

// noteH is the band an "earlier" or "later" note takes: one body line
// and its padding.
func noteH() int { return drawkit.BodyLineH() + 2*notePadY }

// stripH is the all-day strip listing lines lines: the lines and the
// same padding as a note.
func stripH(lines int) int { return lines*drawkit.BodyLineH() + 2*notePadY }

// sections says which of the optional bands this render needs. The bands
// take height only when they have something to say, so an ordinary day
// gives the grid the whole height.
type sections struct {
	// AllDay is how many lines the all-day strip lists, 0 for no strip.
	AllDay  int
	Earlier bool
	Later   bool
}

// layout is where each part of the widget goes. Across the grid, left to
// right: the hour labels, the weather lane, a rule, then the events.
type layout struct {
	// AllDay is the all-day strip across the top, empty when today has
	// nothing all day.
	AllDay image.Rectangle
	// Earlier and Later are the note bands above and below the grid,
	// empty when nothing falls outside the window on that side.
	Earlier, Later image.Rectangle
	// Grid is the hour grid, every column of it.
	Grid image.Rectangle
	// Gutter is the hour-label column on the grid's left.
	Gutter image.Rectangle
	// Lane is the weather lane between the labels and the rule, sharing
	// the grid's rows.
	Lane image.Rectangle
	// Events is the column the blocks are drawn in.
	Events image.Rectangle
	// TopRule is whether the window's opening edge gets a solid rule: only
	// when the all-day strip or the earlier note sits above the grid and
	// needs closing off. Otherwise the grid starts at the widget's top
	// edge with no rule, because what sits above the widget (a screen's
	// separator, or the panel's edge) already closes it, and a second
	// rule just under a separator reads as a double line.
	TopRule bool
}

// computeLayout splits the widget's bounds. The all-day strip comes off
// the top, and everything below is laid out inside whatever is left.
func computeLayout(bounds image.Rectangle, s sections) layout {
	agenda := bounds

	var l layout
	if s.AllDay > 0 {
		l.AllDay = image.Rect(bounds.Min.X, bounds.Min.Y, bounds.Max.X, bounds.Min.Y+stripH(s.AllDay))
		agenda.Min.Y = l.AllDay.Max.Y
	}
	top, bottom := agenda.Min.Y, agenda.Max.Y-gridPadY
	if s.AllDay > 0 {
		top += gridPadY
		l.TopRule = true
	}
	if s.Earlier {
		l.Earlier = image.Rect(agenda.Min.X, agenda.Min.Y, agenda.Max.X, agenda.Min.Y+noteH())
		top = l.Earlier.Max.Y
		l.TopRule = true
	}
	if s.Later {
		l.Later = image.Rect(agenda.Min.X, agenda.Max.Y-noteH(), agenda.Max.X, agenda.Max.Y)
		bottom = l.Later.Min.Y
	}
	l.Grid = image.Rect(agenda.Min.X, top, agenda.Max.X, bottom)
	l.Gutter = image.Rect(l.Grid.Min.X, top, l.Grid.Min.X+gutterW, bottom)
	l.Lane = image.Rect(l.Gutter.Max.X, top, l.Gutter.Max.X+laneW, bottom)
	l.Events = image.Rect(l.Lane.Max.X+ruleW+eventsPadX, top, l.Grid.Max.X-eventsPadX, bottom)
	return l
}

// stripText is where the all-day strip's list goes: under the event
// column, so it lines up with the blocks and the notes, inside the
// strip's padding.
func stripText(l layout) image.Rectangle {
	return image.Rect(l.Events.Min.X, l.AllDay.Min.Y+notePadY, l.Events.Max.X, l.AllDay.Max.Y-notePadY)
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
