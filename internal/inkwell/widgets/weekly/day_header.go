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
	// Month abbr is SemiBold (not Regular) because Terminus's Regular
	// "J" at 12 pt renders with a 1-px stem and a hook that sits to the
	// left of that stem, which reads on-device as a disconnected
	// ".UN" — the user couldn't see the J at all from across the room.
	// SemiBold widens the stem to 2 px and ties the hook into it, so
	// "JUN" reads cleanly black-on-white. Today's inverted column was
	// fine in Regular because the white-on-black contrast hid the gap.
	monthFace = mustLoadHeaderFace(fonts.SemiBold, 12, "month")
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
// date number, and month abbreviation, all centered. Today's column inverts
// to a solid PaperBlack block with PaperWhite text — a soft gray tint
// vanishes on BW (pure threshold) and collapses to pure white in Gray4's
// light bucket, so the inversion is the only device-durable signal.
func renderDayHeader(frame *image.Paletted, bounds image.Rectangle, day time.Time, isToday bool) {
	// All text in the day header renders in solid PaperBlack so the AA
	// fringe doesn't get chopped by the BW threshold path — a gray
	// source color leaves anti-aliased edge pixels above Y=128, which
	// the threshold then drops, leaving glyphs visibly fragmented. The
	// visual hierarchy comes from font weight + size (semi-bold day
	// abbr, large date number, regular month abbr), not color.
	primaryIdx := widget.PaperBlack
	mutedIdx := widget.PaperBlack
	if isToday {
		fillRect(frame, bounds, widget.PaperBlack)
		primaryIdx = widget.PaperWhite
		mutedIdx = widget.PaperWhite
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

	y := startY + abbrMetrics.Ascent.Ceil()
	drawTextCenteredGrayWithFace(frame, bounds.Min.X, bounds.Max.X, y, dayAbbr, dayAbbrFace, mutedIdx)

	y += abbrMetrics.Descent.Ceil() + gap + dateMetrics.Ascent.Ceil()
	drawTextCenteredGrayWithFace(frame, bounds.Min.X, bounds.Max.X, y, dateNum, dateNumFace, primaryIdx)

	y += dateMetrics.Descent.Ceil() + gap + monthMetrics.Ascent.Ceil()
	drawTextCenteredGrayWithFace(frame, bounds.Min.X, bounds.Max.X, y, monthAbbr, monthFace, mutedIdx)
}
