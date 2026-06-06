package weatherview

import (
	"image"
	"strings"
	"testing"

	"github.com/grantlucas/inkwell/internal/inkwell/fonts"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
)

func newTestFrame(w, h int) *image.Paletted {
	return image.NewPaletted(
		image.Rect(0, 0, w, h),
		widget.PaperPalette,
	)
}

func TestSetPixel(t *testing.T) {
	frame := newTestFrame(10, 10)
	setPixel(frame, 5, 5, widget.PaperBlack)
	if frame.ColorIndexAt(5, 5) != widget.PaperBlack {
		t.Error("pixel not set")
	}
}

func TestSetPixel_OutOfBounds(t *testing.T) {
	frame := newTestFrame(10, 10)
	setPixel(frame, -1, -1, widget.PaperBlack)
	setPixel(frame, 100, 100, widget.PaperBlack)
}

func TestDrawHLine(t *testing.T) {
	frame := newTestFrame(20, 10)
	drawHLine(frame, 2, 8, 5, widget.PaperBlack)
	for x := 2; x < 8; x++ {
		if frame.ColorIndexAt(x, 5) != widget.PaperBlack {
			t.Errorf("pixel at (%d, 5) not set", x)
		}
	}
	if frame.ColorIndexAt(1, 5) != widget.PaperWhite {
		t.Error("pixel before line should be white")
	}
}

func TestDrawHLine_Gray(t *testing.T) {
	frame := newTestFrame(20, 10)
	drawHLine(frame, 0, 10, 3, widget.PaperGray30)
	for x := range 10 {
		if frame.ColorIndexAt(x, 3) != widget.PaperGray30 {
			t.Errorf("pixel at (%d,3) not gray", x)
		}
	}
}

func TestDrawVLine(t *testing.T) {
	frame := newTestFrame(10, 20)
	drawVLine(frame, 5, 2, 8, widget.PaperBlack)
	for y := 2; y < 8; y++ {
		if frame.ColorIndexAt(5, y) != widget.PaperBlack {
			t.Errorf("pixel at (5, %d) not set", y)
		}
	}
}

func TestFillRect(t *testing.T) {
	frame := newTestFrame(20, 20)
	r := image.Rect(2, 2, 8, 8)
	fillRect(frame, r, widget.PaperBlack)
	if frame.ColorIndexAt(4, 4) != widget.PaperBlack {
		t.Error("interior pixel not set")
	}
	if frame.ColorIndexAt(1, 1) != widget.PaperWhite {
		t.Error("exterior pixel should be white")
	}

	fillRect(frame, r, widget.PaperWhite)
	if frame.ColorIndexAt(4, 4) != widget.PaperWhite {
		t.Error("cleared pixel should be white")
	}
}

func TestFillRect_Gray(t *testing.T) {
	frame := newTestFrame(20, 20)
	r := image.Rect(2, 2, 8, 8)
	fillRect(frame, r, widget.PaperGray20)
	if frame.ColorIndexAt(4, 4) != widget.PaperGray20 {
		t.Errorf("interior idx = %d, want %d (PaperGray20)",
			frame.ColorIndexAt(4, 4), widget.PaperGray20)
	}
}

func TestDrawLine_Horizontal(t *testing.T) {
	frame := newTestFrame(20, 10)
	drawLine(frame, 2, 5, 8, 5, widget.PaperBlack)
	for x := 2; x <= 8; x++ {
		if frame.ColorIndexAt(x, 5) != widget.PaperBlack {
			t.Errorf("pixel at (%d, 5) not set", x)
		}
	}
}

func TestDrawLine_Vertical(t *testing.T) {
	frame := newTestFrame(10, 20)
	drawLine(frame, 5, 2, 5, 8, widget.PaperBlack)
	for y := 2; y <= 8; y++ {
		if frame.ColorIndexAt(5, y) != widget.PaperBlack {
			t.Errorf("pixel at (5, %d) not set", y)
		}
	}
}

func TestDrawLine_Diagonal(t *testing.T) {
	frame := newTestFrame(20, 20)
	drawLine(frame, 0, 0, 9, 9, widget.PaperBlack)
	if frame.ColorIndexAt(0, 0) != widget.PaperBlack {
		t.Error("start pixel not set")
	}
	if frame.ColorIndexAt(9, 9) != widget.PaperBlack {
		t.Error("end pixel not set")
	}
}

