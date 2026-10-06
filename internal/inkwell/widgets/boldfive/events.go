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
	// eventsGap separates one event from the next. Events are variable
	// height — a one-line title costs two rows, a wrapped one costs
	// three — so a fixed grid would either waste the short ones or
	// clip the long ones.
	eventsGap = 8
	// maxTitleLines caps a single title so one long summary cannot eat
	// the whole column.
	maxTitleLines = 2
)

// agendaStyle is how a column lists its events: the time above the title
// at body size, up to maxEvents of them.
//
// loc is the zone event clock labels are rendered in. A parsed
// Event.Start is a correct instant but carries whatever zone its feed
// serialized it with, so formatting it directly would leak that zone
// onto the panel. It must never be nil.
func agendaStyle(maxEvents int, showLocation bool, loc *time.Location) eventlist.Style {
	return eventlist.Style{
		MaxEvents:    maxEvents,
		TitleLines:   maxTitleLines,
		Gap:          eventsGap,
		ShowLocation: showLocation,
		Location:     loc,
	}
}

// renderEvents draws a day's agenda into bounds under a rule along the
// top of the cell, which separates the agenda from the weather band
// above it. How the events are fitted and counted is the event list's.
func renderEvents(frame *image.Paletted, bounds image.Rectangle, events []calendar.Event, style eventlist.Style) {
	daygrid.DrawHLine(frame, bounds.Min.X, bounds.Max.X, bounds.Min.Y, widget.PaperBlack)

	list := image.Rect(bounds.Min.X+eventsPadX, bounds.Min.Y+eventsTopPad, bounds.Max.X-eventsPadX, bounds.Max.Y)
	if list.Dx() < eventlist.MinChars*daygrid.BodyAdvance() {
		// Too narrow for the list, and so for its empty-day dash: centred
		// in a column this narrow it would overhang the dividers.
		return
	}
	if len(events) == 0 {
		// An empty day says so rather than leaving a blank column that
		// reads as a rendering fault.
		daygrid.DrawTextCentered(frame, bounds.Min.X, bounds.Max.X, list.Min.Y+daygrid.BodyAscent(),
			"--", daygrid.BodyFace, widget.PaperBlack)
		return
	}
	style.Draw(frame, list, events)
}
