package todayhero

import (
	"image"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/drawkit"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/eventlist"
)

const (
	agendaTopPad = 2
	doneText     = "DONE FOR TODAY"
	doneScale    = 2
)

// heroStyle is how the hero agenda lists today's events: the event list's
// large preset, up to maxEvents of them. With nothing left it says so at
// a size you can read from the same distance as the date.
//
// loc is the zone event clock labels are rendered in. It must never be
// nil.
func heroStyle(maxEvents int, showLocation bool, loc *time.Location) eventlist.List {
	s := eventlist.LargeStyle.List(maxEvents, showLocation, loc)
	s.Empty = eventlist.Note{Text: doneText, Scale: doneScale}
	return s
}

// renderHeroAgenda draws today's remaining events under a rule. How they
// are fitted and counted, and what an empty agenda says, is the event
// list's.
//
// "Remaining" is the point of this block: an event that finished two
// hours ago is history, and on the one screen that spends real estate
// on today it would be spending it on the past.
func renderHeroAgenda(frame *image.Paletted, bounds image.Rectangle, events []calendar.Event, style eventlist.List) {
	drawkit.DrawHLine(frame, bounds.Min.X+heroPadX, bounds.Max.X-heroPadX, bounds.Min.Y, widget.PaperBlack)

	list := image.Rect(bounds.Min.X+heroPadX, bounds.Min.Y+agendaTopPad, bounds.Max.X-heroPadX, bounds.Max.Y)
	style.Draw(frame, list, events)
}
