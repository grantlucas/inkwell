package daybadge

import (
	"fmt"
	"image"
	"strings"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daygrid"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/drawkit"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/weatherview"
)

// The compact badge is the top of today-hero's day-row gutter: the tag,
// then the numeral with the condition icon and the high and low beside
// it. today-hero draws the day's combined chart under it. The numeral
// stays at 2x; the high and low are body size, on the numeral's centre
// line, and end at the badge's right edge, where the chart ends.
const (
	compactW = 176
	compactH = 64

	compactPadX     = 12
	compactTagBase  = 20
	compactScale    = 2
	compactDateBase = 56
	compactTempBase = 48

	// The condition icon is centred in the gap between the widest
	// numeral and the high and low. 30 px, so the three share one line
	// and leave the lower half of the gutter to the chart. The glyphs'
	// rays overrun their box by a few pixels, which is why the icon
	// sits clear of the tag above it.
	compactIconSize   = 30
	compactIconY      = 30
	compactNumeralEnd = 54
)

// drawCompact draws the day's tag ("TOMORROW" for tomorrow, its weekday
// otherwise) over its date numeral, then the high and low and the
// condition icon.
//
// Tomorrow is tagged, not promoted. An earlier draft of today-hero moved
// it into the hero column and started the rows at +2, which quietly
// dropped a day; the panel keeps its full span and nothing appears
// twice.
func drawCompact(frame *image.Paletted, r image.Rectangle, day daygrid.Day, now time.Time, unit string) {
	x := r.Min.X + compactPadX
	tag := strings.ToUpper(day.Start.Format("Mon"))
	if isTomorrow(day, now) {
		tag = "TOMORROW"
	}
	drawkit.DrawText(frame, x, r.Min.Y+compactTagBase, tag, drawkit.BodyFace, widget.PaperBlack)
	drawkit.Scaled(drawkit.BodyBoldFace, compactScale, widget.PaperBlack).Draw(
		frame, x, r.Min.Y+compactDateBase, fmt.Sprintf("%d", day.Start.Day()))

	if day.Forecast == nil {
		return
	}
	hiLo := weatherview.NewHighLow(*day.Forecast, unit).Pair()
	tempX := r.Max.X - drawkit.TextWidth(drawkit.BodyFace, hiLo)
	drawkit.DrawText(frame, tempX, r.Min.Y+compactTempBase, hiLo, drawkit.BodyFace, widget.PaperBlack)

	iconX := (r.Min.X + compactNumeralEnd + tempX - compactIconSize) / 2
	weatherview.DrawIcon(frame, iconX, r.Min.Y+compactIconY, compactIconSize, day.Forecast.Condition)
}

// isTomorrow is whether day starts at the local midnight after now's.
// Days start at local midnight in now's zone, so the comparison is of
// dates, not of a 24-hour offset that a DST change would break.
func isTomorrow(day daygrid.Day, now time.Time) bool {
	y, m, d := now.Date()
	return day.Start.Equal(time.Date(y, m, d+1, 0, 0, 0, 0, now.Location()))
}
