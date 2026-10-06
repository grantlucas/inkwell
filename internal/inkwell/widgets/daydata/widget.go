package daydata

import (
	"image"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/weather"
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

// Settings is a day widget's parsed configuration: the shared settings,
// which its day data is built from, and any of its own. A widget's own
// Config embeds Config, which gives it Shared.
type Settings interface {
	Shared() Config
}

// Shared is the shared settings themselves.
func (c Config) Shared() Config { return c }

// Factory is the widget.Factory of the day widget named name. It reads the
// widget's settings with parse, which inherits weather settings from the
// top-level ones, builds the widget's day data from the shared settings
// and the dashboard's dependencies with opts, and hands both, with the
// dashboard's clock, to build. A widget's package then holds only its
// settings, its layout and its constructor.
func Factory[C Settings, W widget.Widget](
	name string,
	parse func(raw map[string]any, inherit *weather.Provider) (C, error),
	build func(bounds image.Rectangle, days Source, now func() time.Time, cfg C) W,
	opts ...Option,
) widget.Factory {
	return func(bounds image.Rectangle, raw map[string]any, deps widget.Deps) (widget.Widget, error) {
		cfg, err := parse(raw, deps.Weather)
		if err != nil {
			return nil, err
		}
		days, err := New(name, cfg.Shared(), deps, opts...)
		if err != nil {
			return nil, err
		}
		return build(bounds, days, deps.Now, cfg), nil
	}
}

// Parser is the parse of a day widget whose settings are exactly the
// shared ones spec declares.
func Parser(spec Spec) func(raw map[string]any, inherit *weather.Provider) (Config, error) {
	return func(raw map[string]any, inherit *weather.Provider) (Config, error) {
		return ParseConfig(spec, raw, inherit)
	}
}
