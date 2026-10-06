package todayweather

import (
	"errors"
	"image"
	"slices"
	"testing"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar"
	"github.com/grantlucas/inkwell/internal/inkwell/testutil"
	"github.com/grantlucas/inkwell/internal/inkwell/testutil/fakehttp"
	"github.com/grantlucas/inkwell/internal/inkwell/weather"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daydata"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/drawkit"
)

// testTime is a Monday mid-afternoon.
var testTime = time.Date(2026, 3, 16, 14, 30, 0, 0, time.UTC)

func fixedClock(t time.Time) func() time.Time { return func() time.Time { return t } }

// rightColumn is where the day-timeline screen places the widget: the top
// of the right-hand third, under the fuzzy clock's header band.
var rightColumn = image.Rect(534, 48, 800, 208)

// goldenBox is rightColumn's size at the origin, so a golden is the
// widget alone. Where it lands on the panel is
// TestWidget_StaysInsideItsBounds's business.
var goldenBox = image.Rect(0, 0, rightColumn.Dx(), rightColumn.Dy())

// today is a forecast for testTime's date only.
func today(cond weather.Condition, high, low float64) []weather.DailyForecast {
	return []weather.DailyForecast{{
		Date:      time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC),
		High:      high,
		Low:       low,
		Condition: cond,
	}}
}

// unitConfig is the parsed config with the one setting the widget draws
// with.
func unitConfig(unit string) daydata.Config {
	return daydata.Config{Weather: daydata.WeatherConfig{TempUnit: unit}}
}

// render draws the widget at bounds into a frame exactly its size.
func render(t *testing.T, w *Widget) *image.Paletted {
	t.Helper()
	frame := image.NewPaletted(w.Bounds(), widget.PaperPalette)
	if err := w.Render(frame); err != nil {
		t.Fatalf("Render: %v", err)
	}
	return frame
}

// Golden renders at the size the day-timeline screen gives the widget.
func TestWidget_Golden(t *testing.T) {
	tests := []struct {
		label    string
		forecast []weather.DailyForecast
		unit     string
		bounds   image.Rectangle
	}{
		{label: "rainy day", forecast: today(weather.Rain, 12.4, 6.6), unit: "C"},
		{label: "dry day", forecast: today(weather.Clear, 24, 13), unit: "C"},
		{label: "negative low", forecast: today(weather.Snow, 2, -7), unit: "C"},
		{label: "fahrenheit", forecast: today(weather.PartlyCloudy, 24, 13), unit: "F"},
		// The widest high and low there are: the low can't fit beside
		// the high, so it takes its own line under it.
		{label: "deep cold", forecast: today(weather.Snow, -12, -18), unit: "C"},
		// The forecast never arrived, or reaches no further than
		// yesterday.
		{label: "no forecast", forecast: nil, unit: "C"},
		{label: "forecast not reaching today", unit: "C", forecast: []weather.DailyForecast{{
			Date: time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC), High: 12, Low: 6, Condition: weather.Rain,
		}}},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			bounds := tt.bounds
			if bounds.Empty() {
				bounds = goldenBox
			}
			w := New(bounds, daydata.InMemory(nil, tt.forecast), fixedClock(testTime), unitConfig(tt.unit))
			testutil.AssertGoldenPNG(t, render(t, w))
		})
	}
}

// paintOutside inks every frame pixel outside bounds solid black,
// standing in for the widgets the compositor put around it.
func paintOutside(frame *image.Paletted, bounds image.Rectangle) {
	for y := frame.Rect.Min.Y; y < frame.Rect.Max.Y; y++ {
		for x := frame.Rect.Min.X; x < frame.Rect.Max.X; x++ {
			if !image.Pt(x, y).In(bounds) {
				frame.SetColorIndex(x, y, widget.PaperBlack)
			}
		}
	}
}

// The draw helpers clip to the frame, not the widget, so anything the
// widget drew past its edge would land on a neighbour. Whatever the
// forecast, every pixel stays inside its bounds, wherever on the panel
// those are.
func TestWidget_StaysInsideItsBounds(t *testing.T) {
	placements := []struct {
		label  string
		bounds image.Rectangle
	}{
		{"right column", rightColumn},
		{"mid panel", image.Rect(200, 100, 200+rightColumn.Dx(), 100+rightColumn.Dy())},
		{"just big enough", image.Rect(300, 100, 300+minWidth, 100+minHeight)},
	}
	forecasts := []struct {
		label    string
		forecast []weather.DailyForecast
		unit     string
	}{
		{"mild", today(weather.PartlyCloudy, 17, 9), "C"},
		{"deep cold", today(weather.Snow, -12, -18), "C"},
		{"hot fahrenheit", today(weather.Clear, 38, 26), "F"},
		{"no forecast", nil, "C"},
	}
	for _, p := range placements {
		for _, f := range forecasts {
			t.Run(p.label+" "+f.label, func(t *testing.T) {
				frame := image.NewPaletted(image.Rect(0, 0, 800, 480), widget.PaperPalette)
				paintOutside(frame, p.bounds)
				w := New(p.bounds, daydata.InMemory(nil, f.forecast), fixedClock(testTime), unitConfig(f.unit))
				if err := w.Render(frame); err != nil {
					t.Fatalf("Render: %v", err)
				}
				for y := range 480 {
					for x := range 800 {
						if !image.Pt(x, y).In(p.bounds) && frame.ColorIndexAt(x, y) != widget.PaperBlack {
							t.Fatalf("painted over a neighbouring widget at (%d,%d)", x, y)
						}
					}
				}
				if !inked(frame, p.bounds) {
					t.Error("drew nothing")
				}
			})
		}
	}
}

