package daytimeline

import (
	"image"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daygrid"
)

const (
	// outlineW is the border of a finished event's block. One pixel of
	// solid ink reads on both packers, and against a solid block it
	// is plainly an outline; a thicker one would eat the rows a half-hour
	// event needs for its label.
	outlineW = 1
	// minBlockH keeps a block visible when its event is shorter than a
	// few pixels of the grid, or has no end at all.
	minBlockH = 4
)

// placement is today's events sorted against the window: the ones drawn
// on the grid, and how many fall wholly before or after it.
type placement struct {
	Placed         []calendar.Event
	Earlier, Later int
}

// place sorts events against the window from start to end. An event
// that only touches an edge from outside is outside: one ending as the
// window opens is earlier, and one starting as it closes is later. An
// event with no end is a moment, inside when it falls in [start, end).
//
// All-day events have no time to place them at, so the grid leaves them
// out; they belong in a strip of their own above it.
func place(events []calendar.Event, start, end time.Time) placement {
	var p placement
	for _, e := range events {
		switch {
		case e.AllDay:
		case !e.End.After(start) && e.Start.Before(start):
			p.Earlier++
		case !e.Start.Before(end):
			p.Later++
		default:
			p.Placed = append(p.Placed, e)
		}
	}
	return p
}

// drawNote writes a note counting events outside the window, at x in
// band.
func drawNote(frame *image.Paletted, band image.Rectangle, x int, text string) {
	daygrid.DrawText(frame, x, band.Min.Y+(band.Dy()-daygrid.BodyLineH())/2+daygrid.BodyAscent(),
		text, daygrid.BodyBoldFace, widget.PaperBlack)
}

// blockRect is where e's block goes: from the row its start falls on to
// the row before its end, leaving one row of paper so back-to-back
// events stay apart.
func blockRect(col image.Rectangle, tl timeline, e calendar.Event) image.Rectangle {
	top := tl.y(e.Start)
	bottom := max(tl.y(e.End)-1, top+minBlockH)
	return image.Rect(col.Min.X, top, col.Max.X, bottom)
}

// finished reports whether e is over. An event still running is not, and
// neither is one ending this minute: a timed event with no end is parsed
// with End == Start, and calling it finished at the minute it fires
// would outline a reminder as history while it is happening.
func finished(e calendar.Event, now time.Time) bool {
	return e.End.Before(now)
}

// drawBlock draws e's block on the grid tl maps: an outline when it has
// finished, solid when it is still to come, labelled, with a mark on
// each edge the window cuts it at. A solid block is a large black fill,
// but it moves with the schedule, so it never sits in one place long
// enough to burn in.
//
// now is the dashboard's clock, in the display zone the label's time is
// written in.
func drawBlock(frame *image.Paletted, col image.Rectangle, tl timeline, e calendar.Event, now time.Time, showLocation bool) {
	r := blockRect(col, tl, e)
	daygrid.FillRect(frame, r, widget.PaperBlack)

	// Whatever is drawn inside the block is in its contrasting colour:
	// paper on a solid block, ink inside an outline.
	inner, ink := r, widget.PaperWhite
	if finished(e, now) {
		inner, ink = r.Inset(outlineW), widget.PaperBlack
		daygrid.FillWhite(frame, inner)
	}
	drawLabel(frame, inner, e, now.Location(), showLocation, ink)
	if e.Start.Before(tl.start) {
		drawMark(frame, inner, ink, true)
	}
	if e.End.After(tl.end) {
		drawMark(frame, inner, ink, false)
	}
}

const (
	// A continuation mark is an arrowhead pointing off the clipped edge,
	// markH rows tall and so 2*markH-1 wide, markPad in from the block's
	// right and clipped edge.
	markH   = 7
	markPad = 3
)

// markW is the width a continuation mark takes, which a label leaves
// free so the two never collide.
const markW = 2*markH - 1

// drawMark draws a continuation mark at inner's top right pointing up
// (up) or at its bottom right pointing down, clipped to inner so a
// sliver of a block doesn't spill its mark onto the grid.
func drawMark(frame *image.Paletted, inner image.Rectangle, ink uint8, up bool) {
	cx := inner.Max.X - markPad - markH
	for row := range markH {
		y := inner.Min.Y + markPad + row
		if !up {
			y = inner.Max.Y - 1 - markPad - row
		}
		for x := cx - row; x <= cx+row; x++ {
			if image.Pt(x, y).In(inner) {
				frame.SetColorIndex(x, y, ink)
			}
		}
	}
}
