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
	"golang.org/x/image/math/fixed"

	"github.com/grantlucas/inkwell/internal/inkwell/fonts"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
)

var _ widget.Widget = (*Widget)(nil)

var dateFace font.Face

func init() {
	f, err := fonts.Face(fonts.SemiBold, 12)
	if err != nil {
		panic("date: load font: " + err.Error())
	}
	dateFace = f
}

// Widget renders a formatted date string.
type Widget struct {
	bounds image.Rectangle
	now    func() time.Time
	format string
}

// New creates a date widget. A nil now falls back to time.Now so the
// widget renders something reasonable when callers wire it up without
// an explicit clock.
func New(bounds image.Rectangle, now func() time.Time, format string) *Widget {
	if now == nil {
		now = time.Now
	}
	return &Widget{bounds: bounds, now: now, format: format}
}

// Bounds returns the widget's display rectangle.
func (w *Widget) Bounds() image.Rectangle { return w.bounds }

// Render draws the formatted date centered in the bounds.
func (w *Widget) Render(frame *image.Paletted) error {
	draw.Draw(frame, w.bounds, image.NewUniform(color.White), image.Point{}, draw.Src)

	text := strings.ToUpper(w.now().Format(w.format))
	textW := font.MeasureString(dateFace, text).Ceil()
	metrics := dateFace.Metrics()
	textH := (metrics.Ascent + metrics.Descent).Ceil()

	x := w.bounds.Min.X + (w.bounds.Dx()-textW)/2
	y := w.bounds.Min.Y + (w.bounds.Dy()-textH)/2 + metrics.Ascent.Ceil()

	d := &font.Drawer{
		Dst:  frame,
		Src:  image.NewUniform(color.Black),
		Face: dateFace,
		Dot:  fixed.P(x, y),
	}
	d.DrawString(text)

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

	now := deps.Now
	if now == nil {
		now = time.Now
	}

	return New(bounds, now, format), nil
}
