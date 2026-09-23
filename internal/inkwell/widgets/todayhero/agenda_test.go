package todayhero

import (
	"image"
	"testing"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar"
	"github.com/grantlucas/inkwell/internal/inkwell/weather"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daygrid"
)

func TestWrapText(t *testing.T) {
	tests := []struct {
		label    string
		text     string
		maxChars int
		maxLines int
		want     []string
	}{
		{"short title stays on one line", "Standup", 20, 2, []string{"Standup"}},
		{"wraps at a word boundary", "Platform architecture review", 20, 2, []string{"Platform", "architecture review"}},
		{"breaks an unbreakable word", "Supercalifragilistic", 10, 2, []string{"Supercalif", "ragilistic"}},
		{"ellipses past the line budget", "one two three four five six seven", 10, 2, []string{"one two", "three fou»"}},
		{"empty text draws nothing", "", 20, 2, nil},
		{"whitespace only draws nothing", "   ", 20, 2, nil},
		// The budget is in characters, so a multi-byte title must not
		// be cut mid-rune — the panel would paint replacement glyphs.
		{"multi-byte title wraps by character", "日本語のミーティング", 6, 2, []string{"日本語のミー", "ティング"}},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			got := wrapText(tt.text, tt.maxChars, tt.maxLines)
			if len(got) != len(tt.want) {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("line %d = %q, want %q", i, got[i], tt.want[i])
				}
				if runeLen(got[i]) > tt.maxChars {
					t.Errorf("line %d (%q) is %d chars, over the %d budget", i, got[i], runeLen(got[i]), tt.maxChars)
				}
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		label    string
		in       string
		maxChars int
		want     string
	}{
		{"fits", "16:15", 10, "16:15"},
		{"exactly fits", "16:15", 5, "16:15"},
		{"ellipsed", "Design review", 6, "Desig»"},
		{"no room for an ellipsis", "Design", 1, "D"},
		{"zero budget", "Design", 0, ""},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			if got := truncate(tt.in, tt.maxChars); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// Event times stay precise — 16:15, not "quarter past four". They are
// data, not a clock: they do not change on a tick, so fuzzing them
// would lose information for no refresh benefit.
func TestTimeLineAndTitle(t *testing.T) {
	toronto, err := time.LoadLocation("America/Toronto")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 3, 16, 20, 15, 0, 0, time.UTC)
	timed := calendar.Event{Summary: "Design review", Location: "Room 2", Start: start, End: start.Add(time.Hour)}
	allDay := calendar.Event{Summary: "Conference", AllDay: true}

	utcOpts := eventOptions{Location: time.UTC}
	if got := timeLineFor(timed, utcOpts); got != "20:15" {
		t.Errorf("timeLineFor = %q, want 20:15", got)
	}
	if got := timeLineFor(allDay, utcOpts); got != "ALL DAY" {
		t.Errorf("all-day timeLineFor = %q, want ALL DAY", got)
	}
	// Labelled in the dashboard's zone, not the feed's.
	if got := timeLineFor(timed, eventOptions{Location: toronto}); got != "16:15" {
		t.Errorf("timeLineFor in Toronto = %q, want 16:15", got)
	}

	if got := titleFor(timed, utcOpts); got != "Design review" {
		t.Errorf("titleFor = %q", got)
	}
	withLoc := eventOptions{Location: time.UTC, ShowLocation: true}
	if got := titleFor(timed, withLoc); got != "Design review @ Room 2" {
		t.Errorf("titleFor with location = %q", got)
	}
	// Nothing to append when the event carries no location.
	if got := titleFor(allDay, withLoc); got != "Conference" {
		t.Errorf("titleFor with no location = %q", got)
	}
}

// An event that finished is history. An event still running is the one
// you most want to see, so it counts as remaining; an all-day event
// applies to the whole day and never finishes partway through it.
func TestRemainingToday(t *testing.T) {
	at := func(h int) time.Time { return time.Date(2026, 3, 16, h, 0, 0, 0, time.UTC) }
	events := []calendar.Event{
		{Summary: "Finished", Start: at(9), End: at(10)},
		{Summary: "Running", Start: at(14), End: at(16)},
		{Summary: "Later", Start: at(18), End: at(19)},
		{Summary: "All day", AllDay: true},
	}

	got := remainingToday(events, at(15))
	want := []string{"Running", "Later", "All day"}
	if len(got) != len(want) {
		t.Fatalf("got %d events, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].Summary != want[i] {
			t.Errorf("event %d = %q, want %q", i, got[i].Summary, want[i])
		}
	}

	// Late enough that only the all-day event survives.
	if got := remainingToday(events, at(23)); len(got) != 1 || !got[0].AllDay {
		t.Errorf("late in the day = %v, want just the all-day event", got)
	}
}

// A hand-built Config is taken verbatim by New, so a zero or negative
// MaxEvents must not blank the agenda or panic on a slice bound.
func TestRenderHeroAgenda_ClampsMaxEvents(t *testing.T) {
	bounds := computeHero(image.Rect(0, 0, 800, 480)).Agenda
	events := []calendar.Event{
		{Summary: "One", Start: time.Date(2026, 3, 16, 9, 0, 0, 0, time.UTC)},
	}
	for _, maxEvents := range []int{0, -1} {
		frame := newTestFrame(800, 480)
		got := renderHeroAgenda(frame, bounds, events, eventOptions{MaxEvents: maxEvents, Location: time.UTC})
		if got != 0 {
			t.Errorf("MaxEvents %d drew %d events, want 0", maxEvents, got)
		}
	}
}

// Too narrow to carry a title at all: draw nothing rather than a column
// of ellipses, which reads as a fault rather than as content.
func TestRenderHeroAgenda_TooNarrow(t *testing.T) {
	frame := newTestFrame(800, 480)
	got := renderHeroAgenda(frame, image.Rect(0, 280, 20, 480),
		[]calendar.Event{{Summary: "One", Start: time.Date(2026, 3, 16, 9, 0, 0, 0, time.UTC)}},
		eventOptions{MaxEvents: 3, Location: time.UTC})
	if got != 0 {
		t.Errorf("drew %d events into a 20 px column, want 0", got)
	}
}

// A row with no room for its agenda draws nothing rather than spilling
// into the next row. Two ways to run out: no room for the agenda's
// left edge at all, and room for the edge but not for a readable
// title.
func TestRenderRowAgenda_TooNarrow(t *testing.T) {
	tests := []struct {
		label string
		width int
	}{
		{"no room for the agenda column", rowAgendaDX + 10},
		{"room for the column but not for a title", 300},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			frame := newTestFrame(800, 480)
			renderRowAgenda(frame, image.Rect(0, 0, tt.width, 120),
				[]calendar.Event{{Summary: "One", Start: time.Date(2026, 3, 17, 9, 0, 0, 0, time.UTC)}},
				eventOptions{Location: time.UTC})
			if got := countIndexIn(frame, frame.Bounds(), widget.PaperBlack); got != 0 {
				t.Errorf("drew %d px into a row with no agenda room", got)
			}
		})
	}
}

