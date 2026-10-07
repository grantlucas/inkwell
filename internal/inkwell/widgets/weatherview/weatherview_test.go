package weatherview

import (
	"testing"

	"github.com/grantlucas/inkwell/internal/inkwell/weather"
)

// The range spans every day's highs, lows and hourly readings, and falls
// back to 0-25°C when there are no days to take it from.
func TestGlobalTempRange(t *testing.T) {
	tests := []struct {
		label string
		days  []weather.DailyForecast
		want  TempRange
	}{
		{
			label: "highs, lows and hours",
			days: []weather.DailyForecast{
				{High: 20, Low: 8, Hourly: []weather.HourlyPoint{{Temperature: 10}, {Temperature: 18}}},
				{High: 25, Low: 12, Hourly: []weather.HourlyPoint{{Temperature: 13}, {Temperature: 24}}},
			},
			want: TempRange{Min: 8, Max: 25},
		},
		{
			label: "an hour outside the high and low",
			days:  []weather.DailyForecast{{High: 20, Low: 8, Hourly: []weather.HourlyPoint{{Temperature: 5}, {Temperature: 22}}}},
			want:  TempRange{Min: 5, Max: 22},
		},
		{label: "no hourly data", days: []weather.DailyForecast{{High: 20, Low: 8}}, want: TempRange{Min: 8, Max: 20}},
		{label: "no days", days: nil, want: TempRange{Min: 0, Max: 25}},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			if got := GlobalTempRange(tt.days); got != tt.want {
				t.Errorf("GlobalTempRange = %+v, want %+v", got, tt.want)
			}
		})
	}
}
