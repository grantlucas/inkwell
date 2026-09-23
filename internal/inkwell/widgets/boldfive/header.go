package boldfive

import (
	"fmt"
	"image"
	"strings"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daygrid"
)

const (
	// weekdayScale/dateScale set the two header sizes. The numeral is
	// the element that decides whether the panel reads from the
	// doorway: 3x of the 20 px tier is a 6.2 mm cap height (~1.06 m
	// legibility) against the 2.1 mm the live weekly view draws.
	weekdayScale = 2
	dateScale    = 3

	// Top offsets of each run within the header band; the baseline is
	// this plus the scaled ascent.
	weekdayTop = 12
	dateTop    = 52
)

// renderDayHeader draws one column's header: the weekday abbreviation
// above the date numeral, both centred.
//
// Every column renders identically — there is deliberately no today
// highlight. Today is always the leftmost column, so an inverted header
// or a column frame would spend ink restating what position already
// says. (A framed column with an otherwise-normal header also read as
// half-finished.) TestRenderDayHeader_TodayIsNotHighlighted pins this,
// because it is a decision rather than an omission.
func renderDayHeader(frame *image.Paletted, bounds image.Rectangle, day time.Time) {
	ascent := daygrid.BodyAscent()

	weekday := strings.ToUpper(day.Format("Mon"))
	daygrid.Scaled(daygrid.BodyBoldFace, weekdayScale, widget.PaperBlack).DrawCentered(
		frame, bounds.Min.X, bounds.Max.X,
		bounds.Min.Y+weekdayTop+ascent*weekdayScale,
		weekday,
	)

	daygrid.Scaled(daygrid.BodyBoldFace, dateScale, widget.PaperBlack).DrawCentered(
		frame, bounds.Min.X, bounds.Max.X,
		bounds.Min.Y+dateTop+ascent*dateScale,
		fmt.Sprintf("%d", day.Day()),
	)
}
