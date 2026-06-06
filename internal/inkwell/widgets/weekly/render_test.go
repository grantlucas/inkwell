package weekly

import (
	"image"
	"testing"

	"github.com/grantlucas/inkwell/internal/inkwell/widget"
)

func newTestFrame(w, h int) *image.Paletted {
	return image.NewPaletted(image.Rect(0, 0, w, h), widget.PaperPalette)
}

func TestFillWhite(t *testing.T) {
	frame := newTestFrame(10, 10)
	frame.SetColorIndex(5, 5, widget.PaperBlack)
	fillWhite(frame, frame.Bounds())
	if frame.ColorIndexAt(5, 5) != widget.PaperWhite {
		t.Error("pixel not cleared to white")
	}
}

func TestDrawText(t *testing.T) {
	frame := newTestFrame(100, 20)
	drawText(frame, 0, 13, "A")
	hasInk := false
	for _, px := range frame.Pix {
		if px != widget.PaperWhite {
			hasInk = true
			break
		}
	}
	if !hasInk {
		t.Error("drawText produced no pixels")
	}
}

func TestDrawTextGray(t *testing.T) {
	// Mid-gray text on a white frame should land on a palette index that
	// is darker than white but not pure black — the renderer is free to
	// pick the nearest grayscale entry.
	frame := newTestFrame(100, 20)
	drawTextGray(frame, 0, 13, "A", widget.PaperGray50)
	sawGray := false
	for _, px := range frame.Pix {
		if px != widget.PaperWhite && px != widget.PaperBlack {
			sawGray = true
			break
		}
	}
	if !sawGray {
		t.Error("drawTextGray produced no intermediate grays")
	}
}

func TestDrawTextCentered(t *testing.T) {
	frame := newTestFrame(100, 20)
	drawTextCentered(frame, 0, 100, 13, "HI")
	hasInk := false
	for _, px := range frame.Pix {
		if px != widget.PaperWhite {
			hasInk = true
			break
		}
	}
	if !hasInk {
		t.Error("drawTextCentered produced no pixels")
	}
}

func TestDrawHLine(t *testing.T) {
	frame := newTestFrame(20, 5)
	drawHLine(frame, 0, 20, 2, widget.PaperBlack)
	for x := range 20 {
		if frame.ColorIndexAt(x, 2) != widget.PaperBlack {
			t.Errorf("pixel at (%d,2) not set", x)
		}
	}
}

func TestDrawHLine_Gray(t *testing.T) {
	frame := newTestFrame(20, 5)
	drawHLine(frame, 0, 20, 2, widget.PaperGray30)
	for x := range 20 {
		if frame.ColorIndexAt(x, 2) != widget.PaperGray30 {
			t.Errorf("pixel at (%d,2) not gray", x)
		}
	}
}

func TestDrawVLine(t *testing.T) {
	frame := newTestFrame(5, 20)
	drawVLine(frame, 2, 0, 20, widget.PaperBlack)
	for y := range 20 {
		if frame.ColorIndexAt(2, y) != widget.PaperBlack {
			t.Errorf("pixel at (2,%d) not set", y)
		}
	}
}

func TestSetPixel_OutOfBounds(t *testing.T) {
	frame := newTestFrame(5, 5)
	setPixel(frame, -1, -1, widget.PaperBlack)
	setPixel(frame, 10, 10, widget.PaperBlack)
}

func TestFillRect(t *testing.T) {
	frame := newTestFrame(10, 10)
	fillRect(frame, image.Rect(2, 2, 8, 8), widget.PaperBlack)
	if frame.ColorIndexAt(5, 5) != widget.PaperBlack {
		t.Error("interior pixel not black")
	}
	if frame.ColorIndexAt(0, 0) != widget.PaperWhite {
		t.Error("exterior pixel not white")
	}
}

func TestFillRect_Gray(t *testing.T) {
	frame := newTestFrame(10, 10)
	fillRect(frame, image.Rect(2, 2, 8, 8), widget.PaperGray20)
	if frame.ColorIndexAt(5, 5) != widget.PaperGray20 {
		t.Errorf("interior pixel idx = %d, want %d (PaperGray20)",
			frame.ColorIndexAt(5, 5), widget.PaperGray20)
	}
}

func TestDrawTextCenteredGray(t *testing.T) {
	frame := newTestFrame(100, 20)
	drawTextCenteredGrayWithFace(frame, 0, 100, 13, "OK", defaultFace, widget.PaperGray70)
	sawAnyInk := false
	for _, px := range frame.Pix {
		if px != widget.PaperWhite {
			sawAnyInk = true
			break
		}
	}
	if !sawAnyInk {
		t.Error("drawTextCenteredGrayWithFace produced no pixels")
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
