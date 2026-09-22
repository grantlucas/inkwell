package weekly

import (
	"context"
	"fmt"
	"image"
	"log"
	"net/http"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar"
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

// Config holds parsed weekly-calendar configuration.
type Config struct {
	Feeds            []calendar.Feed
	Refresh          time.Duration
	WeekStart        time.Weekday
	Days             int
	MaxEvents        int
	ShowLocation     bool
	ShowWeather      bool
	ShowWeatherLabel bool
	HighlightHour    int

	// Weather carries the location, unit and model, along with which of
	// them this widget set. Factory fills the rest from the shared
	// Provider's defaults, so a dashboard sets them once at the top
	// level and any widget may override them.
	Weather daygrid.WeatherConfig
}

// Widget renders a rolling multi-day calendar+weather dashboard.
type Widget struct {
	bounds  image.Rectangle
	cal     calendar.Source
	weather weather.Source
	now     func() time.Time
	config  Config
}

// New creates a weekly Widget with pre-built data sources. A cfg.Days that
// was never set falls back to the full week, so a hand-built Config (rather
// than one through parseConfig, which defaults it) still renders columns
// instead of none.
func New(bounds image.Rectangle, cal calendar.Source, ws weather.Source, now func() time.Time, cfg Config) *Widget {
	if cfg.Days < 1 {
		cfg.Days = defaultDays
	}
	return &Widget{
		bounds:  bounds,
		cal:     cal,
		weather: ws,
		now:     now,
		config:  cfg,
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
	loc := now.Location()
	days := daygrid.Days(now, w.config.Days)
	weekStart, weekEnd := daygrid.Window(days)

	// One render-scope context shared by calendar + weather fetches.
	// A slow upstream on either side won't stall the render loop past
	// the timeout now that both HTTP paths honor ctx.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// A calendar fetch failure (network, parse, etc.) shouldn't blank
	// the dashboard — render with whatever events made it through (or
	// an empty list) but log so the failure shows up in the operator's
	// terminal instead of being silently dropped.
	events, err := w.cal.Events(ctx, weekStart, weekEnd)
	if err != nil {
		log.Printf("weekly: fetch calendar events: %v", err)
	}

	var forecast *weather.Forecast
	weatherH := 0
	if w.config.ShowWeather && w.weather != nil {
		f, err := w.weather.Forecast(ctx, w.config.Weather.Location(), w.config.Days)
		if err != nil {
			log.Printf("weekly: fetch weather forecast: %v", err)
		}
		if f != nil {
			forecast = f
			weatherH = defaultWeatherH
		}
	}

	cols := computeColumns(w.bounds, weatherH, w.config.Days)

	var forecastDays []weather.DailyForecast
	if forecast != nil {
		forecastDays = forecast.Days
	}
	globalMin, globalMax := weatherview.GlobalTempRange(forecastDays)

	for i, col := range cols {
		day := days[i]

		renderDayHeader(frame, col.Header, day.Start, day.IsToday)

		if weatherH > 0 {
			dayForecast := daygrid.FindForecast(forecastDays, day)
			opts := weatherview.Options{
				TempUnit:      w.config.Weather.TempUnit,
				ShowLabel:     w.config.ShowWeatherLabel,
				GlobalTempMin: globalMin,
				GlobalTempMax: globalMax,
				HighlightHour: now.Hour(),
				IsToday:       day.IsToday,
			}
			weatherview.RenderDayWeather(frame, col.Weather, dayForecast, opts)
		}

		dayEvents := daygrid.FilterEventsForDay(events, day)
		renderEvents(frame, col.Events, dayEvents, eventOptions{
			MaxEvents:    w.config.MaxEvents,
			ShowLocation: w.config.ShowLocation,
			Location:     loc,
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
	cfg, err := parseConfig(config)
	if err != nil {
		return nil, err
	}

	now := deps.Now
	if now == nil {
		now = time.Now
	}

	var httpClient calendar.HTTPClient
	if deps.DataSources != nil {
		if c, ok := deps.DataSources["http_client"].(calendar.HTTPClient); ok {
			httpClient = c
		}
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	calSource := calendar.NewHTTPSource(cfg.Feeds, httpClient)
	cachedCal := calendar.NewCachedSource(calSource, cfg.Refresh, now)

	var provider *weather.Provider
	if deps.DataSources != nil {
		provider, _ = deps.DataSources["weather"].(*weather.Provider)
	}
	daygrid.ResolveDefaults(&cfg.Weather, provider)

	var ws weather.Source
	if cfg.ShowWeather {
		// A caller-injected weather_source (tests, custom transports) wins;
		// otherwise draw from the shared Provider, bound to the resolved model
		// so every weather widget deduplicates fetches through one cache.
		switch src, ok := deps.DataSources["weather_source"].(weather.Source); {
		case ok:
			ws = src
		case provider != nil:
			ws = provider.SourceForModel(cfg.Weather.Model)
		}
	}

	return New(bounds, cachedCal, ws, now, cfg), nil
}

// parseConfig validates and extracts config values.
func parseConfig(config map[string]any) (Config, error) {
	cfg := Config{
		Refresh:          15 * time.Minute,
		WeekStart:        time.Monday,
		Days:             defaultDays,
		MaxEvents:        5,
		ShowWeather:      true,
		ShowWeatherLabel: true,
		HighlightHour:    15,
	}

	f, ok := config["feeds"]
	if !ok {
		return cfg, fmt.Errorf("weekly-calendar: feeds is required")
	}
	feeds, err := daygrid.ParseFeeds(widgetName, f)
	if err != nil {
		return cfg, err
	}
	cfg.Feeds = feeds

	if v, ok := config["refresh"]; ok {
		s, ok := v.(string)
		if !ok {
			return cfg, fmt.Errorf("weekly-calendar: refresh must be a string, got %T", v)
		}
		d, err := time.ParseDuration(s)
		if err != nil {
			return cfg, fmt.Errorf("weekly-calendar: invalid refresh %q: %w", s, err)
		}
		if d < time.Minute {
			return cfg, fmt.Errorf("weekly-calendar: refresh must be >= 1m, got %v", d)
		}
		cfg.Refresh = d
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

	if v, ok := config["max_events"]; ok {
		n, ok := v.(int)
		if !ok {
			return cfg, fmt.Errorf("weekly-calendar: max_events must be an integer, got %T", v)
		}
		if n <= 0 {
			return cfg, fmt.Errorf("weekly-calendar: max_events must be positive, got %d", n)
		}
		cfg.MaxEvents = n
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

	if v, ok := config["show_location"]; ok {
		b, ok := v.(bool)
		if !ok {
			return cfg, fmt.Errorf("weekly-calendar: show_location must be a bool, got %T", v)
		}
		cfg.ShowLocation = b
	}

	if v, ok := config["show_weather"]; ok {
		b, ok := v.(bool)
		if !ok {
			return cfg, fmt.Errorf("weekly-calendar: show_weather must be a bool, got %T", v)
		}
		cfg.ShowWeather = b
	}
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

	// Location, unit and model are shared with every other
	// calendar-plus-weather screen, so they are parsed once in daygrid
	// rather than restated here.
	if err := daygrid.ParseWeatherKeys(widgetName, config, &cfg.Weather); err != nil {
		return cfg, err
	}

	return cfg, nil
}
