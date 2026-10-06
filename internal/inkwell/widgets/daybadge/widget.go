package daybadge

import (
	"image"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/weather"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daydata"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/drawkit"
)

// widgetName prefixes every config error so a dashboard that fails to
// load says which widget rejected it.
const widgetName = "day-badge"

var _ widget.Widget = (*Widget)(nil)

// Config is the day-badge widget's parsed configuration: the weather
// settings and day every one-day widget shares, and its style.
type Config struct {
	daydata.Config
	// Style is the shape the badge is drawn in.
	Style Style
}

// Widget is one day's badge placed on a screen on its own.
type Widget struct {
	daydata.Base[Config]
}

// New creates a day-badge Widget drawing cfg.Day's badge from days.
func New(bounds image.Rectangle, days daydata.Source, now func() time.Time, cfg Config) *Widget {
	return &Widget{daydata.NewBase(bounds, days, now, cfg)}
}

// Render draws the day's badge at the top-left of the widget's bounds.
// The badge is clipped to the bounds, so an icon's rays never reach a
// neighbouring widget; bounds smaller than the style's size draw
// nothing, since the text would be cut off. A failed fetch is logged by
// the day data module and never returned: the badge still names the day,
// and draws no weather.
func (w *Widget) Render(frame *image.Paletted) error {
	b := w.Bounds()
	drawkit.FillWhite(frame, b)

	if !daydata.Fits(widgetName, b, w.Config.Style.Size(), "the "+w.Config.Style.String()+" style") {
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
var spec = daydata.Spec{
	Widget: widgetName,
	Reads:  daydata.WeatherOnly,
	OneDay: true,
	Extra:  []string{"style"},
}

// parseConfig reads the shared settings through the shared parser,
// inheriting weather settings from inherit, then the style.
func parseConfig(raw map[string]any, inherit *weather.Provider) (Config, error) {
	shared, err := daydata.ParseConfig(spec, raw, inherit)
	cfg := Config{Config: shared, Style: Column}
	if err != nil {
		return cfg, err
	}
	keys := daydata.ReadKeys(widgetName, raw)
	daydata.Choice(keys, "style", StyleNames, &cfg.Style)
	return cfg, keys.Err()
}

// Factory creates a day-badge Widget from config and dependencies. Its
// day data reads the forecast only.
var Factory = daydata.Factory(widgetName, parseConfig, New)
