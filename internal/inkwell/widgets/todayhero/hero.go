package todayhero

import (
	"image"

	"github.com/grantlucas/inkwell/internal/inkwell/weather"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/weatherview"
)

// heroPadX insets the hero agenda and its rule from the column's edges,
// in line with the day badge's text above them.
const heroPadX = 14

// renderHeroChart draws today's combined chart: precipitation bars with
// the temperature line over them. This is the widest chart cell of any
// screen and the main reason to pick this one: 15 px bars with a marker
// at the current hour, so an afternoon band reads as a band rather than
// as a texture.
//
// rng is the screen's shared range, so today's line sits at the same
// height as a row's line for the same temperature. A dry day still
// draws the line, so the chart is never blank.
func renderHeroChart(frame *image.Paletted, bounds image.Rectangle, day *weather.DailyForecast, nowHour int, rng weatherview.TempRange) {
	if day == nil {
		return
	}
	weatherview.RenderCombinedChart(frame, bounds, day.Hourly, rng, weatherview.CombinedOptions{
		NowHour:       nowHour,
		ShowNowMarker: true,
	})
}
