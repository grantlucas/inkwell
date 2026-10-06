package daybadge_test

import (
	"bytes"
	"image"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/weather"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daybadge"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daygrid"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/drawkit"
)

// testTime is a Monday mid-afternoon.
var testTime = time.Date(2026, 3, 16, 14, 30, 0, 0, time.UTC)

// styles is every style, for the rules every badge keeps.
var styles = []daybadge.Style{daybadge.Column, daybadge.Row, daybadge.Compact, daybadge.Hero}

// forecastFor is a forecast for March 16+i with the given high and low.
func forecastFor(i int, cond weather.Condition, high, low float64) *weather.DailyForecast {
	var hourly []weather.HourlyPoint
	for h := range 24 {
		hourly = append(hourly, weather.HourlyPoint{Hour: h, Temperature: low + (high-low)*math.Sin(math.Pi*float64(h)/24)})
	}
	return &weather.DailyForecast{
		Date: time.Date(2026, 3, 16+i, 0, 0, 0, 0, time.UTC), High: high, Low: low, Condition: cond, Hourly: hourly,
	}
}

// dayAt is March 16+i as the day data module builds it from testTime.
func dayAt(i int, f *weather.DailyForecast) daygrid.Day {
	return daygrid.Day{Start: time.Date(2026, 3, 16+i, 0, 0, 0, 0, time.UTC), IsToday: i == 0, Forecast: f}
}

func newFrame() *image.Paletted {
	return image.NewPaletted(image.Rect(0, 0, 500, 300), widget.PaperPalette)
}

// at is the rect a style draws into in these tests: its own size, set
// in from the frame's corner so ink on any side of it would show.
func at(s daybadge.Style) image.Rectangle {
	return image.Rectangle{Min: image.Pt(40, 30), Max: image.Pt(40, 30).Add(s.Size())}
}

func draw(s daybadge.Style, d daygrid.Day, unit string) *image.Paletted {
	frame := newFrame()
	s.Draw(frame, at(s), d, testTime, unit)
	return frame
}

func inkIn(frame *image.Paletted, r image.Rectangle) int {
	n := 0
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			if frame.ColorIndexAt(x, y) != widget.PaperWhite {
				n++
			}
		}
	}
	return n
}

// A badge's readings stop at its right edge, whatever they read, so the
// widget that draws it can put a chart or a list right beside it. The
// widest readings are the ones that would overrun: deep cold, and three
// digits of Fahrenheit. (The condition icons are anti-aliased glyphs
// whose rays overrun their box by a few pixels on the left and below;
// the full-screen widgets have always drawn them that way, and the
// day-badge widget clips to its bounds.)
func TestStyle_ReadingsStopAtTheRightEdge(t *testing.T) {
	readings := []struct {
		label string
		f     *weather.DailyForecast
		unit  string
	}{
		{"deep cold", forecastFor(0, weather.Snow, -15, -22), "C"},
		{"three-digit fahrenheit", forecastFor(0, weather.PartlyCloudy, 37.8, 20), "F"},
		{"no forecast", nil, "C"},
	}
	for _, s := range styles {
		for _, r := range readings {
			t.Run(s.String()+"/"+r.label, func(t *testing.T) {
				frame := draw(s, dayAt(0, r.f), r.unit)
				right := image.Rect(at(s).Max.X, 0, frame.Bounds().Max.X, frame.Bounds().Max.Y)
				if got := inkIn(frame, right); got != 0 {
					t.Errorf("%d px of ink past the right edge of %v", got, at(s))
				}
			})
		}
	}
}

// What a badge says depends on the forecast it has. A day the forecast
// doesn't reach still names the day, but draws no weather: a zero would
// state a temperature nobody forecast. The guard keys off whether there
// is a forecast, not its value, so a real 0° is drawn; and the reading
// is converted to the unit asked for, so 12°C and 54°F differ.
func TestStyle_Weather(t *testing.T) {
	for _, s := range styles {
		t.Run(s.String(), func(t *testing.T) {
			none := inkIn(draw(s, dayAt(0, nil), "C"), at(s))
			zero := inkIn(draw(s, dayAt(0, forecastFor(0, weather.Clear, 0, 0)), "C"), at(s))
			celsius := draw(s, dayAt(0, forecastFor(0, weather.Clear, 12, 4)), "C")
			fahrenheit := draw(s, dayAt(0, forecastFor(0, weather.Clear, 12, 4)), "F")

			if none == 0 {
				t.Error("a day with no forecast drew no date")
			}
			if zero <= none {
				t.Errorf("a real 0° forecast drew %d px against %d for none", zero, none)
			}
			if bytes.Equal(celsius.Pix, fahrenheit.Pix) {
				t.Error("C and F drew the same pixels; the conversion may not be applied")
			}
		})
	}
}

