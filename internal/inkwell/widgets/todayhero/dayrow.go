package todayhero

import (
	"fmt"
	"image"
	"log"
	"math"
	"strings"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar"
	"github.com/grantlucas/inkwell/internal/inkwell/weather"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daygrid"
)

const (
	rowPadX = 12

	// The date gutter: a tag, then the numeral, then the hi/lo.
	tagBaseline     = 24
	rowDateScale    = 2
	rowDateBaseline = 72
	rowTempBaseline = 96

	// The condition icon, and where the agenda starts after it.
	rowIconSize = 40
	rowIconDX   = 116
	rowIconDY   = 40
	rowAgendaDX = 186

	// Up to three events a row; a fourth would leave no room for the
	// overflow marker to say how many were dropped.
	rowMaxEvents = 3

	// Upper case, like every other label this screen paints —
	// TOMORROW, ALL DAY, DONE FOR TODAY, NO RAIN TODAY. Mixed case in
	// the rows alone would read as a second typographic system.
	emptyRowMsg = "NOTHING SCHEDULED"
)

// dayRowOptions carries what a row needs beyond its events.
type dayRowOptions struct {
	// IsTomorrow tags the row "TOMORROW" instead of its weekday.
	IsTomorrow bool
	TempUnit   string
	Events     eventOptions
}

// renderDayRow draws one following day: the date gutter, a condition
// icon, and a short agenda.
//
// There is deliberately no precipitation chart here. It was tried and
// reverted: a 462 px row cannot carry a legible bar chart *and* a
// legible title, and titles dropped to about 12 characters. Future-day
// rain is the condition icon and nothing more — which does mean this is
// the one screen where the week's rain timing is genuinely missing. If
// that matters it is an argument for bold-five or row-agenda, not a
// thing to add back here.
func renderDayRow(
	frame *image.Paletted, bounds image.Rectangle, day daygrid.Day,
	forecast weather.DailyForecast, events []calendar.Event, opts dayRowOptions,
) {
	top := bounds.Min.Y
	x := bounds.Min.X + rowPadX

	// Tomorrow is tagged, not promoted. An earlier draft moved it into
	// the hero column and started the rows at +2, which quietly dropped
	// a day; the panel keeps its full five-day span and nothing appears
	// twice.
	tag := strings.ToUpper(day.Start.Format("Mon"))
	if opts.IsTomorrow {
		tag = "TOMORROW"
	}
	daygrid.DrawText(frame, x, top+tagBaseline, tag, daygrid.BodyFace, widget.PaperBlack)

	daygrid.Scaled(daygrid.BodyBoldFace, rowDateScale, widget.PaperBlack).Draw(
		frame, x, top+rowDateBaseline, fmt.Sprintf("%d", day.Start.Day()))

	renderRowWeather(frame, bounds, forecast, opts.TempUnit)
	renderRowAgenda(frame, bounds, events, opts.Events)
}

// renderRowWeather draws the row's hi/lo pair and condition icon.
func renderRowWeather(frame *image.Paletted, bounds image.Rectangle, forecast weather.DailyForecast, unit string) {
	if forecast.Date.IsZero() {
		// Nothing forecast for this day. Drawing a zero would state a
		// temperature nobody predicted.
		return
	}
	top := bounds.Min.Y
	hi, lo := forecast.High, forecast.Low
	if unit == "F" {
		hi, lo = weather.CelsiusToFahrenheit(hi), weather.CelsiusToFahrenheit(lo)
	}
	daygrid.DrawText(frame, bounds.Min.X+rowPadX, top+rowTempBaseline,
		fmt.Sprintf("%d° %d°", int(math.Round(hi)), int(math.Round(lo))),
		daygrid.BodyFace, widget.PaperBlack)

	if err := drawIcon(frame, bounds.Min.X+rowIconDX, top+rowIconDY, rowIconSize, forecast.Condition); err != nil {
		log.Printf("todayhero: draw row icon for condition %d: %v", forecast.Condition, err)
	}
}

// renderRowAgenda draws up to three events as a time and a title on one
// line each, then an overflow marker for the rest.
func renderRowAgenda(frame *image.Paletted, bounds image.Rectangle, events []calendar.Event, opts eventOptions) {
	x := bounds.Min.X + rowAgendaDX
	if x >= bounds.Max.X-rowPadX {
		return
	}

	lineH := daygrid.BodyLineH()
	// Sized for the widest label, not for a clock time: "ALL DAY" is
	// seven characters against 00:00's five, and measuring the clock
	// alone ran the all-day label straight into the title.
	timeW := daygrid.TextWidth(daygrid.BodyFace, "ALL DAY ")
	titleX := x + timeW
	maxChars := (bounds.Max.X - rowPadX - titleX) / daygrid.BodyAdvance()
	if maxChars < 3 {
		return
	}

	y := bounds.Min.Y + rowPadX + daygrid.BodyAscent()
	if len(events) == 0 {
		daygrid.DrawText(frame, x, y, emptyRowMsg, daygrid.BodyFace, widget.PaperBlack)
		return
	}

	shown := min(len(events), rowMaxEvents)
	for _, e := range events[:shown] {
		daygrid.DrawText(frame, x, y, timeLineFor(e, opts), daygrid.BodyFace, widget.PaperBlack)
		daygrid.DrawText(frame, titleX, y, truncate(titleFor(e, opts), maxChars), daygrid.BodyFace, widget.PaperBlack)
		y += lineH
	}

	if remaining := len(events) - shown; remaining > 0 && y+daygrid.BodyAscent() <= bounds.Max.Y {
		// The marker occupies a slot of its own rather than
		// overprinting the last event — an earlier draft drew it on
		// top of the third one.
		daygrid.DrawText(frame, x, y, fmt.Sprintf("+%d MORE", remaining), daygrid.BodyFace, widget.PaperBlack)
	}
}
