package todayhero

import (
	"fmt"
	"image"
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
	fuzzyBaseline = 104

	// The rule under the identity band, and the paper left between it
	// and the band's bottom edge so it does not butt against the icon.
	identityRuleW   = 2
	identityRuleGap = 2

	// Today's weather.
	heroIconSize = 58
	heroIconX    = 10
	heroIconY    = 122
	hiScale      = 3
	hiX          = 80
	hiBaseline   = 160
	loGap        = 12
	condBaseline = 192
)

// renderIdentity draws the date at 3x, then the month and the fuzzy
// clock beneath it, all in PaperBlack on paper, closed off by a rule.
//
// This used to be a solid black block with the text knocked out of it.
// A large fill that lands in the same place on every refresh is a
// burn-in risk on this panel, and today is already obvious from being
// the left column, so the band carries no fill or outline at all
// (CLAUDE.md). The rule is what still separates it from the weather.
func renderIdentity(frame *image.Paletted, bounds image.Rectangle, now time.Time) {
	x := bounds.Min.X + heroPadX
	daygrid.Scaled(daygrid.BodyBoldFace, dateScale, widget.PaperBlack).Draw(
		frame, x, bounds.Min.Y+dateBaseline,
		strings.ToUpper(now.Format("Mon"))+" "+fmt.Sprintf("%d", now.Day()),
	)

	daygrid.DrawText(frame, x, bounds.Min.Y+monthBaseline,
		strings.ToUpper(now.Format("January")), daygrid.BodyFace, widget.PaperBlack)

	// The clock is fuzzy because a precise one would change every
	// minute and the panel only refreshes every fifteen — a clock that
	// is usually wrong is worse than one that is deliberately vague.
	// Event times stay precise: they are data, not a clock, and they do
	// not change on a tick.
	daygrid.DrawText(frame, x, bounds.Min.Y+fuzzyBaseline,
		strings.ToUpper(fuzzyclock.Phrase(now, fuzzyclock.Options{})),
		daygrid.BodyFace, widget.PaperBlack)

	// Two pixels rather than the agenda rule's one: this one closes the
	// band that names the day, so it is the heavier break.
	for i := range identityRuleW {
		daygrid.DrawHLine(frame, x, bounds.Max.X-heroPadX, bounds.Max.Y-identityRuleGap-identityRuleW+i, widget.PaperBlack)
	}
}

// renderHeroWeather draws today's condition icon, its high and low, and
// the condition label.
func renderHeroWeather(frame *image.Paletted, bounds image.Rectangle, day *weather.DailyForecast, unit string) {
	if day == nil {
		// No forecast reaches today. Drawing a zero would state a
		// forecast nobody made — a clear sky at 0°C is plausible here
		// in January.
		return
	}

	top := bounds.Min.Y
	weatherview.DrawIcon(frame, bounds.Min.X+heroIconX, top+heroIconY-weatherTop, heroIconSize, day.Condition)

	temps := weatherview.NewHighLow(*day, unit)
	x := bounds.Min.X + hiX
	drawer := daygrid.Scaled(daygrid.BodyBoldFace, hiScale, widget.PaperBlack)
	drawer.Draw(frame, x, top+hiBaseline-weatherTop, temps.High())

	daygrid.DrawText(frame, x+drawer.Measure(temps.High())+loGap, top+hiBaseline-weatherTop,
		temps.Low(), daygrid.BodyFace, widget.PaperBlack)

	daygrid.DrawText(frame, x, top+condBaseline-weatherTop,
		strings.ToUpper(day.Condition.Label()), daygrid.BodyFace, widget.PaperBlack)
}

// renderHeroChart draws today's combined chart: precipitation bars with
// the temperature line over them. This is the widest chart cell of any
// screen and the main reason to pick this one: 15 px bars with a marker
// at the current hour, so an afternoon band reads as a band rather than
// as a texture.
//
// rng is the screen's shared range, so today's line sits at the same
// height as a row's line for the same temperature. A dry day still
// draws the line, so the chart is never blank.
func renderHeroChart(frame *image.Paletted, bounds image.Rectangle, day *weather.DailyForecast, nowHour int, rng weatherview.TempRange) {
	if day == nil {
		return
	}
	weatherview.RenderCombinedChart(frame, bounds, day.Hourly, rng, weatherview.CombinedOptions{
		NowHour:       nowHour,
		ShowNowMarker: true,
	})
}
