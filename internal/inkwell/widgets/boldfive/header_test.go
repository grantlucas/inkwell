package boldfive

import (
	"bytes"
	"image"
	"testing"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/widget"
)

func newTestFrame(w, h int) *image.Paletted {
	frame := image.NewPaletted(image.Rect(0, 0, w, h), widget.PaperPalette)
	fillWhite(frame, frame.Bounds())
	return frame
}

// countIndex reports how many pixels carry the given palette index.
func countIndex(frame *image.Paletted, idx uint8) int {
	n := 0
	for _, px := range frame.Pix {
		if px == idx {
			n++
		}
	}
	return n
}

// There is deliberately no today highlight: today is always the leftmost
// column, so an inverted header would spend ink restating what position
// already says. That is a decision, not an omission, so it needs a test
// that fails if someone reintroduces the highlight.
func TestRenderDayHeader_TodayIsNotHighlighted(t *testing.T) {
	const w, h = 160, headerH
	today := time.Date(2026, 3, 16, 9, 0, 0, 0, time.UTC)

	// Two columns whose only difference is that one day is "today" as
	// far as the caller is concerned. renderDayHeader takes no such
	// flag, and this pins that it never grows one.
	first := newTestFrame(w, h)
	renderDayHeader(first, first.Bounds(), today)

	second := newTestFrame(w, h)
	renderDayHeader(second, second.Bounds(), today)

	if !bytes.Equal(first.Pix, second.Pix) {
		t.Fatal("the same day rendered twice differs")
	}

	// An inverted header would flood the band with PaperBlack. Text
	// alone covers a small fraction of it.
	black := countIndex(first, widget.PaperBlack)
	if black > w*h/2 {
		t.Errorf("header is %d/%d black — today's column looks inverted", black, w*h)
	}
}

// The header's two runs must land inside the band and not collide: the
// weekday sits above the numeral, and the numeral's descent is what the
// band's height was chosen around.
func TestRenderDayHeader_RunsFitTheBand(t *testing.T) {
	frame := newTestFrame(160, headerH+40)
	renderDayHeader(frame, image.Rect(0, 0, 160, headerH), time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC))

	rowInked := func(y int) bool {
		for x := range 160 {
			if frame.ColorIndexAt(x, y) == widget.PaperBlack {
				return true
			}
		}
		return false
	}

	// Nothing may spill below the band into the weather zone.
	for y := headerH; y < headerH+40; y++ {
		if rowInked(y) {
			t.Errorf("ink at y=%d, below the %d px header band", y, headerH)
			break
		}
	}

	// Both runs drew something, in their own halves of the band.
	var weekdayRows, dateRows int
	for y := range headerH {
		if !rowInked(y) {
			continue
		}
		if y < dateTop {
			weekdayRows++
		} else {
			dateRows++
		}
	}
	if weekdayRows == 0 {
		t.Error("no weekday abbreviation drawn")
	}
	if dateRows == 0 {
		t.Error("no date numeral drawn")
	}
}

// The numeral is the element that decides whether the panel reads from
// the doorway, so its scale is not an incidental constant.
func TestRenderDayHeader_NumeralIsScaledUp(t *testing.T) {
	frame := newTestFrame(160, headerH)
	renderDayHeader(frame, frame.Bounds(), time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC))

	// Measure the inked height of the numeral run.
	top, bottom := -1, -1
	for y := dateTop; y < headerH; y++ {
		for x := range 160 {
			if frame.ColorIndexAt(x, y) == widget.PaperBlack {
				if top < 0 {
					top = y
				}
				bottom = y
				break
			}
		}
	}
	if top < 0 {
		t.Fatal("no numeral drawn")
	}
	// At 3x the 20 px tier a digit's cap is ~42 px before dilation.
	if got := bottom - top + 1; got < 30 {
		t.Errorf("numeral is %d px tall, want a display-sized run", got)
	}
}
