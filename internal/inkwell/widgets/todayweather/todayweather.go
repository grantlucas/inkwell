// Package todayweather is the today-weather widget: today's forecast as a
// summary, with a large condition icon, the high as the headline number,
// the low beside it and the condition name beneath. It draws no events and
// no chart, so it can sit anywhere on a screen with its own refresh
// cadence, separate from weather-ahead.
package todayweather

import (
	"image"
	"strings"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/fonts"
	"github.com/grantlucas/inkwell/internal/inkwell/weather"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daydata"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/drawkit"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/weatherview"
)

var _ widget.Widget = (*Widget)(nil)

// The block is today-hero's weather block at the same sizes, so the two
// screens read alike: a 58 px icon, the high at 3x and the low and the
// condition at body size.
const (
	// padX is weather-ahead's, so stacked in one column, as on the
	// day-timeline screen, today's icon starts where the rows below do.
	padX     = 6
	padY     = 8
	iconSize = 58
	// iconGap is wider than today-hero's: some glyphs (partly cloudy's
	// sun) run past their 58 px box, and here the high starts right
	// after it.
	iconGap = 18
	hiScale = 3
	loGap   = 12

	// hiBaseline is the high's baseline below the block's top: the 3x
	// cap height plus the stroke the scale's dilation adds above it.
	hiBaseline = 44
	// condGap is from the high's baseline to the condition's, a body
	// line with room for the high's dilated foot.
	condGap = 32
	// wrappedLoGap is from the high's baseline to the low's when the low
	// has to take its own line; the condition is then a body line under
	// the low.
	wrappedLoGap = 28

	// widestHigh is the widest high there is, in either unit.
	widestHigh = "-00°C"

	// noForecast is what the widget says when no forecast reaches today.
	noForecast = "NO FORECAST"
)

var (
	// minWidth holds the block with the widest high there is, so no
	// forecast can push text past the widget's edge.
	minWidth = 2*padX + iconSize + iconGap + highDrawer().Measure(widestHigh)
	// minHeight holds the taller, wrapped block.
	minHeight = 2*padY + hiBaseline + wrappedLoGap + drawkit.BodyLineH()
)

// highDrawer draws the high, the headline number.
func highDrawer() fonts.ScaledDrawer {
	return drawkit.Scaled(drawkit.BodyBoldFace, hiScale, widget.PaperBlack)
}

// Widget renders today's weather.
type Widget struct {
	daydata.Base[daydata.Config]
}

// New creates a today-weather Widget drawing today from days. Of cfg it
// reads only the temperature unit.
func New(bounds image.Rectangle, days daydata.Source, now func() time.Time, cfg daydata.Config) *Widget {
	return &Widget{daydata.NewBase(bounds, days, now, cfg)}
}

// Render draws today's icon, high, low and condition, centred down the
// widget's bounds. With no forecast for today it says so rather than
// drawing numbers nobody forecast. A failed fetch is logged by the day
// data module and never returned: the compositor drops the whole frame
// on a render error, which would blank every other widget too.
func (w *Widget) Render(frame *image.Paletted) error {
	b := w.Bounds()
	drawkit.FillWhite(frame, b)

	if !daydata.Fits(widgetName, b, image.Pt(minWidth, minHeight), "") {
		return nil
	}

	day := w.Days.Days(w.Now(), 1).Days[0].Forecast
	if day == nil {
		drawkit.DrawTextCentered(frame, b.Min.X, b.Max.X, b.Min.Y+(b.Dy()+drawkit.BodyAscent())/2,
			noForecast, drawkit.BodyFace, widget.PaperBlack)
		return nil
	}
	renderForecast(frame, b, *day, w.Config.Weather.TempUnit)
	return nil
}

// renderForecast draws day's block, left-aligned and centred down b. The
// low sits beside the high when it fits and takes its own line under it
// when it doesn't, as in deep cold, where both are widest.
func renderForecast(frame *image.Paletted, b image.Rectangle, day weather.DailyForecast, unit string) {
	temps := weatherview.NewHighLow(day, unit)
	hi := highDrawer()
	x := b.Min.X + padX
	textX := x + iconSize + iconGap
	loX := textX + hi.Measure(temps.High()) + loGap
	loBaseline := hiBaseline
	if loX+drawkit.TextWidth(drawkit.BodyFace, temps.Low()) > b.Max.X-padX {
		loX, loBaseline = textX, hiBaseline+wrappedLoGap
	}
	condBaseline := hiBaseline + condGap
	if loBaseline != hiBaseline {
		condBaseline = loBaseline + drawkit.BodyLineH()
	}

	// The condition label is upper case, so the block ends at its
	// baseline.
	top := b.Min.Y + (b.Dy()-condBaseline)/2
	weatherview.DrawIcon(frame, x, top+(condBaseline-iconSize)/2, iconSize, day.Condition)
	hi.Draw(frame, textX, top+hiBaseline, temps.High())
	drawkit.DrawText(frame, loX, top+loBaseline, temps.Low(), drawkit.BodyFace, widget.PaperBlack)
	drawkit.DrawText(frame, textX, top+condBaseline,
		strings.ToUpper(day.Condition.Label()), drawkit.BodyFace, widget.PaperBlack)
}

// Factory creates a today-weather Widget from config and dependencies.
var Factory = daydata.Factory(widgetName, daydata.Parser(spec), New)
