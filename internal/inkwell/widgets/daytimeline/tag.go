package daytimeline

import (
	"image"

	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daygrid"
)

const (
	// tagBorder is the outline round a tag: one pixel of solid ink, like
	// a finished block's, so it reads on both packers without a fill.
	tagBorder = 1
	// tagPadX is the paper either side of a tag's count.
	tagPadX = 3
)

// tag is a small box counting events there was no lane for.
type tag struct {
	Rect image.Rectangle
	Text string
}

// newTag is a tag reading text, its right edge at right and its top at
// top: a line tall, and as wide as its text and padding.
func newTag(text string, right, top int) tag {
	w := daygrid.TextWidth(daygrid.BodyBoldFace, text) + 2*(tagPadX+tagBorder)
	return tag{Rect: image.Rect(right-w, top, right, top+daygrid.BodyLineH()), Text: text}
}

// drawTag draws t: its count in bold ink, in a box outlined in ink on
// paper. It is never solid: a tag sits beside solid blocks, and an
// outline is what sets it apart from them.
func drawTag(frame *image.Paletted, t tag) {
	daygrid.FillRect(frame, t.Rect, widget.PaperBlack)
	daygrid.FillWhite(frame, t.Rect.Inset(tagBorder))
	daygrid.DrawText(frame, t.Rect.Min.X+tagBorder+tagPadX, labelBaseline(t.Rect.Inset(tagBorder)),
		t.Text, daygrid.BodyBoldFace, widget.PaperBlack)
}
