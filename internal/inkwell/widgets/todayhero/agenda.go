package todayhero

import (
	"image"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daygrid"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/eventlist"
)

const (
	// The hero agenda. The time is scaled because it is what you scan
	// for; the title is body size because it is what you read once you
	// are close enough to care.
	timeScale     = 2
	agendaTopPad  = 2
	maxTitleLines = 2
	// agendaGap is the paper between one event and the next, with a
	// hairline across its middle so a two-line title does not run into
	// the next event's time. It is wide because the next time is drawn
	// at twice body size.
	agendaGap = 20
	doneText  = "DONE FOR TODAY"
	doneScale = 2
)

// heroStyle is how the hero agenda lists today's events: the time at
// twice body size with the title tucked a body ascent under it (clock
// labels have no descenders to clear), a hairline between events, and
// up to maxEvents of them.
//
// The day rows write their events with the same style's text rules
// until they move onto the event list's inline layout.
//
// loc is the zone event clock labels are rendered in: a parsed
// Event.Start is a correct instant but carries whatever zone its feed
// serialized it with, so formatting it directly leaks that zone onto the
// panel. It must never be nil.
func heroStyle(maxEvents int, showLocation bool, loc *time.Location) eventlist.Style {
	return eventlist.Style{
		MaxEvents:    maxEvents,
		TitleLines:   maxTitleLines,
		TimeScale:    timeScale,
		TitleLead:    daygrid.BodyAscent(),
		Gap:          agendaGap,
		Rules:        true,
		ShowLocation: showLocation,
		Location:     loc,
	}
}

// renderHeroAgenda draws today's remaining events under a rule. How they
// are fitted and counted is the event list's.
//
// "Remaining" is the point of this block: an event that finished two
// hours ago is history, and on the one screen that spends real estate
// on today it would be spending it on the past. When nothing is left it
// says so, at a size you can read from the same distance as the date.
func renderHeroAgenda(frame *image.Paletted, bounds image.Rectangle, events []calendar.Event, style eventlist.Style) {
	daygrid.DrawHLine(frame, bounds.Min.X+heroPadX, bounds.Max.X-heroPadX, bounds.Min.Y, widget.PaperBlack)

	list := image.Rect(bounds.Min.X+heroPadX, bounds.Min.Y+agendaTopPad, bounds.Max.X-heroPadX, bounds.Max.Y)
	if list.Dx() < eventlist.MinChars*daygrid.BodyAdvance() {
		// Too narrow for the list, and so for "DONE FOR TODAY", which
		// would run over the divider.
		return
	}
	if len(events) == 0 {
		daygrid.Scaled(daygrid.BodyBoldFace, doneScale, widget.PaperBlack).Draw(
			frame, list.Min.X, list.Min.Y+daygrid.BodyAscent()*doneScale, doneText)
		return
	}
	style.Draw(frame, list, events)
}

// remainingToday drops events that have already finished. An event
// still running counts as remaining — it is the one you most want to
// see.
func remainingToday(events []calendar.Event, now time.Time) []calendar.Event {
	var out []calendar.Event
	for _, e := range events {
		// Not After: a timed VEVENT with neither DTEND nor DURATION is
		// parsed with End == Start, so After would drop a reminder at
		// the very minute it fires — and if it were the last one, the
		// panel would say "DONE FOR TODAY" over an event happening now.
		// An all-day event applies to the whole day and never finishes
		// partway through it.
		if e.AllDay || !e.End.Before(now) {
			out = append(out, e)
		}
	}
	return out
}