// Today is shown by position, never by a fill or an outline: a badge is
// text on paper, mostly paper. Only the hero's fuzzy clock differs on
// today, since "now" belongs to today alone.
func TestStyle_TodayIsNotHighlighted(t *testing.T) {
	for _, s := range styles {
		t.Run(s.String(), func(t *testing.T) {
			f := forecastFor(0, weather.Rain, 12, 4)
			today := draw(s, dayAt(0, f), "C")
			notToday := dayAt(0, f)
			notToday.IsToday = false
			other := draw(s, notToday, "C")

			if ink, area := inkIn(today, at(s)), s.Size().X*s.Size().Y; ink*4 > area {
				t.Errorf("badge is %d of %d px ink — that is a filled block, not text", ink, area)
			}
			if same := bytes.Equal(today.Pix, other.Pix); same == (s == daybadge.Hero) {
				t.Errorf("today and another day drew the same pixels: %v; want %v", same, s != daybadge.Hero)
			}
		})
	}
}

// The hero writes the fuzzy clock under the month on today only, in the
// band between the month's baseline and the rule under them.
func TestStyle_HeroClockIsTodays(t *testing.T) {
	r := at(daybadge.Hero)
	clock := image.Rect(r.Min.X, r.Min.Y+86, r.Max.X, r.Min.Y+108)

	if inkIn(draw(daybadge.Hero, dayAt(0, nil), "C"), clock) == 0 {
		t.Error("no fuzzy clock on today's hero")
	}
	tomorrow := draw(daybadge.Hero, dayAt(1, nil), "C")
	if got := inkIn(tomorrow, clock); got != 0 {
		t.Errorf("tomorrow's hero drew %d px where the clock goes", got)
	}
}

// The compact badge tags tomorrow "TOMORROW" rather than its weekday,
// so the first of today-hero's rows reads as the next day at a glance.
// Other days carry their weekday.
func TestStyle_CompactTagsTomorrow(t *testing.T) {
	r := at(daybadge.Compact)
	tag := image.Rect(r.Min.X, r.Min.Y, r.Max.X, r.Min.Y+24)
	tests := []struct {
		label string
		day   int
		want  string
	}{
		{"tomorrow", 1, "TOMORROW"},
		{"the day after", 2, "WED"},
		{"today", 0, "MON"},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			got := draw(daybadge.Compact, dayAt(tt.day, nil), "C")
			want := newFrame()
			drawkit.DrawText(want, r.Min.X+12, r.Min.Y+20, tt.want, drawkit.BodyFace, widget.PaperBlack)
			for y := tag.Min.Y; y < tag.Max.Y; y++ {
				for x := tag.Min.X; x < tag.Max.X; x++ {
					if got.ColorIndexAt(x, y) != want.ColorIndexAt(x, y) {
						t.Fatalf("tag differs from %q at (%d,%d)", tt.want, x, y)
					}
				}
			}
		})
	}
}

// A widget's config names its style. The names are what an operator
// types, so each one round-trips, and anything else is refused.
func TestParseStyle(t *testing.T) {
	for _, s := range styles {
		got, ok := daybadge.ParseStyle(s.String())
		if !ok || got != s {
			t.Errorf("ParseStyle(%q) = %v, %v", s.String(), got, ok)
		}
	}
	if _, ok := daybadge.ParseStyle("header"); ok {
		t.Error(`ParseStyle("header") accepted a style that doesn't exist`)
	}
	if got := daybadge.StyleNames(); !reflect.DeepEqual(got, []string{"column", "row", "compact", "hero"}) {
		t.Errorf("StyleNames = %v", got)
	}
}

// Each style's size is the room the full-screen widget it comes from
// gives it.
func TestStyle_Size(t *testing.T) {
	want := map[daybadge.Style]image.Point{
		daybadge.Column:  {160, 156},
		daybadge.Row:     {234, 76},
		daybadge.Compact: {176, 64},
		daybadge.Hero:    {338, 202},
	}
	for s, size := range want {
		if got := s.Size(); got != size {
			t.Errorf("%v.Size() = %v, want %v", s, got, size)
		}
	}
}
