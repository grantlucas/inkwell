package rowagenda

import (
	"image"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daygrid"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/eventlist"
)

const (
	agendaPadX = 10
	// agendaPadY is the room above the first line and below the last.
	// planRows sizes a row as this twice plus its lines, so changing it
	// changes how many lines a week can carry.
	agendaPadY = 8

	// Upper case, like the "+N MORE" marker below it and the ALL DAY
	// label beside it. Mixed case in this one string read as a second
	// typographic system on the same row.
	emptyMsg = "NOTHING SCHEDULED"
)

// agendaStyle is how a row lists its day: one column of inline lines,
// a time and a title each, as many as the row's line budget holds.
//
// One column whatever the day's load: width is what titles were
// starving for, so a busy day grows downward rather than splitting into
// columns that cut every title short. There is no cap: how many lines a
// row gets is planRows' call, made across the whole week.
//
// loc is the zone event clock labels are rendered in: a parsed
// Event.Start is a correct instant but carries whatever zone its feed
// serialized it with, so formatting it directly leaks that zone onto
// the panel. It must never be nil.
func agendaStyle(showLocation bool, loc *time.Location) eventlist.Style {
	return eventlist.Style{
		Layout:       eventlist.Inline,
		Empty:        emptyMsg,
		ShowLocation: showLocation,
		Location:     loc,
	}
}

// agendaWidth is how wide every row's list is in bounds, which is what
// its line count is measured at before the rows are planned.
func agendaWidth(bounds image.Rectangle) int {
	return bounds.Dx() - agendaX - 2*agendaPadX
}

// agendaList is the rectangle a row's list is drawn into: inset from the
// row, and exactly as many lines tall as planRows gave it. The list fits
// that many lines and no more, so when the week took lines from a row,
// the last line it kept becomes "+N MORE" rather than the list spilling
// into the spare room the rows share.
func agendaList(row rowLayout) image.Rectangle {
	top := row.Agenda.Min.Y + agendaPadY
	// A literal rather than image.Rect, which would swap the edges of a
	// row too narrow for its padding into a list to the left of it.
	return image.Rectangle{
		Min: image.Pt(row.Agenda.Min.X+agendaPadX, top),
		Max: image.Pt(row.Agenda.Max.X-agendaPadX, min(top+row.Lines*daygrid.BodyLineH(), row.Agenda.Max.Y)),
	}
}
