package inkwell

import (
	"fmt"
	"image"
	"image/color"
)

// Widget renders content into a sub-region of the display frame.
type Widget interface {
	// Bounds returns the rectangle this widget occupies on the display.
	Bounds() image.Rectangle
	// Render draws the widget's content into the given frame.
	Render(frame *image.Paletted) error
}

// Compositor collects widgets and renders them into a single display frame.
type Compositor struct {
	profile *DisplayProfile
	widgets []Widget
}

// NewCompositor creates a Compositor for the given display profile.
func NewCompositor(profile *DisplayProfile) *Compositor {
	return &Compositor{profile: profile}
}

// AddWidget adds a widget to the compositor's render list.
func (c *Compositor) AddWidget(w Widget) {
	c.widgets = append(c.widgets, w)
}

// Render creates a new frame, calls each widget's Render in order, and returns
// the composited frame. The frame uses a BW palette: index 0 = white, index 1 = black.
func (c *Compositor) Render() (*image.Paletted, error) {
	if c.profile == nil {
		return nil, fmt.Errorf("compositor profile is nil")
	}
	if c.profile.Width <= 0 || c.profile.Height <= 0 {
		return nil, fmt.Errorf("invalid display dimensions: %dx%d", c.profile.Width, c.profile.Height)
	}

	palette := color.Palette{color.White, color.Black}
	frame := image.NewPaletted(
		image.Rect(0, 0, c.profile.Width, c.profile.Height),
		palette,
	)

	for _, w := range c.widgets {
		if err := w.Render(frame); err != nil {
			return nil, err
		}
	}

	return frame, nil
}
