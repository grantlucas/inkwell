package todayhero

import (
	"image"
	"math"
	"testing"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar"
	"github.com/grantlucas/inkwell/internal/inkwell/calendar/ical"
	"github.com/grantlucas/inkwell/internal/inkwell/testutil"
	"github.com/grantlucas/inkwell/internal/inkwell/testutil/fakehttp"
	"github.com/grantlucas/inkwell/internal/inkwell/weather"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daydata"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/drawkit"
)

// testTime is a Monday mid-afternoon: today's agenda still has events
// to come, and the now-marker falls inside the chart's 06:00-21:00
// window.
var testTime = time.Date(2026, 3, 16, 14, 30, 0, 0, time.UTC)

func fixedClock(t time.Time) func() time.Time { return func() time.Time { return t } }

func newTestFrame(w, h int) *image.Paletted {
	frame := image.NewPaletted(image.Rect(0, 0, w, h), widget.PaperPalette)
	drawkit.FillWhite(frame, frame.Bounds())
	return frame
}

func countIndexIn(frame *image.Paletted, r image.Rectangle, idx uint8) int {
	n := 0
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			if frame.ColorIndexAt(x, y) == idx {
				n++
			}
		}
	}
	return n
}

// sampleForecast covers today plus the four rows, with rain today (so
// the hero chart draws bars and its marker) and a dry last day.
func sampleForecast() []weather.DailyForecast {
	// Wednesday is the cold day, so the shared range shows: its line
	// sits visibly lower than its neighbours'.
	highs := []float64{14, 15, 7, 17, 18}
	lows := []float64{3, 4, -2, 6, 7}
	var days []weather.DailyForecast
	for i := range totalDays {
		high, low := highs[i], lows[i]
		var hourly []weather.HourlyPoint
		for h := range 24 {
			prob := 0.0
			if i < totalDays-1 && h >= 12 && h <= 17 {
				prob = 0.4 + 0.1*float64(i)
			}
			// Coldest at 03:00, warmest at 15:00, spanning the day's
			// low to its high, so the temperature line has a shape.
			hourly = append(hourly, weather.HourlyPoint{
				Hour:              h,
				Temperature:       low + (high-low)*(1-math.Cos(2*math.Pi*float64(h-3)/24))/2,
				PrecipitationProb: prob,
			})
		}
		days = append(days, weather.DailyForecast{
			Date:      time.Date(2026, 3, 16+i, 0, 0, 0, 0, time.UTC),
			High:      high,
			Low:       low,
			Condition: weather.Condition(i % 4),
			Hourly:    hourly,
		})
	}
	return days
}

// dryForecast is the same shape with no rain anywhere: every chart is
// the temperature line alone.
func dryForecast() []weather.DailyForecast {
	f := sampleForecast()
	for i := range f {
		for h := range f[i].Hourly {
			f[i].Hourly[h].PrecipitationProb = 0
		}
	}
	return f
}

// rainyForecast is wet through most of every day, so the temperature
// line spends most of each chart crossing bars and has to turn white
// where it does.
func rainyForecast() []weather.DailyForecast {
	f := sampleForecast()
	for i := range f {
		for h := range f[i].Hourly {
			f[i].Hourly[h].PrecipitationProb = 0.55 + 0.4*math.Abs(math.Sin(float64(h+i)/3))
		}
	}
	return f
}

func ev(summary string, day, hour int) ical.Event {
	start := time.Date(2026, 3, day, hour, 0, 0, 0, time.UTC)
	return ical.Event{UID: summary, Summary: summary, Start: start, End: start.Add(time.Hour)}
}

