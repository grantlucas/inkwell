// Package weatherahead is the weather-ahead widget: the days after today
// as rows of weather only, each with its weekday and date, condition
// icon and name, high and low, and a combined chart. Every chart plots
// against one temperature range taken across the widget's own rows, so a
// cold day sits visibly lower than a warm one.
//
// Today is today-weather's. The two are separate widgets so each can be
// placed, scheduled and reused on its own; on the day-timeline screen
// this one sits under today-weather down the right-hand third.
package weatherahead

import (
	"image"
	"log"
	"strings"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/weather"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daygrid"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/weatherview"
)

var _ widget.Widget = (*Widget)(nil)

// A row follows the day-timeline design: the weekday and date in bold
// across the top ("WEDNESDAY 18"), the condition icon under it with the
// condition word beside the icon and the high and low under the word,
// and the combined chart filling the rest of the row to the right. The
// chart takes the row's whole height rather than only the part under the
// title, so its bars get every pixel a short row has.
const (
	padX = 6
	padY = 4

	// iconSize matches today-hero's day rows, the smallest size the icon
	// font is drawn at anywhere. It spans the two lines beside it.
	iconSize = 30
	// iconGap is the paper between the icon and the text beside it,
	// loGap between the high and the low, and chartGap between the text
	// and the chart. iconGap is wide because some glyphs (partly
	// cloudy's sun) run past their box.
	iconGap  = 12
	loGap    = 8
	chartGap = 10

	// The widest title, condition word and temperatures there are, in
	// either unit, so the chart starts in the same place every day.
	widestTitle     = "WEDNESDAY 30"
	widestCondition = "P.CLOUDY"
	widestHigh      = "-00°C"
	widestLow       = "-00°"

	// noForecast is what a row says when the forecast doesn't reach it.
	noForecast = "NO FORECAST"
)

var (
	// The text block's three baselines below its top, a body line
	// apart: the title, the condition word and the temperatures. The
	// block ends at the temperatures' baseline: they are digits, with
	// nothing below it.
	titleBaseline = daygrid.BodyAscent()
	condBaseline  = titleBaseline + daygrid.BodyLineH()
	tempBaseline  = condBaseline + daygrid.BodyLineH()

	// textX is where the condition word and the temperatures start,
	// right of the icon, as an offset from the row's left edge.
	textX = padX + iconSize + iconGap
	// chartDX is where a row's chart starts: clear of the widest title
	// and the widest text beside the icon.
	chartDX = max(
		padX+daygrid.TextWidth(daygrid.BodyBoldFace, widestTitle),
		textX+daygrid.TextWidth(daygrid.BodyFace, widestCondition),
		textX+daygrid.TextWidth(daygrid.BodyBoldFace, widestHigh)+loGap+daygrid.TextWidth(daygrid.BodyFace, widestLow),
	) + chartGap

	// minChartW keeps a chart wide enough to tell the hours apart.
	minChartW = 60
	// minWidth and minRowH are the smallest a row can be and still hold
	// its text and a chart.
	minWidth = chartDX + minChartW + padX
	minRowH  = tempBaseline + 2*padY
)

// Widget renders the weather for the days after today.
type Widget struct {
	daygrid.Base
	// Config is the widget's parsed settings: the shared ones Base holds
	// as well, and its own.
	Config Config
}

// New creates a weather-ahead Widget listing cfg.Days days after today
// from days.
func New(bounds image.Rectangle, days daygrid.Source, now func() time.Time, cfg Config) *Widget {
	return &Widget{Base: daygrid.NewBase(bounds, days, now, cfg.Config), Config: cfg}
}