// A day with nothing on it says so; a blank strip reads as a fault.
func TestRenderRowAgenda_EmptyDay(t *testing.T) {
	frame := newTestFrame(800, 480)
	renderRowAgenda(frame, image.Rect(0, 0, 800, 120), nil, eventOptions{Location: time.UTC})
	if countIndexIn(frame, frame.Bounds(), widget.PaperBlack) == 0 {
		t.Error("an empty day drew nothing at all")
	}
}

// A row without a forecast draws no temperatures, because a zero
// DailyForecast is indistinguishable from a real 0°/0° reading.
func TestRenderRowWeather_MissingForecast(t *testing.T) {
	frame := newTestFrame(800, 480)
	renderRowWeather(frame, image.Rect(0, 0, 800, 120), weather.DailyForecast{}, "C")
	if got := countIndexIn(frame, frame.Bounds(), widget.PaperBlack); got != 0 {
		t.Errorf("drew %d px for a day with no forecast", got)
	}
}

// A condition glyph that will not load must not take the temperatures
// with it — they are the part of the cell that carries the forecast.
func TestIconFailureStillDrawsTheTemperatures(t *testing.T) {
	orig := drawIcon
	defer func() { drawIcon = orig }()
	drawIcon = func(*image.Paletted, int, int, int, weather.Condition) error {
		return errIconFailed
	}

	day := weather.DailyForecast{
		Date: time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC),
		High: 14, Low: 3, Condition: weather.Condition(0),
	}

	t.Run("hero", func(t *testing.T) {
		frame := newTestFrame(800, 480)
		renderHeroWeather(frame, computeHero(image.Rect(0, 0, 800, 480)).Weather, day, "C")
		if countIndexIn(frame, frame.Bounds(), widget.PaperBlack) == 0 {
			t.Error("nothing drawn after the icon failed")
		}
	})

	t.Run("day row", func(t *testing.T) {
		frame := newTestFrame(800, 480)
		renderRowWeather(frame, image.Rect(0, 0, 800, 120), day, "C")
		if countIndexIn(frame, frame.Bounds(), widget.PaperBlack) == 0 {
			t.Error("nothing drawn after the icon failed")
		}
	})
}