func sampleEvents() []ical.Event {
	return []ical.Event{
		ev("Standup", 16, 9),        // finished by 14:30
		ev("Design review", 16, 16), // still to come
		ev("1:1", 16, 17),
		ev("Retro", 16, 18),
		ev("Grocery run", 16, 19),
		ev("Dentist", 17, 10),
		{
			UID: "trip", Summary: "Conference", AllDay: true,
			Start: time.Date(2026, 3, 18, 0, 0, 0, 0, time.UTC),
			End:   time.Date(2026, 3, 19, 0, 0, 0, 0, time.UTC),
		},
		ev("Standup", 19, 9), ev("Planning", 19, 11), ev("Review", 19, 14), ev("Demo", 19, 16),
	}
}

// drawConfig is a config with the knobs today-hero draws with.
func drawConfig(unit string, showLocation bool) daydata.Config {
	return daydata.Config{MaxEvents: defaultMaxEvents, ShowLocation: showLocation, Weather: daydata.WeatherConfig{TempUnit: unit}}
}

func newWidget(events []ical.Event, forecast []weather.DailyForecast, clock time.Time) *Widget {
	return New(image.Rect(0, 0, 800, 480), daydata.InMemory(events, forecast), fixedClock(clock), drawConfig("C", false))
}

func renderToFrame(t *testing.T, w *Widget) *image.Paletted {
	t.Helper()
	frame := image.NewPaletted(image.Rect(0, 0, 800, 480), widget.PaperPalette)
	if err := w.Render(frame); err != nil {
		t.Fatalf("Render: %v", err)
	}
	return frame
}

func TestWidget_Bounds(t *testing.T) {
	if got := newWidget(nil, nil, testTime).Bounds(); got != image.Rect(0, 0, 800, 480) {
		t.Errorf("Bounds = %v", got)
	}
}

// The identity band is plain black text on paper with a rule beneath
// it. It used to be a solid black block, and a block that lands in the
// same place on every refresh is the burn-in risk CLAUDE.md rules out:
// today is already obvious from being the left column.
func TestWidget_IdentityIsTextAboveARule(t *testing.T) {
	frame := renderToFrame(t, newWidget(nil, sampleForecast(), testTime))
	// The identity band is the top 116 px of the hero column's day badge.
	band := image.Rect(0, 0, split, 116)

	black := countIndexIn(frame, band, widget.PaperBlack)
	white := countIndexIn(frame, band, widget.PaperWhite)
	// Text on paper is mostly paper. A filled block is mostly ink.
	if black == 0 {
		t.Fatal("the identity band drew nothing")
	}
	if black*4 > white {
		t.Errorf("identity band is %d black / %d white — that is a filled block, not text", black, white)
	}

	// The rule runs across the band's padded width, under the text.
	ruled := false
	for y := band.Max.Y - 1; y >= band.Max.Y-2-2 && !ruled; y-- {
		row := image.Rect(band.Min.X+heroPadX, y, band.Max.X-heroPadX, y+1)
		ruled = countIndexIn(frame, row, widget.PaperBlack) == row.Dx()
	}
	if !ruled {
		t.Error("no rule across the bottom of the identity band")
	}
}

// withTemps returns f with day i's hourly temperatures replaced by
// temp(hour), leaving everything else — rain, highs, lows — untouched.
func withTemps(f []weather.DailyForecast, i int, temp func(hour int) float64) []weather.DailyForecast {
	for h := range f[i].Hourly {
		f[i].Hourly[h].Temperature = temp(f[i].Hourly[h].Hour)
	}
	return f
}

func renderForecast(t *testing.T, f []weather.DailyForecast) *image.Paletted {
	t.Helper()
	return renderToFrame(t, newWidget(sampleEvents(), f, testTime))
}

