// Package weatherview provides reusable weather rendering components for
// e-ink display widgets. It renders weather icons, hourly temperature/
// precipitation charts, and condition labels using the Weather Icons font.
package weatherview

import (
	"fmt"
	"image"
	"log"
	"math"

	"github.com/grantlucas/inkwell/internal/inkwell/fonts"
	"github.com/grantlucas/inkwell/internal/inkwell/weather"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"golang.org/x/image/font"
)

var (
	labelFace  font.Face
	tempHiFace font.Face
	tempLoFace font.Face
)

func init() {
	labelFace = mustLoadFace(fonts.SemiBold, 10, "label")
	tempHiFace = mustLoadFace(fonts.Bold, 12, "temp hi")
	tempLoFace = mustLoadFace(fonts.Regular, 10, "temp lo")
}

// mustLoadFace is extracted so the per-face panic branches are
// reachable from tests via fonts.SwapDataForTest. The role label
// flows into the panic message so a failure points at which face
// failed to load.
func mustLoadFace(weight fonts.Weight, size float64, role string) font.Face {
	f, err := fonts.Face(weight, size)
	if err != nil {
		panic("weatherview: load " + role + " font: " + err.Error())
	}
	return f
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
	if err := DrawIcon(frame, iconX, iconY, iconSize, day.Condition); err != nil {
		// Log instead of bubbling — the rest of the day cell (label,
		// temps, chart) is still useful even when the glyph itself
		// can't be rendered.
		log.Printf("weatherview: draw icon for condition %d: %v", day.Condition, err)
	}

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
		// Condition label as a quiet caption above the temps so the
		// temperature reads as the primary number in the row.
		drawTextGrayWithFace(frame, textX, labelY, truncateText(label, maxLabelW/charWidth), labelFace, widget.PaperGray70)
	}

	tempStr := fmt.Sprintf("%d°%s", int(math.Round(hi)), unit)
	loStr := fmt.Sprintf("%d°", int(math.Round(lo)))
	tempY := bounds.Min.Y + condRowH - 4
	drawTextWithFace(frame, textX, tempY, tempStr, tempHiFace)
	hiW := textWidth(tempHiFace, tempStr)
	// Low temp in muted gray — visual hierarchy: high temp is the headline.
	drawTextGrayWithFace(frame, textX+hiW+3, tempY, loStr, tempLoFace, widget.PaperGray70)

	// Soft hairline dividing the condition row from the chart below. Pure
	// black would scream against the small text — Gray30 reads as a clean
	// structural rule without dominating the cell.
	drawHLine(frame, bounds.Min.X, bounds.Max.X, bounds.Min.Y+condRowH, widget.PaperGray30)

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
