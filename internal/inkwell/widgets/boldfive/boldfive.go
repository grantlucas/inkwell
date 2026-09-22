package boldfive

import (
	"context"
	"image"
	"log"
	"net/http"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar"
	"github.com/grantlucas/inkwell/internal/inkwell/weather"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
)

var _ widget.Widget = (*Widget)(nil)

// fetchTimeout bounds the calendar and weather fetches together, so a
// slow upstream on either side cannot stall the render loop.
const fetchTimeout = 10 * time.Second

// Config holds parsed bold-five configuration. The keys match
// weekly-calendar's, so a screen can be swapped between the two in the
// rotation without rewriting its config.
type Config struct {
	Feeds        []calendar.Feed
	Refresh      time.Duration
	MaxEvents    int
	ShowLocation bool
	Latitude     float64
	Longitude    float64
	TempUnit     string
	WeatherModel weather.Model

	// Presence of each weather override. When false, Factory fills the
	// field from the shared Provider's defaults, so a dashboard sets
	// location, model and unit once at the top level.
	latSet   bool
	lonSet   bool
	unitSet  bool
	modelSet bool
}

// Widget renders the bold-five screen.
type Widget struct {
	bounds  image.Rectangle
	cal     calendar.Source
	weather weather.Source
	now     func() time.Time
	config  Config
}

// New creates a bold-five Widget from pre-built data sources.
func New(bounds image.Rectangle, cal calendar.Source, ws weather.Source, now func() time.Time, cfg Config) *Widget {
	return &Widget{bounds: bounds, cal: cal, weather: ws, now: now, config: cfg}
}

// Bounds returns the rectangle this widget occupies.
func (w *Widget) Bounds() image.Rectangle { return w.bounds }

// Render draws five day columns starting with today.
func (w *Widget) Render(frame *image.Paletted) error {
	fillWhite(frame, w.bounds)

	// The clock arrives already in the dashboard's display zone, so
	// everything day- and hour-derived reads from it rather than
	// re-resolving a zone here. Events carry whatever zone their feed
	// serialized them with, so they still need converting.
	now := w.now()
	loc := now.Location()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	weekEnd := today.AddDate(0, 0, columns)

	ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
	defer cancel()

	// A fetch failure must not blank the panel: render with whatever
	// arrived and log so the failure reaches the operator's terminal
	// rather than being silently dropped.
	events, err := w.cal.Events(ctx, today, weekEnd)
	if err != nil {
		log.Printf("boldfive: fetch calendar events: %v", err)
	}

	var forecastDays []weather.DailyForecast
	if w.weather != nil {
		f, err := w.weather.Forecast(ctx, weather.Location{
			Latitude:  w.config.Latitude,
			Longitude: w.config.Longitude,
		}, columns)
		if err != nil {
			log.Printf("boldfive: fetch weather forecast: %v", err)
		}
		if f != nil {
			forecastDays = f.Days
		}
	}

	for i, col := range computeColumns(w.bounds) {
		day := today.AddDate(0, 0, i)

		renderDayHeader(frame, col.Header, day)

		renderWeatherBand(frame, col.Weather, findForecast(forecastDays, day), weatherOptions{
			TempUnit: w.config.TempUnit,
			// Today is always the leftmost column, so the marker goes
			// there and nowhere else — "now" is not a point on any
			// other day's axis.
			ShowNowMarker: i == 0,
			NowHour:       now.Hour(),
		})

		renderEvents(frame, col.Events, filterEventsForDay(events, day, day.AddDate(0, 0, 1)), eventOptions{
			MaxEvents:    w.config.MaxEvents,
			ShowLocation: w.config.ShowLocation,
			Location:     loc,
		})

		if !col.IsLast {
			// Solid PaperBlack: a PaperGrayNN hairline snaps to white
			// under the BW threshold and vanishes into Gray4's light
			// bucket, so it would read as a divider on neither mode.
			drawVLine(frame, col.Bounds.Max.X-1, w.bounds.Min.Y, w.bounds.Max.Y, widget.PaperBlack)
		}
	}
	return nil
}

// findForecast returns the DailyForecast for the given day, or a zero
// value when the forecast does not reach that far.
func findForecast(days []weather.DailyForecast, day time.Time) weather.DailyForecast {
	for _, d := range days {
		if d.Date.Year() == day.Year() && d.Date.YearDay() == day.YearDay() {
			return d
		}
	}
	return weather.DailyForecast{}
}

// Factory creates a bold-five Widget from config and dependencies.
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

	cachedCal := calendar.NewCachedSource(
		calendar.NewHTTPSource(cfg.Feeds, httpClient), cfg.Refresh, now,
	)

	var provider *weather.Provider
	if deps.DataSources != nil {
		provider, _ = deps.DataSources["weather"].(*weather.Provider)
	}
	resolveWeatherDefaults(&cfg, provider)

	// A caller-injected weather_source (tests, custom transports) wins;
	// otherwise draw from the shared Provider bound to the resolved
	// model, so every weather widget deduplicates through one cache.
	var ws weather.Source
	switch src, ok := deps.DataSources["weather_source"].(weather.Source); {
	case ok:
		ws = src
	case provider != nil:
		ws = provider.SourceForModel(cfg.WeatherModel)
	}

	return New(bounds, cachedCal, ws, now, cfg), nil
}

// resolveWeatherDefaults fills any weather field the widget did not set
// from the shared Provider's defaults, so a dashboard configures
// location, model and unit once at the top level.
func resolveWeatherDefaults(cfg *Config, provider *weather.Provider) {
	var def weather.Settings
	if provider != nil {
		def = provider.Defaults()
	}
	if !cfg.latSet {
		cfg.Latitude = def.Location.Latitude
	}
	if !cfg.lonSet {
		cfg.Longitude = def.Location.Longitude
	}
	if !cfg.unitSet {
		cfg.TempUnit = def.TempUnit
	}
	if cfg.TempUnit == "" {
		cfg.TempUnit = "C"
	}
	if !cfg.modelSet {
		cfg.WeatherModel = def.Model
	}
}
