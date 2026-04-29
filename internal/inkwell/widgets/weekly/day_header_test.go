package weekly

import (
	"image"
	"testing"
	"time"
)

func TestRenderDayHeader_Normal(t *testing.T) {
	frame := newTestFrame(114, 44)
	day := time.Date(2026, 4, 29, 0, 0, 0, 0, time.UTC) // Wednesday
	renderDayHeader(frame, image.Rect(0, 0, 114, 44), day, false)

	hasBlack := false
	for _, px := range frame.Pix {
		if px != 0 {
			hasBlack = true
			break
		}
	}
	if !hasBlack {
		t.Error("normal day header drew nothing")
	}
}

func TestRenderDayHeader_Today(t *testing.T) {
	frame := newTestFrame(114, 44)
	day := time.Date(2026, 4, 28, 0, 0, 0, 0, time.UTC) // Tuesday (today)
	renderDayHeader(frame, image.Rect(0, 0, 114, 44), day, true)

	blackCount := 0
	for _, px := range frame.Pix {
		if px == 1 {
			blackCount++
		}
	}
	whiteCount := 0
	for _, px := range frame.Pix {
		if px == 0 {
			whiteCount++
		}
	}
	if blackCount == 0 {
		t.Error("today header has no black pixels (should be inverted background)")
	}
	if whiteCount == 0 {
		t.Error("today header has no white pixels (should have white text)")
	}
}
