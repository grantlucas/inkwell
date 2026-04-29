package weatherview

import (
	"image"
	"testing"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/weather"
)

func sampleDay() weather.DailyForecast {
	hourly := make([]weather.HourlyPoint, 24)
	for h := range 24 {
		hourly[h] = weather.HourlyPoint{
			Hour:              h,
			Temperature:       10 + float64(h)/2,
			PrecipitationProb: float64(h) / 100.0,
		}
	}
	return weather.DailyForecast{
		Date:      time.Date(2026, 4, 28, 0, 0, 0, 0, time.UTC),
		High:      22,
		Low:       10,
		Condition: weather.PartlyCloudy,
		Hourly:    hourly,
	}
}

func TestRenderDayWeather_Basic(t *testing.T) {
	frame := newTestFrame(114, 160)
	day := sampleDay()
	opts := Options{
		TempUnit:      "C",
		ShowLabel:     true,
		GlobalTempMin: 5,
		GlobalTempMax: 24,
		HighlightHour: 15,
	}

	RenderDayWeather(frame, image.Rect(0, 0, 114, 160), day, opts)

	blackCount := 0
	for y := range 160 {
		for x := range 114 {
			if frame.ColorIndexAt(x, y) == 1 {
				blackCount++
			}
		}
	}
	if blackCount < 50 {
		t.Errorf("too few black pixels (%d), rendering may have failed", blackCount)
	}
}

func TestRenderDayWeather_Fahrenheit(t *testing.T) {
	frame := newTestFrame(114, 160)
	day := sampleDay()
	opts := Options{
		TempUnit:      "F",
		ShowLabel:     true,
		GlobalTempMin: 5,
		GlobalTempMax: 24,
		HighlightHour: 15,
	}
	RenderDayWeather(frame, image.Rect(0, 0, 114, 160), day, opts)
}

func TestRenderDayWeather_NoLabel(t *testing.T) {
	frame := newTestFrame(114, 160)
	day := sampleDay()
	opts := Options{
		TempUnit:      "C",
		ShowLabel:     false,
		GlobalTempMin: 5,
		GlobalTempMax: 24,
	}
	RenderDayWeather(frame, image.Rect(0, 0, 114, 160), day, opts)
}

func TestRenderDayWeather_TooSmall(t *testing.T) {
	frame := newTestFrame(10, 10)
	day := sampleDay()
	opts := Options{TempUnit: "C"}
	RenderDayWeather(frame, image.Rect(0, 0, 10, 10), day, opts)
}

func TestRenderDayWeather_CustomIconSize(t *testing.T) {
	frame := newTestFrame(114, 160)
	day := sampleDay()
	opts := Options{
		TempUnit:      "C",
		ShowLabel:     true,
		GlobalTempMin: 5,
		GlobalTempMax: 24,
		IconSize:      32,
	}
	RenderDayWeather(frame, image.Rect(0, 0, 114, 160), day, opts)
}

func TestRenderDayWeather_SmallChart(t *testing.T) {
	frame := newTestFrame(114, 50)
	day := sampleDay()
	opts := Options{
		TempUnit:      "C",
		ShowLabel:     true,
		GlobalTempMin: 5,
		GlobalTempMax: 24,
	}
	RenderDayWeather(frame, image.Rect(0, 0, 114, 50), day, opts)
}

func TestGlobalTempRange(t *testing.T) {
	days := []weather.DailyForecast{
		{High: 20, Low: 8, Hourly: []weather.HourlyPoint{
			{Temperature: 10}, {Temperature: 18},
		}},
		{High: 25, Low: 12, Hourly: []weather.HourlyPoint{
			{Temperature: 13}, {Temperature: 24},
		}},
	}

	minT, maxT := GlobalTempRange(days)
	if minT != 8 {
		t.Errorf("minTemp = %v, want 8", minT)
	}
	if maxT != 25 {
		t.Errorf("maxTemp = %v, want 25", maxT)
	}
}

func TestGlobalTempRange_Empty(t *testing.T) {
	minT, maxT := GlobalTempRange(nil)
	if minT != 0 {
		t.Errorf("minTemp = %v, want 0", minT)
	}
	if maxT != 25 {
		t.Errorf("maxTemp = %v, want 25", maxT)
	}
}

func TestGlobalTempRange_NoHourly(t *testing.T) {
	days := []weather.DailyForecast{
		{High: 20, Low: 8},
	}
	minT, maxT := GlobalTempRange(days)
	if minT != 8 {
		t.Errorf("minTemp = %v, want 8", minT)
	}
	if maxT != 20 {
		t.Errorf("maxTemp = %v, want 20", maxT)
	}
}
