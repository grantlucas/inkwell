package boldfive

import (
	"image"
	"testing"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daygrid"
)

// How events are written, fitted and counted is the event list's, tested
// there. These cover what the column adds around the list.

func eventsRect() image.Rectangle {
	return image.Rect(0, 216, 160, 480)
}

// An empty day says so rather than leaving a blank column that reads as
// a rendering fault.
func TestRenderEvents_EmptyDay(t *testing.T) {
	frame := newTestFrame(160, 480)
	renderEvents(frame, eventsRect(), nil, agendaStyle(defaultMaxEvents, false, time.UTC))
	if countIndex(frame, widget.PaperBlack) <= eventsRect().Dx() {
		t.Error("nothing drawn under the rule — the empty-day dash is missing")
	}
}

// A column too narrow to list events is too narrow for the empty-day dash
// too: centred "--" would overhang the dividers either side. Events it
// cannot list are still announced: the list's "+N MORE" line is cut to
// the width.
func TestRenderEvents_TooNarrowToList(t *testing.T) {
	at := time.Date(2026, 3, 16, 9, 0, 0, 0, time.UTC)
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
			frame := newTestFrame(160, 480)
			narrow := image.Rect(0, 216, 2*eventsPadX+2*daygrid.BodyAdvance(), 480)
			renderEvents(frame, narrow, tt.events, agendaStyle(defaultMaxEvents, false, time.UTC))
			if got := countIndex(frame, widget.PaperBlack); (got > narrow.Dx()) != tt.wantBeyond {
				t.Errorf("%d px inked against the %d px rule; want ink beyond it: %v", got, narrow.Dx(), tt.wantBeyond)
			}
		})
	}
}

// The rule along the top of the agenda is what separates it from the
// weather band; without it the two run together.
func TestRenderEvents_DrawsTopRule(t *testing.T) {
	frame := newTestFrame(160, 480)
	rect := eventsRect()
	renderEvents(frame, rect, nil, agendaStyle(defaultMaxEvents, false, time.UTC))

	for x := rect.Min.X; x < rect.Max.X; x++ {
		if frame.ColorIndexAt(x, rect.Min.Y) != widget.PaperBlack {
			t.Fatalf("no rule pixel at x=%d", x)
		}
	}
}

// moreLineAt reports whether the band of rows around baseline y in
// bounds carries exactly the pixels of want drawn as the overflow line.
// The column's last pixel is left out: in a full render it carries the
// divider to the next day.
func moreLineAt(frame *image.Paletted, bounds image.Rectangle, y int, want string) bool {
	ref := newTestFrame(frame.Bounds().Dx(), frame.Bounds().Dy())
	daygrid.DrawText(ref, bounds.Min.X+eventsPadX, y, want, daygrid.BodyBoldFace, widget.PaperBlack)
	rows := image.Rect(bounds.Min.X, y-daygrid.BodyAscent(), bounds.Max.X-1, y+daygrid.BodyLineH()-daygrid.BodyAscent())
	for yy := rows.Min.Y; yy < rows.Max.Y; yy++ {
		for x := rows.Min.X; x < rows.Max.X; x++ {
			if frame.ColorIndexAt(x, yy) != ref.ColorIndexAt(x, yy) {
				return false
			}
		}
	}
	return countIndexIn(ref, rows, widget.PaperBlack) > 0
}
