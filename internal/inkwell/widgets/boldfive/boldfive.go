package boldfive

import (
	"image"
	"log"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daybadge"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daygrid"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/drawkit"
)

var _ widget.Widget = (*Widget)(nil)

// Widget renders the bold-five screen.
type Widget struct {
	daygrid.Base
}

// New creates a bold-five Widget drawing the days from days. Of cfg it
// reads only how events are listed and the temperature unit; where the
// days come from is the day data module's business.
func New(bounds image.Rectangle, days daygrid.Source, now func() time.Time, cfg daygrid.Config) *Widget {
	return &Widget{daygrid.NewBase(bounds, days, now, cfg)}
}

// Render draws five day columns starting with today.
func (w *Widget) Render(frame *image.Paletted) error {
	drawkit.FillWhite(frame, w.Bounds())

	// Too short to draw into without spilling past the widget's bounds
	// and over its neighbour on the shared frame. A blank region is a
	// misconfiguration an operator can see; ink on top of another
	// widget looks like a rendering fault somewhere else entirely.
	if w.Bounds().Dy() < minHeight {
		log.Printf("boldfive: bounds are %d px tall, need at least %d — drawing nothing",
			w.Bounds().Dy(), minHeight)
		return nil
	}

	// The clock arrives already in the dashboard's display zone, so
	// everything day- and hour-derived reads from it rather than
	// re-resolving a zone here. Events carry whatever zone their feed
	// serialized them with, so they still need converting.
	now := w.Now()
	data := w.Days.Days(now, columns)
	agenda := agendaStyle(w.Config.MaxEvents, w.Config.ShowLocation, now.Location())

	for i, col := range computeColumns(w.Bounds()) {
		day := data.Days[i]

		daybadge.Column.Draw(frame, col.Badge, day, now, w.Config.Weather.TempUnit)

		renderChart(frame, col.Chart, day.Forecast, chartOptions{
			TempRange: data.TempRange,
			// Today is always the leftmost column, so the marker goes
			// there and nowhere else — "now" is not a point on any
			// other day's axis.
			ShowNowMarker: day.IsToday,
			NowHour:       now.Hour(),
		})

		renderEvents(frame, col.Events, day.Events, agenda)

		if !col.IsLast {
			// Solid PaperBlack: a PaperGrayNN hairline snaps to white
			// under the BW threshold and vanishes into Gray4's light
			// bucket, so it would read as a divider on neither mode.
			drawkit.DrawVLine(frame, col.Bounds.Max.X-1, w.Bounds().Min.Y, w.Bounds().Max.Y, widget.PaperBlack)
		}
	}
	return nil
}

// Factory creates a bold-five Widget from config and dependencies. Its
// settings are the ones every calendar widget shares, so a screen can be
// swapped between calendar widgets without rewriting its config.
var Factory = daygrid.Factory(spec, New)
