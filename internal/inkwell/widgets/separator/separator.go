package separator

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/widget"
)

var (
	_ widget.Widget         = (*Widget)(nil)
	_ widget.RefreshCadence = (*Widget)(nil)
)

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

// RefreshEvery marks the separator static: a fixed divider never changes, so
// it should never trigger a panel refresh on its own.
func (w *Widget) RefreshEvery() time.Duration { return 0 }

// Render draws a horizontal divider at the bottom of the bounds. Every
// row renders in PaperBlack: with the BW packer threshold-snapping (no
// more Bayer dither) a "soft" gray interior just disappears, so the
// separator is now a solid bar across its full thickness.
func (w *Widget) Render(frame *image.Paletted) error {
	draw.Draw(frame, w.bounds, image.NewUniform(color.White), image.Point{}, draw.Src)

	topY := max(w.bounds.Max.Y-w.thickness, w.bounds.Min.Y)
	for y := w.bounds.Max.Y - 1; y >= topY; y-- {
		for x := w.bounds.Min.X; x < w.bounds.Max.X; x++ {
			frame.SetColorIndex(x, y, widget.PaperBlack)
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
