package weatherview

import (
	"fmt"
	"math"

	"github.com/grantlucas/inkwell/internal/inkwell/weather"
)

// HighLow is a day's high and low, rounded to whole degrees in the
// configured unit, so every screen writes them the same way.
type HighLow struct {
	high, low int
	unit      string
}

// NewHighLow converts a day's forecast high and low (Celsius) into unit:
// "F" for Fahrenheit, anything else for Celsius.
func NewHighLow(day weather.DailyForecast, unit string) HighLow {
	hi, lo := day.High, day.Low
	label := "C"
	if unit == "F" {
		hi, lo = weather.CelsiusToFahrenheit(hi), weather.CelsiusToFahrenheit(lo)
		label = "F"
	}
	return HighLow{high: int(math.Round(hi)), low: int(math.Round(lo)), unit: label}
}

// High is the high with its unit, "17°C". It names the unit for the pair,
// so the low beside it does not repeat it.
func (t HighLow) High() string { return fmt.Sprintf("%d°%s", t.high, t.unit) }

// Low is the bare low, "9°".
func (t HighLow) Low() string { return fmt.Sprintf("%d°", t.low) }

// Pair is both on one line with no unit, "17° 9°", for a cell too narrow
// to name it.
func (t HighLow) Pair() string { return fmt.Sprintf("%d° %d°", t.high, t.low) }
