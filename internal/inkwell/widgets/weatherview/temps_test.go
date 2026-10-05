package weatherview

import (
	"testing"

	"github.com/grantlucas/inkwell/internal/inkwell/weather"
)

// A day's high and low are written one way on every screen: rounded to
// whole degrees in the configured unit, the high carrying the unit and
// the low not, or both bare when they share a line.
func TestNewHighLow(t *testing.T) {
	cases := []struct {
		label     string
		high, low float64
		unit      string
		wantHigh  string
		wantLow   string
		wantPair  string
	}{
		{"celsius", 17.4, 9.2, "C", "17°C", "9°", "17° 9°"},
		{"rounds half away from zero", 16.5, -2.5, "C", "17°C", "-3°", "17° -3°"},
		{"fahrenheit converts", 20, 0, "F", "68°F", "32°", "68° 32°"},
		{"anything else is celsius", 20, 0, "", "20°C", "0°", "20° 0°"},
		{"widest reading", 38, -73.4, "F", "100°F", "-100°", "100° -100°"},
	}

	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			got := NewHighLow(weather.DailyForecast{High: tc.high, Low: tc.low}, tc.unit)
			if got.High() != tc.wantHigh {
				t.Errorf("High() = %q, want %q", got.High(), tc.wantHigh)
			}
			if got.Low() != tc.wantLow {
				t.Errorf("Low() = %q, want %q", got.Low(), tc.wantLow)
			}
			if got.Pair() != tc.wantPair {
				t.Errorf("Pair() = %q, want %q", got.Pair(), tc.wantPair)
			}
		})
	}
}
