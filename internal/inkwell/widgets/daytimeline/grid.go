package daytimeline

import (
	"fmt"
	"image"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daygrid"
)

const (
	// hourLabelX is where an hour label starts in the gutter.
	hourLabelX = 4
	// dotEvery spaces the dots of an hour rule: one inked pixel, then
	// paper. A rule every hour drawn solid would turn the grid into
	// stripes; dotted, the rows read without competing with the blocks.
	dotEvery = 3
)

// drawGrid draws the hour grid: a two-digit label for each hour in the
// gutter, a dotted rule across the event column at each hour, solid
// rules at the window's edges, and a rule down the gutter's edge.
//
// When the rows are shorter than a line of text, as a whole-day window
// makes them, every other hour is labelled so the labels don't run into
// each other; every hour still gets its rule.
func drawGrid(frame *image.Paletted, l layout, tl timeline, win Window) {
	hours := win.EndHour - win.StartHour
	step := 1
	if tl.y(hourAt(tl.start, 1))-tl.top < daygrid.BodyLineH() {
		step = 2
	}
	for h := 0; h <= hours; h++ {
		y := tl.y(hourAt(tl.start, h))
		if h == 0 || h == hours {
			daygrid.DrawHLine(frame, l.Grid.Min.X, l.Events.Max.X, y, widget.PaperBlack)
		} else {
			for x := l.Gutter.Max.X; x < l.Events.Max.X; x += dotEvery {
				daygrid.DrawHLine(frame, x, x+1, y, widget.PaperBlack)
			}
		}
		if h < hours && h%step == 0 {
			daygrid.DrawText(frame, l.Gutter.Min.X+hourLabelX, y+1+daygrid.BodyAscent(),
				fmt.Sprintf("%02d", win.StartHour+h), daygrid.BodyBoldFace, widget.PaperBlack)
		}
	}
	daygrid.DrawVLine(frame, l.Gutter.Max.X-1, l.Grid.Min.Y, tl.bottom+1, widget.PaperBlack)
}

// hourAt is the wall-clock hour h hours after the window opens at start.
// It is built from the date, like the window's edges, so a day that
// changes the clocks keeps each label on the hour it names.
func hourAt(start time.Time, h int) time.Time {
	y, m, d := start.Date()
	return time.Date(y, m, d, start.Hour()+h, 0, 0, 0, start.Location())
}
