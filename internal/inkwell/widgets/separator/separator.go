package separator

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"

	"github.com/grantlucas/inkwell/internal/inkwell/widget"
)

var _ widget.Widget = (*Widget)(nil)

// Widget draws a solid rule across its bounds: a horizontal one along the
// bottom edge, dividing stacked widgets, or a vertical one down the right
// edge, dividing side-by-side ones.
type Widget struct {
	bounds    image.Rectangle
	thickness int
	vertical  bool
}

// New creates a horizontal separator widget.
func New(bounds image.Rectangle, thickness int) *Widget {
	return &Widget{bounds: bounds, thickness: thickness}
}

// NewVertical creates a vertical separator widget.
func NewVertical(bounds image.Rectangle, thickness int) *Widget {
	return &Widget{bounds: bounds, thickness: thickness, vertical: true}
}

// Bounds returns the widget's display rectangle.
func (w *Widget) Bounds() image.Rectangle { return w.bounds }

// Render draws the rule at the bottom of the bounds, or down their right
// edge for a vertical separator. Every pixel renders in PaperBlack: with
// the BW packer threshold-snapping (no more Bayer dither) a "soft" gray
// interior just disappears, so the separator is a solid bar across its
// full thickness.
func (w *Widget) Render(frame *image.Paletted) error {
	draw.Draw(frame, w.bounds, image.NewUniform(color.White), image.Point{}, draw.Src)

	rule := w.bounds
	if w.vertical {
		rule.Min.X = max(w.bounds.Max.X-w.thickness, w.bounds.Min.X)
	} else {
		rule.Min.Y = max(w.bounds.Max.Y-w.thickness, w.bounds.Min.Y)
	}
	for y := rule.Min.Y; y < rule.Max.Y; y++ {
		for x := rule.Min.X; x < rule.Max.X; x++ {
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

	switch v, ok := config["orientation"]; {
	case !ok || v == "horizontal":
		return New(bounds, thickness), nil
	case v == "vertical":
		return NewVertical(bounds, thickness), nil
	default:
		return nil, fmt.Errorf(`separator: orientation must be "horizontal" or "vertical", got %#v`, v)
	}
}
