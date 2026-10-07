package eventlist_test

import (
	"image"
	"testing"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar"
	"github.com/grantlucas/inkwell/internal/inkwell/testutil"
	"github.com/grantlucas/inkwell/internal/inkwell/testutil/fakehttp"
	"github.com/grantlucas/inkwell/internal/inkwell/weather"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daydata"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/eventlist"
)

// widgetTime is a Monday mid-afternoon in Toronto, so today has both
// finished and upcoming events.
var widgetTime = time.Date(2026, 3, 16, 14, 30, 0, 0, toronto)

func fixedClock(t time.Time) func() time.Time { return func() time.Time { return t } }

// at is a Toronto time on day of March 2026, written in UTC the way a
// feed might serialize it, so a list that forgot the display zone would
// show it five hours out.
func at(day, hour int) time.Time {
	return time.Date(2026, 3, day, hour, 0, 0, 0, toronto).UTC()
}

func event(summary string, day, hour int) calendar.Event {
	return calendar.Event{UID: summary, Summary: summary, Start: at(day, hour), End: at(day, hour+1)}
}

var (
	standup = event("Standup", 16, 9)
	review  = event("Design review", 16, 15)
	dinner  = event("Dinner with the Okafors", 16, 18)
	dentist = event("Dentist", 17, 10)
	movie   = event("Movie night", 18, 20)
	allWeek = calendar.Event{
		UID: "trip", Summary: "Conference", AllDay: true,
		Start: time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC), End: time.Date(2026, 3, 18, 0, 0, 0, 0, time.UTC),
	}
	weekEvents = []calendar.Event{standup, review, dinner, dentist, movie, allWeek}
)

// listConfig is a parsed config listing day in style.
func listConfig(style eventlist.Style, day, maxEvents int) eventlist.Config {
	return eventlist.Config{
		Config: daydata.Config{MaxEvents: maxEvents, Day: day},
		Style:  style,
	}
}

func newList(bounds image.Rectangle, events []calendar.Event, cfg eventlist.Config) *eventlist.Widget {
	return eventlist.New(bounds, daydata.InMemory(events, nil), fixedClock(widgetTime), cfg)
}

func renderList(t *testing.T, w *eventlist.Widget, size image.Rectangle) *image.Paletted {
	t.Helper()
	frame := image.NewPaletted(size, widget.PaperPalette)
	if err := w.Render(frame); err != nil {
		t.Fatalf("Render: %v", err)
	}
	return frame
}

// The widget lists its one day's events, the way its style lists them,
// into its bounds: today unless its config names a later day, the events
// in the display zone, capped at max_events, and with hide_finished only
// what is left of the day. Each row gives the events the list should end
// up with, so the expected frame is the style drawing exactly those.
func TestWidget_ListsItsDay(t *testing.T) {
	bounds := image.Rect(20, 30, 330, 300)
	str := func(s string) *string { return &s }

	tests := []struct {
		label string
		cfg   eventlist.Config
		want  []calendar.Event
		empty string
	}{
		{"today, all-day first", listConfig(eventlist.StackedStyle, 0, 4), []calendar.Event{allWeek, standup, review, dinner}, ""},
		{"tomorrow", listConfig(eventlist.InlineStyle, 1, 3), []calendar.Event{allWeek, dentist}, ""},
		{"capped at max_events", listConfig(eventlist.LargeStyle, 0, 2), []calendar.Event{allWeek, standup, review, dinner}, ""},
		{"only what is left of today", eventlist.Config{
			Config: daydata.Config{MaxEvents: 3}, Style: eventlist.LargeStyle, HideFinished: true,
		}, []calendar.Event{allWeek, review, dinner}, ""},
		{"an empty day says what the config says", eventlist.Config{
			Config: daydata.Config{MaxEvents: 3, Day: 3}, Style: eventlist.LargeStyle, Empty: str("ALL CLEAR"),
		}, nil, "ALL CLEAR"},
		{"an empty day says the style's note", listConfig(eventlist.InlineStyle, 3, 3), nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			frame := renderList(t, newList(bounds, weekEvents, tt.cfg), image.Rect(0, 0, 400, 400))

			ref := image.NewPaletted(image.Rect(0, 0, 400, 400), widget.PaperPalette)
			style := tt.cfg.Style.List(tt.cfg.MaxEvents, false, toronto)
			if tt.empty != "" {
				style.Empty.Text = tt.empty
			}
			style.Draw(ref, bounds, tt.want)
			assertFrame(t, frame, ref, nil)
		})
	}
}

