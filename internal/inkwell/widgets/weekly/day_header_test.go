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
	// Today renders as a solid PaperBlack block with PaperWhite text.
	// Inversion is the only treatment that survives the device path
	// without dithering — a soft gray fill would either snap to white
	// (BW threshold) or collapse to the light-gray bucket and vanish
	// (Gray4). Asserts both the fill is present *and* that there is
	// white text inside it (otherwise an all-black rect would silently
	// satisfy the fill check).
	frame := newTestFrame(114, 44)
	day := time.Date(2026, 4, 28, 0, 0, 0, 0, time.UTC)
	renderDayHeader(frame, image.Rect(0, 0, 114, 44), day, true)

	var blackCount, whiteCount int
	for _, px := range frame.Pix {
		switch px {
		case widget.PaperBlack:
			blackCount++
		case widget.PaperWhite:
			whiteCount++
		}
	}
	if blackCount == 0 {
		t.Error("today header has no PaperBlack fill pixels")
	}
	if whiteCount == 0 {
		t.Error("today header has no PaperWhite text pixels inside the inverted block")
	}
}
