package weekly

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

func TestFillWhite(t *testing.T) {
	frame := newTestFrame(10, 10)
	frame.SetColorIndex(5, 5, 1)
	fillWhite(frame, frame.Bounds())
	if frame.ColorIndexAt(5, 5) != 0 {
		t.Error("pixel not cleared to white")
	}
}

func TestDrawText(t *testing.T) {
	frame := newTestFrame(100, 20)
	drawText(frame, 0, 13, "A")
	hasBlack := false
	for _, px := range frame.Pix {
		if px != 0 {
			hasBlack = true
			break
		}
	}
	if !hasBlack {
		t.Error("drawText produced no pixels")
	}
}

func TestDrawTextWhite(t *testing.T) {
	frame := newTestFrame(100, 20)
	fillRect(frame, frame.Bounds(), 1)
	drawTextWhite(frame, 0, 13, "A")
	hasWhite := slices.Contains(frame.Pix, 0)
	if !hasWhite {
		t.Error("drawTextWhite produced no white pixels")
	}
}

func TestDrawTextCentered(t *testing.T) {
	frame := newTestFrame(100, 20)
	drawTextCentered(frame, 0, 100, 13, "HI")
	hasBlack := false
	for _, px := range frame.Pix {
		if px != 0 {
			hasBlack = true
			break
		}
	}
	if !hasBlack {
		t.Error("drawTextCentered produced no pixels")
	}
}

func TestDrawTextCenteredWhite(t *testing.T) {
	frame := newTestFrame(100, 20)
	fillRect(frame, frame.Bounds(), 1)
	drawTextCenteredWhite(frame, 0, 100, 13, "HI")
	hasWhite := slices.Contains(frame.Pix, 0)
	if !hasWhite {
		t.Error("drawTextCenteredWhite produced no white pixels")
	}
}

func TestDrawHLine(t *testing.T) {
	frame := newTestFrame(20, 5)
	drawHLine(frame, 0, 20, 2)
	for x := range 20 {
		if frame.ColorIndexAt(x, 2) != 1 {
			t.Errorf("pixel at (%d,2) not set", x)
		}
	}
}

func TestDrawVLine(t *testing.T) {
	frame := newTestFrame(5, 20)
	drawVLine(frame, 2, 0, 20)
	for y := range 20 {
		if frame.ColorIndexAt(2, y) != 1 {
			t.Errorf("pixel at (2,%d) not set", y)
		}
	}
}

func TestSetPixel_OutOfBounds(t *testing.T) {
	frame := newTestFrame(5, 5)
	setPixel(frame, -1, -1, 1)
	setPixel(frame, 10, 10, 1)
}

func TestFillRect(t *testing.T) {
	frame := newTestFrame(10, 10)
	fillRect(frame, image.Rect(2, 2, 8, 8), 1)
	if frame.ColorIndexAt(5, 5) != 1 {
		t.Error("interior pixel not black")
	}
	if frame.ColorIndexAt(0, 0) != 0 {
		t.Error("exterior pixel not white")
	}
}

func TestFillRect_White(t *testing.T) {
	frame := newTestFrame(10, 10)
	fillRect(frame, frame.Bounds(), 1)
	fillRect(frame, image.Rect(2, 2, 8, 8), 0)
	if frame.ColorIndexAt(5, 5) != 0 {
		t.Error("interior pixel not cleared to white")
	}
}

func TestTruncateText(t *testing.T) {
	tests := []struct {
		text     string
		max      int
		expected string
	}{
		{"hello", 10, "hello"},
		{"hello world", 8, "hello..."},
		{"hi", 3, "hi"},
		{"abcdef", 3, "abc"},
		{"abcdef", 2, "ab"},
	}
	for _, tc := range tests {
		got := truncateText(tc.text, tc.max)
		if got != tc.expected {
			t.Errorf("truncateText(%q, %d) = %q, want %q", tc.text, tc.max, got, tc.expected)
		}
	}
}
