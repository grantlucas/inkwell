package weekly

import (
	"fmt"
	"image"
	"strings"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/fonts"
	"golang.org/x/image/font"
)

var (
	dayAbbrFace font.Face
	dateNumFace font.Face
	monthFace   font.Face
)

func init() {
	var err error
	dayAbbrFace, err = fonts.Face(fonts.SemiBold, 10)
	if err != nil {
		panic("weekly: load day abbr font: " + err.Error())
	}
	dateNumFace, err = fonts.Face(fonts.SemiBold, 16)
	if err != nil {
		panic("weekly: load date num font: " + err.Error())
	}
	monthFace, err = fonts.Face(fonts.Regular, 10)
	if err != nil {
		panic("weekly: load month font: " + err.Error())
	}
}

// renderDayHeader draws the day header for a column: abbreviated day name,
// date number, and month abbreviation, all centered. Today's column gets
// an inverted (white-on-black) background.
func renderDayHeader(frame *image.Paletted, bounds image.Rectangle, day time.Time, isToday bool) {
	if isToday {
		fillRect(frame, bounds, 1)
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
	if isToday {
		drawTextCenteredWhiteWithFace(frame, bounds.Min.X, bounds.Max.X, y, dayAbbr, dayAbbrFace)
	} else {
		drawTextCenteredWithFace(frame, bounds.Min.X, bounds.Max.X, y, dayAbbr, dayAbbrFace)
	}

	y += abbrMetrics.Descent.Ceil() + gap + dateMetrics.Ascent.Ceil()
	if isToday {
		drawTextCenteredWhiteWithFace(frame, bounds.Min.X, bounds.Max.X, y, dateNum, dateNumFace)
	} else {
		drawTextCenteredWithFace(frame, bounds.Min.X, bounds.Max.X, y, dateNum, dateNumFace)
	}

	y += dateMetrics.Descent.Ceil() + gap + monthMetrics.Ascent.Ceil()
	if isToday {
		drawTextCenteredWhiteWithFace(frame, bounds.Min.X, bounds.Max.X, y, monthAbbr, monthFace)
	} else {
		drawTextCenteredWithFace(frame, bounds.Min.X, bounds.Max.X, y, monthAbbr, monthFace)
	}
}
