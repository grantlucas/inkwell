package weatherview

import (
	"image"
	"math"

	"github.com/grantlucas/inkwell/internal/inkwell/weather"
)

// ChartOptions controls hourly chart rendering.
type ChartOptions struct {
	TempUnit      string
	GlobalTempMin float64
	GlobalTempMax float64
	HighlightHour int
}

const (
	chartStartHour = 6
	chartEndHour   = 20
	chartHours     = chartEndHour - chartStartHour + 1
)

var labelHours = map[int]string{
	6: "6A", 9: "9A", 12: "12", 15: "3P", 20: "8P",
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

	drawHLine(frame, bounds.Min.X, bounds.Max.X, barTop-1)

	for _, hp := range filtered {
		i := hp.Hour - chartStartHour
		cx := bounds.Min.X + int(float64(i)*step) + barW/2

		if hp.Hour == opts.HighlightHour {
			drawDashedVLine(frame, cx, bounds.Min.Y, barTop+barMaxH, 2, 2)
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

	for j := 1; j < len(points); j++ {
		drawLine(frame, points[j-1].x, points[j-1].y, points[j].x, points[j].y)
	}
	for _, p := range points {
		for dy := -1; dy <= 1; dy++ {
			for dx := -1; dx <= 1; dx++ {
				if dx*dx+dy*dy <= 1 {
					setPixel(frame, p.x+dx, p.y+dy, 1)
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
			r := image.Rect(bx, barTop+barMaxH-barH, bx+barW, barTop+barMaxH)
			fillRect(frame, r, 1)
		}

		setPixel(frame, bx, barTop+barMaxH, 1)
		setPixel(frame, bx+barW-1, barTop+barMaxH, 1)
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
		drawText(frame, cx-textW/2, labelY, label)
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
