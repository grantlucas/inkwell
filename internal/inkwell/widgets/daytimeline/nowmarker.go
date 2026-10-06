package daytimeline

import (
	"image"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/widgets/weatherview"
)

// nowMarkerW is the marker's thickness: the contrast line's own two
// rows, which read at a metre on both packers.
const nowMarkerW = 2

// drawNowMarker draws the now marker across the weather lane and the
// events at now, so what is past and what is still to come read apart
// at a glance. It is drawn last, under the combined chart's line-inversion
// rule: ink over paper and paper over whatever is drawn, so it carries on
// through a solid block, a bar or the temperature line rather than
// vanishing into them. It skips the rule between the lane and the events,
// which would otherwise be notched.
//
// Outside the window, the window's end included, there is no now on the
// grid and nothing is drawn.
//
// The marker sits a row clear of the hour rules: laid on one, the
// inversion rule would turn the rule's dots to paper, and at the window's
// edge it would rub out the edge rule. A row is a couple of minutes, so
// it is never more than that off the time it marks.
//
// It passes behind labels, the boxes their text takes, leaving a little
// paper either side: a line through words reads as struck through, as if
// the event were cancelled.
func drawNowMarker(frame *image.Paletted, l layout, tl timeline, now time.Time, labels []image.Rectangle) {
	if now.Before(tl.start) || !now.Before(tl.end) {
		return
	}
	h := 0
	for !now.Before(hourAt(tl.start, h+1)) {
		h++
	}
	top, bottom := rowOf(tl, h)
	y := min(max(tl.y(now), top+2), bottom-1-nowMarkerW)

	// The marker's rows, across the lane and across the events.
	runs := []image.Rectangle{
		image.Rect(l.Lane.Min.X, y, l.Lane.Max.X, y+nowMarkerW),
		image.Rect(l.Lane.Max.X+ruleW, y, l.Events.Max.X, y+nowMarkerW),
	}
	for _, label := range labels {
		runs = cut(runs, image.Rect(label.Min.X-labelClearX, label.Min.Y, label.Max.X+labelClearX, label.Max.Y))
	}
	for _, r := range runs {
		weatherview.DrawContrastLine(frame, []image.Point{r.Min, {r.Max.X - 1, r.Min.Y}}, weatherview.RunsAcross)
	}
}

// labelClearX is the paper the now marker leaves either side of a label
// it passes behind: the label's own inset, so on the left the gap starts
// at the block's edge rather than leaving a stub of marker inside it.
const labelClearX = labelPadX

// cut removes the columns of hole from every run whose rows it covers,
// splitting a run hole falls in the middle of in two.
func cut(runs []image.Rectangle, hole image.Rectangle) []image.Rectangle {
	var out []image.Rectangle
	for _, r := range runs {
		if !r.Overlaps(hole) {
			out = append(out, r)
			continue
		}
		if left := image.Rect(r.Min.X, r.Min.Y, hole.Min.X, r.Max.Y); !left.Empty() {
			out = append(out, left)
		}
		if right := image.Rect(hole.Max.X, r.Min.Y, r.Max.X, r.Max.Y); !right.Empty() {
			out = append(out, right)
		}
	}
	return out
}
