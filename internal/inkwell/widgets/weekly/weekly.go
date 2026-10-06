package weekly

import (
	"fmt"
	"image"
	"slices"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/weather"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daygrid"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/weatherview"
)

var _ widget.Widget = (*Widget)(nil)

const defaultWeatherH = 145

// defaultDays is the number of day columns rendered when `days` is omitted,
// and the most the panel can fit.
const defaultDays = 7

// widgetName prefixes every config error so a dashboard that fails to
// load says which widget rejected it.
const widgetName = "weekly-calendar"

// spec declares weekly-calendar to the shared calendar-widget parser: the
// shared settings, a default of five events a column, and the keys only
// this widget takes, which parseConfig reads itself.
var spec = daygrid.Spec{
	Widget:    widgetName,
	MaxEvents: 5,
	Extra:     []string{"days", "week_start", "show_weather", "show_weather_label", "highlight_hour"},
}

// Config holds parsed weekly-calendar configuration: the settings every
// calendar widget shares, and this widget's own.
type Config struct {
	daygrid.Config

	WeekStart        time.Weekday
	Days             int
	ShowWeather      bool
	ShowWeatherLabel bool
	HighlightHour    int
}

// Widget renders a rolling multi-day calendar+weather dashboard.
type Widget struct {
	bounds image.Rectangle
	days   daygrid.Source
	now    func() time.Time
	config Config
}

// New creates a weekly Widget drawing the days from days. A cfg.Days that
// was never set falls back to the full week, so a hand-built Config (rather
// than one through parseConfig, which defaults it) still renders columns
// instead of none.
func New(bounds image.Rectangle, days daygrid.Source, now func() time.Time, cfg Config) *Widget {
	if cfg.Days < 1 {
		cfg.Days = defaultDays
	}
	return &Widget{
		bounds: bounds,
		days:   days,
		now:    now,
		config: cfg,
	}
}

// Bounds returns the rectangle this widget occupies on the display.
func (w *Widget) Bounds() image.Rectangle { return w.bounds }

// Render draws the calendar+weather dashboard into frame, one column per
// configured day starting with today.
func (w *Widget) Render(frame *image.Paletted) error {
	fillWhite(frame, w.bounds)

	// The clock arrives already in the dashboard's display zone (see the
	// top-level timezone config), so everything day- and hour-derived reads
	// from it rather than re-resolving a zone here. Events carry whatever
	// zone their feed serialized them with, so they still need converting.
	now := w.now()
	data := w.days.Days(now, w.config.Days)

	// The band is there when there is a forecast to draw in it. With
	// none — show_weather off, or no forecast arrived — its height goes
	// back to the events.
	weatherH := 0
	if w.config.ShowWeather && slices.ContainsFunc(data.Days, func(d daygrid.Day) bool { return d.Forecast != nil }) {
		weatherH = defaultWeatherH
	}

	cols := computeColumns(w.bounds, weatherH, w.config.Days)

	for i, col := range cols {
		day := data.Days[i]

		renderDayHeader(frame, col.Header, day.Start, day.IsToday)

		if weatherH > 0 {
			// A day the forecast doesn't reach still gets its cell, as
			// it always has; #127 retires this widget rather than
			// changing what it draws.
			var dayForecast weather.DailyForecast
			if day.Forecast != nil {
				dayForecast = *day.Forecast
			}
			opts := weatherview.Options{
				TempUnit:      w.config.Weather.TempUnit,
				ShowLabel:     w.config.ShowWeatherLabel,
				GlobalTempMin: data.TempRange.Min,
				GlobalTempMax: data.TempRange.Max,
				HighlightHour: now.Hour(),
				IsToday:       day.IsToday,
			}
			weatherview.RenderDayWeather(frame, col.Weather, dayForecast, opts)
		}

		renderEvents(frame, col.Events, day.Events, eventOptions{
			MaxEvents:    w.config.MaxEvents,
			ShowLocation: w.config.ShowLocation,
			Location:     now.Location(),
		})

		if !col.IsLast {
			// Column divider in PaperBlack so it stays a continuous rule
			// on the device. PaperGray40 (Y=0x99) only read as a "soft"
			// divider under the now-removed Bayer dither; without it the
			// stroke disappears on both the BW threshold and Gray4 paths.
			drawVLine(frame, col.Bounds.Max.X-1, w.bounds.Min.Y, w.bounds.Max.Y, widget.PaperBlack)
		}
	}

	return nil
}

// Factory creates a weekly-calendar Widget from config and dependencies.
func Factory(bounds image.Rectangle, config map[string]any, deps widget.Deps) (widget.Widget, error) {
	cfg, err := parseConfig(config, deps.Weather)
	if err != nil {
		return nil, err
	}
	days, err := daygrid.New(widgetName, cfg.Config, deps)
	if err != nil {
		return nil, err
	}

	now := deps.Now
	if now == nil {
		now = time.Now
	}
	return New(bounds, days, now, cfg), nil
}

// parseConfig validates and extracts config values: the shared settings
// through the shared parser, inheriting weather settings from inherit,
// then this widget's own keys.
func parseConfig(config map[string]any, inherit *weather.Provider) (Config, error) {
	shared, err := daygrid.ParseConfig(spec, config, inherit)
	cfg := Config{
		Config:           shared,
		WeekStart:        time.Monday,
		Days:             defaultDays,
		ShowWeather:      true,
		ShowWeatherLabel: true,
		HighlightHour:    15,
	}
	if err != nil {
		return cfg, err
	}

	if v, ok := config["week_start"]; ok {
		s, ok := v.(string)
		if !ok {
			return cfg, fmt.Errorf("weekly-calendar: week_start must be a string, got %T", v)
		}
		switch s {
		case "monday":
			cfg.WeekStart = time.Monday
		case "sunday":
			cfg.WeekStart = time.Sunday
		default:
			return cfg, fmt.Errorf("weekly-calendar: invalid week_start %q (must be monday or sunday)", s)
		}
	}

	if v, ok := config["days"]; ok {
		n, ok := v.(int)
		if !ok {
			return cfg, fmt.Errorf("weekly-calendar: days must be an integer, got %T", v)
		}
		if n < 1 || n > defaultDays {
			return cfg, fmt.Errorf("weekly-calendar: days must be in [1, %d], got %d", defaultDays, n)
		}
		cfg.Days = n
	}

	if v, ok := config["show_weather"]; ok {
		b, ok := v.(bool)
		if !ok {
			return cfg, fmt.Errorf("weekly-calendar: show_weather must be a bool, got %T", v)
		}
		cfg.ShowWeather = b
	}
	// A screen that hides its weather fetches none to throw away.
	cfg.NoWeather = !cfg.ShowWeather

	if v, ok := config["show_weather_label"]; ok {
		b, ok := v.(bool)
		if !ok {
			return cfg, fmt.Errorf("weekly-calendar: show_weather_label must be a bool, got %T", v)
		}
		cfg.ShowWeatherLabel = b
	}
	if v, ok := config["highlight_hour"]; ok {
		n, ok := v.(int)
		if !ok {
			return cfg, fmt.Errorf("weekly-calendar: highlight_hour must be an integer, got %T", v)
		}
		if n < 0 || n > 23 {
			return cfg, fmt.Errorf("weekly-calendar: highlight_hour must be in [0, 23], got %d", n)
		}
		cfg.HighlightHour = n
	}

	return cfg, nil
}
