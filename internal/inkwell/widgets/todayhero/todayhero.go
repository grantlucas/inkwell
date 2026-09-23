package todayhero

import (
	"context"
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

// fetchTimeout bounds the calendar and weather fetches *together*, so a
// slow upstream on either side cannot stall the render loop for longer
// than this in total. The render loop is what the budget protects, and
// it does not care which of the two was slow.
//
// The cost is that a calendar fetch which eats the whole budget leaves
// the weather call with an expired context, and that render falls back
// to the weather cache or draws no weather at all. That is a visible
// degradation for one cycle, recovered on the next, and it is the
// trade weekly-calendar already makes for the same reason. Fetching
// the two concurrently under the shared deadline would remove the
// starvation without widening the bound — worth doing for all four
// screens at once in daygrid rather than for this one in isolation
// (issue #111).
const fetchTimeout = 10 * time.Second

// Config holds parsed today-hero configuration. The keys match
// weekly-calendar's, so a screen can be swapped between them in the
// rotation without rewriting its config.
type Config struct {
	Feeds        []calendar.Feed
	Refresh      time.Duration
	MaxEvents    int
	ShowLocation bool

	// Weather carries the location, unit and model, along with which of
	// them this widget set — Factory fills the rest from the shared
	// Provider's defaults.
	Weather daygrid.WeatherConfig
}

// Widget renders the today-hero screen.
type Widget struct {
	bounds  image.Rectangle
	cal     calendar.Source
	weather weather.Source
	now     func() time.Time
	config  Config
}

// New creates a today-hero Widget from pre-built data sources.
func New(bounds image.Rectangle, cal calendar.Source, ws weather.Source, now func() time.Time, cfg Config) *Widget {
	return &Widget{bounds: bounds, cal: cal, weather: ws, now: now, config: cfg}
}

// Bounds returns the rectangle this widget occupies.
func (w *Widget) Bounds() image.Rectangle { return w.bounds }

// Render draws today down the left and the next four days as rows down
// the right.
func (w *Widget) Render(frame *image.Paletted) error {
	daygrid.FillWhite(frame, w.bounds)

	// Too small to draw into without spilling past the widget's bounds
	// and over its neighbour on the shared frame. A blank region is a
	// misconfiguration an operator can see; ink on another widget looks
	// like a fault somewhere else entirely.
	if w.bounds.Dy() < minHeight || w.bounds.Dx() < minWidth {
		log.Printf("todayhero: bounds are %dx%d, need at least %dx%d — drawing nothing",
			w.bounds.Dx(), w.bounds.Dy(), minWidth, minHeight)
		return nil
	}

	// The clock arrives already in the dashboard's display zone, so
	// everything day- and hour-derived reads from it rather than
	// re-resolving a zone here. Events carry whatever zone their feed
	// serialized them with, so they still need converting.
	now := w.now()
	loc := now.Location()
	days := daygrid.Days(now, totalDays)
	winStart, winEnd := daygrid.Window(days)

	ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
	defer cancel()

	// A fetch failure must not blank the panel: render with whatever
	// arrived and log so it reaches the operator's terminal.
	events, err := w.cal.Events(ctx, winStart, winEnd)
	if err != nil {
		log.Printf("todayhero: fetch calendar events: %v", err)
	}

	var forecastDays []weather.DailyForecast
	if w.weather != nil {
		f, err := w.weather.Forecast(ctx, w.config.Weather.Location(), totalDays)
		if err != nil {
			log.Printf("todayhero: fetch weather forecast: %v", err)
		}
		if f != nil {
			forecastDays = f.Days
		}
	}

	eventOpts := eventOptions{
		MaxEvents:    w.config.MaxEvents,
		ShowLocation: w.config.ShowLocation,
		Location:     loc,
	}

	today := days[0]
	hero := computeHero(w.bounds)
	renderIdentity(frame, hero.Identity, now)
	renderHeroWeather(frame, hero.Weather, daygrid.FindForecast(forecastDays, today), w.config.Weather.TempUnit)
	renderHeroChart(frame, hero.Chart, daygrid.FindForecast(forecastDays, today), now.Hour())
	renderHeroAgenda(frame, hero.Agenda,
		remainingToday(daygrid.FilterEventsForDay(events, today), now), eventOpts)

	// The divider separates two different kinds of content, so it is
	// heavier than a column rule and runs the full height.
	for i := range dividerW {
		daygrid.DrawVLine(frame, w.bounds.Min.X+split+i, w.bounds.Min.Y, w.bounds.Max.Y, widget.PaperBlack)
	}

	for i, row := range computeDayRows(w.bounds) {
		day := days[i+1]
		renderDayRow(frame, row, day,
			daygrid.FindForecast(forecastDays, day),
			daygrid.FilterEventsForDay(events, day),
			dayRowOptions{
				IsTomorrow: i == 0,
				TempUnit:   w.config.Weather.TempUnit,
				Events:     eventOpts,
			})
		if i < dayRows-1 {
			daygrid.DrawHLine(frame, row.Min.X, row.Max.X, row.Max.Y-1, widget.PaperBlack)
		}
	}

	return nil
}

// Factory creates a today-hero Widget from config and dependencies.
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
