package todayhero

import (
	"image"
	"testing"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daygrid"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/weatherview"
)

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

// An agenda too narrow to list events is too narrow for "DONE FOR TODAY"
// too: drawn anyway, it would run over the divider. Events it cannot list
// are still announced: the list's "+N MORE" line is cut to the width.
func TestRenderHeroAgenda_TooNarrowToList(t *testing.T) {
	at := time.Date(2026, 3, 16, 15, 0, 0, 0, time.UTC)
	tests := []struct {
		label      string
		events     []calendar.Event
		wantBeyond bool
	}{
		{"an empty day draws only the rule", nil, false},
		{"hidden events are still announced", []calendar.Event{{Summary: "Standup", Start: at, End: at.Add(time.Hour)}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			frame := newTestFrame(800, 480)
			narrow := image.Rect(0, 280, 2*heroPadX+2*daygrid.BodyAdvance(), 480)
			renderHeroAgenda(frame, narrow, tt.events, heroStyle(3, false, time.UTC))
			rule := narrow.Dx() - 2*heroPadX
			if got := countIndexIn(frame, frame.Bounds(), widget.PaperBlack); (got > rule) != tt.wantBeyond {
				t.Errorf("%d px inked against the %d px rule; want ink beyond it: %v", got, rule, tt.wantBeyond)
			}
		})
	}
}

// A row's list ends above the rule drawn along the row's last pixel, so
// a line that would end exactly on the row's edge does not fit: it
// would sit on the rule. A row one line and its padding tall therefore
// has no room for an event, nor for the "+N MORE" line.
func TestRenderDayRow_ListEndsAboveTheRule(t *testing.T) {
	row := image.Rect(0, 0, 800, rowPadX+daygrid.BodyLineH())
	day := daygrid.Day{
		Start:  time.Date(2026, 3, 17, 0, 0, 0, 0, time.UTC),
		Events: []calendar.Event{{Summary: "Dentist", Start: time.Date(2026, 3, 17, 10, 0, 0, 0, time.UTC)}},
	}
	frame := newTestFrame(800, 480)
	renderDayRow(frame, row, day, dayRowOptions{Agenda: dayRowStyle(false, time.UTC)})

	agenda := image.Rect(rowAgendaDX, 0, 800, 480)
	if got := countIndexIn(frame, agenda, widget.PaperBlack); got != 0 {
		t.Errorf("drew %d px in the agenda of a row with no room above its rule", got)
	}
}

// A row without a forecast draws no temperatures, because drawing a zero
// would state a 0°/0° reading nobody forecast.
func TestRenderRowWeather_MissingForecast(t *testing.T) {
	frame := newTestFrame(800, 480)
	renderRowWeather(frame, image.Rect(0, 0, 800, 120), nil, "C", weatherview.TempRange{Max: 20})
	if got := countIndexIn(frame, frame.Bounds(), widget.PaperBlack); got != 0 {
		t.Errorf("drew %d px for a day with no forecast", got)
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
