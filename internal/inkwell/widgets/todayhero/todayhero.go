package todayhero

import (
	"image"
	"log"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daygrid"
)

var _ widget.Widget = (*Widget)(nil)

// Widget renders the today-hero screen.
type Widget struct {
	bounds image.Rectangle
	days   daygrid.Source
	now    func() time.Time
	config daygrid.Config
}

// New creates a today-hero Widget drawing the days from days. Of cfg it
// reads only how events are listed and the temperature unit; where the
// days come from is the day data module's business.
func New(bounds image.Rectangle, days daygrid.Source, now func() time.Time, cfg daygrid.Config) *Widget {
	return &Widget{bounds: bounds, days: days, now: now, config: cfg}
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
	data := w.days.Days(now, totalDays)
	unit := w.config.Weather.TempUnit

	eventOpts := eventOptions{
		MaxEvents:    w.config.MaxEvents,
		ShowLocation: w.config.ShowLocation,
		Location:     now.Location(),
	}

	// Every chart on the screen plots against the one temperature range
	// the module takes across the days shown.
	today := data.Days[0]
	hero := computeHero(w.bounds)
	renderIdentity(frame, hero.Identity, now)
	renderHeroWeather(frame, hero.Weather, today.Forecast, unit)
	renderHeroChart(frame, hero.Chart, today.Forecast, now.Hour(), data.TempRange)
	renderHeroAgenda(frame, hero.Agenda, remainingToday(today.Events, now), eventOpts)

	// The divider separates two different kinds of content, so it is
	// heavier than a column rule and runs the full height.
	for i := range dividerW {
		daygrid.DrawVLine(frame, w.bounds.Min.X+split+i, w.bounds.Min.Y, w.bounds.Max.Y, widget.PaperBlack)
	}

	for i, row := range computeDayRows(w.bounds) {
		renderDayRow(frame, row, data.Days[i+1], dayRowOptions{
			IsTomorrow: i == 0,
			TempUnit:   unit,
			TempRange:  data.TempRange,
			Events:     eventOpts,
		})
		if i < dayRows-1 {
			daygrid.DrawHLine(frame, row.Min.X, row.Max.X, row.Max.Y-1, widget.PaperBlack)
		}
	}

	return nil
}

// Factory creates a today-hero Widget from config and dependencies. Its
// settings are the ones every calendar widget shares, so a screen can be
// swapped between calendar widgets without rewriting its config.
func Factory(bounds image.Rectangle, config map[string]any, deps widget.Deps) (widget.Widget, error) {
	cfg, err := daygrid.ParseConfig(spec, config, deps.Weather)
	if err != nil {
		return nil, err
	}
	days, err := daygrid.New(widgetName, cfg, deps)
	if err != nil {
		return nil, err
	}

	now := deps.Now
	if now == nil {
		now = time.Now
	}
	return New(bounds, days, now, cfg), nil
}
