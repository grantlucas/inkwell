package daybadge

import (
	"fmt"
	"image"
	"log"
	"strings"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daygrid"
)

// widgetName prefixes every config error so a dashboard that fails to
// load says which widget rejected it.
const widgetName = "day-badge"

var _ widget.Widget = (*Widget)(nil)

// Config is the day-badge widget's parsed configuration: the weather
// settings and day every one-day widget shares, and its style.
type Config struct {
	daygrid.Config
	// Style is the shape the badge is drawn in.
	Style Style
}

// Widget is one day's badge placed on a screen on its own.
type Widget struct {
	daygrid.Base
	// Config is the widget's parsed settings: the shared ones Base holds
	// as well, and its own.
	Config Config
}

// New creates a day-badge Widget drawing cfg.Day's badge from days.
func New(bounds image.Rectangle, days daygrid.Source, now func() time.Time, cfg Config) *Widget {
	return &Widget{Base: daygrid.NewBase(bounds, days, now, cfg.Config), Config: cfg}
}

// Render draws the day's badge at the top-left of the widget's bounds.
// The badge is clipped to the bounds, so an icon's rays never reach a
// neighbouring widget; bounds smaller than the style's size draw
// nothing, since the text would be cut off. A failed fetch is logged by
// the day data module and never returned: the badge still names the day,
// and draws no weather.
func (w *Widget) Render(frame *image.Paletted) error {
	b := w.Bounds()
	daygrid.FillWhite(frame, b)

	size := w.Config.Style.Size()
	if b.Dx() < size.X || b.Dy() < size.Y {
		log.Printf("%s: bounds are %dx%d, the %s style needs at least %dx%d — drawing nothing",
			widgetName, b.Dx(), b.Dy(), w.Config.Style, size.X, size.Y)
		return nil
	}

	now := w.Now()
	day := w.Days.Days(now, w.Config.Day+1).Days[w.Config.Day]
	clip, _ := frame.SubImage(b).(*image.Paletted)
	w.Config.Style.Draw(clip, b, day, now, w.Config.Weather.TempUnit)
	return nil
}

// spec declares day-badge to the shared parser: the weather settings, day
// and its own style.
var spec = daygrid.Spec{
	Widget:      widgetName,
	WeatherOnly: true,
	OneDay:      true,
	Extra:       []string{"style"},
}

// parseConfig reads the shared settings through the shared parser,
// inheriting weather settings from the top level through deps, then the
// style.
func parseConfig(raw map[string]any, deps widget.Deps) (Config, error) {
	shared, err := daygrid.ParseConfig(spec, raw, deps.Weather)
	cfg := Config{Config: shared, Style: Column}
	if err != nil {
		return cfg, err
	}
	v, ok := raw["style"]
	if !ok {
		return cfg, nil
	}
	name, ok := v.(string)
	if !ok {
		return cfg, fmt.Errorf("%s: style must be a string, got %T", widgetName, v)
	}
	if cfg.Style, ok = ParseStyle(name); !ok {
		return cfg, fmt.Errorf("%s: style must be one of %s, got %q", widgetName, strings.Join(StyleNames(), ", "), name)
	}
	return cfg, nil
}

// Factory creates a day-badge Widget from config and dependencies. Its
// day data reads the forecast only.
func Factory(bounds image.Rectangle, config map[string]any, deps widget.Deps) (widget.Widget, error) {
	cfg, err := parseConfig(config, deps)
	if err != nil {
		return nil, err
	}
	days, err := daygrid.New(widgetName, cfg.Config, deps)
	if err != nil {
		return nil, err
	}
	return New(bounds, days, deps.Now, cfg), nil
}
