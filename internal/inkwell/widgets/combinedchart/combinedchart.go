// Package combinedchart is the combined-chart widget: one day's combined
// chart (precipitation bars with the temperature line over them) placed
// on a screen on its own, filling its bounds.
//
// A screen that shows several days wants every chart on one shared
// temperature range, so a cold day sits visibly lower than a warm one.
// Placed charts don't know about each other, so the range comes from the
// day data module instead: each chart asks it for the same span of days
// (range_days, from today), and the module computes the range across that
// span from the forecast every widget shares. Charts with the same span
// and the same weather settings therefore plot on the same scale, whichever
// day each one draws. See ADR 0015.
package combinedchart

import (
	"fmt"
	"image"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daygrid"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/weatherview"
)

// widgetName prefixes every config error so a dashboard that fails to
// load says which widget rejected it.
const widgetName = "combined-chart"

// defaultRangeDays is the span the shared range covers when config
// doesn't say: the five days bold-five, today-hero and row-agenda show.
const defaultRangeDays = 5

// maxRangeDays is a week from today, the furthest a chart can be placed.
const maxRangeDays = daygrid.MaxDay + 1

var _ widget.Widget = (*Widget)(nil)

// Config is the combined-chart widget's parsed configuration: the weather
// settings and day every one-day widget shares, and the span its
// temperature range covers.
type Config struct {
	daygrid.Config
	// RangeDays is how many days from today the shared temperature range
	// spans. It always reaches Day.
	RangeDays int
}

// Widget is one day's combined chart placed on a screen on its own.
type Widget struct {
	daygrid.Base
	// Config is the widget's parsed settings: the shared ones Base holds
	// as well, and its own.
	Config Config
}

// New creates a combined-chart Widget drawing cfg.Day's chart from days.
func New(bounds image.Rectangle, days daygrid.Source, now func() time.Time, cfg Config) *Widget {
	return &Widget{Base: daygrid.NewBase(bounds, days, now, cfg.Config), Config: cfg}
}

// Render draws the day's combined chart into the widget's bounds, on the
// temperature range across the first RangeDays days, with the now marker
// on today's chart only. A day the forecast doesn't reach draws nothing.
// The chart keeps to its bounds and draws nothing into a cell too small
// to carry it, so there is no size guard here. A failed fetch is logged
// by the day data module and never returned.
func (w *Widget) Render(frame *image.Paletted) error {
	daygrid.FillWhite(frame, w.Bounds())

	now := w.Now()
	data := w.Days.Days(now, w.Config.RangeDays)
	day := data.Days[w.Config.Day]
	if day.Forecast == nil {
		return nil
	}
	weatherview.RenderCombinedChart(frame, w.Bounds(), day.Forecast.Hourly, data.TempRange, weatherview.CombinedOptions{
		NowHour: now.Hour(),
		// "Now" is a point on today's axis and no other day's.
		ShowNowMarker: day.IsToday,
	})
	return nil
}

// spec declares combined-chart to the shared parser: the weather settings,
// day, and its own range_days.
var spec = daygrid.Spec{
	Widget:      widgetName,
	WeatherOnly: true,
	OneDay:      true,
	Extra:       []string{"range_days"},
}

// parseConfig reads the shared settings through the shared parser,
// inheriting weather settings from the top level through deps, then
// range_days, which must reach the chart's own day.
func parseConfig(raw map[string]any, deps widget.Deps) (Config, error) {
	shared, err := daygrid.ParseConfig(spec, raw, deps.Weather)
	cfg := Config{Config: shared, RangeDays: defaultRangeDays}
	if err != nil {
		return cfg, err
	}
	if v, ok := raw["range_days"]; ok {
		n, ok := v.(int)
		if !ok {
			return cfg, fmt.Errorf("%s: range_days must be an integer, got %T", widgetName, v)
		}
		if n < 1 || n > maxRangeDays {
			return cfg, fmt.Errorf("%s: range_days must be in [1, %d], got %d", widgetName, maxRangeDays, n)
		}
		cfg.RangeDays = n
	}
	if cfg.RangeDays <= cfg.Day {
		return cfg, fmt.Errorf("%s: range_days must be at least %d to reach day %d, got %d",
			widgetName, cfg.Day+1, cfg.Day, cfg.RangeDays)
	}
	return cfg, nil
}

// Factory creates a combined-chart Widget from config and dependencies.
// Its day data reads the forecast only.
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
