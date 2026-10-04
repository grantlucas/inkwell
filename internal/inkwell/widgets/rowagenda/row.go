package rowagenda

import (
	"fmt"
	"image"
	"log"
	"math"
	"strings"

	"github.com/grantlucas/inkwell/internal/inkwell/weather"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daygrid"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/weatherview"
)

// The offsets below are from the top of the minRowH block planRows
// centres in each row, so the date, the icon and the chart sit level
// with each other however tall the row's agenda makes it.
const (
	gutterPadX = 10
	dateScale  = 3
	// The 3x numeral and the stacked weekday and month are centred on
	// the block, with the month's baseline level with the numeral's.
	dateBaseline    = 56
	abbrGap         = 8
	weekdayBaseline = 36
	monthBaseline   = 56

	// The weather badge: icon and hi/lo pair centred on the block. The
	// partly-cloudy glyph's sun ray reaches past its nominal size, and
	// butted against the high it read as a minus sign ("-16°C"), so the
	// icon sits flush left and the pair starts clear of the ray.
	badgeIconX  = 0
	badgeIconY  = (minRowH - badgeIconSz) / 2
	badgeIconSz = 38
	hiDX        = 52
	hiBaseline  = 34
	loBaseline  = 56

	// tempMaxChars is the widest reading the temperature block has to
	// hold — "-100°F" — and chartDX is placed clear of it. The two are
	// drawn into the same badge and the chart is drawn second, so an
	// overlap does not look like a layout slip, it looks like bars
	// painted through the digits. At the design sketch's 104 they did
	// overlap; TestRenderBadge_TemperatureNeverReachesTheChart pins
	// the relationship so they cannot drift back together.
	tempMaxChars = 6
	chartDX      = 112
	chartW       = 106
	chartPadY    = 4
	chartLabelH  = 14
)

// renderGutter draws the date block: the numeral at 3x with the weekday
// and month stacked beside it.
//
// The same plain date on every row, today included. Today is the first
// row, and position says that on its own; a filled block would sit in
// the same place every refresh, which is what burns into an e-paper
// panel.
func renderGutter(frame *image.Paletted, bounds image.Rectangle, day daygrid.Day) {
	x := bounds.Min.X + gutterPadX
	numeral := fmt.Sprintf("%d", day.Start.Day())
	drawer := daygrid.Scaled(daygrid.BodyBoldFace, dateScale, widget.PaperBlack)
	drawer.Draw(frame, x, bounds.Min.Y+dateBaseline, numeral)

	abbrX := x + drawer.Measure(numeral) + abbrGap
	daygrid.DrawText(frame, abbrX, bounds.Min.Y+weekdayBaseline,
		strings.ToUpper(day.Start.Format("Mon")), daygrid.BodyFace, widget.PaperBlack)
	daygrid.DrawText(frame, abbrX, bounds.Min.Y+monthBaseline,
		strings.ToUpper(day.Start.Format("Jan")), daygrid.BodyFace, widget.PaperBlack)
}

// renderBadge draws the condition icon, the hi/lo pair, and the row's
// combined chart: precipitation bars with the temperature line over
// them, plotted on rng, the range shared by all five rows.
//
// The chart is on every row whether or not rain is due, so a dry day
// still shows the shape of its temperature rather than a blank cell,
// and a cold day visibly sits lower than a warm one. The bars are the
// narrowest of the screens that carry them, so the 50% guide stays off:
// the dashes would compete with the bars they are meant to measure.
func renderBadge(frame *image.Paletted, bounds image.Rectangle, forecast weather.DailyForecast, unit string, isToday, marker bool, nowHour int, rng weatherview.TempRange) {
	if forecast.Date.IsZero() {
		// A zero DailyForecast is indistinguishable from a real
		// reading, so drawing it would state a forecast nobody made.
		return
	}
	top := bounds.Min.Y

	if err := drawIcon(frame, bounds.Min.X+badgeIconX, top+badgeIconY, badgeIconSz, forecast.Condition); err != nil {
		// Logged, not bubbled: the temperatures and the chart are
		// still worth drawing when the glyph will not load.
		log.Printf("rowagenda: draw icon for condition %d: %v", forecast.Condition, err)
	}

	hi, lo := forecast.High, forecast.Low
	label := "C"
	if unit == "F" {
		hi, lo = weather.CelsiusToFahrenheit(hi), weather.CelsiusToFahrenheit(lo)
		label = "F"
	}
	// Body size, not scaled. A 2x high is 120 px at its widest, which
	// runs straight through the chart — the badge is 222 px and cannot
	// hold a 38 px icon, a display-sized temperature and a legible
	// chart at once. The row already has a 3x date numeral carrying it
	// at distance, and this is the same compressed-day shape as
	// today-hero's rows, which are 1x too.
	x := bounds.Min.X + hiDX
	daygrid.DrawText(frame, x, top+hiBaseline,
		fmt.Sprintf("%d°%s", int(math.Round(hi)), label), daygrid.BodyBoldFace, widget.PaperBlack)
	daygrid.DrawText(frame, x, top+loBaseline,
		fmt.Sprintf("%d°", int(math.Round(lo))), daygrid.BodyFace, widget.PaperBlack)

	chart := image.Rect(
		bounds.Min.X+chartDX, top+chartPadY,
		bounds.Min.X+chartDX+chartW, bounds.Max.Y-chartPadY,
	)
	weatherview.RenderPrecipChart(frame, chart, forecast.Hourly, weatherview.PrecipChartOptions{
		LabelHeight:   chartLabelH,
		NowHour:       nowHour,
		ShowNowMarker: isToday && marker,
		ShowGuide:     false,
		TempRange:     &rng,
	})
}

// drawIcon is indirected through a var so the failure branch above is
// reachable from tests; weatherview's own font seam is package-private.
var drawIcon = weatherview.DrawIcon