// Every chart on the screen is the combined chart: the temperature line
// is drawn over the precipitation bars, so the chart follows the day's
// temperature even when it is dry. The behaviour is observed by
// reshaping one day's temperatures and nothing else — if that chart's
// pixels move, the line is being drawn.
func TestWidget_ChartsCarryTheTemperatureLine(t *testing.T) {
	bounds := image.Rect(0, 0, 800, 480)
	rows := computeDayRows(bounds)
	tests := []struct {
		label string
		day   int
		chart image.Rectangle
		base  func() []weather.DailyForecast
	}{
		{"today, dry", 0, computeHero(bounds).Chart, dryForecast},
		{"today, rainy", 0, computeHero(bounds).Chart, sampleForecast},
		{"first row, dry", 1, rowChart(rows[0]), dryForecast},
		{"last row, rainy", dayRows, rowChart(rows[dayRows-1]), sampleForecast},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			before := renderForecast(t, tt.base())
			// A day that warms then cools instead of one that climbs
			// steadily, inside the same overall range so the shared
			// scale itself does not move.
			after := renderForecast(t, withTemps(tt.base(), tt.day, func(h int) float64 {
				return 8 + float64(tt.day) + 2*math.Abs(float64(h-12))/6
			}))
			if testutil.SameIn(before, after, tt.chart) {
				t.Error("reshaping the day's temperatures did not change its chart — no temperature line")
			}
		})
	}
}

// One temperature range serves every chart on the screen, so a cold day
// sits visibly lower than a warm one. Observed from the outside: warming
// only the last day widens the shared range, and that moves the line in
// today's chart even though today's own temperatures did not change.
func TestWidget_ChartsShareOneTemperatureRange(t *testing.T) {
	chart := computeHero(image.Rect(0, 0, 800, 480)).Chart
	before := renderForecast(t, dryForecast())
	after := renderForecast(t, withTemps(dryForecast(), dayRows, func(int) float64 { return 35 }))
	if testutil.SameIn(before, after, chart) {
		t.Error("a hot day four rows down did not move today's line — the charts are not on one range")
	}
}

// No large filled area may sit in a fixed position: a black block that
// lands in the same place on every refresh invites ghosting. Today is
// shown by position alone. Precipitation bars are filled, but they are
// PaperGray70 under a black cap and move with the forecast, so the
// check is for solid black.
func TestWidget_NoLargeFixedFill(t *testing.T) {
	tests := []struct {
		label string
		f     func() []weather.DailyForecast
		clock time.Time
	}{
		{"dry afternoon", dryForecast, testTime},
		{"rainy afternoon", rainyForecast, testTime},
		{"evening", sampleForecast, time.Date(2026, 3, 16, 23, 0, 0, 0, time.UTC)},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			frame := renderToFrame(t, newWidget(sampleEvents(), tt.f(), tt.clock))
			if testutil.HasSolidSquare(frame, 20) {
				t.Error("found a solid black 20x20 block — a fixed fill is a burn-in risk")
			}
		})
	}
}

// Today's agenda shows what is left of the day. An event that finished
// two hours ago is history, and this is the one screen that spends real
// estate on today — spending it on the past would waste the whole idea.
func TestWidget_HeroAgendaShowsOnlyRemainingEvents(t *testing.T) {
	agenda := computeHero(image.Rect(0, 0, 800, 480)).Agenda

	// 08:00: everything is still to come.
	morning := renderToFrame(t, newWidget(sampleEvents(), sampleForecast(),
		time.Date(2026, 3, 16, 8, 0, 0, 0, time.UTC)))

	// 23:00: nothing is.
	night := renderToFrame(t, newWidget(sampleEvents(), sampleForecast(),
		time.Date(2026, 3, 16, 23, 0, 0, 0, time.UTC)))

	morningInk := countIndexIn(morning, agenda, widget.PaperBlack)
	nightInk := countIndexIn(night, agenda, widget.PaperBlack)
	if nightInk >= morningInk {
		t.Errorf("end-of-day agenda has %d px of ink against the morning's %d; it should have collapsed to the done marker",
			nightInk, morningInk)
	}
	if nightInk == 0 {
		t.Error("end of day drew nothing at all — the done marker is missing")
	}
}

