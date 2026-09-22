package boldfive

import (
	"fmt"
	"image"
	"log"
	"math"

	"github.com/grantlucas/inkwell/internal/inkwell/weather"
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
}

// renderWeatherBand draws one column's weather: condition icon, the
// hi/lo pair, and the precipitation chart beneath them.
func renderWeatherBand(frame *image.Paletted, bounds image.Rectangle, day weather.DailyForecast, opts weatherOptions) {
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
	scaled(bodyBoldFace, hiScale).DrawRight(
		frame, rightEdge, top+hiBaseline,
		fmt.Sprintf("%d°%s", int(math.Round(hi)), unit),
	)
	drawTextRight(frame, rightEdge, top+loBaseline, fmt.Sprintf("%d°", int(math.Round(lo))), bodyFace)

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
		// No dry-day text either — a 144 px cell cannot carry a legible
		// phrase, and an empty cell already reads as a dry day.
		DryText: "",
	})
}
