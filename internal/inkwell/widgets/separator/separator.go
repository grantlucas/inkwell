package separator

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"

	"github.com/grantlucas/inkwell/internal/inkwell/widget"
)

var _ widget.Widget = (*Widget)(nil)

// Widget draws a horizontal line across its full bounds.
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

// Render draws a horizontal line at the bottom of the bounds.
func (w *Widget) Render(frame *image.Paletted) error {
	draw.Draw(frame, w.bounds, image.NewUniform(color.White), image.Point{}, draw.Src)

	for dy := range w.thickness {
		y := w.bounds.Max.Y - 1 - dy
		if y < w.bounds.Min.Y {
			break
		}
		for x := w.bounds.Min.X; x < w.bounds.Max.X; x++ {
			frame.SetColorIndex(x, y, 1)
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
