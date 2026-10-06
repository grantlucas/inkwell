package rowagenda

import (
	"image"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/widgets/drawkit"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/eventlist"
)

const (
	agendaPadX = 10
	// agendaPadY is the room above the first line and below the last.
	// planRows sizes a row as this twice plus its lines, so changing it
	// changes how many lines a week can carry.
	agendaPadY = 8
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
	return eventlist.PresetInline.Style(0, showLocation, loc)
}

// agendaWidth is how wide every row's list is in bounds, which is what
// its line count is measured at before the rows are planned. Every row's
// agenda column spans the same x range, so it is taken from agendaList
// over that range rather than worked out a second way.
func agendaWidth(bounds image.Rectangle) int {
	column := image.Rect(bounds.Min.X+agendaX, bounds.Min.Y, bounds.Max.X, bounds.Max.Y)
	return agendaList(rowLayout{Agenda: column}).Dx()
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
		Max: image.Pt(row.Agenda.Max.X-agendaPadX, min(top+row.Lines*drawkit.BodyLineH(), row.Agenda.Max.Y)),
	}
}
