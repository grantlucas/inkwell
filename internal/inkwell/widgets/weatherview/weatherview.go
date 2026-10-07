// Package weatherview provides reusable weather rendering components for
// e-ink display widgets: the combined precipitation and temperature chart,
// condition icons from the Weather Icons font, and a day's high and low.
package weatherview

import (
	"math"

	"github.com/grantlucas/inkwell/internal/inkwell/weather"
)

// GlobalTempRange computes the temperature range across all days' highs,
// lows and hourly data, for use in chart normalization. With no days it
// falls back to 0-25°C.
func GlobalTempRange(days []weather.DailyForecast) TempRange {
	minTemp, maxTemp := math.Inf(1), math.Inf(-1)
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
	return TempRange{Min: minTemp, Max: maxTemp}
}
