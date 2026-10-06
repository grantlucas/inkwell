package rowagenda

import (
	"fmt"
	"image"
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
	var days []weather.DailyForecast
	for i := range rows {
		var hourly []weather.HourlyPoint
		for h := range 24 {
			prob := 0.0
			if i < rows-1 && h >= 12 && h <= 17 {
				prob = 0.4 + 0.1*float64(i)
			}
			hourly = append(hourly, weather.HourlyPoint{
				Hour:              h,
				Temperature:       8 + float64(i) + float64(h)/6,
				PrecipitationProb: prob,
			})
		}
		days = append(days, weather.DailyForecast{
			Date:      time.Date(2026, 3, 16+i, 0, 0, 0, 0, time.UTC),
			High:      14 + float64(i),
			Low:       3 + float64(i),
			Condition: weather.Condition(i % 4),
			Hourly:    hourly,
		})
	}
	return days
}

// dryForecast is the same shape with no rain anywhere, for the
// "NO RAIN TODAY" path.
func dryForecast() []weather.DailyForecast {
	f := sampleForecast()
	for i := range f {
		for h := range f[i].Hourly {
			f[i].Hourly[h].PrecipitationProb = 0
		}
	}
	return f
}

func ev(summary string, day, hour int) ical.Event {
	start := time.Date(2026, 3, day, hour, 0, 0, 0, time.UTC)
	return ical.Event{UID: summary, Summary: summary, Start: start, End: start.Add(time.Hour)}
}

// sampleEvents is a busy week that still fits: five events today, four
// on Thursday and an empty Friday, with every event getting a line.
func sampleEvents() []ical.Event {
	return []ical.Event{
		ev("Standup", 16, 9), // finished by 14:30
		ev("Platform architecture review with infra", 16, 16), // still to come
		ev("1:1", 16, 17),
		ev("Retro", 16, 18),
		ev("Grocery run", 16, 19),
		ev("Dentist - Maeve", 17, 10),
		{
			UID: "trip", Summary: "Conference", AllDay: true,
			Start: time.Date(2026, 3, 18, 0, 0, 0, 0, time.UTC),
			End:   time.Date(2026, 3, 19, 0, 0, 0, 0, time.UTC),
		},
		ev("Standup", 19, 9), ev("Quarterly planning", 19, 11), ev("Review", 19, 14), ev("Demo", 19, 16),
	}
}

// quietEvents is a week with almost nothing in it, so every row sits
// at the minimum height plus an even share of the spare room.
func quietEvents() []ical.Event {
	return []ical.Event{
		ev("Sabres @ Stoney Creek", 16, 10),
		ev("Practice Green", 19, 18),
	}
}

// overflowingEvents is more than five rows can hold: Monday, Wednesday
// and Friday are packed and lose lines, while the quiet Tuesday and
// Thursday keep every event.
func overflowingEvents() []ical.Event {
	var out []ical.Event
	for h := range 9 {
		out = append(out, ev("Back-to-back meeting number "+string(rune('A'+h)), 16, 8+h))
	}
	out = append(out, ev("Dentist", 17, 10), ev("Book club", 17, 19))
	for h := range 7 {
		out = append(out, ev("Workshop session "+string(rune('A'+h)), 18, 9+h))
	}
	out = append(out, ev("Swim lessons", 19, 17))
	for h := range 6 {
		out = append(out, ev("Interview loop "+string(rune('A'+h)), 20, 9+h))
	}
	return out
}

// withoutToday drops today's events, leaving today's row empty.
func withoutToday(events []ical.Event) []ical.Event {
	var out []ical.Event
	for _, e := range events {
		if e.Start.Day() != 16 {
			out = append(out, e)
		}
	}
	return out
}

// drawConfig is a config with the knobs row-agenda draws with.
func drawConfig(unit string, showLocation bool) daydata.Config {
	return daydata.Config{ShowLocation: showLocation, Weather: daydata.WeatherConfig{TempUnit: unit}}
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
			// A neighbour already on the frame, outside these bounds.
			neighbour := image.Rect(0, 481-1, 800, 480)
			_ = neighbour
			drawkit.FillRect(frame, image.Rect(0, 0, 800, 480), widget.PaperWhite)

			w := New(tt.bounds, daydata.InMemory(sampleEvents(), sampleForecast()), fixedClock(testTime), drawConfig("C", false))
			if err := w.Render(frame); err != nil {
				t.Fatalf("Render: %v", err)
			}
			if got := countIndexIn(frame, frame.Bounds(), widget.PaperBlack); got != 0 {
				t.Errorf("drew %d px into bounds too small to draw into", got)
			}
		})
	}
}

// Today is shown by position — it is the first row — and never by a
// fill. The date gutter is the same plain date on every row, so no
// large black area lands in the same place every refresh and burns into
// the panel.
func TestWidget_NoFilledDateGutter(t *testing.T) {
	frame := renderToFrame(t, newWidget(sampleEvents(), sampleForecast(), testTime))

	// gutterW is the date block at the left of each row's day badge.
	const band, gutterW = 48, 122
	for y := 0; y < 480; y += band {
		r := image.Rect(0, y, gutterW, y+band)
		if black := countIndexIn(frame, r, widget.PaperBlack); black > r.Dx()*r.Dy()/4 {
			t.Errorf("the gutter at y=%d is %d/%d black — a filled block, not a plain date", y, black, r.Dx()*r.Dy())
		}
	}
	if countIndexIn(frame, image.Rect(0, 0, gutterW, 480), widget.PaperWhite) == 0 {
		t.Error("the gutter has no paper at all")
	}
}

