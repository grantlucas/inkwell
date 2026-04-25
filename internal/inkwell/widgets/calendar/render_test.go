package calendar

import (
	"image"
	"image/color"
	"slices"
	"testing"
)

func newTestFrame(w, h int) *image.Paletted {
	return image.NewPaletted(
		image.Rect(0, 0, w, h),
		color.Palette{color.White, color.Black},
	)
}

func hasBlackPixels(frame *image.Paletted, bounds image.Rectangle) bool {
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if frame.ColorIndexAt(x, y) != 0 {
				return true
			}
		}
	}
	return false
}

func TestDrawText_RendersPixels(t *testing.T) {
	frame := newTestFrame(100, 20)
	drawText(frame, 0, 13, "Hello")
	if !hasBlackPixels(frame, frame.Bounds()) {
		t.Error("drawText produced no black pixels")
	}
}

func TestDrawTextCentered_RendersPixels(t *testing.T) {
	frame := newTestFrame(200, 20)
	drawTextCentered(frame, 0, 200, 13, "Hi")
	if !hasBlackPixels(frame, frame.Bounds()) {
		t.Error("drawTextCentered produced no black pixels")
	}
}

func TestDrawTextRight_RendersPixels(t *testing.T) {
	frame := newTestFrame(200, 20)
	drawTextRight(frame, 200, 13, "Hi")
	if !hasBlackPixels(frame, frame.Bounds()) {
		t.Error("drawTextRight produced no black pixels")
	}
}

func TestDrawHLine_RendersPixels(t *testing.T) {
	frame := newTestFrame(100, 10)
	drawHLine(frame, 10, 90, 5)
	black := 0
	for x := 10; x < 90; x++ {
		if frame.ColorIndexAt(x, 5) != 0 {
			black++
		}
	}
	if black != 80 {
		t.Errorf("drawHLine: got %d black pixels, want 80", black)
	}
}

func TestDrawInvertedRect_FillsBlack(t *testing.T) {
	frame := newTestFrame(100, 100)
	r := image.Rect(10, 10, 50, 50)
	drawInvertedRect(frame, r)
	allBlack := true
	for y := 10; y < 50; y++ {
		for x := 10; x < 50; x++ {
			if frame.ColorIndexAt(x, y) == 0 {
				allBlack = false
				break
			}
		}
	}
	if !allBlack {
		t.Error("drawInvertedRect did not fill all pixels black")
	}
}

func TestDrawTextInverted_RendersWhitePixels(t *testing.T) {
	frame := newTestFrame(100, 20)
	// Fill with black first.
	drawInvertedRect(frame, frame.Bounds())
	drawTextInverted(frame, 0, 13, "Hi")
	// Should have some white pixels now.
	hasWhite := slices.Contains(frame.Pix, 0)
	if !hasWhite {
		t.Error("drawTextInverted produced no white pixels")
	}
}

func TestTruncateText(t *testing.T) {
	tests := []struct {
		text     string
		max      int
		expected string
	}{
		{"Hello", 10, "Hello"},
		{"Hello", 5, "Hello"},
		{"Hello World", 8, "Hello..."},
		{"Hi", 3, "Hi"},
		{"Hello", 3, "Hel"},
		{"Hello", 2, "He"},
	}
	for _, tt := range tests {
		got := truncateText(tt.text, tt.max)
		if got != tt.expected {
			t.Errorf("truncateText(%q, %d) = %q, want %q", tt.text, tt.max, got, tt.expected)
		}
	}
}

func TestFillWhite(t *testing.T) {
	frame := newTestFrame(50, 50)
	// Dirty the frame with black.
	drawInvertedRect(frame, frame.Bounds())
	fillWhite(frame, frame.Bounds())
	for _, px := range frame.Pix {
		if px != 0 {
			t.Fatal("fillWhite did not clear all pixels to white")
		}
	}
}
