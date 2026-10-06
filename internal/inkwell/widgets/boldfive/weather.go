package boldfive

import (
	"image"

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
)

// weatherOptions carries the per-column weather knobs that come from
// config or from which day the column is.
type weatherOptions struct {
	TempUnit string
	// ShowNowMarker is set for today's column only. On any other day
	// "now" is not a point on that day's axis, so a marker there would
	// assert something meaningless.
	ShowNowMarker bool
	NowHour       int
	// TempRange is the temperature scale shared by all five columns,
	// so a cold day sits visibly lower than a warm one.
	TempRange weatherview.TempRange
}

// renderWeatherBand draws one column's weather: condition icon, the
// hi/lo pair, and the precipitation chart beneath them.
//
// A day the forecast never covered (a nil day) draws nothing at all.
// Drawing anything would state a temperature nobody forecast, and an
// operator could not tell an outage from the weather. An empty band is
// unmistakably an empty band. weekly-calendar collapses its whole
// weather zone for the same reason.
func renderWeatherBand(frame *image.Paletted, bounds image.Rectangle, day *weather.DailyForecast, opts weatherOptions) {
	if day == nil {
		return
	}

	top := bounds.Min.Y

	weatherview.DrawIcon(frame, bounds.Min.X+iconX, top+iconY, iconSize, day.Condition)

	temps := weatherview.NewHighLow(*day, opts.TempUnit)
	rightEdge := bounds.Max.X - tempPadX
	daygrid.Scaled(daygrid.BodyBoldFace, hiScale, widget.PaperBlack).DrawRight(
		frame, rightEdge, top+hiBaseline, temps.High(),
	)
	daygrid.DrawTextRight(frame, rightEdge, top+loBaseline, temps.Low(), daygrid.BodyFace, widget.PaperBlack)

	chart := image.Rect(
		bounds.Min.X+chartPadX, top+chartTop,
		bounds.Max.X-chartPadX, bounds.Max.Y,
	)
	// A dry day draws its temperature line with no bars, so no
	// column's chart is blank and there is no dry-day caption to fit.
	weatherview.RenderCombinedChart(frame, chart, day.Hourly, opts.TempRange, weatherview.CombinedOptions{
		NowHour:       opts.NowHour,
		ShowNowMarker: opts.ShowNowMarker,
	})
}
