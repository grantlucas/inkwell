package weekly

import (
	"fmt"
	"image"
	"strings"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/fonts"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"golang.org/x/image/font"
)

var (
	dayAbbrFace font.Face
	dateNumFace font.Face
	monthFace   font.Face
)

func init() {
	dayAbbrFace = mustLoadHeaderFace(fonts.SemiBold, 12, "day abbr")
	dateNumFace = mustLoadHeaderFace(fonts.SemiBold, 16, "date num")
	monthFace = mustLoadHeaderFace(fonts.Regular, 12, "month")
}

// mustLoadHeaderFace is extracted so the per-face panic branches are
// reachable from tests via fonts.SwapDataForTest. The role label
// flows into the panic message so a failure identifies which face
// failed to load.
func mustLoadHeaderFace(weight fonts.Weight, size float64, role string) font.Face {
	f, err := fonts.Face(weight, size)
	if err != nil {
		panic("weekly: load " + role + " font: " + err.Error())
	}
	return f
}

// renderDayHeader draws the day header for a column: abbreviated day name,
// date number, and month abbreviation, all centered. Today's column gets a
// soft gray background tint with normal dark text on top, plus a subtle
// underscore — far easier on the eye than the old hard inverse block.
func renderDayHeader(frame *image.Paletted, bounds image.Rectangle, day time.Time, isToday bool) {
	if isToday {
		fillRect(frame, bounds, widget.PaperGray10)
		// 1-px underline grounding the today cell. PaperBlack is the
		// only device-safe choice for a 1-pixel stroke — a flat
		// PaperGray60 row dithers into a dashed dotted line, which
		// reads as broken rather than as a "subtle hairline".
		drawHLine(frame, bounds.Min.X, bounds.Max.X, bounds.Max.Y-1, widget.PaperBlack)
	}

	dayAbbr := strings.ToUpper(day.Format("Mon"))
	dateNum := fmt.Sprintf("%d", day.Day())
	monthAbbr := strings.ToUpper(day.Format("Jan"))

	abbrMetrics := dayAbbrFace.Metrics()
	dateMetrics := dateNumFace.Metrics()
	monthMetrics := monthFace.Metrics()

	abbrH := abbrMetrics.Ascent.Ceil() + abbrMetrics.Descent.Ceil()
	dateH := dateMetrics.Ascent.Ceil() + dateMetrics.Descent.Ceil()
	monthH := monthMetrics.Ascent.Ceil() + monthMetrics.Descent.Ceil()

	gap := 2
	totalH := abbrH + gap + dateH + gap + monthH
	startY := bounds.Min.Y + (bounds.Dy()-totalH)/2

	// Day-of-week (MON, TUE, …) and month abbr render in muted gray so the
	// date number stays the visual anchor. Today inherits the same treatment;
	// the background tint already signals "today" without needing inverse text.
	y := startY + abbrMetrics.Ascent.Ceil()
	drawTextCenteredGrayWithFace(frame, bounds.Min.X, bounds.Max.X, y, dayAbbr, dayAbbrFace, widget.PaperGray70)

	y += abbrMetrics.Descent.Ceil() + gap + dateMetrics.Ascent.Ceil()
	drawTextCenteredWithFace(frame, bounds.Min.X, bounds.Max.X, y, dateNum, dateNumFace)

	y += dateMetrics.Descent.Ceil() + gap + monthMetrics.Ascent.Ceil()
	drawTextCenteredGrayWithFace(frame, bounds.Min.X, bounds.Max.X, y, monthAbbr, monthFace, widget.PaperGray70)
}
