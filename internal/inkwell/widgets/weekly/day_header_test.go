package weekly

import (
	"image"
	"testing"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/widget"
)

func TestRenderDayHeader_Normal(t *testing.T) {
	frame := newTestFrame(114, 44)
	day := time.Date(2026, 4, 29, 0, 0, 0, 0, time.UTC) // Wednesday
	renderDayHeader(frame, image.Rect(0, 0, 114, 44), day, false)

	hasInk := false
	for _, px := range frame.Pix {
		if px != widget.PaperWhite {
			hasInk = true
			break
		}
	}
	if !hasInk {
		t.Error("normal day header drew nothing")
	}
}

func TestRenderDayHeader_Today(t *testing.T) {
	// Today renders as a soft gray background tint with dark text on top,
	// and a hairline accent at the bottom edge — no more inverse block.
	frame := newTestFrame(114, 44)
	day := time.Date(2026, 4, 28, 0, 0, 0, 0, time.UTC)
	renderDayHeader(frame, image.Rect(0, 0, 114, 44), day, true)

	var tintCount, accentCount int
	for _, px := range frame.Pix {
		switch px {
		case widget.PaperGray10:
			tintCount++
		case widget.PaperGray60:
			accentCount++
		}
	}
	if tintCount == 0 {
		t.Error("today header has no background tint pixels (PaperGray10)")
	}
	if accentCount == 0 {
		t.Error("today header has no hairline accent pixels (PaperGray60)")
	}
}
