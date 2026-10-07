package boldfive

import (
	"image"

	"github.com/grantlucas/inkwell/internal/inkwell/weather"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/weatherview"
)

// chartOptions carries the per-column chart knobs that come from which
// day the column is.
type chartOptions struct {
	// ShowNowMarker is set for today's column only. On any other day
	// "now" is not a point on that day's axis, so a marker there would
	// assert something meaningless.
	ShowNowMarker bool
	NowHour       int
	// TempRange is the temperature scale shared by all five columns,
	// so a cold day sits visibly lower than a warm one.
	TempRange weatherview.TempRange
}

// renderChart draws one column's combined chart under its day badge.
//
// A day the forecast never covered (a nil day) draws nothing at all.
// Drawing anything would state a temperature nobody forecast, and an
// operator could not tell an outage from the weather. An empty band is
// unmistakably an empty band. A dry day draws its temperature line with
// no bars, so no column's chart is blank and there is no dry-day caption
// to fit.
func renderChart(frame *image.Paletted, bounds image.Rectangle, day *weather.DailyForecast, opts chartOptions) {
	if day == nil {
		return
	}
	weatherview.RenderCombinedChart(frame, bounds, day.Hourly, opts.TempRange, weatherview.CombinedOptions{
		NowHour:       opts.NowHour,
		ShowNowMarker: opts.ShowNowMarker,
	})
}
