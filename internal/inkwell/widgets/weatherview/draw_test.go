package weatherview

import (
	"image"
	"image/color"
	"testing"
)

func newTestFrame(w, h int) *image.Paletted {
	return image.NewPaletted(
		image.Rect(0, 0, w, h),
		color.Palette{color.White, color.Black},
	)
}

func TestSetPixel(t *testing.T) {
	frame := newTestFrame(10, 10)
	setPixel(frame, 5, 5, 1)
	if frame.ColorIndexAt(5, 5) != 1 {
		t.Error("pixel not set")
	}
}

func TestSetPixel_OutOfBounds(t *testing.T) {
	frame := newTestFrame(10, 10)
	setPixel(frame, -1, -1, 1)
	setPixel(frame, 100, 100, 1)
}

func TestDrawHLine(t *testing.T) {
	frame := newTestFrame(20, 10)
	drawHLine(frame, 2, 8, 5)
	for x := 2; x < 8; x++ {
		if frame.ColorIndexAt(x, 5) != 1 {
			t.Errorf("pixel at (%d, 5) not set", x)
		}
	}
	if frame.ColorIndexAt(1, 5) != 0 {
		t.Error("pixel before line should be white")
	}
}

func TestDrawVLine(t *testing.T) {
	frame := newTestFrame(10, 20)
	drawVLine(frame, 5, 2, 8)
	for y := 2; y < 8; y++ {
		if frame.ColorIndexAt(5, y) != 1 {
			t.Errorf("pixel at (5, %d) not set", y)
		}
	}
}

func TestDrawDashedVLine(t *testing.T) {
	frame := newTestFrame(10, 20)
	drawDashedVLine(frame, 5, 0, 10, 2, 2)
	if frame.ColorIndexAt(5, 0) != 1 {
		t.Error("dash pixel 0 not set")
	}
	if frame.ColorIndexAt(5, 1) != 1 {
		t.Error("dash pixel 1 not set")
	}
	if frame.ColorIndexAt(5, 2) != 0 {
		t.Error("gap pixel 2 should be white")
	}
	if frame.ColorIndexAt(5, 3) != 0 {
		t.Error("gap pixel 3 should be white")
	}
	if frame.ColorIndexAt(5, 4) != 1 {
		t.Error("dash pixel 4 not set")
	}
}

func TestFillRect(t *testing.T) {
	frame := newTestFrame(20, 20)
	r := image.Rect(2, 2, 8, 8)
	fillRect(frame, r, 1)
	if frame.ColorIndexAt(4, 4) != 1 {
		t.Error("interior pixel not set")
	}
	if frame.ColorIndexAt(1, 1) != 0 {
		t.Error("exterior pixel should be white")
	}

	fillRect(frame, r, 0)
	if frame.ColorIndexAt(4, 4) != 0 {
		t.Error("cleared pixel should be white")
	}
}

func TestDrawLine_Horizontal(t *testing.T) {
	frame := newTestFrame(20, 10)
	drawLine(frame, 2, 5, 8, 5)
	for x := 2; x <= 8; x++ {
		if frame.ColorIndexAt(x, 5) != 1 {
			t.Errorf("pixel at (%d, 5) not set", x)
		}
	}
}

func TestDrawLine_Vertical(t *testing.T) {
	frame := newTestFrame(10, 20)
	drawLine(frame, 5, 2, 5, 8)
	for y := 2; y <= 8; y++ {
		if frame.ColorIndexAt(5, y) != 1 {
			t.Errorf("pixel at (5, %d) not set", y)
		}
	}
}

func TestDrawLine_Diagonal(t *testing.T) {
	frame := newTestFrame(20, 20)
	drawLine(frame, 0, 0, 9, 9)
	if frame.ColorIndexAt(0, 0) != 1 {
		t.Error("start pixel not set")
	}
	if frame.ColorIndexAt(9, 9) != 1 {
		t.Error("end pixel not set")
	}
}

func TestDrawLine_Reverse(t *testing.T) {
	frame := newTestFrame(20, 20)
	drawLine(frame, 9, 9, 0, 0)
	if frame.ColorIndexAt(0, 0) != 1 {
		t.Error("start pixel not set")
	}
	if frame.ColorIndexAt(9, 9) != 1 {
		t.Error("end pixel not set")
	}
}

func TestDrawText(t *testing.T) {
	frame := newTestFrame(100, 20)
	drawText(frame, 0, 13, "Hi")
}

func TestDrawTextCentered(t *testing.T) {
	frame := newTestFrame(100, 20)
	drawTextCentered(frame, 0, 100, 13, "OK")
}

func TestTruncateText(t *testing.T) {
	tests := []struct {
		text     string
		max      int
		expected string
	}{
		{"hello", 10, "hello"},
		{"hello world", 5, "he..."},
		{"hi", 2, "hi"},
		{"hello", 3, "hel"},
		{"hello", 4, "h..."},
	}
	for _, tt := range tests {
		got := truncateText(tt.text, tt.max)
		if got != tt.expected {
			t.Errorf("truncateText(%q, %d) = %q, want %q", tt.text, tt.max, got, tt.expected)
		}
	}
}

func TestAbs(t *testing.T) {
	if abs(-5) != 5 {
		t.Error("abs(-5) != 5")
	}
	if abs(5) != 5 {
		t.Error("abs(5) != 5")
	}
	if abs(0) != 0 {
		t.Error("abs(0) != 0")
	}
}
