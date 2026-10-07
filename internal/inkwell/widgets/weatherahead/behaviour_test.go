package weatherahead

import (
	"image"
	"testing"

	"github.com/grantlucas/inkwell/internal/inkwell/testutil"
	"github.com/grantlucas/inkwell/internal/inkwell/weather"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daydata"
)

// The charts share one range taken across the widget's own rows: change
// only the last day's temperatures and the first row's chart moves.
// Today is left out of it, since today is another widget's: however hot
// today is, nothing this widget draws changes.
func TestWidget_SharedRangeIsTheRowsOwn(t *testing.T) {
	week := func(today, friday day) []weather.DailyForecast {
		return forecast(today, day{weather.Rain, 12, 6, 0.9}, day{weather.Cloudy, 10, 4, 0}, friday)
	}
	mildFriday := day{weather.Clear, 14, 5, 0}
	tests := []struct {
		label     string
		forecast  []weather.DailyForecast
		wantMoved bool
	}{
		{label: "a scorching today", forecast: week(day{weather.Clear, 38, 25, 0}, mildFriday), wantMoved: false},
		{label: "a freezing friday", forecast: week(today, day{weather.Snow, -15, -25, 0.5}), wantMoved: true},
	}
	cfg := config(3, "C")
	base := render(t, New(goldenBox, daydata.InMemory(nil, week(today, mildFriday)), fixedClock(testTime), cfg))
	firstRow := image.Rect(0, 0, goldenBox.Dx(), goldenBox.Dy()/3)
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			got := render(t, New(goldenBox, daydata.InMemory(nil, tt.forecast), fixedClock(testTime), cfg))
			if moved := !testutil.SameIn(base, got, firstRow); moved != tt.wantMoved {
				t.Errorf("first row changed = %v, want %v", moved, tt.wantMoved)
			}
		})
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
		days   int
	}{
		{"right column", rightColumn, 4},
		{"mid panel", image.Rect(200, 100, 200+rightColumn.Dx(), 100+rightColumn.Dy()), 4},
		{"just big enough", image.Rect(300, 100, 300+minWidth, 100+4*minRowH), 4},
		{"a week down the panel", image.Rect(534, 0, 800, 480), 7},
	}
	wet := func(cond weather.Condition, high, low float64) day { return day{cond, high, low, 1} }
	forecasts := []struct {
		label    string
		forecast []weather.DailyForecast
		unit     string
	}{
		{"mixed", mixedWeek, "C"},
		{"deep cold", forecast(today, wet(weather.PartlyCloudy, -12, -18), wet(weather.Snow, -10, -14),
			wet(weather.Thunderstorm, -11, -17), wet(weather.Drizzle, -12, -18)), "C"},
		{"hot fahrenheit", forecast(today, wet(weather.PartlyCloudy, 38, 26), wet(weather.Clear, 40, 30),
			wet(weather.Fog, 38, 26), wet(weather.Rain, 37, 25)), "F"},
		{"no forecast", nil, "C"},
	}
	for _, p := range placements {
		for _, f := range forecasts {
			t.Run(p.label+" "+f.label, func(t *testing.T) {
				frame := image.NewPaletted(image.Rect(0, 0, 800, 480), widget.PaperPalette)
				testutil.PaintOutside(frame, p.bounds)
				w := New(p.bounds, daydata.InMemory(nil, f.forecast), fixedClock(testTime), config(p.days, f.unit))
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
				if !testutil.Inked(frame, p.bounds) {
					t.Error("drew nothing")
				}
			})
		}
	}
}

// Too small to hold its rows, the widget draws nothing rather than
// spilling onto its neighbours: blank bounds are a misconfiguration an
// operator can see, ink on another widget looks like a fault elsewhere.
func TestWidget_TooSmallDrawsNothing(t *testing.T) {
	tests := []struct {
		label  string
		bounds image.Rectangle
		days   int
	}{
		{"too narrow", image.Rect(0, 0, minWidth-1, 4*minRowH), 4},
		{"too short for four days", image.Rect(0, 0, minWidth, 4*minRowH-1), 4},
		{"too short for a week", rightColumn, 7},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			w := New(tt.bounds, daydata.InMemory(nil, mixedWeek), fixedClock(testTime), config(tt.days, "C"))
			if testutil.Inked(render(t, w), tt.bounds) {
				t.Error("drew into bounds too small to hold the rows")
			}
		})
	}
}

// No large filled area sits in a fixed position: a black block that lands
// in the same place on every refresh invites ghosting. Rain all afternoon
// in every row puts the most ink there is on the widget.
func TestWidget_NoLargeFixedFill(t *testing.T) {
	for _, cond := range []weather.Condition{
		weather.Clear, weather.PartlyCloudy, weather.Cloudy, weather.Rain,
		weather.Snow, weather.Thunderstorm, weather.Fog, weather.Drizzle,
	} {
		t.Run(cond.Label(), func(t *testing.T) {
			wet := day{cond, -12, -18, 1}
			w := New(goldenBox, daydata.InMemory(nil, forecast(today, wet, wet, wet, wet)), fixedClock(testTime), config(4, "C"))
			if testutil.HasSolidSquare(render(t, w), 20) {
				t.Error("found a solid black 20x20 block — a fixed fill is a burn-in risk")
			}
		})
	}
}
