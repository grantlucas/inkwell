package todayhero

import (
	"image"
	"log"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar"
	"github.com/grantlucas/inkwell/internal/inkwell/weather"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daygrid"
)

var _ widget.Widget = (*Widget)(nil)

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

	ctx, cancel := daygrid.FetchContext()
	defer cancel()

	fetched := daygrid.Fetch(ctx, widgetName, w.cal, w.weather, days, w.config.Weather.Location())
	events, forecastDays := fetched.Events, fetched.Days()

	eventOpts := eventOptions{
		MaxEvents:    w.config.MaxEvents,
		ShowLocation: w.config.ShowLocation,
		Location:     loc,
	}

	// One forecast per day shown, today first, and one temperature range
	// across all of them for every chart on the screen.
	forecasts := make([]weather.DailyForecast, len(days))
	for i, day := range days {
		forecasts[i] = daygrid.FindForecast(forecastDays, day)
	}
	rng := sharedRange(forecasts)

	today := days[0]
	hero := computeHero(w.bounds)
	renderIdentity(frame, hero.Identity, now)
	renderHeroWeather(frame, hero.Weather, forecasts[0], w.config.Weather.TempUnit)
	renderHeroChart(frame, hero.Chart, forecasts[0], now.Hour(), rng)
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
			forecasts[i+1],
			daygrid.FilterEventsForDay(events, day),
			dayRowOptions{
				IsTomorrow: i == 0,
				TempUnit:   w.config.Weather.TempUnit,
				TempRange:  rng,
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
	if err := daygrid.RequireDeps("today-hero", deps); err != nil {
		return nil, err
	}

	now := deps.Now
	if now == nil {
		now = time.Now
	}

	// Draw from the shared calendar module, so every widget showing a feed
	// shares one cache of it.
	cal := deps.Calendar.Source(cfg.Feeds, cfg.Refresh)

	// Draw from the shared Provider bound to the resolved model, so every
	// weather widget deduplicates through one cache.
	daygrid.ResolveDefaults(&cfg.Weather, deps.Weather)
	ws := deps.Weather.SourceForModel(cfg.Weather.Model)

	return New(bounds, cal, ws, now, cfg), nil
}
