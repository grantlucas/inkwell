package weekly

import (
	"fmt"
	"image"
	"strings"
	"time"
)

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

	y := bounds.Min.Y + lineHeight + 2

	if isToday {
		drawTextCenteredWhite(frame, bounds.Min.X, bounds.Max.X, y, dayAbbr)
		y += lineHeight + 1
		drawTextCenteredWhite(frame, bounds.Min.X, bounds.Max.X, y, dateNum)
		y += lineHeight + 1
		drawTextCenteredWhite(frame, bounds.Min.X, bounds.Max.X, y, monthAbbr)
	} else {
		drawTextCentered(frame, bounds.Min.X, bounds.Max.X, y, dayAbbr)
		y += lineHeight + 1
		drawTextCentered(frame, bounds.Min.X, bounds.Max.X, y, dateNum)
		y += lineHeight + 1
		drawTextCentered(frame, bounds.Min.X, bounds.Max.X, y, monthAbbr)
	}
}
