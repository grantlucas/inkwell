package testutil_test

import (
	"image"
	"testing"

	"github.com/grantlucas/inkwell/internal/inkwell/testutil"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
)

// paper is a blank frame with a black square at (30,40)-(50,60) and a
// gray pixel at (5,5).
func paper() *image.Paletted {
	f := image.NewPaletted(image.Rect(0, 0, 100, 100), widget.PaperPalette)
	for y := 40; y < 60; y++ {
		for x := 30; x < 50; x++ {
			f.SetColorIndex(x, y, widget.PaperBlack)
		}
	}
	f.SetColorIndex(5, 5, widget.PaperGray70)
	return f
}

// A rectangle is inked when anything but paper is drawn in it, gray as
// well as black.
func TestInked(t *testing.T) {
	tests := []struct {
		label string
		r     image.Rectangle
		want  bool
	}{
		{"blank paper", image.Rect(60, 60, 100, 100), false},
		{"black", image.Rect(45, 55, 60, 70), true},
		{"gray", image.Rect(0, 0, 10, 10), true},
	}
	f := paper()
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			if got := testutil.Inked(f, tt.r); got != tt.want {
				t.Errorf("Inked(%v) = %v, want %v", tt.r, got, tt.want)
			}
		})
	}
}

// Two frames are the same in a rectangle when every pixel of it agrees;
// a difference outside it doesn't count.
func TestSameIn(t *testing.T) {
	a, b := paper(), paper()
	b.SetColorIndex(90, 90, widget.PaperBlack)
	tests := []struct {
		label string
		r     image.Rectangle
		want  bool
	}{
		{"agree everywhere in it", image.Rect(0, 0, 80, 80), true},
		{"differ at one pixel in it", image.Rect(80, 80, 100, 100), false},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			if got := testutil.SameIn(a, b, tt.r); got != tt.want {
				t.Errorf("SameIn(%v) = %v, want %v", tt.r, got, tt.want)
			}
		})
	}
}

// Painting outside a widget's bounds stands in for its neighbours: every
// pixel outside them is black, and none inside is touched.
func TestPaintOutside(t *testing.T) {
	f := image.NewPaletted(image.Rect(0, 0, 40, 30), widget.PaperPalette)
	bounds := image.Rect(10, 5, 30, 20)
	testutil.PaintOutside(f, bounds)
	for y := range 30 {
		for x := range 40 {
			want := uint8(widget.PaperBlack)
			if image.Pt(x, y).In(bounds) {
				want = widget.PaperWhite
			}
			if got := f.ColorIndexAt(x, y); got != want {
				t.Fatalf("(%d,%d) = %d, want %d", x, y, got, want)
			}
		}
	}
}

// The burn-in guard has to be able to fail: it finds a solid black
// square of the size asked for, and nothing larger or on blank paper.
func TestHasSolidSquare(t *testing.T) {
	blank := image.NewPaletted(image.Rect(0, 0, 100, 100), widget.PaperPalette)
	tests := []struct {
		label string
		frame *image.Paletted
		side  int
		want  bool
	}{
		{"blank paper", blank, 20, false},
		{"a 20x20 black square", paper(), 20, true},
		{"nothing larger than it", paper(), 21, false},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			if got := testutil.HasSolidSquare(tt.frame, tt.side); got != tt.want {
				t.Errorf("HasSolidSquare(%d) = %v, want %v", tt.side, got, tt.want)
			}
		})
	}
}
