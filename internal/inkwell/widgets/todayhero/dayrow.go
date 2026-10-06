package todayhero

import (
	"image"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/weather"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daybadge"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daydata"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/eventlist"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/weatherview"
)

const (
	rowPadX = 12

	// rowChartBottom is the paper under a row's chart, above the rule
	// along the row's last pixel.
	rowChartBottom = 4

	// Where the agenda starts. Unchanged: the events keep their room.
	rowAgendaDX = 186

	// Up to three events a row, whatever max_events says (that is the
	// hero agenda's). A 120 px row holds five lines; three events and
	// the "+N MORE" line under them leave the row room to breathe.
	rowMaxEvents = 3
)

// dayRowOptions carries what a row needs beyond its events.
type dayRowOptions struct {
	// Now is the dashboard's clock, by which the badge tags tomorrow.
	Now      time.Time
	TempUnit string
	// TempRange is the screen's shared temperature scale, the same one
	// today's chart plots against.
	TempRange weatherview.TempRange
	// Agenda lists the row's events.
	Agenda eventlist.Style
}

// renderDayRow draws one following day: the day badge (tag, numeral,
// condition icon and high and low) with a small combined chart under it
// in the date gutter, then a short agenda.
//
// The chart lives in the gutter, under the date, rather than beside the
// agenda. A chart beside the agenda was tried and reverted: a 462 px
// row cannot carry a legible bar chart *and* a legible title side by
// side, and titles dropped to about 12 characters. Stacked under the
// date it costs the agenda nothing.
func renderDayRow(frame *image.Paletted, bounds image.Rectangle, day daydata.Day, opts dayRowOptions) {
	daybadge.Compact.Draw(frame, rowBadge(bounds), day, opts.Now, opts.TempUnit)
	renderRowChart(frame, rowChart(bounds), day.Forecast, opts.TempRange)
	opts.Agenda.Draw(frame, rowAgenda(bounds), day.Events)
}

// renderRowChart draws a row's combined chart. No now-marker: it belongs
// to today's chart alone. A dry day still draws its temperature line; a
// day the forecast doesn't reach draws nothing, since a line would state
// a temperature nobody predicted.
func renderRowChart(frame *image.Paletted, bounds image.Rectangle, forecast *weather.DailyForecast, rng weatherview.TempRange) {
	if forecast == nil {
		return
	}
	weatherview.RenderCombinedChart(frame, bounds, forecast.Hourly, rng, weatherview.CombinedOptions{})
}

// rowBadge is the rect a row's day badge takes: the top of the date
// gutter.
func rowBadge(row image.Rectangle) image.Rectangle {
	return image.Rectangle{Min: row.Min, Max: row.Min.Add(daybadge.Compact.Size())}
}

// rowChart is the rect a row's combined chart takes: the lower half of
// the date gutter, under the badge and ending where the badge's high
// and low end.
func rowChart(row image.Rectangle) image.Rectangle {
	badge := rowBadge(row)
	return image.Rect(row.Min.X+rowPadX, badge.Max.Y, badge.Max.X, row.Max.Y-rowChartBottom)
}

// dayRowStyle is how a row lists its day: the event list's inline preset,
// a time and a title a line, up to rowMaxEvents of them and then
// "+N MORE".
//
// loc is the zone event clock labels are rendered in. It must never be
// nil.
func dayRowStyle(showLocation bool, loc *time.Location) eventlist.Style {
	return eventlist.PresetInline.Style(rowMaxEvents, showLocation, loc)
}

// rowAgenda is the rectangle a row's events are listed in: right of the
// date gutter, and ending above the rule Render draws along the row's
// last pixel, so a line that just fits can never touch it. A literal
// rather than image.Rect, which would swap the edges of a row too
// narrow for the gutter into a list to the left of it; the list draws
// nothing into a negative width.
func rowAgenda(row image.Rectangle) image.Rectangle {
	return image.Rectangle{
		Min: image.Pt(row.Min.X+rowAgendaDX, row.Min.Y+rowPadX),
		Max: image.Pt(row.Max.X-rowPadX, row.Max.Y-1),
	}
}
