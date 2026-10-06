package boldfive

import (
	"image"
	"testing"
	"time"

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
	renderEvents(frame, eventsRect(), nil, eventStyle(defaultMaxEvents, false, time.UTC))
	if countIndex(frame, widget.PaperBlack) <= eventsRect().Dx() {
		t.Error("nothing drawn under the rule — the empty marker is missing")
	}
}

// A column too narrow for the event list is too narrow for the empty
// marker too: centred "--" would overhang the dividers either side.
func TestRenderEvents_TooNarrowForTheEmptyMarker(t *testing.T) {
	frame := newTestFrame(160, 480)
	narrow := image.Rect(0, 216, 2*eventsPadX+2*daygrid.BodyAdvance(), 480)
	renderEvents(frame, narrow, nil, eventStyle(defaultMaxEvents, false, time.UTC))
	if got := countIndex(frame, widget.PaperBlack); got != narrow.Dx() {
		t.Errorf("%d px inked, want only the %d px rule", got, narrow.Dx())
	}
}

// The rule along the top of the agenda is what separates it from the
// weather band; without it the two run together.
func TestRenderEvents_DrawsTopRule(t *testing.T) {
	frame := newTestFrame(160, 480)
	rect := eventsRect()
	renderEvents(frame, rect, nil, eventStyle(defaultMaxEvents, false, time.UTC))

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
