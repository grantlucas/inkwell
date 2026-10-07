package weatherview

import (
	"math"
	"testing"

	"github.com/grantlucas/inkwell/internal/inkwell/weather"
)

// A line running down a plot, as the weather lane's does, is placed
// across the plot's width: the coldest temperature on the left edge and
// the warmest as far right as a line two pixels wide still fits. Values
// outside the range clamp to the edges, and a NaN reads as the coldest.
func TestTempRange_X(t *testing.T) {
	rng := TempRange{Min: 0, Max: 20}
	const left, width = 100, 22
	cases := []struct {
		label string
		temp  float64
		want  int
	}{
		{"coldest", 0, 100},
		{"warmest", 20, 120},
		{"middle", 10, 110},
		{"a quarter", 5, 105},
		{"colder than the range", -5, 100},
		{"warmer than the range", 30, 120},
		{"not a number", math.NaN(), 100},
	}
	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			if got := rng.X(tc.temp, left, width); got != tc.want {
				t.Errorf("X(%v) = %d, want %d", tc.temp, got, tc.want)
			}
		})
	}
}

// A bar's length is its probability of the room it has, with a trace
// chance floored to a visible stub and a malformed probability clamped,
// whichever way the bar grows.
func TestBarLength(t *testing.T) {
	cases := []struct {
		label string
		prob  float64
		room  int
		want  int
	}{
		{"none", 0, 40, 0},
		{"half", 0.5, 40, 20},
		{"certain", 1, 40, 40},
		{"a trace floors to a stub", 0.01, 40, 2},
		{"a trace never outgrows the room", 0.01, 1, 1},
		{"over 100% clamps", 1.7, 40, 40},
		{"below 0% clamps", -0.3, 40, 0},
	}
	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			if got := BarLength(tc.prob, tc.room); got != tc.want {
				t.Errorf("BarLength(%v, %d) = %d, want %d", tc.prob, tc.room, got, tc.want)
			}
		})
	}
}

// A day is dry when no hour's chance reaches 15%: below that the bars are
// stubs, and the line alone says the day is dry.
func TestDry(t *testing.T) {
	cases := []struct {
		label string
		probs []float64
		want  bool
	}{
		{"no hours", nil, true},
		{"all zero", []float64{0, 0, 0}, true},
		{"traces only", []float64{0.05, 0.14, 0.1}, true},
		{"one hour at the threshold", []float64{0, 0.15, 0}, false},
		{"a rainy afternoon", []float64{0.1, 0.6, 0.9}, false},
	}
	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			var points []weather.HourlyPoint
			for i, p := range tc.probs {
				points = append(points, weather.HourlyPoint{Hour: 6 + i, PrecipitationProb: p})
			}
			if got := Dry(points); got != tc.want {
				t.Errorf("Dry(%v) = %v, want %v", tc.probs, got, tc.want)
			}
		})
	}
}
