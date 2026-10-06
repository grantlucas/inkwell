package daytimeline

import (
	"image"
	"testing"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/weather"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daygrid"
)

// forecastToday is today's forecast with every hour's temperature and
// chance of precipitation from temp and prob, and the day's high and low
// taken from the hours.
func forecastToday(temp func(h int) float64, prob func(h int) float64) []weather.DailyForecast {
	day := weather.DailyForecast{Date: at(0, 0), High: -100, Low: 100}
	for h := range 24 {
		t := temp(h)
		day.High, day.Low = max(day.High, t), min(day.Low, t)
		day.Hourly = append(day.Hourly, weather.HourlyPoint{Hour: h, Temperature: t, PrecipitationProb: prob(h)})
	}
	return []weather.DailyForecast{day}
}

// springDay is a temperature through the day: 1° before dawn, climbing
// to 16° at three in the afternoon and falling away after.
func springDay(h int) float64 { return 16 - float64(max(15-h, h-15)) }

// rainyAfternoon is showers building after midday, peaking at four and
// clearing by the evening.
func rainyAfternoon(h int) float64 {
	return map[int]float64{11: 0.05, 12: 0.2, 13: 0.35, 14: 0.6, 15: 0.85, 16: 0.95, 17: 0.7, 18: 0.4, 19: 0.15}[h]
}

// rainyToday and dryToday are today's forecasts for the goldens.
func rainyToday() []weather.DailyForecast { return forecastToday(springDay, rainyAfternoon) }
func dryToday() []weather.DailyForecast   { return forecastToday(springDay, constant(0.05)) }

// probs is a chance of precipitation per hour, zero for any hour not
// listed.
func probs(byHour map[int]float64) func(int) float64 {
	return func(h int) float64 { return byHour[h] }
}

// constant is the same value every hour.
func constant(v float64) func(int) float64 { return func(int) float64 { return v } }

// renderWeather renders the widget over today's forecast, at now.
func renderWeather(t *testing.T, forecast []weather.DailyForecast, cfg Config, now time.Time) *image.Paletted {
	t.Helper()
	w := New(testBounds, daygrid.InMemory(nil, forecast), fixedClock(now), cfg)
	return renderToFrame(t, w)
}

// reach is how long the ink at row y of the lane is: from its leftmost
// pixel that isn't paper to its rightmost, or 0 when the row is bare.
func reach(frame *image.Paletted, lane image.Rectangle, y int) int {
	first, last := -1, -1
	for x := lane.Min.X; x < lane.Max.X; x++ {
		if frame.ColorIndexAt(x, y) != widget.PaperWhite {
			if first < 0 {
				first = x
			}
			last = x
		}
	}
	if first < 0 {
		return 0
	}
	return last - first + 1
}

// centreOf is the middle row of hour (of the day) on the grid tl maps.
func centreOf(tl timeline, win Window, hour int) int {
	top, bottom := rowOf(tl, hour-win.StartHour)
	return (top + bottom) / 2
}

// The temperature line runs down the lane through each hour's row,
// across today's own range: today's coldest on the plot's left edge and
// its warmest on the right, whatever the days after it do. Each pixel is
// black over paper and white over a bar, so the line reads against a
// bar on both packers.
func TestLane_TemperatureLine(t *testing.T) {
	temps := map[int]float64{10: 20, 15: 10}
	forecast := forecastToday(probs(temps), probs(map[int]float64{13: 1}))
	// Tomorrow is far warmer; the lane draws only today, on its own scale.
	forecast = append(forecast, weather.DailyForecast{Date: at(24, 0), High: 40, Low: 30,
		Hourly: []weather.HourlyPoint{{Hour: 12, Temperature: 40}}})
	frame := renderWeather(t, forecast, defaultConfig(), at(6, 0))
	l, tl := gridOf(testBounds, defaultConfig().Window)
	win := defaultConfig().Window
	left, right := l.Lane.Min.X+lanePadX, l.Lane.Max.X-lanePadX

	inkAt := func(hour int) []int {
		var xs []int
		for x := l.Lane.Min.X; x < l.Lane.Max.X; x++ {
			if frame.ColorIndexAt(x, centreOf(tl, win, hour)) == widget.PaperBlack {
				xs = append(xs, x)
			}
		}
		return xs
	}

	if got := inkAt(19); len(got) != 2 || got[0] != left || got[1] != left+1 {
		t.Errorf("at today's coldest the line is at %v, want black at the plot's left edge %d and %d", got, left, left+1)
	}
	if got := inkAt(10); len(got) == 0 || got[len(got)-1] != right-1 {
		t.Errorf("at today's warmest the line ends at %v, want the plot's right edge %d", got, right-1)
	}

	// The line crosses hour 13's bar at the plot's left, where the bar
	// starts: white there, with the bar's fill beside it.
	y := centreOf(tl, win, 13)
	for _, x := range []int{left, left + 1} {
		if got := frame.ColorIndexAt(x, y); got != widget.PaperWhite {
			t.Errorf("line over the bar at x=%d is index %d, want PaperWhite", x, got)
		}
	}
	if got := frame.ColorIndexAt(left+2, y); got != widget.PaperGray70 {
		t.Errorf("bar beside the line is index %d, want PaperGray70", got)
	}
}

