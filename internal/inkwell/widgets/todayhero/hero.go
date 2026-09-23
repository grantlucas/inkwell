package todayhero

import (
	"fmt"
	"image"
	"log"
	"math"
	"strings"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/weather"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daygrid"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/fuzzyclock"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/weatherview"
)

const (
	heroPadX = 14

	// The identity block's three runs. The date is 3x — a 6.2 mm cap,
	// readable from about a metre — and the two labels under it are
	// body size, because they are what you read once you have walked
	// up to the panel.
	dateScale     = 3
	dateBaseline  = 54
	monthBaseline = 84
	fuzzyBaseline = 108

	// Today's weather.
	heroIconSize = 58
	heroIconX    = 10
	heroIconY    = 122
	hiScale      = 3
	hiX          = 80
	hiBaseline   = 160
	loGap        = 12
	condBaseline = 192

	// The chart is the widest of any screen's, and the only one with
	// room for the 50% guide and a dry-day caption.
	chartLabelH = 20
	dryText     = "NO RAIN TODAY"
)

// renderIdentity draws the inverted block: the date at 3x in
// PaperWhite, then the month and the fuzzy clock beneath it.
//
// Inversion rather than a tint: a PaperGray20 background vanishes in
// Gray4's light bucket and snaps to white under the BW threshold, so it
// would read as a highlight on neither mode (CLAUDE.md). A solid black
// field with white text reads on both.
func renderIdentity(frame *image.Paletted, bounds image.Rectangle, now time.Time) {
	daygrid.FillRect(frame, bounds, widget.PaperBlack)

	x := bounds.Min.X + heroPadX
	daygrid.Scaled(daygrid.BodyBoldFace, dateScale, widget.PaperWhite).Draw(
		frame, x, bounds.Min.Y+dateBaseline,
		strings.ToUpper(now.Format("Mon"))+" "+fmt.Sprintf("%d", now.Day()),
	)

	daygrid.DrawText(frame, x, bounds.Min.Y+monthBaseline,
		strings.ToUpper(now.Format("January")), daygrid.BodyFace, widget.PaperWhite)

	// The clock is fuzzy because a precise one would change every
	// minute and the panel only refreshes every fifteen — a clock that
	// is usually wrong is worse than one that is deliberately vague.
	// Event times stay precise: they are data, not a clock, and they do
	// not change on a tick.
	daygrid.DrawText(frame, x, bounds.Min.Y+fuzzyBaseline,
		strings.ToUpper(fuzzyclock.Phrase(now, fuzzyclock.Options{})),
		daygrid.BodyFace, widget.PaperWhite)
}

// renderHeroWeather draws today's condition icon, its high and low, and
// the condition label.
func renderHeroWeather(frame *image.Paletted, bounds image.Rectangle, day weather.DailyForecast, unit string) {
	if day.Date.IsZero() {
		// A zero DailyForecast is indistinguishable from a real
		// reading — a clear sky at 0°C is plausible here in January —
		// so drawing it would state a forecast nobody made.
		return
	}

	top := bounds.Min.Y
	if err := drawIcon(frame, bounds.Min.X+heroIconX, top+heroIconY-weatherTop, heroIconSize, day.Condition); err != nil {
		// Logged, not bubbled: the temperatures are still worth having
		// when the glyph will not load.
		log.Printf("todayhero: draw icon for condition %d: %v", day.Condition, err)
	}

	hi, lo := day.High, day.Low
	label := "C"
	if unit == "F" {
		hi, lo = weather.CelsiusToFahrenheit(hi), weather.CelsiusToFahrenheit(lo)
		label = "F"
	}

	hiText := fmt.Sprintf("%d°%s", int(math.Round(hi)), label)
	x := bounds.Min.X + hiX
	drawer := daygrid.Scaled(daygrid.BodyBoldFace, hiScale, widget.PaperBlack)
	drawer.Draw(frame, x, top+hiBaseline-weatherTop, hiText)

	daygrid.DrawText(frame, x+drawer.Measure(hiText)+loGap, top+hiBaseline-weatherTop,
		fmt.Sprintf("%d°", int(math.Round(lo))), daygrid.BodyFace, widget.PaperBlack)

	daygrid.DrawText(frame, x, top+condBaseline-weatherTop,
		strings.ToUpper(day.Condition.Label()), daygrid.BodyFace, widget.PaperBlack)
}

// renderHeroChart draws today's precipitation. This is the widest
// precipitation cell of any screen and the main reason to pick this
// one: 15 px bars with a marker at the current hour, so an afternoon
// band reads as a band rather than as a texture. It is the only cell
// with room for the 50% guide and for saying so when the day is dry.
func renderHeroChart(frame *image.Paletted, bounds image.Rectangle, day weather.DailyForecast, nowHour int) {
	if day.Date.IsZero() {
		return
	}
	weatherview.RenderPrecipChart(frame, bounds, day.Hourly, weatherview.PrecipChartOptions{
		LabelHeight:   chartLabelH,
		NowHour:       nowHour,
		ShowNowMarker: true,
		ShowGuide:     true,
		DryText:       dryText,
	})
}

// drawIcon is indirected through a var so the failure branch above is
// reachable from tests; weatherview's own font seam is package-private.
var drawIcon = weatherview.DrawIcon
