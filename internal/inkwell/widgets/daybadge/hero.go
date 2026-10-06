package daybadge

import (
	"fmt"
	"image"
	"strings"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daygrid"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/drawkit"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/fuzzyclock"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/weatherview"
)

// The hero badge is the top of today-hero's left column: the identity
// band naming the day, then today's weather. today-hero draws the
// combined chart under it. 338 px is the hero column's width, 42% of the
// panel.
const (
	heroW = 338
	heroH = 202

	heroPadX = 14

	// The identity band's three runs. The date is 3x, a 6.2 mm cap
	// readable from about a metre, and the two labels under it are body
	// size, because they are what you read once you have walked up to
	// the panel.
	heroIdentityH = 116
	heroDateScale = 3
	heroDateBase  = 54
	heroMonthBase = 84
	heroClockBase = 104

	// The rule under the identity band, and the paper left between it
	// and the band's bottom edge so it does not butt against the icon.
	heroRuleW   = 2
	heroRuleGap = 2

	// The weather, from the top of the badge.
	heroIconSize = 58
	heroIconX    = 10
	heroIconY    = 122
	heroHiScale  = 3
	heroHiX      = 80
	heroHiBase   = 160
	heroLoGap    = 12
	heroCondBase = 192
)

// drawHero draws the identity band (the weekday and date at 3x, the
// month, and on today the fuzzy clock beneath it, closed off by a rule)
// and then the day's condition icon, its high and low, and the
// condition's name.
//
// The identity band used to be a solid black block with the text
// knocked out of it. A large fill that lands in the same place on every
// refresh is a burn-in risk on this panel, and today is already obvious
// from being the left column, so it carries no fill or outline at all
// (CLAUDE.md). The rule is what still separates it from the weather.
func drawHero(frame *image.Paletted, r image.Rectangle, day daygrid.Day, now time.Time, unit string) {
	x := r.Min.X + heroPadX
	drawkit.Scaled(drawkit.BodyBoldFace, heroDateScale, widget.PaperBlack).Draw(
		frame, x, r.Min.Y+heroDateBase,
		strings.ToUpper(day.Start.Format("Mon"))+" "+fmt.Sprintf("%d", day.Start.Day()),
	)
	drawkit.DrawText(frame, x, r.Min.Y+heroMonthBase,
		strings.ToUpper(day.Start.Format("January")), drawkit.BodyFace, widget.PaperBlack)

	// The clock is fuzzy because a precise one would change every
	// minute and the panel refreshes far less often: a clock that is
	// usually wrong is worse than one that is deliberately vague. It
	// is today's alone; "now" is no time on any other day.
	if day.IsToday {
		drawkit.DrawText(frame, x, r.Min.Y+heroClockBase,
			strings.ToUpper(fuzzyclock.Phrase(now, fuzzyclock.Options{})),
			drawkit.BodyFace, widget.PaperBlack)
	}

	// Two pixels rather than an agenda rule's one: this one closes the
	// band that names the day, so it is the heavier break.
	ruleY := r.Min.Y + heroIdentityH - heroRuleGap - heroRuleW
	for i := range heroRuleW {
		drawkit.DrawHLine(frame, x, r.Max.X-heroPadX, ruleY+i, widget.PaperBlack)
	}

	if day.Forecast == nil {
		return
	}
	weatherview.DrawIcon(frame, r.Min.X+heroIconX, r.Min.Y+heroIconY, heroIconSize, day.Forecast.Condition)

	temps := weatherview.NewHighLow(*day.Forecast, unit)
	hiX := r.Min.X + heroHiX
	drawer := drawkit.Scaled(drawkit.BodyBoldFace, heroHiScale, widget.PaperBlack)
	drawer.Draw(frame, hiX, r.Min.Y+heroHiBase, temps.High())
	drawkit.DrawText(frame, hiX+drawer.Measure(temps.High())+heroLoGap, r.Min.Y+heroHiBase,
		temps.Low(), drawkit.BodyFace, widget.PaperBlack)
	drawkit.DrawText(frame, hiX, r.Min.Y+heroCondBase,
		strings.ToUpper(day.Forecast.Condition.Label()), drawkit.BodyFace, widget.PaperBlack)
}