var errIconFailed = errIcon{}

type errIcon struct{}

func (errIcon) Error() string { return "no glyph" }

// A row with more events than fit, and no room left for the marker,
// must not draw the marker off the bottom of the row.
func TestRenderRowAgenda_OverflowMarkerNeedsRoom(t *testing.T) {
	events := make([]calendar.Event, 6)
	for i := range events {
		start := time.Date(2026, 3, 17, 9+i, 0, 0, 0, time.UTC)
		events[i] = calendar.Event{Summary: "Event", Start: start, End: start.Add(time.Hour)}
	}

	// A row just tall enough for the three events and nothing more.
	short := image.Rect(0, 0, 800, rowPadX+3*daygrid.BodyLineH()+daygrid.BodyAscent())
	frame := newTestFrame(800, 480)
	renderRowAgenda(frame, short, events, eventOptions{Location: time.UTC})

	for y := short.Max.Y; y < 480; y++ {
		for x := range 800 {
			if frame.ColorIndexAt(x, y) != widget.PaperWhite {
				t.Fatalf("drew below the row at (%d,%d)", x, y)
			}
		}
	}
}

// The agenda can run out of room before it runs out of cap, and both
// have to leave the marker a line. Reserving it only when the cap
// truncated the list meant any max_events past what fits stopped
// silently — which is the failure the reserve exists to prevent, still
// reachable through the other door.
func TestRenderHeroAgenda_MarkerSurvivesEitherLimit(t *testing.T) {
	bounds := computeHero(image.Rect(0, 0, 800, 480)).Agenda
	events := make([]calendar.Event, 5)
	for i := range events {
		start := time.Date(2026, 3, 16, 9+i, 0, 0, 0, time.UTC)
		events[i] = calendar.Event{Summary: "Event", Start: start, End: start.Add(time.Hour)}
	}

	tests := []struct {
		label     string
		maxEvents int
	}{
		{"the cap is what truncates", 3},
		{"the room is what truncates", 5},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			frame := newTestFrame(800, 480)
			drawn := renderHeroAgenda(frame, bounds, events,
				eventOptions{MaxEvents: tt.maxEvents, Location: time.UTC})
			if drawn >= len(events) {
				t.Fatalf("drew %d of %d events — this case is meant to overflow", drawn, len(events))
			}

			// The marker sits wherever the last event ended, so rather
			// than guess at a band, compare against the same agenda
			// rendered with nothing left over: the difference is the
			// marker.
			quiet := newTestFrame(800, 480)
			renderHeroAgenda(quiet, bounds, events[:drawn],
				eventOptions{MaxEvents: tt.maxEvents, Location: time.UTC})

			withMarker := countIndexIn(frame, bounds, widget.PaperBlack)
			without := countIndexIn(quiet, bounds, widget.PaperBlack)
			if withMarker <= without {
				t.Errorf("drew %d of %d events and said nothing about the rest", drawn, len(events))
			}
		})
	}
}

// A timed VEVENT with neither DTEND nor DURATION is parsed with
// End == Start, so an exclusive comparison drops it at the very minute
// it fires — and if it were the last one, the panel would say "DONE FOR
// TODAY" over an event happening now.
func TestRemainingToday_ZeroDurationEventIsStillCurrent(t *testing.T) {
	at := func(h, m int) time.Time { return time.Date(2026, 3, 16, h, m, 0, 0, time.UTC) }
	reminder := calendar.Event{Summary: "Pick up parcel", Start: at(16, 0), End: at(16, 0)}

	if got := remainingToday([]calendar.Event{reminder}, at(15, 45)); len(got) != 1 {
		t.Error("dropped before it fires")
	}
	if got := remainingToday([]calendar.Event{reminder}, at(16, 0)); len(got) != 1 {
		t.Error("dropped at the minute it fires — the panel would read DONE FOR TODAY over it")
	}
	if got := remainingToday([]calendar.Event{reminder}, at(16, 1)); len(got) != 0 {
		t.Error("still shown a minute after it fired")
	}
}
