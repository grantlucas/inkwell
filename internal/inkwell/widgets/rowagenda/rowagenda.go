package rowagenda

import (
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

// Config holds parsed row-agenda configuration. The keys match
// weekly-calendar's, so a screen can be swapped between them in the
// rotation without rewriting its config.
type Config struct {
	Feeds        []calendar.Feed
	Refresh      time.Duration
	ShowLocation bool

	// Weather carries the location, unit and model, along with which of
	// them this widget set — Factory fills the rest from the shared
	// Provider's defaults.
	Weather daygrid.WeatherConfig
}

// Widget renders the row-agenda screen.
type Widget struct {
	bounds  image.Rectangle
	cal     calendar.Source
	weather weather.Source
	now     func() time.Time
	config  Config
}

// New creates a row-agenda Widget from pre-built data sources.
func New(bounds image.Rectangle, cal calendar.Source, ws weather.Source, now func() time.Time, cfg Config) *Widget {
	return &Widget{bounds: bounds, cal: cal, weather: ws, now: now, config: cfg}
}

// Bounds returns the rectangle this widget occupies.
func (w *Widget) Bounds() image.Rectangle { return w.bounds }

// Render draws five day rows, today first.
func (w *Widget) Render(frame *image.Paletted) error {
	daygrid.FillWhite(frame, w.bounds)

	// Too small to draw into without spilling past the widget's bounds
	// and over its neighbour on the shared frame. A blank region is a
	// misconfiguration an operator can see; ink on another widget looks
	// like a fault somewhere else entirely.
	if w.bounds.Dy() < minHeight || w.bounds.Dx() < minWidth {
		log.Printf("rowagenda: bounds are %dx%d, need at least %dx%d — drawing nothing",
			w.bounds.Dx(), w.bounds.Dy(), minWidth, minHeight)
		return nil
	}

	// The clock arrives already in the dashboard's display zone, so
	// everything day- and hour-derived reads from it rather than
	// re-resolving a zone here. Events carry whatever zone their feed
	// serialized them with, so they still need converting.
	now := w.now()
	loc := now.Location()
	days := daygrid.Days(now, rows)

	ctx, cancel := daygrid.FetchContext()
	defer cancel()

	fetched := daygrid.Fetch(ctx, widgetName, w.cal, w.weather, days, w.config.Weather.Location())
	events, forecastDays := fetched.Events, fetched.Days()

	eventOpts := eventOptions{ShowLocation: w.config.ShowLocation, Location: loc}

	// Events are bucketed before anything is drawn because the row
	// heights depend on every day's count, not just the row's own.
	perDay := make([][]calendar.Event, len(days))
	counts := make([]int, len(days))
	forecasts := make([]weather.DailyForecast, len(days))
	var shown []weather.DailyForecast
	for i, day := range days {
		perDay[i] = daygrid.FilterEventsForDay(events, day)
		counts[i] = len(perDay[i])
		forecasts[i] = daygrid.FindForecast(forecastDays, day)
		if !forecasts[i].Date.IsZero() {
			shown = append(shown, forecasts[i])
		}
	}

	// One temperature range across the five rows, so every chart is
	// plotted on the same scale and a cold day sits lower than a warm
	// one. Taken from the days drawn, not from whatever else the
	// forecast carried.
	var rng weatherview.TempRange
	rng.Min, rng.Max = weatherview.GlobalTempRange(shown)

	for i, row := range planRows(w.bounds, counts) {
		day := days[i]
		renderGutter(frame, row.Gutter, day)
		renderBadge(frame, row.Badge, forecasts[i],
			w.config.Weather.TempUnit, day.IsToday, true, now.Hour(), rng)

		// A hairline between the badge and the agenda, so the two read
		// as separate columns rather than as one run of text.
		daygrid.DrawVLine(frame, row.Agenda.Min.X-ruleInset, row.Bounds.Min.Y, row.Bounds.Max.Y, widget.PaperBlack)

		renderAgenda(frame, row.Agenda, perDay[i], row.Lines, eventOpts)

		if !row.IsLast {
			daygrid.DrawHLine(frame, row.Bounds.Min.X, row.Bounds.Max.X, row.Bounds.Max.Y-1, widget.PaperBlack)
		}
	}

	return nil
}

// Factory creates a row-agenda Widget from config and dependencies.
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
