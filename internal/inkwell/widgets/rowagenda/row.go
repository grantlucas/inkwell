package rowagenda

import (
	"image"

	"github.com/grantlucas/inkwell/internal/inkwell/weather"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/weatherview"
)

// renderChart draws a row's combined chart beside its day badge:
// precipitation bars with the temperature line over them, plotted on
// rng, the range shared by all five rows, with the now marker on today's
// row only.
//
// The chart is on every row whether or not rain is due, so a dry day
// still shows the shape of its temperature rather than a blank cell,
// and a cold day visibly sits lower than a warm one. A day the forecast
// doesn't reach draws none: a line would state a forecast nobody made.
func renderChart(frame *image.Paletted, bounds image.Rectangle, forecast *weather.DailyForecast, isToday bool, nowHour int, rng weatherview.TempRange) {
	if forecast == nil {
		return
	}
	weatherview.RenderCombinedChart(frame, bounds, forecast.Hourly, rng, weatherview.CombinedOptions{
		NowHour:       nowHour,
		ShowNowMarker: isToday,
	})
}
