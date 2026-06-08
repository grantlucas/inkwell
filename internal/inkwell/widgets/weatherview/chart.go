package weatherview

import (
	"image"
	"math"

	"github.com/grantlucas/inkwell/internal/inkwell/weather"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
)

// ChartOptions controls hourly chart rendering.
type ChartOptions struct {
	TempUnit      string
	GlobalTempMin float64
	GlobalTempMax float64
	HighlightHour int
	IsToday       bool
}

const (
	chartStartHour = 6
	chartEndHour   = 20
	chartHours     = chartEndHour - chartStartHour + 1
)

var labelHours = map[int]string{
	6: "6", 9: "9", 12: "12", 15: "3", 20: "8",
}

// RenderHourlyChart draws a temperature polyline and precipitation bars
// for hours 6–20 into the given bounds.
func RenderHourlyChart(frame *image.Paletted, bounds image.Rectangle, hourly []weather.HourlyPoint, opts ChartOptions) {
	w := bounds.Dx()
	h := bounds.Dy()
	if w < 10 || h < 10 || len(hourly) == 0 {
		return
	}

	filtered := filterHours(hourly, chartStartHour, chartEndHour)
	if len(filtered) == 0 {
		return
	}

	labelH := lineHeight
	tempH := int(float64(h-labelH) * 0.45)
	sepH := 3
	barMaxH := max(h-tempH-sepH-labelH, 4)

	step := float64(w) / float64(chartHours)
	barW := max(int(step)-2, 2)

	tMin := opts.GlobalTempMin
	tMax := opts.GlobalTempMax
	if opts.TempUnit == "F" {
		tMin = weather.CelsiusToFahrenheit(tMin)
		tMax = weather.CelsiusToFahrenheit(tMax)
	}
	if tMax <= tMin {
		tMax = tMin + 1
	}

	tempY := func(temp float64) int {
		if opts.TempUnit == "F" {
			temp = weather.CelsiusToFahrenheit(temp)
		}
		norm := (temp - tMin) / (tMax - tMin)
		norm = max(0, min(1, norm))
		return bounds.Min.Y + tempH - 2 - int(norm*float64(tempH-4))
	}

	barTop := bounds.Min.Y + tempH + sepH

	// Axis line between the temperature curve and the precipitation
	// bars. Solid PaperBlack so it reads as a single crisp rule on both
	// the BW threshold path and Gray4.
	drawHLine(frame, bounds.Min.X, bounds.Max.X, barTop-1, widget.PaperBlack)

	// "Now" indicator: a 2-px solid PaperBlack vertical stroke at the
	// current hour. The previous soft-gray fill vanished on hardware —
	// PaperGray20 collapses to white in Gray4's light bucket and BW now
	// thresholds without dither, so any subtle fill disappears. A solid
	// stroke is the only treatment that survives unchanged across all
	// three render paths (source, gray4, BW). Drawn before the data so
	// the temp polyline and bars overlay it; the user still sees one
	// clear vertical marker that intersects the data at the right hour.
	if opts.IsToday {
		for _, hp := range filtered {
			if hp.Hour != opts.HighlightHour {
				continue
			}
			i := hp.Hour - chartStartHour
			cx := bounds.Min.X + int(float64(i)*step) + barW/2
			markerW := 2
			marker := image.Rect(cx-markerW/2, bounds.Min.Y, cx+markerW/2+markerW%2, barTop+barMaxH)
			fillRect(frame, marker, widget.PaperBlack)
			break
		}
	}

	type chartPoint struct {
		x, y int
	}
	var points []chartPoint

	for _, hp := range filtered {
		i := hp.Hour - chartStartHour
		cx := bounds.Min.X + int(float64(i)*step) + barW/2
		cy := tempY(hp.Temperature)
		points = append(points, chartPoint{cx, cy})
	}

	// Temperature polyline drawn as 1-px Bresenham strokes — those have
	// to be black on-device, otherwise the dither breaks the curve into
	// dots that don't read as a line. Plus signs at each vertex give
	// the curve weight without a glaring solid run.
	for j := 1; j < len(points); j++ {
		drawLine(frame, points[j-1].x, points[j-1].y, points[j].x, points[j].y, widget.PaperBlack)
	}
	for _, p := range points {
		for dy := -1; dy <= 1; dy++ {
			for dx := -1; dx <= 1; dx++ {
				if dx*dx+dy*dy <= 1 {
					setPixel(frame, p.x+dx, p.y+dy, widget.PaperBlack)
				}
			}
		}
	}

	for _, hp := range filtered {
		i := hp.Hour - chartStartHour
		bx := bounds.Min.X + int(float64(i)*step)

		prob := hp.PrecipitationProb
		barH := int(math.Round(prob * float64(barMaxH)))
		if barH < 1 && prob > 0 {
			barH = 2
		}
		if barH > 0 {
			// Precip bars: PaperGray70 fill (Y=0x4D) so they land black on
			// the BW threshold and dark-gray on Gray4. PaperGray40 would
			// disappear without the old Bayer dither. Cap stays
			// PaperBlack for a crisp rim against the chart background.
			r := image.Rect(bx, barTop+barMaxH-barH, bx+barW, barTop+barMaxH)
			fillRect(frame, r, widget.PaperGray70)
			drawHLine(frame, r.Min.X, r.Max.X, r.Min.Y, widget.PaperBlack)
		}

		// Tick marks at the base of the bar gutter for x-axis grounding.
		// Single pixels survive only as on/off, so keep them PaperBlack.
		setPixel(frame, bx, barTop+barMaxH, widget.PaperBlack)
		setPixel(frame, bx+barW-1, barTop+barMaxH, widget.PaperBlack)
	}

	for _, hp := range filtered {
		i := hp.Hour - chartStartHour
		label, ok := labelHours[hp.Hour]
		if !ok {
			continue
		}
		cx := bounds.Min.X + int(float64(i)*step) + barW/2
		labelY := barTop + barMaxH + lineHeight
		textW := len(label) * charWidth
		// Hour labels in solid PaperBlack so they survive the BW
		// threshold cleanly. A muted-gray source color would lose its
		// AA fringe and at this small size (10pt) the labels would
		// fragment badly.
		drawTextCentered(frame, cx-textW/2, cx-textW/2+textW, labelY, label)
	}
}

func filterHours(hourly []weather.HourlyPoint, start, end int) []weather.HourlyPoint {
	var out []weather.HourlyPoint
	for _, hp := range hourly {
		if hp.Hour >= start && hp.Hour <= end {
			out = append(out, hp)
		}
	}
	return out
}
