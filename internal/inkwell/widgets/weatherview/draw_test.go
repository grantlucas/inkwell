package weatherview

import (
	"image"
	"slices"
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

// walkLine visits the Bresenham line from one end to the other, both ends
// included, in whichever direction it is asked to walk.
func TestWalkLine(t *testing.T) {
	tests := []struct {
		label          string
		x1, y1, x2, y2 int
		want           []image.Point
	}{
		{label: "horizontal", x1: 2, y1: 5, x2: 5, y2: 5, want: []image.Point{{2, 5}, {3, 5}, {4, 5}, {5, 5}}},
		{label: "vertical", x1: 5, y1: 2, x2: 5, y2: 4, want: []image.Point{{5, 2}, {5, 3}, {5, 4}}},
		{label: "diagonal", x1: 0, y1: 0, x2: 2, y2: 2, want: []image.Point{{0, 0}, {1, 1}, {2, 2}}},
		{label: "reversed", x1: 2, y1: 2, x2: 0, y2: 0, want: []image.Point{{2, 2}, {1, 1}, {0, 0}}},
		{label: "shallow", x1: 0, y1: 0, x2: 4, y2: 2, want: []image.Point{{0, 0}, {1, 0}, {2, 1}, {3, 1}, {4, 2}}},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			var got []image.Point
			walkLine(tt.x1, tt.y1, tt.x2, tt.y2, func(x, y int) { got = append(got, image.Pt(x, y)) })
			if !slices.Equal(got, tt.want) {
				t.Errorf("walkLine visited %v, want %v", got, tt.want)
			}
		})
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
