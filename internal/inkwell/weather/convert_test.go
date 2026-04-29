package weather

import (
	"math"
	"testing"
)

func TestCelsiusToFahrenheit(t *testing.T) {
	tests := []struct {
		celsius    float64
		fahrenheit float64
	}{
		{0, 32},
		{100, 212},
		{-40, -40},
		{37, 98.6},
	}
	for _, tt := range tests {
		got := CelsiusToFahrenheit(tt.celsius)
		if math.Abs(got-tt.fahrenheit) > 0.1 {
			t.Errorf("CelsiusToFahrenheit(%v) = %v, want %v", tt.celsius, got, tt.fahrenheit)
		}
	}
}

func TestFahrenheitToCelsius(t *testing.T) {
	tests := []struct {
		fahrenheit float64
		celsius    float64
	}{
		{32, 0},
		{212, 100},
		{-40, -40},
		{98.6, 37},
	}
	for _, tt := range tests {
		got := FahrenheitToCelsius(tt.fahrenheit)
		if math.Abs(got-tt.celsius) > 0.1 {
			t.Errorf("FahrenheitToCelsius(%v) = %v, want %v", tt.fahrenheit, got, tt.celsius)
		}
	}
}
