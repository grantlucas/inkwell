package separator

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"

	"github.com/grantlucas/inkwell/internal/inkwell/widget"
)

var _ widget.Widget = (*Widget)(nil)

// Widget draws a horizontal hairline across its full bounds. A multi-row
// separator anti-aliases the edge rows so the line reads as a soft division
// rather than a hard 2-bit slab.
type Widget struct {
	bounds    image.Rectangle
	thickness int
}

// New creates a separator widget.
func New(bounds image.Rectangle, thickness int) *Widget {
	return &Widget{bounds: bounds, thickness: thickness}
}

// Bounds returns the widget's display rectangle.
func (w *Widget) Bounds() image.Rectangle { return w.bounds }

// Render draws a soft horizontal divider at the bottom of the bounds. A
// 1px line is a single mid-gray row; thicker lines render the interior in
// gray with a darker pixel at the top edge for a subtle hairline.
func (w *Widget) Render(frame *image.Paletted) error {
	draw.Draw(frame, w.bounds, image.NewUniform(color.White), image.Point{}, draw.Src)

	// Compute the highest (topmost) row we'll actually draw, so we can put
	// the darker accent there even when the requested thickness is clipped
	// by the widget's bounds.
	topY := max(w.bounds.Max.Y-w.thickness, w.bounds.Min.Y)

	height := w.bounds.Max.Y - topY
	for y := w.bounds.Max.Y - 1; y >= topY; y-- {
		// 1-px strokes must be black on the device — a flat PaperGrayNN
		// row dithers to a dashed dotted line, not a hairline.
		// Multi-row separators can still afford a gray interior because
		// the dither has vertical room to express a halftone pattern.
		var idx uint8
		switch {
		case height == 1:
			idx = widget.PaperBlack
		case y == topY:
			// Top edge of a multi-row separator: keep crisp.
			idx = widget.PaperBlack
		default:
			idx = widget.PaperGray40
		}
		for x := w.bounds.Min.X; x < w.bounds.Max.X; x++ {
			frame.SetColorIndex(x, y, idx)
		}
	}

	return nil
}

// Factory creates a separator widget from config and dependencies.
func Factory(bounds image.Rectangle, config map[string]any, _ widget.Deps) (widget.Widget, error) {
	thickness := 2
	if v, ok := config["thickness"]; ok {
		switch n := v.(type) {
		case int:
			thickness = n
		case float64:
			thickness = int(n)
		default:
			return nil, fmt.Errorf("separator: thickness must be a number, got %T", v)
		}
		if thickness <= 0 {
			return nil, fmt.Errorf("separator: thickness must be positive, got %d", thickness)
		}
	}

	return New(bounds, thickness), nil
}