// A row that lost lines to a crowded week gives the last line it kept to
// "+N MORE", counting every event it could not show, rather than
// overprinting an event or dropping them silently.
func TestWidget_ARowThatLostEventsSaysHowMany(t *testing.T) {
	frame := renderToFrame(t, newWidget(overflowingEvents(), sampleForecast(), testTime))

	// Monday carries nine events and is the busiest row, so it loses
	// lines first.
	row := planRows(panel, []int{9, 2, 7, 1, 6})[0]
	if row.Lines >= 9 {
		t.Fatalf("Monday kept %d lines for 9 events; the week should have trimmed it", row.Lines)
	}
	x := row.Agenda.Min.X + agendaPadX
	baseline := row.Agenda.Min.Y + agendaPadY + (row.Lines-1)*drawkit.BodyLineH() + drawkit.BodyAscent()
	box := image.Rect(x, baseline-drawkit.BodyAscent(), row.Agenda.Max.X, baseline-drawkit.BodyAscent()+drawkit.BodyLineH())

	want := newTestFrame(800, 480)
	drawkit.DrawText(want, x, baseline, fmt.Sprintf("+%d MORE", 9-(row.Lines-1)), drawkit.BodyBoldFace, widget.PaperBlack)
	for y := box.Min.Y; y < box.Max.Y; y++ {
		for xx := box.Min.X; xx < box.Max.X; xx++ {
			if frame.ColorIndexAt(xx, y) != want.ColorIndexAt(xx, y) {
				t.Fatalf("Monday's last line differs from %q at (%d,%d)", fmt.Sprintf("+%d MORE", 9-(row.Lines-1)), xx, y)
			}
		}
	}
}

// At the narrowest bounds the widget accepts, "NOTHING SCHEDULED" is
// wider than the agenda. It is cut like every other line, so it never
// paints past the widget's edge over whatever shares the frame.
func TestWidget_EmptyDayStaysInBounds(t *testing.T) {
	frame := newTestFrame(800, 480)
	w := New(image.Rect(0, 0, minWidth, 480), daydata.InMemory(nil, nil), fixedClock(testTime), drawConfig("C", false))
	if err := w.Render(frame); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if got := countIndexIn(frame, image.Rect(minWidth, 0, 800, 480), widget.PaperBlack); got != 0 {
		t.Errorf("drew %d px past the widget's %d px edge", got, minWidth)
	}
	if countIndexIn(frame, image.Rect(agendaX, 0, minWidth, 480), widget.PaperBlack) == 0 {
		t.Error("the empty days said nothing")
	}
}

func TestWidget_Golden(t *testing.T) {
	tests := []struct {
		label    string
		events   []ical.Event
		forecast []weather.DailyForecast
		unit     string
		location bool
	}{
		{
			// Every event gets a line: today's row and Thursday's grow,
			// the rest share the spare room, Friday says it is empty.
			label:    "a busy week that fits",
			events:   sampleEvents(),
			forecast: sampleForecast(),
		},
		{
			label:    "a quiet week",
			events:   quietEvents(),
			forecast: sampleForecast(),
		},
		{
			// The packed rows trim toward each other and end in
			// "+N MORE"; the quiet rows keep everything.
			label:    "a week that overflows",
			events:   overflowingEvents(),
			forecast: sampleForecast(),
		},
		{
			// Today's own row is the empty one.
			label:    "an empty day",
			events:   withoutToday(sampleEvents()),
			forecast: sampleForecast(),
		},
		{
			label:    "no events at all",
			forecast: sampleForecast(),
		},
		{
			label:    "a dry week",
			events:   sampleEvents(),
			forecast: dryForecast(),
		},
		{
			label:  "no weather at all",
			events: sampleEvents(),
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
			w := New(image.Rect(0, 0, 800, 480), daydata.InMemory(tt.events, tt.forecast), fixedClock(testTime), drawConfig(unit, tt.location))
			testutil.AssertGoldenPNG(t, renderToFrame(t, w))
		})
	}
}

// Factory is the shared day-widget factory, tested in daydata: parsing
// the shared settings, building the day data and taking the clock. What is
// row-agenda's own is that it takes the example config and the reasons it gives for settings it has no use for.
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
			// The row-agenda screen in inkwell.example.yaml.
			label: "the example config",
			config: map[string]any{
				"feeds":         []any{"https://example.com/my-calendar.ics"},
				"show_location": false,
				"refresh":       "15m",
			},
		},
		{
			// Rows grow to fit their events, so a cap could only
			// contradict that, and the error says so.
			label:   "explains max_events",
			config:  map[string]any{"feeds": []any{"https://example.com/a.ics"}, "max_events": 3},
			wantErr: "row-agenda: max_events is not supported: each row grows to fit its events, and when the week is too full the busiest rows give up lines first",
		},
		{
			label:   "explains a weekly-calendar key",
			config:  map[string]any{"feeds": []any{"https://example.com/a.ics"}, "show_weather": false},
			wantErr: "row-agenda: show_weather is not supported: the weather badge is part of the layout; a day with no forecast already draws nothing",
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
			if _, ok := w.(*Widget); !ok {
				t.Errorf("Factory built a %T", w)
			}
		})
	}
}
