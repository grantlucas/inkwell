package daygrid

import (
	"image"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/widget"
)

// Base is what every day widget holds: its bounds, its day data, the
// dashboard's clock and its parsed settings. A widget embeds it and adds
// only how it lays its days out.
type Base struct {
	bounds image.Rectangle
	// Days is the widget's day data module.
	Days Source
	// Now is the dashboard's clock, already in the display zone.
	Now func() time.Time
	// Config is the widget's parsed shared settings.
	Config Config
}

// NewBase holds a day widget's bounds, day data, clock and settings.
func NewBase(bounds image.Rectangle, days Source, now func() time.Time, cfg Config) Base {
	return Base{bounds: bounds, Days: days, Now: now, Config: cfg}
}

// Bounds returns the rectangle the widget occupies.
func (b Base) Bounds() image.Rectangle { return b.bounds }

// Factory is the widget.Factory of a day widget whose settings are exactly
// the shared ones spec declares. It parses them, builds the widget's day
// data from the dashboard's dependencies, and hands both, with the
// dashboard's clock, to build. A widget's package then holds only its
// layout and its constructor.
func Factory[W widget.Widget](spec Spec, build func(bounds image.Rectangle, days Source, now func() time.Time, cfg Config) W) widget.Factory {
	return func(bounds image.Rectangle, raw map[string]any, deps widget.Deps) (widget.Widget, error) {
		cfg, err := ParseConfig(spec, raw, deps.Weather)
		if err != nil {
			return nil, err
		}
		days, err := New(spec.Widget, cfg, deps)
		if err != nil {
			return nil, err
		}
		return build(bounds, days, deps.Now, cfg), nil
	}
}
