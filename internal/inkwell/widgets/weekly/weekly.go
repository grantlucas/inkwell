package weekly

import (
	"image"
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

	// The band's height follows from whether a forecast came back at
	// all, not from whether it carried any days: a 200 response with
	// no daily data is a forecast with none, and collapsing the band
	// for that would reflow the whole screen for one cycle.
	weatherH := 0
	if w.config.ShowWeather && data.ForecastArrived {
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
	// A screen that hides its weather fetches none to throw away.
	var opts []daygrid.Option
	if !cfg.ShowWeather {
		opts = append(opts, daygrid.WithoutWeather())
	}
	days, err := daygrid.New(widgetName, cfg.Config, deps, opts...)
	if err != nil {
		return nil, err
	}

	now := deps.Now
	if now == nil {
		now = time.Now
	}
	return New(bounds, days, now, cfg), nil
}
