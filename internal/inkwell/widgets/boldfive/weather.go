package boldfive

import (
	"fmt"
	"image"
	"log"
	"math"

	"github.com/grantlucas/inkwell/internal/inkwell/weather"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daygrid"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/weatherview"
)

const (
	iconSize = 38
	iconX    = 6
	iconY    = 2

	// The hi/lo pair is right-aligned against the cell's right edge so
	// it bookends the icon rather than clumping against it. The high is
	// scaled; the low is body size, because the pair only needs one
	// element large enough to read at distance and two would crowd the
	// chart out.
	tempPadX    = 6
	hiScale     = 2
	hiBaseline  = 30
	loBaseline  = 56
	chartTop    = 64
	chartPadX   = 8
	chartLabelH = 14
)

// drawIcon is indirected through a var so the failure branch below is
// reachable from tests — weatherview's own font seam is package-private,
// so there is no way to make the real DrawIcon fail from out here.
var drawIcon = weatherview.DrawIcon

// weatherOptions carries the per-column weather knobs that come from
// config or from which day the column is.
type weatherOptions struct {
	TempUnit string
	// ShowNowMarker is set for today's column only. On any other day
	// "now" is not a point on that day's axis, so a marker there would
	// assert something meaningless.
	ShowNowMarker bool
	NowHour       int
	// TempRange is the temperature scale shared by all five columns.
	// It turns the precipitation chart into the combined chart, so a
	// dry day still draws its temperature line rather than an empty
	// band, and a cold day sits visibly lower than a warm one.
	TempRange *weatherview.TempRange
}

// sharedTempRange is the one temperature scale every column's chart is
// drawn against. Only the days the forecast actually reached count: a
// missing day is a zero DailyForecast, and its 0°C would drag the scale
// down to a temperature nobody forecast.
func sharedTempRange(days []weather.DailyForecast) *weatherview.TempRange {
	var known []weather.DailyForecast
	for _, d := range days {
		if !d.Date.IsZero() {
			known = append(known, d)
		}
	}
	lo, hi := weatherview.GlobalTempRange(known)
	return &weatherview.TempRange{Min: lo, Max: hi}
}

// renderWeatherBand draws one column's weather: condition icon, the
// hi/lo pair, and the precipitation chart beneath them.
//
// A day the forecast never covered draws nothing at all. The zero
// DailyForecast is indistinguishable from a real reading — a clear sky
// at 0°C is entirely plausible in Hamilton in January — so drawing it
// would state a temperature nobody forecast, and an operator could not
// tell an outage from the weather. An empty band is unmistakably an
// empty band. weekly-calendar collapses its whole weather zone for the
// same reason.
func renderWeatherBand(frame *image.Paletted, bounds image.Rectangle, day weather.DailyForecast, opts weatherOptions) {
	if day.Date.IsZero() {
		return
	}

	top := bounds.Min.Y

	if err := drawIcon(frame, bounds.Min.X+iconX, top+iconY, iconSize, day.Condition); err != nil {
		// Logged rather than bubbled: the temperatures and the chart
		// are still worth drawing when the glyph itself will not load.
		log.Printf("boldfive: draw icon for condition %d: %v", day.Condition, err)
	}

	hi, lo := day.High, day.Low
	unit := "C"
	if opts.TempUnit == "F" {
		hi = weather.CelsiusToFahrenheit(hi)
		lo = weather.CelsiusToFahrenheit(lo)
		unit = "F"
	}
	rightEdge := bounds.Max.X - tempPadX
	daygrid.Scaled(daygrid.BodyBoldFace, hiScale, widget.PaperBlack).DrawRight(
		frame, rightEdge, top+hiBaseline,
		fmt.Sprintf("%d°%s", int(math.Round(hi)), unit),
	)
	daygrid.DrawTextRight(frame, rightEdge, top+loBaseline, fmt.Sprintf("%d°", int(math.Round(lo))), daygrid.BodyFace, widget.PaperBlack)

	chart := image.Rect(
		bounds.Min.X+chartPadX, top+chartTop,
		bounds.Max.X-chartPadX, bounds.Max.Y,
	)
	weatherview.RenderPrecipChart(frame, chart, day.Hourly, weatherview.PrecipChartOptions{
		// No LabelFace override: the hour labels are axis furniture,
		// not body text, and the 14 px band is sized for the chart's
		// own 12 px tier. The 20 px body face needs 21 px to sit in,
		// so passing it here drops the labels below the cell and over
		// whatever the compositor put underneath.
		LabelHeight:   chartLabelH,
		NowHour:       opts.NowHour,
		ShowNowMarker: opts.ShowNowMarker,
		// The 50% guide is off here: at 144 px wide the dashes compete
		// with the bars they are meant to measure. Only the hero cell
		// in today-hero has the room for it.
		ShowGuide: false,
		// The shared range makes this the combined chart: a dry day
		// draws its temperature line with no bars, so no column's chart
		// is blank and there is no dry-day caption to fit.
		TempRange: opts.TempRange,
	})
}
