package weatherahead

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar"
	"github.com/grantlucas/inkwell/internal/inkwell/testutil/fakehttp"
	"github.com/grantlucas/inkwell/internal/inkwell/weather"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daygrid"
)

// The widget shows only the weather, so it takes the weather settings
// every day widget takes, no calendar, and its own days: how many days
// after today it lists, four unless told otherwise.
func TestFactory(t *testing.T) {
	deps := widget.Deps{
		Now:      fixedClock(testTime),
		Calendar: calendar.NewProvider(fakehttp.New(), fixedClock(testTime)),
		Weather:  weather.NewProvider(fakehttp.New(), time.Hour, fixedClock(testTime), weather.Settings{TempUnit: "F"}),
	}

	tests := []struct {
		label    string
		config   map[string]any
		wantDays int
		wantUnit string
		wantErr  string
	}{
		{label: "no settings shows four days and inherits the top level", config: nil, wantDays: 4, wantUnit: "F"},
		{label: "its own weather settings", config: map[string]any{"temp_unit": "C", "latitude": 51.5}, wantDays: 4, wantUnit: "C"},
		{label: "one day", config: map[string]any{"days": 1}, wantDays: 1, wantUnit: "F"},
		{label: "a week", config: map[string]any{"days": 7}, wantDays: 7, wantUnit: "F"},
		{
			label:   "no days",
			config:  map[string]any{"days": 0},
			wantErr: "weather-ahead: days must be in [1, 7], got 0",
		},
		{
			label:   "past a week",
			config:  map[string]any{"days": 8},
			wantErr: "weather-ahead: days must be in [1, 7], got 8",
		},
		{
			label:   "days as a string",
			config:  map[string]any{"days": "4"},
			wantErr: "weather-ahead: days must be an integer, got string",
		},
		{
			label:   "feeds",
			config:  map[string]any{"feeds": []any{"https://example.com/a.ics"}},
			wantErr: "weather-ahead: feeds is not supported: weather-ahead shows only the weather, so it reads no calendar",
		},
		{
			label:   "a misspelt key",
			config:  map[string]any{"day": 4},
			wantErr: `weather-ahead: unsupported setting "day" (accepted: days, latitude, longitude, temp_unit, weather_model)`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			w, err := Factory(rightColumn, tt.config, deps)
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("err = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Factory: %v", err)
			}
			if got := w.Bounds(); got != rightColumn {
				t.Errorf("Bounds = %v, want %v", got, rightColumn)
			}
			got := w.(*Widget)
			if got.Config.Days != tt.wantDays {
				t.Errorf("Days = %d, want %d", got.Config.Days, tt.wantDays)
			}
			if got.Config.Weather.TempUnit != tt.wantUnit {
				t.Errorf("TempUnit = %q, want %q", got.Config.Weather.TempUnit, tt.wantUnit)
			}
		})
	}
}

// A dashboard wired without a clock is a fault to report at load, not a
// widget that quietly reads the wall clock.
func TestFactory_NeedsAClock(t *testing.T) {
	deps := widget.Deps{
		Calendar: calendar.NewProvider(fakehttp.New(), fixedClock(testTime)),
		Weather:  weather.NewProvider(fakehttp.New(), time.Hour, fixedClock(testTime), weather.Settings{}),
	}
	if _, err := Factory(rightColumn, nil, deps); err == nil || err.Error() != "weather-ahead: no clock" {
		t.Fatalf("err = %v, want %q", err, "weather-ahead: no clock")
	}
}

// A failed forecast fetch is not a render error: the compositor drops the
// whole frame on the first one, which would blank every other widget on
// the screen. With no forecast to draw, every row says so, as in the
// "no forecast" golden, and draws no number nobody forecast.
func TestWidget_FetchFailureSaysNoForecast(t *testing.T) {
	failures := []struct {
		label string
		reply fakehttp.Reply
	}{
		{"server error", fakehttp.Reply{Status: 500}},
		{"network error", fakehttp.Reply{Err: errors.New("connection refused")}},
	}
	for _, f := range failures {
		t.Run(f.label, func(t *testing.T) {
			tr := fakehttp.New()
			tr.Set("https://api.open-meteo.com/v1/forecast", f.reply)
			deps := widget.Deps{
				Now:      fixedClock(testTime),
				Calendar: calendar.NewProvider(tr, fixedClock(testTime)),
				Weather:  weather.NewProvider(tr, time.Hour, fixedClock(testTime), weather.Settings{TempUnit: "C"}),
			}
			w, err := Factory(goldenBox, nil, deps)
			if err != nil {
				t.Fatalf("Factory: %v", err)
			}

			got := render(t, w.(*Widget))

			if tr.Total() == 0 {
				t.Fatal("the forecast was never asked for")
			}
			want := render(t, New(goldenBox, daygrid.InMemory(nil, nil), fixedClock(testTime), config(4, "C")))
			if !slices.Equal(got.Pix, want.Pix) {
				t.Error("a failed fetch drew something other than the no-forecast state")
			}
		})
	}
}
