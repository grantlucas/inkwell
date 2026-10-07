package testutil

import (
	"image"

	"github.com/grantlucas/inkwell/internal/inkwell/widget"
)

// Inked reports whether anything but paper is drawn in r.
func Inked(frame *image.Paletted, r image.Rectangle) bool {
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			if frame.ColorIndexAt(x, y) != widget.PaperWhite {
				return true
			}
		}
	}
	return false
}

// SameIn reports whether a and b agree on every pixel of r.
func SameIn(a, b *image.Paletted, r image.Rectangle) bool {
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			if a.ColorIndexAt(x, y) != b.ColorIndexAt(x, y) {
				return false
			}
		}
	}
	return true
}

// PaintOutside inks every pixel of frame outside bounds solid black,
// standing in for the widgets the compositor puts around a widget. The
// draw helpers clip to the frame, not to the widget, so a widget that
// draws past its bounds shows up as paper or gray on that black.
func PaintOutside(frame *image.Paletted, bounds image.Rectangle) {
	for y := frame.Rect.Min.Y; y < frame.Rect.Max.Y; y++ {
		for x := frame.Rect.Min.X; x < frame.Rect.Max.X; x++ {
			if !image.Pt(x, y).In(bounds) {
				frame.SetColorIndex(x, y, widget.PaperBlack)
			}
		}
	}
}

// HasSolidSquare reports whether frame holds a side x side square that
// is entirely PaperBlack: a large fill that, sitting in the same place on
// every refresh, would invite burn-in.
func HasSolidSquare(frame *image.Paletted, side int) bool {
	b := frame.Bounds()
	// run[x] is how many PaperBlack pixels end at (x, y) going up.
	run := make([]int, b.Dx())
	for y := b.Min.Y; y < b.Max.Y; y++ {
		wide := 0
		for x := b.Min.X; x < b.Max.X; x++ {
			i := x - b.Min.X
			if frame.ColorIndexAt(x, y) == widget.PaperBlack {
				run[i]++
			} else {
				run[i] = 0
			}
			if run[i] >= side {
				wide++
			} else {
				wide = 0
			}
			if wide >= side {
				return true
			}
		}
	}
	return false
}
