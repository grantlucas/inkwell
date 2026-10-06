package daybadge

import (
	"fmt"
	"image"
	"strings"

	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daygrid"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/drawkit"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/weatherview"
)

// The row badge is row-agenda's date gutter and the weather beside it,
// one minimum row tall; row-agenda draws the combined chart right of it.
// The offsets are from the top of the badge, so the date, the icon and
// the chart sit level with each other however tall a row is.
const (
	rowW = rowGutterW + rowChartDX
	rowH = 76

	// rowGutterW is the date block: numeral plus the stacked weekday and
	// month abbreviations.
	rowGutterW = 122
	rowPadX    = 10
	rowScale   = 3
	// The 3x numeral and the stacked weekday and month are centred on
	// the badge, with the month's baseline level with the numeral's.
	rowDateBase    = 56
	rowAbbrGap     = 8
	rowWeekdayBase = 36
	rowMonthBase   = 56

	// The weather: icon and high over low, centred on the badge. The
	// partly-cloudy glyph's sun ray reaches past its nominal size, and
	// butted against the high it read as a minus sign ("-16°C"), so the
	// icon sits flush left and the readings start clear of the ray.
	rowIconSize = 38
	rowIconY    = (rowH - rowIconSize) / 2
	rowHiDX     = 52
	rowHiBase   = 34
	rowLoBase   = 56

	// rowChartDX is where row-agenda's chart starts, and so where the
	// badge ends: clear of the widest reading there is, "-100°F", at
	// rowHiDX. Drawn into the badge before the chart, an overlap would
	// not look like a layout slip, it would look like bars painted
	// through the digits. TestStyle_KeepsToItsRect pins it.
	rowChartDX = 112
)

// drawRow draws the date numeral at 3x with the weekday and month
// stacked beside it, then the condition icon with the high over the low.
//
// The same plain date on every row, today included. Today is the first
// row, and position says that on its own; a filled block would sit in
// the same place every refresh, which is what burns into an e-paper
// panel. The high is body size, not scaled: the badge cannot hold a
// 38 px icon, a display-sized temperature and a legible chart at once,
// and the 3x date numeral already carries the row at distance.
func drawRow(frame *image.Paletted, r image.Rectangle, day daygrid.Day, unit string) {
	x := r.Min.X + rowPadX
	numeral := fmt.Sprintf("%d", day.Start.Day())
	drawer := drawkit.Scaled(drawkit.BodyBoldFace, rowScale, widget.PaperBlack)
	drawer.Draw(frame, x, r.Min.Y+rowDateBase, numeral)

	abbrX := x + drawer.Measure(numeral) + rowAbbrGap
	drawkit.DrawText(frame, abbrX, r.Min.Y+rowWeekdayBase,
		strings.ToUpper(day.Start.Format("Mon")), drawkit.BodyFace, widget.PaperBlack)
	drawkit.DrawText(frame, abbrX, r.Min.Y+rowMonthBase,
		strings.ToUpper(day.Start.Format("Jan")), drawkit.BodyFace, widget.PaperBlack)

	if day.Forecast == nil {
		return
	}
	bx := r.Min.X + rowGutterW
	weatherview.DrawIcon(frame, bx, r.Min.Y+rowIconY, rowIconSize, day.Forecast.Condition)

	temps := weatherview.NewHighLow(*day.Forecast, unit)
	drawkit.DrawText(frame, bx+rowHiDX, r.Min.Y+rowHiBase, temps.High(), drawkit.BodyBoldFace, widget.PaperBlack)
	drawkit.DrawText(frame, bx+rowHiDX, r.Min.Y+rowLoBase, temps.Low(), drawkit.BodyFace, widget.PaperBlack)
}
