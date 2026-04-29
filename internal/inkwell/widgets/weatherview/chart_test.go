package weatherview

import (
	"image"
	"testing"

	"github.com/grantlucas/inkwell/internal/inkwell/weather"
)

func sampleHourly() []weather.HourlyPoint {
	var points []weather.HourlyPoint
	for h := 0; h < 24; h++ {
		points = append(points, weather.HourlyPoint{
			Hour:              h,
			Temperature:       10 + float64(h)/2,
			PrecipitationProb: float64(h) / 100.0,
		})
	}
	return points
}

func TestRenderHourlyChart_Basic(t *testing.T) {
	frame := newTestFrame(120, 80)
	opts := ChartOptions{
		TempUnit:      "C",
		GlobalTempMin: 5,
		GlobalTempMax: 24,
		HighlightHour: 15,
	}

	RenderHourlyChart(frame, image.Rect(0, 0, 120, 80), sampleHourly(), opts)

	hasBlack := false
	for y := range 80 {
		for x := range 120 {
			if frame.ColorIndexAt(x, y) == 1 {
				hasBlack = true
				break
			}
		}
		if hasBlack {
			break
		}
	}
	if !hasBlack {
		t.Error("chart drew no black pixels")
	}
}

func TestRenderHourlyChart_Fahrenheit(t *testing.T) {
	frame := newTestFrame(120, 80)
	opts := ChartOptions{
		TempUnit:      "F",
		GlobalTempMin: 5,
		GlobalTempMax: 24,
		HighlightHour: 15,
	}

	RenderHourlyChart(frame, image.Rect(0, 0, 120, 80), sampleHourly(), opts)

	hasBlack := false
	for y := range 80 {
		for x := range 120 {
			if frame.ColorIndexAt(x, y) == 1 {
				hasBlack = true
				break
			}
		}
		if hasBlack {
			break
		}
	}
	if !hasBlack {
		t.Error("chart drew no black pixels in Fahrenheit mode")
	}
}

func TestRenderHourlyChart_EmptyHourly(t *testing.T) {
	frame := newTestFrame(120, 80)
	opts := ChartOptions{TempUnit: "C", GlobalTempMin: 5, GlobalTempMax: 24}
	RenderHourlyChart(frame, image.Rect(0, 0, 120, 80), nil, opts)
}

func TestRenderHourlyChart_TooSmallBounds(t *testing.T) {
	frame := newTestFrame(5, 5)
	opts := ChartOptions{TempUnit: "C", GlobalTempMin: 5, GlobalTempMax: 24}
	RenderHourlyChart(frame, image.Rect(0, 0, 5, 5), sampleHourly(), opts)
}

func TestRenderHourlyChart_NoMatchingHours(t *testing.T) {
	frame := newTestFrame(120, 80)
	hourly := []weather.HourlyPoint{
		{Hour: 0, Temperature: 10, PrecipitationProb: 0},
		{Hour: 1, Temperature: 11, PrecipitationProb: 0},
	}
	opts := ChartOptions{TempUnit: "C", GlobalTempMin: 5, GlobalTempMax: 24}
	RenderHourlyChart(frame, image.Rect(0, 0, 120, 80), hourly, opts)
}

func TestRenderHourlyChart_EqualTempRange(t *testing.T) {
	frame := newTestFrame(120, 80)
	hourly := []weather.HourlyPoint{
		{Hour: 10, Temperature: 15, PrecipitationProb: 0.5},
	}
	opts := ChartOptions{
		TempUnit:      "C",
		GlobalTempMin: 15,
		GlobalTempMax: 15,
	}
	RenderHourlyChart(frame, image.Rect(0, 0, 120, 80), hourly, opts)
}

func TestRenderHourlyChart_HighPrecip(t *testing.T) {
	frame := newTestFrame(120, 80)
	hourly := []weather.HourlyPoint{
		{Hour: 12, Temperature: 15, PrecipitationProb: 0.95},
		{Hour: 13, Temperature: 16, PrecipitationProb: 0.01},
	}
	opts := ChartOptions{
		TempUnit:      "C",
		GlobalTempMin: 5,
		GlobalTempMax: 24,
	}
	RenderHourlyChart(frame, image.Rect(0, 0, 120, 80), hourly, opts)
}

func TestFilterHours(t *testing.T) {
	hourly := sampleHourly()
	filtered := filterHours(hourly, 6, 20)
	if len(filtered) != 15 {
		t.Errorf("got %d filtered, want 15", len(filtered))
	}
	if filtered[0].Hour != 6 {
		t.Errorf("first hour = %d, want 6", filtered[0].Hour)
	}
	if filtered[len(filtered)-1].Hour != 20 {
		t.Errorf("last hour = %d, want 20", filtered[len(filtered)-1].Hour)
	}
}

func TestFilterHours_Empty(t *testing.T) {
	filtered := filterHours(nil, 6, 20)
	if len(filtered) != 0 {
		t.Errorf("got %d, want 0", len(filtered))
	}
}

func TestRenderHourlyChart_VerySmallHeight(t *testing.T) {
	frame := newTestFrame(120, 22)
	opts := ChartOptions{TempUnit: "C", GlobalTempMin: 5, GlobalTempMax: 24}
	RenderHourlyChart(frame, image.Rect(0, 0, 120, 22), sampleHourly(), opts)
}

func TestRenderHourlyChart_ZeroPrecip(t *testing.T) {
	frame := newTestFrame(120, 80)
	hourly := []weather.HourlyPoint{
		{Hour: 10, Temperature: 15, PrecipitationProb: 0},
		{Hour: 12, Temperature: 18, PrecipitationProb: 0},
	}
	opts := ChartOptions{TempUnit: "C", GlobalTempMin: 5, GlobalTempMax: 24}
	RenderHourlyChart(frame, image.Rect(0, 0, 120, 80), hourly, opts)
}
