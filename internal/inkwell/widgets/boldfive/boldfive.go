package boldfive

import (
	"image"
	"log"
	"net/http"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar"
	"github.com/grantlucas/inkwell/internal/inkwell/weather"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daygrid"
)

var _ widget.Widget = (*Widget)(nil)

// Config holds parsed bold-five configuration. The keys match
// weekly-calendar's, so a screen can be swapped between the two in the
// rotation without rewriting its config.
type Config struct {
	Feeds        []calendar.Feed
	Refresh      time.Duration
	MaxEvents    int
	ShowLocation bool

	// Weather carries the location, unit and model, along with which of
	// them this widget actually set — Factory fills the rest from the
	// shared Provider's defaults, so a dashboard configures them once
	// at the top level.
	Weather daygrid.WeatherConfig
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
	daygrid.FillWhite(frame, w.bounds)

	// Too short to draw into without spilling past the widget's bounds
	// and over its neighbour on the shared frame. A blank region is a
	// misconfiguration an operator can see; ink on top of another
	// widget looks like a rendering fault somewhere else entirely.
	if w.bounds.Dy() < minHeight {
		log.Printf("boldfive: bounds are %d px tall, need at least %d — drawing nothing",
			w.bounds.Dy(), minHeight)
		return nil
	}

	// The clock arrives already in the dashboard's display zone, so
	// everything day- and hour-derived reads from it rather than
	// re-resolving a zone here. Events carry whatever zone their feed
	// serialized them with, so they still need converting.
	now := w.now()
	loc := now.Location()
	days := daygrid.Days(now, columns)

	ctx, cancel := daygrid.FetchContext()
	defer cancel()

	events, forecastDays := daygrid.Fetch(ctx, widgetName, w.cal, w.weather, days, w.config.Weather.Location())

	for i, col := range computeColumns(w.bounds) {
		day := days[i]

		renderDayHeader(frame, col.Header, day.Start)

		renderWeatherBand(frame, col.Weather, daygrid.FindForecast(forecastDays, day), weatherOptions{
			TempUnit: w.config.Weather.TempUnit,
			// Today is always the leftmost column, so the marker goes
			// there and nowhere else — "now" is not a point on any
			// other day's axis.
			ShowNowMarker: day.IsToday,
			NowHour:       now.Hour(),
		})

		renderEvents(frame, col.Events, daygrid.FilterEventsForDay(events, day), eventOptions{
			MaxEvents:    w.config.MaxEvents,
			ShowLocation: w.config.ShowLocation,
			Location:     loc,
		})

		if !col.IsLast {
			// Solid PaperBlack: a PaperGrayNN hairline snaps to white
			// under the BW threshold and vanishes into Gray4's light
			// bucket, so it would read as a divider on neither mode.
			daygrid.DrawVLine(frame, col.Bounds.Max.X-1, w.bounds.Min.Y, w.bounds.Max.Y, widget.PaperBlack)
		}
	}
	return nil
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
	daygrid.ResolveDefaults(&cfg.Weather, provider)

	// A caller-injected weather_source (tests, custom transports) wins;
	// otherwise draw from the shared Provider bound to the resolved
	// model, so every weather widget deduplicates through one cache.
	var ws weather.Source
	switch src, ok := deps.DataSources["weather_source"].(weather.Source); {
	case ok:
		ws = src
	case provider != nil:
		ws = provider.SourceForModel(cfg.Weather.Model)
	}

	return New(bounds, cachedCal, ws, now, cfg), nil
}
