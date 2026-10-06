package boldfive

import (
	"image"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daygrid"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/eventlist"
)

const (
	// eventsPadX keeps text off the column divider on both sides.
	eventsPadX = 6
	// eventsTopPad puts the first baseline 26 px below the rule.
	eventsTopPad = 12
)

// agendaStyle is how a column lists its events: the event list's stacked
// preset, up to maxEvents of them, with "--" on an empty day.
//
// loc is the zone event clock labels are rendered in. It must never be
// nil.
func agendaStyle(maxEvents int, showLocation bool, loc *time.Location) eventlist.Style {
	return eventlist.PresetStacked.Style(maxEvents, showLocation, loc)
}

// renderEvents draws a day's agenda into bounds under a rule along the
// top of the cell, which separates the agenda from the weather band
// above it. How the events are fitted and counted, and what an empty day
// says, is the event list's.
func renderEvents(frame *image.Paletted, bounds image.Rectangle, events []calendar.Event, style eventlist.Style) {
	daygrid.DrawHLine(frame, bounds.Min.X, bounds.Max.X, bounds.Min.Y, widget.PaperBlack)

	list := image.Rect(bounds.Min.X+eventsPadX, bounds.Min.Y+eventsTopPad, bounds.Max.X-eventsPadX, bounds.Max.Y)
	style.Draw(frame, list, events)
}
