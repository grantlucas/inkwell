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
	// Lo temp is SemiBold (not Regular) for the same reason monthFace
	// in day_header.go is — Terminus's Regular at 10 pt produces glyphs
	// with 1-px stems and detached features that fragment on-device
	// after the BW threshold. SemiBold's 2-px stems are robust. The
	// hi/lo hierarchy is still readable: Bold 12 vs. SemiBold 10.
	tempLoFace = mustLoadFace(fonts.SemiBold, 10, "temp lo")
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
// hourly chart into the given bounds. The condition row puts the icon
// flush-left and right-aligns the label and temps to the cell's right edge so
// the two groups bookend the available width:
//
//	┌─────────────────┐
//	│ [icon]    LABEL  │  condition row
//	│         17°C  9° │  hi/lo temps
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

	// textX is the left bound the right-aligned condition text may not cross
	// (it keeps clear of the icon). rightEdge mirrors the icon's 4px left
	// margin so the label/temps hug the cell's right edge and bookend the
	// icon instead of clumping against it.
	textX := iconX + iconSize + 3
	rightEdge := bounds.Max.X - 4
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
		// Truncate to the gap between the icon and the right edge, then
		// right-align the (possibly truncated) label to that edge.
		label := truncateText(day.Condition.Label(), (rightEdge-textX)/charWidth)
		// Condition label renders in solid PaperBlack; the BW threshold
		// chops AA fringe off any gray source color, so a "muted" label
		// in PaperGray70 ends up fragmented. Hierarchy comes from font
		// weight (semi-bold label vs. regular temps) and position.
		labelX := max(textX, rightEdge-textWidth(labelFace, label))
		drawTextWithFace(frame, labelX, labelY, label, labelFace)
	}

	tempStr := fmt.Sprintf("%d°%s", int(math.Round(hi)), unit)
	loStr := fmt.Sprintf("%d°", int(math.Round(lo)))
	tempY := bounds.Min.Y + condRowH - 4
	hiW := textWidth(tempHiFace, tempStr)
	loW := textWidth(tempLoFace, loStr)
	// Right-align the hi/lo pair as one group, clamped so it never overruns
	// the icon on a very narrow cell.
	groupX := max(textX, rightEdge-(hiW+3+loW))
	drawTextWithFace(frame, groupX, tempY, tempStr, tempHiFace)
	// Low temp also in PaperBlack — see the label note above. Hierarchy
	// between hi/lo is carried by font weight (bold hi vs. regular lo).
	drawTextWithFace(frame, groupX+hiW+3, tempY, loStr, tempLoFace)

	// Condition-row divider: PaperBlack so it survives the BW threshold
	// and the Gray4 quantization. The old PaperGray30 (Y=0xB3) only read
	// as a soft hairline thanks to the Bayer dither — without it the rule
	// either vanished (light bucket on Gray4) or disappeared entirely
	// (above 128 on the BW threshold).
	drawHLine(frame, bounds.Min.X, bounds.Max.X, bounds.Min.Y+condRowH, widget.PaperBlack)

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
