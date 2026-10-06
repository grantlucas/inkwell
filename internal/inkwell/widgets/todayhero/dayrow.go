package todayhero

import (
	"fmt"
	"image"
	"strings"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/weather"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daygrid"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/eventlist"
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
	// agenda column.
	rowChartTop    = 64
	rowChartRight  = 176
	rowChartBottom = 4

	// Where the agenda starts. Unchanged: the events keep their room.
	rowAgendaDX = 186

	// Up to three events a row, whatever max_events says (that is the
	// hero agenda's). A 120 px row holds five lines; three events and
	// the "+N MORE" line under them leave the row room to breathe.
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
	// Agenda lists the row's events.
	Agenda eventlist.Style
}

// renderDayRow draws one following day: the date gutter with its
// condition icon and a small combined chart, then a short agenda.
//
// The chart lives in the gutter, under the date, rather than beside the
// agenda. A chart beside the agenda was tried and reverted: a 462 px
// row cannot carry a legible bar chart *and* a legible title side by
// side, and titles dropped to about 12 characters. Stacked under the
// date it costs the agenda nothing.
func renderDayRow(frame *image.Paletted, bounds image.Rectangle, day daygrid.Day, opts dayRowOptions) {
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

	renderRowWeather(frame, bounds, day.Forecast, opts.TempUnit, opts.TempRange)
	opts.Agenda.Draw(frame, rowAgenda(bounds), day.Events)
}

// renderRowWeather draws the row's hi/lo pair, condition icon and
// combined chart.
func renderRowWeather(frame *image.Paletted, bounds image.Rectangle, forecast *weather.DailyForecast, unit string, rng weatherview.TempRange) {
	if forecast == nil {
		// Nothing forecast for this day. Drawing a zero would state a
		// temperature nobody predicted.
		return
	}
	top := bounds.Min.Y
	hiLo := weatherview.NewHighLow(*forecast, unit).Pair()
	tempX := bounds.Min.X + rowChartRight - daygrid.TextWidth(daygrid.BodyFace, hiLo)
	daygrid.DrawText(frame, tempX, top+rowTempBaseline, hiLo, daygrid.BodyFace, widget.PaperBlack)

	iconX := (bounds.Min.X + rowNumeralEnd + tempX - rowIconSize) / 2
	weatherview.DrawIcon(frame, iconX, top+rowIconDY, rowIconSize, forecast.Condition)

	// No now-marker: it belongs to today's chart alone. A dry day still
	// draws its temperature line.
	weatherview.RenderCombinedChart(frame, rowChart(bounds), forecast.Hourly, rng, weatherview.CombinedOptions{})
}

// rowChart is the rect a row's combined chart takes: the lower half of
// the date gutter.
func rowChart(row image.Rectangle) image.Rectangle {
	return image.Rect(
		row.Min.X+rowPadX, row.Min.Y+rowChartTop,
		row.Min.X+rowChartRight, row.Max.Y-rowChartBottom,
	)
}

// dayRowStyle is how a row lists its day: inline, a time and a title a
// line, up to rowMaxEvents of them and then "+N MORE".
//
// loc is the zone event clock labels are rendered in. It must never be
// nil.
func dayRowStyle(showLocation bool, loc *time.Location) eventlist.Style {
	return eventlist.Style{
		Layout:       eventlist.Inline,
		MaxEvents:    rowMaxEvents,
		Empty:        emptyRowMsg,
		ShowLocation: showLocation,
		Location:     loc,
	}
}

// rowAgenda is the rectangle a row's events are listed in, right of the
// date gutter. A literal rather than image.Rect, which would swap the
// edges of a row too narrow for the gutter into a list to the left of
// it; the list draws nothing into a negative width.
func rowAgenda(row image.Rectangle) image.Rectangle {
	return image.Rectangle{
		Min: image.Pt(row.Min.X+rowAgendaDX, row.Min.Y+rowPadX),
		Max: image.Pt(row.Max.X-rowPadX, row.Max.Y),
	}
}
