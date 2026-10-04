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
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/weatherview"
)

const (
	rowPadX = 12

	// The date gutter: the tag, then the numeral with the condition
	// icon and the hi/lo beside it, then the day's combined chart. The
	// numeral stays at 2x; the hi/lo is body size, on the numeral's
	// centre line, and ends where the chart ends.
	tagBaseline     = 20
	rowDateScale    = 2
	rowDateBaseline = 56
	rowTempBaseline = 48

	// The condition icon is centred in the gap between the widest
	// numeral and the hi/lo. 30 px rather than the old 40, so the three
	// share one line and leave the lower half of the gutter to the
	// chart. The glyphs' rays overrun their box by a few pixels, which
	// is why the icon sits clear of the tag above it.
	rowIconSize   = 30
	rowIconDY     = 30
	rowNumeralEnd = 54

	// The chart fills the gutter under that line and stops short of the
	// agenda column. The 14 px label band is bold-five's and
	// row-agenda's, sized for the chart's own 12 px tier.
	rowChartTop    = 64
	rowChartRight  = 176
	rowChartBottom = 4
	rowChartLabelH = 14

	// Where the agenda starts. Unchanged: the events keep their room.
	rowAgendaDX = 186

	// Up to three events a row; a fourth would leave no room for the
	// overflow marker to say how many were dropped.
	rowMaxEvents = 3

	// Upper case, like every other label this screen paints —
	// TOMORROW, ALL DAY, DONE FOR TODAY. Mixed case in the rows alone
	// would read as a second typographic system.
	emptyRowMsg = "NOTHING SCHEDULED"
)

// dayRowOptions carries what a row needs beyond its events.
type dayRowOptions struct {
	// IsTomorrow tags the row "TOMORROW" instead of its weekday.
	IsTomorrow bool
	TempUnit   string
	// TempRange is the screen's shared temperature scale, the same one
	// today's chart plots against.
	TempRange weatherview.TempRange
	Events    eventOptions
}

// renderDayRow draws one following day: the date gutter with its
// condition icon and a small combined chart, then a short agenda.
//
// The chart lives in the gutter, under the date, rather than beside the
// agenda. A chart beside the agenda was tried and reverted: a 462 px
// row cannot carry a legible bar chart *and* a legible title side by
// side, and titles dropped to about 12 characters. Stacked under the
// date it costs the agenda nothing.
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

	renderRowWeather(frame, bounds, forecast, opts.TempUnit, opts.TempRange)
	renderRowAgenda(frame, bounds, events, opts.Events)
}

// renderRowWeather draws the row's hi/lo pair, condition icon and
// combined chart.
func renderRowWeather(frame *image.Paletted, bounds image.Rectangle, forecast weather.DailyForecast, unit string, rng weatherview.TempRange) {
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
	hiLo := fmt.Sprintf("%d° %d°", int(math.Round(hi)), int(math.Round(lo)))
	tempX := bounds.Min.X + rowChartRight - daygrid.TextWidth(daygrid.BodyFace, hiLo)
	daygrid.DrawText(frame, tempX, top+rowTempBaseline, hiLo, daygrid.BodyFace, widget.PaperBlack)

	iconX := (bounds.Min.X + rowNumeralEnd + tempX - rowIconSize) / 2
	if err := drawIcon(frame, iconX, top+rowIconDY, rowIconSize, forecast.Condition); err != nil {
		log.Printf("todayhero: draw row icon for condition %d: %v", forecast.Condition, err)
	}

	// No now-marker, guide or dry caption: the marker belongs to today's
	// chart alone, and at 164 px the guide's dashes would compete with
	// the bars. A dry day still draws its temperature line.
	weatherview.RenderPrecipChart(frame, rowChart(bounds), forecast.Hourly, weatherview.PrecipChartOptions{
		LabelHeight: rowChartLabelH,
		TempRange:   &rng,
	})
}

// rowChart is the rect a row's combined chart takes: the lower half of
// the date gutter.
func rowChart(row image.Rectangle) image.Rectangle {
	return image.Rect(
		row.Min.X+rowPadX, row.Min.Y+rowChartTop,
		row.Min.X+rowChartRight, row.Max.Y-rowChartBottom,
	)
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
