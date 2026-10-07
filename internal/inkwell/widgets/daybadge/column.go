package daybadge

import (
	"fmt"
	"image"
	"strings"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daydata"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/drawkit"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/weatherview"
)

// The column badge is bold-five's column head, 160 px wide (a fifth of
// the panel) and two bands tall: the date band, then the weather summary
// whose combined chart bold-five draws below it.
const (
	columnW = 160
	columnH = columnDateH + columnChartTop

	// columnDateH covers the weekday abbreviation and the date numeral.
	// The weekday's 2x baseline sits at 8+28 and the numeral's 3x
	// baseline at 44+42; a numeral has no descender, so the band ends a
	// few pixels under the digits' dilated base rather than reserving
	// the descent.
	columnDateH = 92

	// weekdayScale/dateScale set the two date sizes. The numeral is the
	// element that decides whether the panel reads from the doorway: 3x
	// of the 20 px tier is a 6.2 mm cap height (~1.06 m legibility).
	weekdayScale = 2
	dateScale    = 3

	// Top offsets of each run within the date band; the baseline is
	// this plus the scaled ascent. Both runs are capitals and digits,
	// which carry no descenders, so the numeral sits on its baseline
	// rather than on a descent nothing below it uses.
	weekdayTop = 8
	dateTop    = 44

	// The weather summary, from the top of its band. The high and low
	// are right-aligned against the column's right edge so they bookend
	// the icon rather than clumping against it. The high is scaled; the
	// low is body size, because the pair only needs one element large
	// enough to read at distance and two would crowd the chart out.
	columnIconSize = 38
	columnIconX    = 6
	columnIconY    = 2
	columnTempPadX = 6
	columnHiScale  = 2
	columnHiBase   = 30
	columnLoBase   = 56
	// columnChartTop is where the weather band's chart starts, which is
	// where the badge ends.
	columnChartTop = 64
)

// drawColumn draws the weekday above the date numeral, both centred, and
// under them the weather summary.
//
// Every column renders identically: there is deliberately no today
// highlight. Today is always the leftmost column, so an inverted header
// or a column frame would spend ink restating what position already
// says. (A framed column with an otherwise-normal header also read as
// half-finished.)
func drawColumn(frame *image.Paletted, r image.Rectangle, day daydata.Day, _ time.Time, unit string) {
	ascent := drawkit.BodyAscent()
	drawkit.Scaled(drawkit.BodyBoldFace, weekdayScale, widget.PaperBlack).DrawCentered(
		frame, r.Min.X, r.Max.X, r.Min.Y+weekdayTop+ascent*weekdayScale,
		strings.ToUpper(day.Start.Format("Mon")),
	)
	drawkit.Scaled(drawkit.BodyBoldFace, dateScale, widget.PaperBlack).DrawCentered(
		frame, r.Min.X, r.Max.X, r.Min.Y+dateTop+ascent*dateScale,
		fmt.Sprintf("%d", day.Start.Day()),
	)

	if day.Forecast == nil {
		return
	}
	top := r.Min.Y + columnDateH
	weatherview.DrawIcon(frame, r.Min.X+columnIconX, top+columnIconY, columnIconSize, day.Forecast.Condition)

	temps := weatherview.NewHighLow(*day.Forecast, unit)
	right := r.Max.X - columnTempPadX
	drawkit.Scaled(drawkit.BodyBoldFace, columnHiScale, widget.PaperBlack).DrawRight(frame, right, top+columnHiBase, temps.High())
	drawkit.DrawTextRight(frame, right, top+columnLoBase, temps.Low(), drawkit.BodyFace, widget.PaperBlack)
}