// A day row lists up to three events, a time and a title a line, and
// the line after them says how many more there are, in bold like every
// other "+N MORE". The three is today-hero's own: max_events sets the
// hero agenda's cap, not the rows'.
func TestWidget_DayRowListsThreeAndSaysHowManyMore(t *testing.T) {
	var events []ical.Event
	for h := range 5 {
		events = append(events, ev("Event", 17, 9+h))
	}
	cfg := drawConfig("C", false)
	cfg.MaxEvents = 6
	panel := image.Rect(0, 0, 800, 480)
	frame := renderToFrame(t, New(panel, daydata.InMemory(events, nil), fixedClock(testTime), cfg))

	// Tuesday is tomorrow, the first row.
	row := computeDayRows(panel)[0]
	x := row.Min.X + rowAgendaDX
	titleX := x + drawkit.TextWidth(drawkit.BodyFace, "ALL DAY ")
	baseline := func(i int) int { return row.Min.Y + rowPadX + i*drawkit.BodyLineH() + drawkit.BodyAscent() }

	want := newTestFrame(800, 480)
	for i := range 3 {
		drawkit.DrawText(want, x, baseline(i), events[i].Start.Format("15:04"), drawkit.BodyFace, widget.PaperBlack)
		drawkit.DrawText(want, titleX, baseline(i), "Event", drawkit.BodyFace, widget.PaperBlack)
	}
	drawkit.DrawText(want, x, baseline(3), "+2 MORE", drawkit.BodyBoldFace, widget.PaperBlack)

	// The agenda column, above the rule under the row.
	agenda := image.Rect(x, row.Min.Y, row.Max.X, row.Max.Y-1)
	for y := agenda.Min.Y; y < agenda.Max.Y; y++ {
		for xx := agenda.Min.X; xx < agenda.Max.X; xx++ {
			if frame.ColorIndexAt(xx, y) != want.ColorIndexAt(xx, y) {
				t.Fatalf("tomorrow's agenda differs from three events and a bold \"+2 MORE\" at (%d,%d)", xx, y)
			}
		}
	}
}

// A screen missing its events, its forecast or both still draws a usable
// panel: a fetch that failed must not blank it.
func TestWidget_RendersWithMissingData(t *testing.T) {
	tests := []struct {
		label    string
		events   []ical.Event
		forecast []weather.DailyForecast
	}{
		{"no events", nil, sampleForecast()},
		{"no forecast", sampleEvents(), nil},
		{"neither", nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			frame := renderToFrame(t, newWidget(tt.events, tt.forecast, testTime))
			if countIndexIn(frame, frame.Bounds(), widget.PaperBlack) == 0 {
				t.Error("nothing rendered at all")
			}
		})
	}
}

// Every element is placed at a fixed offset from its band and the draw
// helpers clip to the frame, not to the widget's bounds, so a widget
// given less room than the layout needs would paint over its neighbour.
func TestWidget_TooSmallDrawsNothing(t *testing.T) {
	tests := []struct {
		label  string
		bounds image.Rectangle
	}{
		{"too short", image.Rect(0, 0, 800, 200)},
		{"too narrow", image.Rect(0, 0, 400, 480)},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			frame := image.NewPaletted(image.Rect(0, 0, 800, 480), widget.PaperPalette)
			drawkit.FillRect(frame, image.Rect(0, 0, 800, 480), widget.PaperWhite)

			// A neighbour already on the shared frame, in the space
			// this widget would spill into. The draw helpers clip to
			// the frame, not to the widget, so nothing but the guard
			// keeps this intact.
			neighbour := image.Rect(tt.bounds.Max.X-1, tt.bounds.Max.Y, 800, 480)
			if neighbour.Empty() {
				neighbour = image.Rect(0, tt.bounds.Max.Y, 800, 480)
			}
			drawkit.FillRect(frame, neighbour, widget.PaperGray70)

			w := New(tt.bounds, daydata.InMemory(sampleEvents(), sampleForecast()), fixedClock(testTime), drawConfig("C", false))
			if err := w.Render(frame); err != nil {
				t.Fatalf("Render: %v", err)
			}
			if got := countIndexIn(frame, frame.Bounds(), widget.PaperBlack); got != 0 {
				t.Errorf("drew %d px into bounds too small to draw into", got)
			}
			if got := countIndexIn(frame, neighbour, widget.PaperGray70); got != neighbour.Dx()*neighbour.Dy() {
				t.Error("painted over the neighbouring widget")
			}
		})
	}
}

