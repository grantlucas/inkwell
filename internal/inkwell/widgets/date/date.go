// Package date implements a widget that renders the current date as
// an uppercase formatted string for e-ink displays.
package date

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"strings"
	"time"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"

	"github.com/grantlucas/inkwell/internal/inkwell/widget"
)

var _ widget.Widget = (*Widget)(nil)

var face = basicfont.Face7x13

const (
	charWidth  = 7
	lineHeight = 13
)

// Widget renders a formatted date string.
type Widget struct {
	bounds image.Rectangle
	now    func() time.Time
	format string
	border bool
}

// New creates a date widget.
func New(bounds image.Rectangle, now func() time.Time, format string, border bool) *Widget {
	return &Widget{bounds: bounds, now: now, format: format, border: border}
}

// Bounds returns the widget's display rectangle.
func (w *Widget) Bounds() image.Rectangle { return w.bounds }

// Render draws the formatted date centered in the bounds.
func (w *Widget) Render(frame *image.Paletted) error {
	draw.Draw(frame, w.bounds, image.NewUniform(color.White), image.Point{}, draw.Src)

	text := strings.ToUpper(w.now().Format(w.format))
	textW := len(text) * charWidth
	x := w.bounds.Min.X + (w.bounds.Dx()-textW)/2
	y := w.bounds.Min.Y + (w.bounds.Dy()-lineHeight)/2 + face.Ascent

	d := &font.Drawer{
		Dst:  frame,
		Src:  image.NewUniform(color.Black),
		Face: face,
		Dot:  fixed.P(x, y),
	}
	d.DrawString(text)

	if w.border {
		by := w.bounds.Max.Y - 1
		for x := w.bounds.Min.X; x < w.bounds.Max.X; x++ {
			frame.SetColorIndex(x, by, 1)
			frame.SetColorIndex(x, by-1, 1)
		}
	}

	return nil
}

// Factory creates a date widget from config and dependencies.
func Factory(bounds image.Rectangle, config map[string]any, deps widget.Deps) (widget.Widget, error) {
	format := "Monday, January 2"
	if v, ok := config["format"]; ok {
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("date: format must be a string, got %T", v)
		}
		if s == "" {
			return nil, fmt.Errorf("date: format must not be empty")
		}
		format = s
	}

	border := true
	if v, ok := config["border"]; ok {
		b, ok := v.(bool)
		if !ok {
			return nil, fmt.Errorf("date: border must be a bool, got %T", v)
		}
		border = b
	}

	now := deps.Now
	if now == nil {
		now = time.Now
	}

	return New(bounds, now, format, border), nil
}