func TestDrawLine_Reverse(t *testing.T) {
	frame := newTestFrame(20, 20)
	drawLine(frame, 9, 9, 0, 0, widget.PaperBlack)
	if frame.ColorIndexAt(0, 0) != widget.PaperBlack {
		t.Error("start pixel not set")
	}
	if frame.ColorIndexAt(9, 9) != widget.PaperBlack {
		t.Error("end pixel not set")
	}
}

func TestDrawLine_Gray(t *testing.T) {
	frame := newTestFrame(20, 20)
	drawLine(frame, 0, 0, 5, 5, widget.PaperGray70)
	if got := frame.ColorIndexAt(0, 0); got != widget.PaperGray70 {
		t.Errorf("(0,0) = %d, want %d (PaperGray70)", got, widget.PaperGray70)
	}
	if got := frame.ColorIndexAt(5, 5); got != widget.PaperGray70 {
		t.Errorf("(5,5) = %d, want %d (PaperGray70)", got, widget.PaperGray70)
	}
}

func TestDrawText(t *testing.T) {
	frame := newTestFrame(100, 20)
	drawText(frame, 0, 13, "Hi")
	if !anyNonWhite(frame) {
		t.Error("drawText produced no pixels")
	}
}

func TestDrawTextCentered(t *testing.T) {
	frame := newTestFrame(100, 20)
	drawTextCentered(frame, 0, 100, 13, "OK")
	if !anyNonWhite(frame) {
		t.Error("drawTextCentered produced no pixels")
	}
}

func anyNonWhite(frame *image.Paletted) bool {
	for _, px := range frame.Pix {
		if px != widget.PaperWhite {
			return true
		}
	}
	return false
}

func TestDrawTextCenteredGray(t *testing.T) {
	frame := newTestFrame(100, 20)
	drawTextCenteredGray(frame, 0, 100, 13, "OK", widget.PaperGray70)
	sawAny := false
	for _, px := range frame.Pix {
		if px != widget.PaperWhite {
			sawAny = true
			break
		}
	}
	if !sawAny {
		t.Error("drawTextCenteredGray produced no pixels")
	}
}

func TestDrawTextGrayWithFace(t *testing.T) {
	frame := newTestFrame(100, 20)
	drawTextGrayWithFace(frame, 0, 13, "Hi", defaultFace, widget.PaperGray50)
	sawAny := false
	for _, px := range frame.Pix {
		if px != widget.PaperWhite {
			sawAny = true
			break
		}
	}
	if !sawAny {
		t.Error("drawTextGrayWithFace produced no pixels")
	}
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
		// Multi-byte characters were previously sliced mid-sequence
		// because truncation worked on bytes. The Japanese label below
		// is 3 runes / 9 bytes — slicing at maxChars=2 must give the
		// first two runes, not the first two bytes of "あ" (which would
		// be invalid UTF-8).
		{"あいう", 2, "あい"},
		{"あいうえお", 4, "あ..."},
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

// mustLoadDefaultFace runs at package init with valid embedded
// fonts. Pin its panic branch by swapping in bad TTF data.
func TestMustLoadDefaultFace_PanicsOnFontError(t *testing.T) {
	restore := fonts.SwapDataForTest([]byte("bad"), []byte("bad"))
	defer restore()

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic from mustLoadDefaultFace")
		}
		msg, ok := r.(string)
		if !ok || !strings.Contains(msg, "weatherview: load font") {
			t.Errorf("panic = %v, want a string mentioning 'weatherview: load font'", r)
		}
	}()
	_ = mustLoadDefaultFace()
}

// mustLoadFace covers the per-face panic branch used by the
// weatherview.go init. The role label must appear in the panic
// message so it's easy to tell which face failed.
func TestMustLoadFace_PanicsOnFontError(t *testing.T) {
	restore := fonts.SwapDataForTest([]byte("bad"), []byte("bad"))
	defer restore()

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic from mustLoadFace")
		}
		msg, ok := r.(string)
		if !ok || !strings.Contains(msg, "weatherview: load smoke font") {
			t.Errorf("panic = %v, want a string mentioning 'weatherview: load smoke font'", r)
		}
	}()
	_ = mustLoadFace(fonts.Regular, 10, "smoke")
}
