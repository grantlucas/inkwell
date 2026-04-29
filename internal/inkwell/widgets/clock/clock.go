package clock

import (
	"image"
	"image/color"
	"image/draw"
	"time"

	"fmt"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"

	"github.com/grantlucas/inkwell/internal/inkwell/widget"
)

// Compile-time interface check.
var _ widget.Widget = (*Widget)(nil)

// Align controls text alignment within the widget bounds.
type Align int

const (
	AlignCenter Align = iota
	AlignLeft
	AlignRight
)

// Widget renders the current time into a region of the frame.
type Widget struct {
	bounds image.Rectangle
	now    func() time.Time
	format string
	align  Align
}

// New creates a clock Widget that renders into bounds using the given time
// format string. The now function is called each Render to determine the
// displayed time; pass time.Now for live output or a fixed function for
// deterministic tests.
func New(bounds image.Rectangle, now func() time.Time, format string) *Widget {
	return &Widget{bounds: bounds, now: now, format: format, align: AlignCenter}
}

// Factory creates a clock Widget from config and dependencies.
// Supported config keys:
//   - format (string): Go time format string. Default: "15:04"
//   - align (string): "center" (default), "left", or "right"
func Factory(bounds image.Rectangle, config map[string]any, deps widget.Deps) (widget.Widget, error) {
	format := "15:04"
	if v, ok := config["format"]; ok {
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("clock: format must be a string, got %T", v)
		}
		if s == "" {
			return nil, fmt.Errorf("clock: format must not be empty")
		}
		format = s
	}

	align := AlignCenter
	if v, ok := config["align"]; ok {
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("clock: align must be a string, got %T", v)
		}
		switch s {
		case "center":
			align = AlignCenter
		case "left":
			align = AlignLeft
		case "right":
			align = AlignRight
		default:
			return nil, fmt.Errorf("clock: invalid align %q (must be center, left, or right)", s)
		}
	}

	now := deps.Now
	if now == nil {
		now = time.Now
	}
	return &Widget{bounds: bounds, now: now, format: format, align: align}, nil
}

// Bounds returns the rectangle this widget occupies on the display.
func (w *Widget) Bounds() image.Rectangle {
	return w.bounds
}

// Render draws the current time into frame using the 7×13 basicfont.
// Text is black on white, vertically centered, with horizontal
// alignment controlled by the widget's Align setting.
func (w *Widget) Render(frame *image.Paletted) error {
	text := w.now().Format(w.format)

	face := basicfont.Face7x13
	advance := font.MeasureString(face, text)
	textW := advance.Ceil()
	textH := face.Ascent + face.Descent

	bw := w.bounds.Dx()
	bh := w.bounds.Dy()

	var x int
	switch w.align {
	case AlignLeft:
		x = w.bounds.Min.X + 4
	case AlignRight:
		x = w.bounds.Max.X - textW - 4
	default:
		x = w.bounds.Min.X + (bw-textW)/2
	}
	y := w.bounds.Min.Y + (bh-textH)/2 + face.Ascent

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
