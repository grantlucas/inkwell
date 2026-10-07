// Package daytimeline draws today on an hourly grid, each event at its
// real start and end, so a long meeting looks long and a short one looks
// short.
package daytimeline

import (
	"fmt"
	"image"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daydata"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/drawkit"
)

var _ widget.Widget = (*Widget)(nil)

// Widget renders the day-timeline.
type Widget struct {
	daydata.Base[Config]
}

// New creates a day-timeline Widget drawing today from days over the
// window cfg sets.
func New(bounds image.Rectangle, days daydata.Source, now func() time.Time, cfg Config) *Widget {
	return &Widget{daydata.NewBase(bounds, days, now, cfg)}
}

// Render draws today's hourly grid.
func (w *Widget) Render(frame *image.Paletted) error {
	drawkit.FillWhite(frame, w.Bounds())

	// Too small to draw into without a grid too cramped to read, or
	// spilling past the widget's bounds onto its neighbour.
	if !daydata.Fits(widgetName, w.Bounds(), image.Pt(minWidth, minHeight), "") {
		return nil
	}

	// The clock arrives already in the dashboard's display zone, so the
	// day and its window read from it rather than re-resolving a zone.
	now := w.Now()
	data := w.Days.Days(now, 1)
	today := data.Days[0]

	// What is all day and which events are outside the window decide
	// whether the strip and the note bands take any height, and so where
	// the grid's rows fall. The strip lists over the events, which are
	// as wide whatever the bands take.
	start, end := w.Config.Window.on(today.Start)
	p := place(today, start, end)
	list := allDayList(now.Location(), w.Config.ShowLocation)
	width := computeLayout(w.Bounds(), sections{}).Events.Dx()
	l := computeLayout(w.Bounds(), sections{
		Unavailable: data.CalendarUnavailable,
		AllDay:      min(list.Lines(p.AllDay, width), stripLines),
		Earlier:     p.Earlier > 0,
		Later:       p.Later > 0,
	})
	tl := newTimeline(today.Start, w.Config.Window, l.Grid)

	// A feed with nothing to draw from would leave its events' hours
	// looking free, so the widget says so over whatever did arrive.
	if data.CalendarUnavailable {
		drawUnavailable(frame, l.Unavailable)
	}
	if len(p.AllDay) > 0 {
		list.Draw(frame, stripText(l), p.AllDay)
	}

	// The lane goes down before the grid, so the hour rules cross it and
	// the temperature line, sitting between them, never meets one.
	if today.Forecast != nil {
		drawLane(frame, l.Lane, tl, w.Config.Window, today.Forecast.Hourly, data.TempRange)
	}
	drawGrid(frame, l, tl, w.Config.Window)
	a := arrange(p.Placed, l.Events, tl)
	// The words on the grid, which the now marker passes behind.
	var labels []image.Rectangle
	for _, b := range a.Blocks {
		labels = append(labels, drawBlock(frame, b.Rect, tl, b.Event, now, w.Config.ShowLocation)...)
	}
	for _, t := range a.Tags {
		drawTag(frame, t)
		labels = append(labels, t.Rect)
	}
	if p.Earlier > 0 {
		drawNote(frame, l.Earlier, l.Events.Min.X, fmt.Sprintf("+%d EARLIER", p.Earlier))
	}
	if p.Later > 0 {
		drawNote(frame, l.Later, l.Events.Min.X, fmt.Sprintf("+%d LATER", p.Later))
	}
	drawNowMarker(frame, l, tl, now, labels)
	return nil
}

// Factory creates a day-timeline Widget from config and dependencies.
var Factory = daydata.Factory(widgetName, parseConfig, New)