// Every widget clears its own bounds and draws nothing outside them, so
// it can be placed beside any other on the shared frame.
func TestWidget_KeepsToItsBounds(t *testing.T) {
	bounds := image.Rect(100, 100, 260, 300)
	frame := image.NewPaletted(image.Rect(0, 0, 400, 400), widget.PaperPalette)
	for y := range 400 {
		for x := range 400 {
			frame.SetColorIndex(x, y, widget.PaperBlack)
		}
	}

	w := newList(bounds, weekEvents, listConfig(eventlist.LargeStyle, 0, 4))
	if err := w.Render(frame); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if w.Bounds() != bounds {
		t.Errorf("Bounds = %v, want %v", w.Bounds(), bounds)
	}
	white := 0
	for y := range 400 {
		for x := range 400 {
			in := image.Pt(x, y).In(bounds)
			if !in && frame.ColorIndexAt(x, y) != widget.PaperBlack {
				t.Fatalf("drew outside its bounds at (%d,%d)", x, y)
			}
			if in && frame.ColorIndexAt(x, y) == widget.PaperWhite {
				white++
			}
		}
	}
	if white < bounds.Dx()*bounds.Dy()/2 {
		t.Errorf("only %d px of its bounds are paper; it did not clear them", white)
	}
}

// Golden renders at the sizes the full-screen widgets give their lists.
func TestWidget_Golden(t *testing.T) {
	tests := []struct {
		label  string
		cfg    eventlist.Config
		bounds image.Rectangle
	}{
		{"stacked in a bold-five column", listConfig(eventlist.StackedStyle, 0, 4), image.Rect(0, 0, 148, 272)},
		{"large for what is left of today", eventlist.Config{
			Config: daydata.Config{MaxEvents: 3}, Style: eventlist.LargeStyle, HideFinished: true,
		}, image.Rect(0, 0, 310, 198)},
		{"inline for tomorrow", listConfig(eventlist.InlineStyle, 1, 3), image.Rect(0, 0, 440, 96)},
		{"stacked on an empty day", listConfig(eventlist.StackedStyle, 3, 4), image.Rect(0, 0, 148, 272)},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			testutil.AssertGoldenPNG(t, renderList(t, newList(tt.bounds, weekEvents, tt.cfg), tt.bounds))
		})
	}
}

// The widget lists events only, so it takes the calendar settings, day
// and its own: which style, whether finished events are dropped, and
// what an empty day says. max_events defaults to what the full-screen
// widget the style comes from shows.
func TestFactory(t *testing.T) {
	deps := widget.Deps{
		Now:      fixedClock(widgetTime),
		Calendar: calendar.NewProvider(fakehttp.New(), fixedClock(widgetTime)),
		Weather:  weather.NewProvider(fakehttp.New(), time.Hour, fixedClock(widgetTime), weather.Settings{}),
	}
	feeds := []any{"https://example.com/a.ics"}
	with := func(kv ...any) map[string]any {
		c := map[string]any{"feeds": feeds}
		for i := 0; i < len(kv); i += 2 {
			c[kv[i].(string)] = kv[i+1]
		}
		return c
	}
	bounds := image.Rect(0, 0, 160, 280)

	tests := []struct {
		label   string
		config  map[string]any
		check   func(*testing.T, eventlist.Config)
		wantErr string
	}{
		{label: "stacked today, four events, by default", config: with(), check: func(t *testing.T, c eventlist.Config) {
			if c.Style != eventlist.StackedStyle || c.Day != 0 || c.MaxEvents != 4 || c.HideFinished || c.Empty != nil {
				t.Errorf("Config = %+v", c)
			}
		}},
		{label: "large shows three", config: with("style", "large"), check: func(t *testing.T, c eventlist.Config) {
			if c.Style != eventlist.LargeStyle || c.MaxEvents != 3 {
				t.Errorf("Style, MaxEvents = %v, %d", c.Style, c.MaxEvents)
			}
		}},
		{label: "inline shows three", config: with("style", "inline"), check: func(t *testing.T, c eventlist.Config) {
			if c.Style != eventlist.InlineStyle || c.MaxEvents != 3 {
				t.Errorf("Style, MaxEvents = %v, %d", c.Style, c.MaxEvents)
			}
		}},
		{
			label:  "every setting",
			config: with("style", "inline", "day", 2, "max_events", 6, "hide_finished", true, "empty", "", "show_location", true),
			check: func(t *testing.T, c eventlist.Config) {
				if c.Day != 2 || c.MaxEvents != 6 || !c.HideFinished || c.Empty == nil || *c.Empty != "" || !c.ShowLocation {
					t.Errorf("Config = %+v", c)
				}
			},
		},
		{
			label: "an unknown style", config: with("style", "column"),
			wantErr: `event-list: style must be one of stacked, large, inline, got "column"`,
		},
		{label: "style not a string", config: with("style", 2), wantErr: "event-list: style must be a string, got int"},
		{label: "hide_finished not a bool", config: with("hide_finished", "yes"), wantErr: "event-list: hide_finished must be a bool, got string"},
		{label: "empty not a string", config: with("empty", 0), wantErr: "event-list: empty must be a string, got int"},
		{label: "day past a week out", config: with("day", 7), wantErr: "event-list: day must be in [0, 6], got 7"},
		{
			label: "a weather setting", config: with("temp_unit", "C"),
			wantErr: "event-list: temp_unit is not supported: event-list shows only events, so it reads no forecast",
		},
		{label: "no feeds", config: map[string]any{}, wantErr: "event-list: feeds is required"},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			w, err := eventlist.Factory(bounds, tt.config, deps)
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
			tt.check(t, w.(*eventlist.Widget).Config)
		})
	}
}
