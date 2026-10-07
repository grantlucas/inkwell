package daytimeline

import (
	"fmt"
	"image"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/drawkit"
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
// gutter, a dotted rule across the lane and events at each hour, a solid
// rule at the window's end and, when a band of the widget's own sits above
// the grid, at its start, and a rule down the grid between the labels and
// lane on its left and the events on its right.
//
// When the rows are shorter than a line of text, as a whole-day window
// makes them, every other hour is labelled so the labels don't run into
// each other; every hour still gets its rule.
func drawGrid(frame *image.Paletted, l layout, tl timeline, win Window) {
	hours := win.EndHour - win.StartHour
	step := 1
	if tl.y(hourAt(tl.start, 1))-tl.top < drawkit.BodyLineH() {
		step = 2
	}
	for h := 0; h <= hours; h++ {
		y := tl.y(hourAt(tl.start, h))
		switch {
		case h == 0 && !l.TopRule:
			// Closed off by whatever sits above the widget.
		case h == 0 || h == hours:
			drawkit.DrawHLine(frame, l.Grid.Min.X, l.Events.Max.X, y, widget.PaperBlack)
		default:
			for x := l.Gutter.Max.X; x < l.Events.Max.X; x += dotEvery {
				drawkit.DrawHLine(frame, x, x+1, y, widget.PaperBlack)
			}
		}
		if h < hours && h%step == 0 {
			drawkit.DrawText(frame, l.Gutter.Min.X+hourLabelX, y+1+drawkit.BodyAscent(),
				fmt.Sprintf("%02d", win.StartHour+h), drawkit.BodyBoldFace, widget.PaperBlack)
		}
	}
	drawkit.DrawVLine(frame, l.Lane.Max.X, l.Grid.Min.Y, l.Grid.Max.Y, widget.PaperBlack)
}

// hourAt is the wall-clock hour h hours after the window opens at start.
// It is built from the date, like the window's edges, so a day that
// changes the clocks keeps each label on the hour it names.
func hourAt(start time.Time, h int) time.Time {
	y, m, d := start.Date()
	return time.Date(y, m, d, start.Hour()+h, 0, 0, 0, start.Location())
}
