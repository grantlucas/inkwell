package rowagenda

import (
	"image"
	"log"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daygrid"
)

var _ widget.Widget = (*Widget)(nil)

// Widget renders the row-agenda screen.
type Widget struct {
	bounds image.Rectangle
	days   daygrid.Source
	now    func() time.Time
	config daygrid.Config
}

// New creates a row-agenda Widget drawing the days from days. Of cfg it
// reads only whether locations are shown and the temperature unit; where
// the days come from is the day data module's business.
func New(bounds image.Rectangle, days daygrid.Source, now func() time.Time, cfg daygrid.Config) *Widget {
	return &Widget{bounds: bounds, days: days, now: now, config: cfg}
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
	data := w.days.Days(now, rows)

	agenda := agendaStyle(w.config.ShowLocation, now.Location())

	// The row heights depend on every day's line count, not just the
	// row's own, so the counts are taken before anything is drawn.
	width := agendaWidth(w.bounds)
	counts := make([]int, len(data.Days))
	for i, day := range data.Days {
		counts[i] = agenda.Lines(day.Events, width)
	}

	for i, row := range planRows(w.bounds, counts) {
		day := data.Days[i]
		renderGutter(frame, row.Gutter, day)
		// One temperature range across the five rows, so every chart is
		// plotted on the same scale and a cold day sits lower than a
		// warm one.
		renderBadge(frame, row.Badge, day.Forecast,
			w.config.Weather.TempUnit, day.IsToday, true, now.Hour(), data.TempRange)

		// A hairline between the badge and the agenda, so the two read
		// as separate columns rather than as one run of text.
		daygrid.DrawVLine(frame, row.Agenda.Min.X-ruleInset, row.Bounds.Min.Y, row.Bounds.Max.Y, widget.PaperBlack)

		agenda.Draw(frame, agendaList(row), day.Events)

		if !row.IsLast {
			daygrid.DrawHLine(frame, row.Bounds.Min.X, row.Bounds.Max.X, row.Bounds.Max.Y-1, widget.PaperBlack)
		}
	}

	return nil
}

// Factory creates a row-agenda Widget from config and dependencies. Its
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
