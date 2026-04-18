package inkwell

import (
	"fmt"
	"image"
	"image/color"

	"github.com/grantlucas/inkwell/internal/inkwell/widget"
)

// Compositor renders widgets into a single display frame.
type Compositor struct {
	profile *DisplayProfile
}

// NewCompositor creates a Compositor for the given display profile.
func NewCompositor(profile *DisplayProfile) *Compositor {
	return &Compositor{profile: profile}
}

// Render creates a new frame, calls each widget's Render in order, and returns
// the composited frame. The palette is determined by the display profile's color depth.
// Nil widgets in the slice are silently skipped.
func (c *Compositor) Render(widgets []widget.Widget) (*image.Paletted, error) {
	if c.profile == nil {
		return nil, fmt.Errorf("compositor profile is nil")
	}
	if c.profile.Width <= 0 || c.profile.Height <= 0 {
		return nil, fmt.Errorf("invalid display dimensions: %dx%d", c.profile.Width, c.profile.Height)
	}

	var palette color.Palette
	switch c.profile.Color {
	case BW:
		palette = color.Palette{color.White, color.Black}
	default:
		return nil, fmt.Errorf("unsupported color depth: %v", c.profile.Color)
	}
	frame := image.NewPaletted(
		image.Rect(0, 0, c.profile.Width, c.profile.Height),
		palette,
	)

	for _, w := range widgets {
		if w == nil {
			continue
		}
		if err := w.Render(frame); err != nil {
			return nil, err
		}
	}

	return frame, nil
}
