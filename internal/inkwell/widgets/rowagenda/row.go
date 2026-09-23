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

const (
	gutterPadX      = 10
	dateScale       = 3
	dateBaseline    = 52
	abbrGap         = 8
	weekdayBaseline = 36
	monthBaseline   = 56

	// The weather badge.
	badgeIconX  = 4
	badgeIconY  = 8
	badgeIconSz = 38
	hiDX        = 46
	hiBaseline  = 40
	loBaseline  = 68

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
// Today's gutter inverts. Here the highlight earns its ink in a way it
// does not on bold-five: rows have no "today is leftmost" convention to
// lean on, so without it there is nothing saying which row is now.
// Inversion rather than a tint, because a PaperGray20 field vanishes in
// Gray4's light bucket and snaps to white under the BW threshold.
func renderGutter(frame *image.Paletted, bounds image.Rectangle, day daygrid.Day) {
	ink := widget.PaperBlack
	if day.IsToday {
		daygrid.FillRect(frame, bounds, widget.PaperBlack)
		ink = widget.PaperWhite
	}

	x := bounds.Min.X + gutterPadX
	numeral := fmt.Sprintf("%d", day.Start.Day())
	drawer := daygrid.Scaled(daygrid.BodyBoldFace, dateScale, ink)
	drawer.Draw(frame, x, bounds.Min.Y+dateBaseline, numeral)

	abbrX := x + drawer.Measure(numeral) + abbrGap
	daygrid.DrawText(frame, abbrX, bounds.Min.Y+weekdayBaseline,
		strings.ToUpper(day.Start.Format("Mon")), daygrid.BodyFace, ink)
	daygrid.DrawText(frame, abbrX, bounds.Min.Y+monthBaseline,
		strings.ToUpper(day.Start.Format("Jan")), daygrid.BodyFace, ink)
}

// renderBadge draws the condition icon, the hi/lo pair, and the row's
// precipitation chart.
//
// These bars are the narrowest of the three screens that carry them —
// 110 px against bold-five's 160 — so they are still a shape, but a
// cramped one. The 50% guide stays off at this width: the dashes would
// compete with the bars they are meant to measure.
func renderBadge(frame *image.Paletted, bounds image.Rectangle, forecast weather.DailyForecast, unit string, isToday, marker bool, nowHour int) {
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
		// No dry-day caption: 110 px cannot carry a legible phrase, and
		// an empty cell already reads as a dry day.
		DryText: "",
	})
}

// drawIcon is indirected through a var so the failure branch above is
// reachable from tests; weatherview's own font seam is package-private.
var drawIcon = weatherview.DrawIcon
