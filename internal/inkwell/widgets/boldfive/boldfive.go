package boldfive

import (
	"image"
	"log"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daygrid"
)

var _ widget.Widget = (*Widget)(nil)

// Config is how bold-five draws its days. Where the days come from is
// the day data module's business; see Factory.
type Config struct {
	MaxEvents    int
	ShowLocation bool
	TempUnit     string
}

// Widget renders the bold-five screen.
type Widget struct {
	bounds image.Rectangle
	days   daygrid.Source
	now    func() time.Time
	config Config
}

// New creates a bold-five Widget drawing the days from days.
func New(bounds image.Rectangle, days daygrid.Source, now func() time.Time, cfg Config) *Widget {
	return &Widget{bounds: bounds, days: days, now: now, config: cfg}
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
	data := w.days.Days(now, columns)

	for i, col := range computeColumns(w.bounds) {
		day := data.Days[i]

		renderDayHeader(frame, col.Header, day.Start)

		renderWeatherBand(frame, col.Weather, day.Forecast, weatherOptions{
			TempUnit:  w.config.TempUnit,
			TempRange: data.TempRange,
			// Today is always the leftmost column, so the marker goes
			// there and nowhere else — "now" is not a point on any
			// other day's axis.
			ShowNowMarker: day.IsToday,
			NowHour:       now.Hour(),
		})

		renderEvents(frame, col.Events, day.Events, eventOptions{
			MaxEvents:    w.config.MaxEvents,
			ShowLocation: w.config.ShowLocation,
			Location:     now.Location(),
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

// Factory creates a bold-five Widget from config and dependencies. Its
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
	return New(bounds, days, now, Config{
		MaxEvents:    cfg.MaxEvents,
		ShowLocation: cfg.ShowLocation,
		TempUnit:     cfg.Weather.TempUnit,
	}), nil
}
