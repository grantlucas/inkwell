package rowagenda

import (
	"image"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daybadge"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daydata"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/drawkit"
)

var _ widget.Widget = (*Widget)(nil)

// Widget renders the row-agenda screen.
type Widget struct {
	daydata.Base[daydata.Config]
}

// New creates a row-agenda Widget drawing the days from days. Of cfg it
// reads only whether locations are shown and the temperature unit; where
// the days come from is the day data module's business.
func New(bounds image.Rectangle, days daydata.Source, now func() time.Time, cfg daydata.Config) *Widget {
	return &Widget{daydata.NewBase(bounds, days, now, cfg)}
}

// Render draws five day rows, today first.
func (w *Widget) Render(frame *image.Paletted) error {
	drawkit.FillWhite(frame, w.Bounds())

	// Too small to draw into without spilling past the widget's bounds
	// and over its neighbour on the shared frame.
	if !daydata.Fits(widgetName, w.Bounds(), image.Pt(minWidth, minHeight), "") {
		return nil
	}

	// The clock arrives already in the dashboard's display zone, so
	// everything day- and hour-derived reads from it rather than
	// re-resolving a zone here. Events carry whatever zone their feed
	// serialized them with, so they still need converting.
	now := w.Now()
	data := w.Days.Days(now, rows)

	agenda := agendaStyle(w.Config.ShowLocation, now.Location())

	// The row heights depend on every day's line count, not just the
	// row's own, so the counts are taken before anything is drawn.
	width := agendaWidth(w.Bounds())
	counts := make([]int, len(data.Days))
	for i, day := range data.Days {
		counts[i] = agenda.Lines(day.Events, width)
	}

	for i, row := range planRows(w.Bounds(), counts) {
		day := data.Days[i]
		daybadge.Row.Draw(frame, row.Badge, day, now, w.Config.Weather.TempUnit)
		// One temperature range across the five rows, so every chart is
		// plotted on the same scale and a cold day sits lower than a
		// warm one.
		renderChart(frame, row.Chart, day.Forecast, day.IsToday, now.Hour(), data.TempRange)

		// A hairline between the badge and the agenda, so the two read
		// as separate columns rather than as one run of text.
		drawkit.DrawVLine(frame, row.Agenda.Min.X-ruleInset, row.Bounds.Min.Y, row.Bounds.Max.Y, widget.PaperBlack)

		agenda.Draw(frame, agendaList(row), day.Events)

		if !row.IsLast {
			drawkit.DrawHLine(frame, row.Bounds.Min.X, row.Bounds.Max.X, row.Bounds.Max.Y-1, widget.PaperBlack)
		}
	}

	return nil
}

// Factory creates a row-agenda Widget from config and dependencies. Its
// settings are the ones every calendar widget shares, so a screen can be
// swapped between calendar widgets without rewriting its config.
var Factory = daydata.Factory(widgetName, daydata.Parser(spec), New)