func TestWidget_Golden(t *testing.T) {
	tests := []struct {
		label    string
		events   []ical.Event
		forecast []weather.DailyForecast
		clock    time.Time
		unit     string
		location bool
	}{
		{
			// Mid-afternoon: today still has events, the chart has its
			// bars and marker, one day row overflows to "+N more" and
			// the last row has no events at all.
			label:    "mid-afternoon with events remaining",
			events:   sampleEvents(),
			forecast: sampleForecast(),
		},
		{
			label:    "end of day with nothing left",
			events:   sampleEvents(),
			forecast: sampleForecast(),
			clock:    time.Date(2026, 3, 16, 23, 0, 0, 0, time.UTC),
		},
		{
			// Dry everywhere: every chart is baseline, ticks and the
			// temperature line, and none of them is blank.
			label:    "a dry today",
			events:   sampleEvents(),
			forecast: dryForecast(),
		},
		{
			// Wet everywhere: the line spends most of each chart over
			// bars, where it turns white.
			label:    "a rainy day",
			events:   sampleEvents(),
			forecast: rainyForecast(),
		},
		{
			label:  "no weather at all",
			events: sampleEvents(),
		},
		{
			label:    "no events at all",
			forecast: sampleForecast(),
		},
		{
			label:    "fahrenheit with locations",
			events:   sampleEvents(),
			forecast: sampleForecast(),
			unit:     "F",
			location: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			unit := tt.unit
			if unit == "" {
				unit = "C"
			}
			clock := tt.clock
			if clock.IsZero() {
				clock = testTime
			}
			w := New(image.Rect(0, 0, 800, 480), daydata.InMemory(tt.events, tt.forecast), fixedClock(clock), drawConfig(unit, tt.location))
			testutil.AssertGoldenPNG(t, renderToFrame(t, w))
		})
	}
}

// Factory is the shared day-widget factory, tested in daydata: parsing
// the shared settings, building the day data and taking the clock. What is
// today-hero's own is its default event cap, that it takes the example config, and the reasons it gives for settings it has no use for.
func TestFactory(t *testing.T) {
	deps := widget.Deps{
		Now:      fixedClock(testTime),
		Calendar: calendar.NewProvider(fakehttp.New(), fixedClock(testTime)),
		Weather:  weather.NewProvider(fakehttp.New(), time.Hour, fixedClock(testTime), weather.Settings{}),
	}

	tests := []struct {
		label   string
		config  map[string]any
		wantErr string
	}{
		{
			// The today-hero screen in inkwell.example.yaml.
			label: "the example config",
			config: map[string]any{
				"feeds":         []any{"https://example.com/my-calendar.ics"},
				"max_events":    3,
				"show_location": false,
				"refresh":       "15m",
			},
		},
		{label: "caps events at its default", config: map[string]any{"feeds": []any{"https://example.com/a.ics"}}},
		{
			label:   "explains a weekly-calendar key",
			config:  map[string]any{"feeds": []any{"https://example.com/a.ics"}, "show_weather": false},
			wantErr: "today-hero: show_weather is not supported: the weather band is part of the layout; a day with no forecast already draws nothing",
		},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			w, err := Factory(image.Rect(0, 0, 800, 480), tt.config, deps)
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("err = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Factory: %v", err)
			}
			if got := w.(*Widget).Config.MaxEvents; got != defaultMaxEvents {
				t.Errorf("MaxEvents = %d, want %d", got, defaultMaxEvents)
			}
		})
	}
}