// inked reports whether any pixel inside r is PaperBlack.
func inked(frame *image.Paletted, r image.Rectangle) bool {
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			if frame.ColorIndexAt(x, y) == widget.PaperBlack {
				return true
			}
		}
	}
	return false
}

// A failed forecast fetch is not a render error: the compositor drops the
// whole frame on the first one, which would blank every other widget on
// the screen. With no forecast to draw, the widget says so, as in the
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
			want := render(t, New(goldenBox, daydata.InMemory(nil, nil), fixedClock(testTime), unitConfig("C")))
			if !slices.Equal(got.Pix, want.Pix) {
				t.Error("a failed fetch drew something other than the no-forecast state")
			}
		})
	}
}

// Too small to hold the block, the widget draws nothing rather than
// spilling onto its neighbours: a blank region is a misconfiguration an
// operator can see, ink on another widget looks like a fault elsewhere.
func TestWidget_TooSmallDrawsNothing(t *testing.T) {
	tests := []struct {
		label  string
		bounds image.Rectangle
	}{
		{"too narrow", image.Rect(0, 0, minWidth-1, minHeight)},
		{"too short", image.Rect(0, 0, minWidth, minHeight-1)},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			w := New(tt.bounds, daydata.InMemory(nil, today(weather.Rain, 12, 6)), fixedClock(testTime), unitConfig("C"))
			if inked(render(t, w), tt.bounds) {
				t.Error("drew into bounds too small to hold the block")
			}
		})
	}
}

// No large filled area sits in a fixed position: a black block that lands
// in the same place on every refresh invites ghosting.
func TestWidget_NoLargeFixedFill(t *testing.T) {
	for _, cond := range []weather.Condition{
		weather.Clear, weather.PartlyCloudy, weather.Cloudy, weather.Rain,
		weather.Snow, weather.Thunderstorm, weather.Fog, weather.Drizzle,
	} {
		t.Run(cond.Label(), func(t *testing.T) {
			w := New(goldenBox, daydata.InMemory(nil, today(cond, -12, -18)), fixedClock(testTime), unitConfig("C"))
			if hasSolidSquare(render(t, w), 20) {
				t.Error("found a solid black 20x20 block — a fixed fill is a burn-in risk")
			}
		})
	}
}

// hasSolidSquare reports whether frame holds a side x side square that
// is entirely PaperBlack.
func hasSolidSquare(frame *image.Paletted, side int) bool {
	b := frame.Bounds()
	// run[x] is how many PaperBlack pixels end at (x, y) going up.
	run := make([]int, b.Dx())
	for y := b.Min.Y; y < b.Max.Y; y++ {
		wide := 0
		for x := b.Min.X; x < b.Max.X; x++ {
			i := x - b.Min.X
			if frame.ColorIndexAt(x, y) == widget.PaperBlack {
				run[i]++
			} else {
				run[i] = 0
			}
			if run[i] >= side {
				wide++
			} else {
				wide = 0
			}
			if wide >= side {
				return true
			}
		}
	}
	return false
}

// The guard has to be able to fail.
func TestHasSolidSquare(t *testing.T) {
	frame := image.NewPaletted(image.Rect(0, 0, 100, 100), widget.PaperPalette)
	drawkit.FillWhite(frame, frame.Rect)
	if hasSolidSquare(frame, 20) {
		t.Fatal("blank paper reported a solid square")
	}
	drawkit.FillRect(frame, image.Rect(30, 40, 50, 60), widget.PaperBlack)
	if !hasSolidSquare(frame, 20) {
		t.Error("missed a 20x20 black square")
	}
	if hasSolidSquare(frame, 21) {
		t.Error("reported a 21x21 square inside a 20x20 one")
	}
}

// The widget shows only the weather, so it takes the weather settings
// every day widget takes and no calendar. A calendar key pasted from
// another widget's config is rejected with the reason.
func TestFactory(t *testing.T) {
	deps := widget.Deps{
		Now:      fixedClock(testTime),
		Calendar: calendar.NewProvider(fakehttp.New(), fixedClock(testTime)),
		Weather:  weather.NewProvider(fakehttp.New(), time.Hour, fixedClock(testTime), weather.Settings{TempUnit: "F"}),
	}

	tests := []struct {
		label    string
		config   map[string]any
		wantUnit string
		wantErr  string
	}{
		{label: "no settings inherits the top level", config: nil, wantUnit: "F"},
		{label: "its own weather settings", config: map[string]any{"temp_unit": "C", "latitude": 51.5}, wantUnit: "C"},
		{
			label:   "feeds",
			config:  map[string]any{"feeds": []any{"https://example.com/a.ics"}},
			wantErr: "today-weather: feeds is not supported: today-weather shows only the weather, so it reads no calendar",
		},
		{
			label:   "days",
			config:  map[string]any{"days": 4},
			wantErr: "today-weather: days is not supported: today-weather always shows today; weather-ahead shows the days after it",
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
			if got := w.(*Widget).Config.Weather.TempUnit; got != tt.wantUnit {
				t.Errorf("TempUnit = %q, want %q", got, tt.wantUnit)
			}
		})
	}
}
