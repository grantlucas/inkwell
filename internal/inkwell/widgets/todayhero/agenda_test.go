package todayhero

import (
	"image"
	"testing"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daygrid"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/drawkit"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/weatherview"
)

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
			narrow := image.Rect(0, 280, 2*heroPadX+2*drawkit.BodyAdvance(), 480)
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
	row := image.Rect(0, 0, 800, rowPadX+drawkit.BodyLineH())
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

// A row without a forecast draws no chart, because a line would state a
// temperature nobody forecast.
func TestRenderRowChart_MissingForecast(t *testing.T) {
	frame := newTestFrame(800, 480)
	renderRowChart(frame, rowChart(image.Rect(0, 0, 800, 120)), nil, weatherview.TempRange{Max: 20})
	if got := countIndexIn(frame, frame.Bounds(), widget.PaperBlack); got != 0 {
		t.Errorf("drew %d px for a day with no forecast", got)
	}
}
