package todayhero

import (
	"image"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daybadge"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daydata"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/drawkit"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/eventlist"
)

var _ widget.Widget = (*Widget)(nil)

// Widget renders the today-hero screen.
type Widget struct {
	daydata.Base
}

// New creates a today-hero Widget drawing the days from days. Of cfg it
// reads only how events are listed and the temperature unit; where the
// days come from is the day data module's business.
func New(bounds image.Rectangle, days daydata.Source, now func() time.Time, cfg daydata.Config) *Widget {
	return &Widget{daydata.NewBase(bounds, days, now, cfg)}
}

// Render draws today down the left and the next four days as rows down
// the right.
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
	data := w.Days.Days(now, totalDays)
	unit := w.Config.Weather.TempUnit

	agenda := heroStyle(w.Config.MaxEvents, w.Config.ShowLocation, now.Location())
	rowStyle := dayRowStyle(w.Config.ShowLocation, now.Location())

	// Every chart on the screen plots against the one temperature range
	// the module takes across the days shown.
	today := data.Days[0]
	hero := computeHero(w.Bounds())
	daybadge.Hero.Draw(frame, hero.Badge, today, now, unit)
	renderHeroChart(frame, hero.Chart, today.Forecast, now.Hour(), data.TempRange)
	renderHeroAgenda(frame, hero.Agenda, eventlist.Remaining(today.Events, now), agenda)

	// The divider separates two different kinds of content, so it is
	// heavier than a column rule and runs the full height.
	for i := range dividerW {
		drawkit.DrawVLine(frame, w.Bounds().Min.X+split+i, w.Bounds().Min.Y, w.Bounds().Max.Y, widget.PaperBlack)
	}

	for i, row := range computeDayRows(w.Bounds()) {
		renderDayRow(frame, row, data.Days[i+1], dayRowOptions{
			Now:       now,
			TempUnit:  unit,
			TempRange: data.TempRange,
			Agenda:    rowStyle,
		})
		if i < dayRows-1 {
			drawkit.DrawHLine(frame, row.Min.X, row.Max.X, row.Max.Y-1, widget.PaperBlack)
		}
	}

	return nil
}

// Factory creates a today-hero Widget from config and dependencies. Its
// settings are the ones every calendar widget shares, so a screen can be
// swapped between calendar widgets without rewriting its config.
var Factory = daydata.Factory(widgetName, daydata.Parser(spec), New)
