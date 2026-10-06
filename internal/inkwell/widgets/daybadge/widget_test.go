package daybadge_test

import (
	"image"
	"testing"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar"
	"github.com/grantlucas/inkwell/internal/inkwell/testutil"
	"github.com/grantlucas/inkwell/internal/inkwell/testutil/fakehttp"
	"github.com/grantlucas/inkwell/internal/inkwell/weather"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daybadge"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daydata"
)

func fixedClock(t time.Time) func() time.Time { return func() time.Time { return t } }

// week is a forecast for today and the four days after it.
func week() []weather.DailyForecast {
	conds := []weather.Condition{weather.Thunderstorm, weather.Rain, weather.Snow, weather.Fog, weather.PartlyCloudy}
	var out []weather.DailyForecast
	for i, c := range conds {
		out = append(out, *forecastFor(i, c, []float64{24, 12, 2, 7, 13}[i], []float64{15, 8, -4, -2, 9}[i]))
	}
	return out
}

func badgeConfig(s daybadge.Style, d int, unit string) daybadge.Config {
	return daybadge.Config{Config: daydata.Config{Day: d, Weather: daydata.WeatherConfig{TempUnit: unit}}, Style: s}
}

func newBadge(bounds image.Rectangle, forecast []weather.DailyForecast, cfg daybadge.Config) *daybadge.Widget {
	return daybadge.New(bounds, daydata.InMemory(nil, forecast), fixedClock(testTime), cfg)
}

// blackFrame is a frame inked solid, standing in for the widgets around
// the badge on the shared frame.
func blackFrame() *image.Paletted {
	frame := newFrame()
	for i := range frame.Pix {
		frame.Pix[i] = widget.PaperBlack
	}
	return frame
}

// The widget draws its one day's badge into its bounds, in its style
// and unit: today unless its config names a later day. The expected
// frame is the style drawing that day as the day data module builds it.
func TestWidget_DrawsItsDay(t *testing.T) {
	tests := []struct {
		label string
		cfg   daybadge.Config
		day   int
	}{
		{"today's column", badgeConfig(daybadge.Column, 0, "C"), 0},
		{"tomorrow's row in fahrenheit", badgeConfig(daybadge.Row, 1, "F"), 1},
		{"tomorrow, compact", badgeConfig(daybadge.Compact, 1, "C"), 1},
		{"today's hero", badgeConfig(daybadge.Hero, 0, "C"), 0},
		{"a day the forecast doesn't reach", badgeConfig(daybadge.Column, 6, "C"), 6},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			bounds := at(tt.cfg.Style)
			got := newFrame()
			if err := newBadge(bounds, week(), tt.cfg).Render(got); err != nil {
				t.Fatalf("Render: %v", err)
			}

			var f *weather.DailyForecast
			if tt.day < len(week()) {
				f = &week()[tt.day]
			}
			want := newFrame()
			tt.cfg.Style.Draw(want, bounds, dayAt(tt.day, f), testTime, tt.cfg.Weather.TempUnit)
			for y := range want.Bounds().Dy() {
				for x := range want.Bounds().Dx() {
					if image.Pt(x, y).In(bounds) && got.ColorIndexAt(x, y) != want.ColorIndexAt(x, y) {
						t.Fatalf("differs from the style's badge at (%d,%d)", x, y)
					}
				}
			}
		})
	}
}

// Every widget clears its own bounds and draws nothing outside them, so
// it can be placed beside any other on the shared frame. That includes
// the rays of the condition icons, which overrun their box: the widget
// clips them to its bounds.
func TestWidget_KeepsToItsBounds(t *testing.T) {
	for _, s := range styles {
		t.Run(s.String(), func(t *testing.T) {
			bounds := at(s)
			frame := blackFrame()
			w := newBadge(bounds, week(), badgeConfig(s, 4, "C"))
			if err := w.Render(frame); err != nil {
				t.Fatalf("Render: %v", err)
			}
			if w.Bounds() != bounds {
				t.Errorf("Bounds = %v, want %v", w.Bounds(), bounds)
			}
			for y := range frame.Bounds().Dy() {
				for x := range frame.Bounds().Dx() {
					if !image.Pt(x, y).In(bounds) && frame.ColorIndexAt(x, y) != widget.PaperBlack {
						t.Fatalf("drew outside its bounds at (%d,%d)", x, y)
					}
				}
			}
			if inkIn(frame, bounds) > bounds.Dx()*bounds.Dy()/2 {
				t.Error("its bounds are mostly ink; it did not clear them")
			}
		})
	}
}

