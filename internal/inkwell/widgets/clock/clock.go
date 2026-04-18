package clock

import (
	"image"
	"image/color"
	"image/draw"
	"time"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"

	"github.com/grantlucas/inkwell/internal/inkwell/widget"
)

// Compile-time interface check.
var _ widget.Widget = (*Widget)(nil)

// Widget renders the current time as "HH:MM" into a region of the frame.
type Widget struct {
	bounds image.Rectangle
	now    func() time.Time
}

// New creates a clock Widget that renders into bounds. The now
// function is called each Render to determine the displayed time; pass
// time.Now for live output or a fixed function for deterministic tests.
func New(bounds image.Rectangle, now func() time.Time) *Widget {
	return &Widget{bounds: bounds, now: now}
}

// Bounds returns the rectangle this widget occupies on the display.
func (w *Widget) Bounds() image.Rectangle {
	return w.bounds
}

// Render draws the current time as "HH:MM" into frame using the 7×13
// basicfont. Text is black on white, centred vertically within the bounds.
func (w *Widget) Render(frame *image.Paletted) error {
	text := w.now().Format("15:04")

	face := basicfont.Face7x13
	advance := font.MeasureString(face, text)
	textW := advance.Ceil()
	textH := face.Ascent + face.Descent

	// Centre the text within the widget's bounds.
	bw := w.bounds.Dx()
	bh := w.bounds.Dy()
	x := w.bounds.Min.X + (bw-textW)/2
	y := w.bounds.Min.Y + (bh-textH)/2 + face.Ascent

	// Fill the widget background white.
	draw.Draw(frame, w.bounds, image.NewUniform(color.White), image.Point{}, draw.Src)

	d := &font.Drawer{
		Dst:  frame,
		Src:  image.NewUniform(color.Black),
		Face: face,
		Dot:  fixed.P(x, y),
	}
	d.DrawString(text)

	return nil
}
