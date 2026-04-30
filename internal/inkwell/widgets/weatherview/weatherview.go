// Package weatherview provides reusable weather rendering components for
// e-ink display widgets. It renders weather icons, hourly temperature/
// precipitation charts, and condition labels using the Weather Icons font.
package weatherview

import (
	"fmt"
	"image"
	"math"

	"github.com/grantlucas/inkwell/internal/inkwell/fonts"
	"github.com/grantlucas/inkwell/internal/inkwell/weather"
	"golang.org/x/image/font"
)

var (
	labelFace   font.Face
	tempHiFace  font.Face
	tempLoFace  font.Face
)

func init() {
	var err error
	labelFace, err = fonts.Face(fonts.SemiBold, 10)
	if err != nil {
		panic("weatherview: load label font: " + err.Error())
	}
	tempHiFace, err = fonts.Face(fonts.Bold, 12)
	if err != nil {
		panic("weatherview: load temp hi font: " + err.Error())
	}
	tempLoFace, err = fonts.Face(fonts.Regular, 10)
	if err != nil {
		panic("weatherview: load temp lo font: " + err.Error())
	}
}

// Options controls how day weather is rendered.
type Options struct {
	TempUnit      string
	ShowLabel     bool
	GlobalTempMin float64
	GlobalTempMax float64
	HighlightHour int
	IsToday       bool
	IconSize      int
}

// RenderDayWeather draws a weather summary (icon + label + temps) and
// hourly chart into the given bounds. The layout is:
//
//	┌─────────────────┐
//	│ [icon] LABEL     │  condition row
//	│        17°C  9°C │  hi/lo temps
//	├─────────────────┤
//	│ ~~temp curve~~   │  hourly chart
//	│ ▐ ▐▐▐▐▐▐ ▐      │  precip bars
//	│ 6A 9A 12 3P 8P  │  time labels
//	└─────────────────┘
func RenderDayWeather(frame *image.Paletted, bounds image.Rectangle, day weather.DailyForecast, opts Options) {
	w := bounds.Dx()
	h := bounds.Dy()
	if w < 20 || h < 20 {
		return
	}

	iconSize := opts.IconSize
	if iconSize == 0 {
		iconSize = min(24, w/3)
	}

	condRowH := max(iconSize+4, 2*lineHeight+4)

	iconX := bounds.Min.X + 4
	iconY := bounds.Min.Y + (condRowH-iconSize)/2
	_ = DrawIcon(frame, iconX, iconY, iconSize, day.Condition)

	textX := iconX + iconSize + 3
	hi := day.High
	lo := day.Low
	unit := "C"
	if opts.TempUnit == "F" {
		hi = weather.CelsiusToFahrenheit(hi)
		lo = weather.CelsiusToFahrenheit(lo)
		unit = "F"
	}

	if opts.ShowLabel {
		labelY := bounds.Min.Y + labelFace.Metrics().Ascent.Ceil() + 4
		label := day.Condition.Label()
		maxLabelW := w - textX + bounds.Min.X
		drawTextWithFace(frame, textX, labelY, truncateText(label, maxLabelW/charWidth), labelFace)
	}

	tempStr := fmt.Sprintf("%d°%s", int(math.Round(hi)), unit)
	loStr := fmt.Sprintf("%d°", int(math.Round(lo)))
	tempY := bounds.Min.Y + condRowH - 4
	drawTextWithFace(frame, textX, tempY, tempStr, tempHiFace)
	hiW := textWidth(tempHiFace, tempStr)
	drawTextWithFace(frame, textX+hiW+3, tempY, loStr, tempLoFace)

	drawHLine(frame, bounds.Min.X, bounds.Max.X, bounds.Min.Y+condRowH)

	chartBounds := image.Rect(
		bounds.Min.X+2,
		bounds.Min.Y+condRowH+2,
		bounds.Max.X-2,
		bounds.Max.Y,
	)
	if chartBounds.Dy() > 20 {
		chartOpts := ChartOptions{
			TempUnit:      opts.TempUnit,
			GlobalTempMin: opts.GlobalTempMin,
			GlobalTempMax: opts.GlobalTempMax,
			HighlightHour: opts.HighlightHour,
			IsToday:       opts.IsToday,
		}
		RenderHourlyChart(frame, chartBounds, day.Hourly, chartOpts)
	}
}

// GlobalTempRange computes the min and max temperatures across all days'
// hourly data, for use in chart normalization.
func GlobalTempRange(days []weather.DailyForecast) (minTemp, maxTemp float64) {
	minTemp = math.Inf(1)
	maxTemp = math.Inf(-1)
	for _, day := range days {
		for _, hp := range day.Hourly {
			if hp.Temperature < minTemp {
				minTemp = hp.Temperature
			}
			if hp.Temperature > maxTemp {
				maxTemp = hp.Temperature
			}
		}
		if day.Low < minTemp {
			minTemp = day.Low
		}
		if day.High > maxTemp {
			maxTemp = day.High
		}
	}
	if math.IsInf(minTemp, 1) {
		minTemp = 0
	}
	if math.IsInf(maxTemp, -1) {
		maxTemp = 25
	}
	return minTemp, maxTemp
}