// Bounds smaller than the style's size would put text past the widget's
// edge, so the widget draws nothing there but clear paper.
func TestWidget_TooSmallDrawsNothing(t *testing.T) {
	for _, s := range styles {
		for _, short := range []image.Point{{1, 0}, {0, 1}} {
			t.Run(s.String(), func(t *testing.T) {
				bounds := at(s)
				bounds.Max = bounds.Max.Sub(short)
				frame := blackFrame()
				if err := newBadge(bounds, week(), badgeConfig(s, 0, "C")).Render(frame); err != nil {
					t.Fatalf("Render: %v", err)
				}
				if got := inkIn(frame, bounds); got != 0 {
					t.Errorf("drew %d px into bounds %v, smaller than the style's %v", got, bounds.Size(), s.Size())
				}
			})
		}
	}
}

// Golden renders of each style at its size, on a day with weather and
// on one the forecast doesn't reach.
func TestWidget_Golden(t *testing.T) {
	tests := []struct {
		label string
		cfg   daybadge.Config
	}{
		{"column today", badgeConfig(daybadge.Column, 0, "C")},
		{"row tomorrow", badgeConfig(daybadge.Row, 1, "C")},
		{"compact tomorrow", badgeConfig(daybadge.Compact, 1, "C")},
		{"compact in fahrenheit", badgeConfig(daybadge.Compact, 2, "F")},
		{"hero today", badgeConfig(daybadge.Hero, 0, "C")},
		{"column with no forecast", badgeConfig(daybadge.Column, 6, "C")},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			bounds := image.Rectangle{Max: tt.cfg.Style.Size()}
			frame := image.NewPaletted(bounds, widget.PaperPalette)
			if err := newBadge(bounds, week(), tt.cfg).Render(frame); err != nil {
				t.Fatalf("Render: %v", err)
			}
			testutil.AssertGoldenPNG(t, frame)
		})
	}
}

// The widget shows a day's name and weather, so it takes the weather
// settings, day and its own style, column unless told otherwise.
func TestFactory(t *testing.T) {
	deps := widget.Deps{
		Now:      fixedClock(testTime),
		Calendar: calendar.NewProvider(fakehttp.New(), fixedClock(testTime)),
		Weather:  weather.NewProvider(fakehttp.New(), time.Hour, fixedClock(testTime), weather.Settings{TempUnit: "F"}),
	}
	bounds := image.Rect(0, 0, 160, 156)

	tests := []struct {
		label     string
		config    map[string]any
		wantStyle daybadge.Style
		wantDay   int
		wantUnit  string
		wantErr   string
	}{
		{label: "today's column by default, in the top-level unit", config: nil, wantStyle: daybadge.Column, wantUnit: "F"},
		{
			label: "every setting", config: map[string]any{"style": "hero", "day": 2, "temp_unit": "C"},
			wantStyle: daybadge.Hero, wantDay: 2, wantUnit: "C",
		},
		{
			label: "an unknown style", config: map[string]any{"style": "header"},
			wantErr: `day-badge: style must be one of column, row, compact, hero, got "header"`,
		},
		{label: "style not a string", config: map[string]any{"style": 1}, wantErr: "day-badge: style must be a string, got int"},
		{label: "day past a week out", config: map[string]any{"day": 9}, wantErr: "day-badge: day must be in [0, 6], got 9"},
		{
			label: "feeds", config: map[string]any{"feeds": []any{"https://example.com/a.ics"}},
			wantErr: "day-badge: feeds is not supported: day-badge shows only the weather, so it reads no calendar",
		},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			w, err := daybadge.Factory(bounds, tt.config, deps)
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("err = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Factory: %v", err)
			}
			if w.Bounds() != bounds {
				t.Errorf("Bounds = %v, want %v", w.Bounds(), bounds)
			}
			cfg := w.(*daybadge.Widget).Config
			if cfg.Style != tt.wantStyle || cfg.Day != tt.wantDay || cfg.Weather.TempUnit != tt.wantUnit {
				t.Errorf("Style, Day, TempUnit = %v, %d, %q", cfg.Style, cfg.Day, cfg.Weather.TempUnit)
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
	if _, err := daybadge.Factory(image.Rect(0, 0, 160, 156), nil, deps); err == nil || err.Error() != "day-badge: no clock" {
		t.Fatalf("err = %v, want %q", err, "day-badge: no clock")
	}
}