// A dry day still draws the temperature line, and no bars: under the
// combined chart's dry rule a day of trace chances is a row of stubs that
// reads as a broken lane. One wet hour outside the window doesn't make
// the window wet.
func TestLane_DryDayDrawsTheLineAlone(t *testing.T) {
	traces := func(h int) float64 {
		if h == 23 {
			return 0.9
		}
		return 0.1
	}
	frame := renderWeather(t, forecastToday(func(h int) float64 { return float64(h) }, traces), defaultConfig(), at(6, 0))
	l, tl := gridOf(testBounds, defaultConfig().Window)

	if n := countIndexIn(frame, l.Lane, widget.PaperGray70); n != 0 {
		t.Errorf("a dry day draws %d px of bars", n)
	}
	for h := defaultStartHour; h < defaultEndHour; h++ {
		y := centreOf(tl, defaultConfig().Window, h)
		if countIndexIn(frame, image.Rect(l.Lane.Min.X, y, l.Lane.Max.X, y+1), widget.PaperBlack) == 0 {
			t.Errorf("no temperature line in hour %d's row", h)
		}
	}
}

// The lane follows the configured window: its first row is the window's
// first hour, and rain outside the window draws nothing, so a wet night
// doesn't fill a working day's lane.
func TestLane_FollowsTheWindow(t *testing.T) {
	win := Window{StartHour: 9, EndHour: 17}
	cfg := Config{Window: win}
	forecast := forecastToday(constant(10), probs(map[int]float64{8: 1, 9: 1, 17: 1, 23: 1}))
	frame := renderWeather(t, forecast, cfg, at(6, 0))
	l, tl := gridOf(testBounds, win)

	top, bottom := rowOf(tl, 0)
	first := image.Rect(l.Lane.Min.X, top, l.Lane.Max.X, bottom)
	inFirst, inLane := countIndexIn(frame, first, widget.PaperGray70), countIndexIn(frame, l.Lane, widget.PaperGray70)
	if inFirst == 0 {
		t.Error("no bar in the first row for the window's first hour")
	}
	if inLane != inFirst {
		t.Errorf("%d px of bars outside the first row; hours outside the window drew", inLane-inFirst)
	}
}

// Without a forecast the lane is bare paper but for the hour rules
// crossing it: nothing in it can be read as a reading.
func TestLane_NoForecastLeavesItBare(t *testing.T) {
	frame := renderWeather(t, nil, defaultConfig(), at(6, 0))
	l, tl := gridOf(testBounds, defaultConfig().Window)
	for h := defaultStartHour; h < defaultEndHour; h++ {
		top, bottom := rowOf(tl, h-defaultStartHour)
		if n := countIndexIn(frame, image.Rect(l.Lane.Min.X, top+1, l.Lane.Max.X, bottom), widget.PaperWhite); n != l.Lane.Dx()*(bottom-top-1) {
			t.Errorf("hour %d's row has %d px of ink", h, l.Lane.Dx()*(bottom-top-1)-n)
		}
	}
}

// Each hour of the window has a row in the lane, and the chance of
// precipitation that hour is a bar growing sideways from the lane's left:
// a certain hour nearly fills the lane and an even chance half of it.
// The temperature sits at the lane's left, under the bars' start, so the
// bars' far ends are all that reach out.
func TestLane_BarsGrowSideways(t *testing.T) {
	forecast := forecastToday(constant(10), probs(map[int]float64{12: 1, 14: 0.5}))
	frame := renderWeather(t, forecast, defaultConfig(), at(6, 0))
	l, tl := gridOf(testBounds, defaultConfig().Window)

	centre := func(hour int) int { return centreOf(tl, defaultConfig().Window, hour) }
	full, half := reach(frame, l.Lane, centre(12)), reach(frame, l.Lane, centre(14))

	if full == 0 || full < l.Lane.Dx()*3/4 {
		t.Errorf("a certain hour's bar reaches %d of the lane's %d px", full, l.Lane.Dx())
	}
	if diff := 2*half - full; diff < -2 || diff > 2 {
		t.Errorf("an even chance reaches %d px, want about half of %d", half, full)
	}
	if n := countIndexIn(frame, image.Rect(l.Lane.Min.X, centre(13)-2, l.Lane.Max.X, centre(13)+3), widget.PaperGray70); n != 0 {
		t.Errorf("a dry hour has %d px of bar", n)
	}
}