// Render draws one row per day after today, splitting the bounds' height
// evenly between them, with a rule between rows. A day the forecast
// doesn't reach says so rather than drawing numbers nobody forecast. A
// failed fetch is logged by the day data module and never returned: the
// compositor drops the whole frame on a render error, which would blank
// every other widget too.
func (w *Widget) Render(frame *image.Paletted) error {
	b := w.Bounds()
	daygrid.FillWhite(frame, b)

	n := w.Config.Days
	rowH := b.Dy() / n
	// The draw helpers clip to the frame, not to the widget, so a widget
	// too small for its rows would ink its neighbours.
	if b.Dx() < minWidth || rowH < minRowH {
		log.Printf("%s: bounds are %dx%d, need at least %dx%d for %d days — drawing nothing",
			widgetName, b.Dx(), b.Dy(), minWidth, n*minRowH, n)
		return nil
	}

	// Today is the first day asked for and today-weather's to show; the
	// rows are the days after it.
	rows := w.Days.Days(w.Now(), 1+n).Days[1:]
	rng := sharedRange(rows)
	unit := w.Config.Weather.TempUnit
	for i, d := range rows {
		r := image.Rect(b.Min.X, b.Min.Y+i*rowH, b.Max.X, b.Min.Y+(i+1)*rowH)
		if i == n-1 {
			// Any remainder goes to the last row, so the rows above keep
			// a common height and line up.
			r.Max.Y = b.Max.Y
		} else {
			daygrid.DrawHLine(frame, r.Min.X, r.Max.X, r.Max.Y-1, widget.PaperBlack)
		}
		renderRow(frame, r, d, unit, rng)
	}
	return nil
}

// sharedRange is the temperature range across the rows that have a
// forecast. It leaves today out: today is another widget's, and its
// range would squash these charts for a day this widget doesn't show.
func sharedRange(rows []daygrid.Day) weatherview.TempRange {
	var known []weather.DailyForecast
	for _, d := range rows {
		if d.Forecast != nil {
			known = append(known, *d.Forecast)
		}
	}
	return weatherview.GlobalTempRange(known)
}

// renderRow draws one day into r: its title, then its weather, with the
// text block centred down the row.
func renderRow(frame *image.Paletted, r image.Rectangle, d daygrid.Day, unit string, rng weatherview.TempRange) {
	x := r.Min.X + padX
	top := r.Min.Y + (r.Dy()-tempBaseline)/2
	daygrid.DrawText(frame, x, top+titleBaseline,
		strings.ToUpper(d.Start.Format("Monday 2")), daygrid.BodyBoldFace, widget.PaperBlack)

	if d.Forecast == nil {
		daygrid.DrawText(frame, x, top+condBaseline, noForecast, daygrid.BodyFace, widget.PaperBlack)
		return
	}
	f := *d.Forecast

	// The icon is centred on the two lines beside it: from the condition
	// word's cap top to the temperatures' baseline.
	linesTop := top + condBaseline - daygrid.BodyAscent()
	linesH := tempBaseline - condBaseline + daygrid.BodyAscent()
	weatherview.DrawIcon(frame, x, linesTop+(linesH-iconSize)/2, iconSize, f.Condition)
	daygrid.DrawText(frame, r.Min.X+textX, top+condBaseline,
		strings.ToUpper(f.Condition.Label()), daygrid.BodyFace, widget.PaperBlack)

	temps := weatherview.NewHighLow(f, unit)
	daygrid.DrawText(frame, r.Min.X+textX, top+tempBaseline, temps.High(), daygrid.BodyBoldFace, widget.PaperBlack)
	loX := r.Min.X + textX + daygrid.TextWidth(daygrid.BodyBoldFace, temps.High()) + loGap
	daygrid.DrawText(frame, loX, top+tempBaseline, temps.Low(), daygrid.BodyFace, widget.PaperBlack)

	// No now-marker: "now" is not on any of these days.
	chart := image.Rect(r.Min.X+chartDX, r.Min.Y+padY, r.Max.X-padX, r.Max.Y-padY)
	weatherview.RenderCombinedChart(frame, chart, f.Hourly, rng, weatherview.CombinedOptions{})
}
