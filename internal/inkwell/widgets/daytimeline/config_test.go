package daytimeline

import (
	"image"
	"maps"
	"testing"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar"
	"github.com/grantlucas/inkwell/internal/inkwell/testutil/fakehttp"
	"github.com/grantlucas/inkwell/internal/inkwell/weather"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
)

// The window is the widget's own setting: whole hours, in order, wide
// enough to read. Everything else is the shared calendar settings, which
// daydata's parser tests cover.
func TestFactory_Window(t *testing.T) {
	deps := widget.Deps{
		Now:      fixedClock(testTime),
		Calendar: calendar.NewProvider(fakehttp.New(), fixedClock(testTime)),
		Weather:  weather.NewProvider(fakehttp.New(), time.Hour, fixedClock(testTime), weather.Settings{}),
	}
	feed := []any{"https://example.com/a.ics"}

	tests := []struct {
		label     string
		window    map[string]any
		wantStart int
		wantEnd   int
		wantErr   string
	}{
		{label: "defaults to 7 to 22", wantStart: 7, wantEnd: 22},
		{label: "takes a start", window: map[string]any{"start_hour": 6}, wantStart: 6, wantEnd: 22},
		{label: "takes an end", window: map[string]any{"end_hour": 18}, wantStart: 7, wantEnd: 18},
		{label: "takes the whole day", window: map[string]any{"start_hour": 0, "end_hour": 24}, wantStart: 0, wantEnd: 24},
		{label: "takes exactly six hours", window: map[string]any{"start_hour": 9, "end_hour": 15}, wantStart: 9, wantEnd: 15},
		{
			label:   "rejects a fractional start",
			window:  map[string]any{"start_hour": 7.5},
			wantErr: "day-timeline: start_hour must be a whole number of hours, got 7.5",
		},
		{
			label:   "rejects a string end",
			window:  map[string]any{"end_hour": "22"},
			wantErr: `day-timeline: end_hour must be a whole number of hours, got "22"`,
		},
		{
			label:   "rejects a negative start",
			window:  map[string]any{"start_hour": -1},
			wantErr: "day-timeline: start_hour must be from 0 to 24, got -1",
		},
		{
			label:   "rejects an end past midnight",
			window:  map[string]any{"end_hour": 25},
			wantErr: "day-timeline: end_hour must be from 0 to 24, got 25",
		},
		{
			label:   "rejects an end before the start",
			window:  map[string]any{"start_hour": 20, "end_hour": 8},
			wantErr: "day-timeline: end_hour (8) must be after start_hour (20)",
		},
		{
			label:   "rejects an end equal to the start",
			window:  map[string]any{"start_hour": 9, "end_hour": 9},
			wantErr: "day-timeline: end_hour (9) must be after start_hour (9)",
		},
		{
			label:   "rejects a start past the default end",
			window:  map[string]any{"start_hour": 23},
			wantErr: "day-timeline: end_hour (22) must be after start_hour (23)",
		},
		{
			label:   "rejects a window under six hours",
			window:  map[string]any{"start_hour": 9, "end_hour": 14},
			wantErr: "day-timeline: start_hour 9 to end_hour 14 is 5 hours; the window must span at least 6",
		},
		{
			label:   "explains max_events",
			window:  map[string]any{"max_events": 3},
			wantErr: "day-timeline: max_events is not supported: every event in the window is placed at its time, and the rest are counted as earlier or later",
		},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			config := map[string]any{"feeds": feed}
			maps.Copy(config, tt.window)
			w, err := Factory(image.Rect(0, 0, 500, 432), config, deps)
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("err = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Factory: %v", err)
			}
			cfg := w.(*Widget).Window
			if cfg.StartHour != tt.wantStart || cfg.EndHour != tt.wantEnd {
				t.Errorf("window = %d to %d, want %d to %d", cfg.StartHour, cfg.EndHour, tt.wantStart, tt.wantEnd)
			}
		})
	}
}

// The shared settings go through the shared parser, so a widget built
// without a calendar module fails the way every calendar widget does.
func TestFactory_SharedFailures(t *testing.T) {
	tests := []struct {
		label   string
		config  map[string]any
		deps    widget.Deps
		wantErr string
	}{
		{
			label:   "feeds are required",
			config:  map[string]any{},
			deps:    widget.Deps{Now: fixedClock(testTime)},
			wantErr: "day-timeline: feeds is required",
		},
		{
			label:   "needs the calendar module",
			config:  map[string]any{"feeds": []any{"https://example.com/a.ics"}},
			deps:    widget.Deps{Now: fixedClock(testTime)},
			wantErr: "day-timeline: no calendar module",
		},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			_, err := Factory(image.Rect(0, 0, 500, 432), tt.config, tt.deps)
			if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("err = %v, want %q", err, tt.wantErr)
			}
		})
	}
}
