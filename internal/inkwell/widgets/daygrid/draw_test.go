package daygrid

import (
	"image"
	"strings"
	"testing"

	"github.com/grantlucas/inkwell/internal/inkwell/fonts"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
)

// MustLoadFace runs at package init with valid embedded fonts. Pin its
// failure branch by swapping in data that will not parse.
func TestMustLoadFace_PanicsOnFontError(t *testing.T) {
	restore := fonts.SwapDataForTest([]byte("bad"), []byte("bad"))
	defer restore()

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected a panic")
		}
		msg, ok := r.(string)
		if !ok || !strings.Contains(msg, "daygrid: load smoke font") {
			t.Errorf("panic = %v, want a string naming the failed face", r)
		}
	}()
	_ = MustLoadFace(fonts.Regular, 16, "smoke")
}

// The body tier is the floor for text on this panel at distance, and
// every vertical measurement in the screens is derived from it, so a
// change here moves every layout.
func TestBodyMetrics(t *testing.T) {
	if got := BodyAscent(); got != 14 {
		t.Errorf("BodyAscent = %d, want 14 (the 20 px tier)", got)
	}
	if got := BodyLineH(); got != 20 {
		t.Errorf("BodyLineH = %d, want 20", got)
	}
	if got := BodyAdvance(); got != 10 {
		t.Errorf("BodyAdvance = %d, want 10 (monospaced)", got)
	}
}

func newFrame(w, h int) *image.Paletted {
	frame := image.NewPaletted(image.Rect(0, 0, w, h), widget.PaperPalette)
	FillWhite(frame, frame.Bounds())
	return frame
}

func countIdx(frame *image.Paletted, idx uint8) int {
	n := 0
	for _, px := range frame.Pix {
		if px == idx {
			n++
		}
	}
	return n
}

func TestFillWhiteAndFillRect(t *testing.T) {
	frame := newFrame(20, 10)
	if got := countIdx(frame, widget.PaperWhite); got != 200 {
		t.Errorf("FillWhite left %d white pixels, want 200", got)
	}

	FillRect(frame, image.Rect(0, 0, 10, 10), widget.PaperBlack)
	if got := countIdx(frame, widget.PaperBlack); got != 100 {
		t.Errorf("FillRect inked %d pixels, want 100", got)
	}
}

func TestDrawLinesAndPixels(t *testing.T) {
	frame := newFrame(20, 20)

	DrawHLine(frame, 2, 8, 5, widget.PaperBlack)
	DrawVLine(frame, 3, 10, 15, widget.PaperBlack)
	if got := countIdx(frame, widget.PaperBlack); got != 11 {
		t.Errorf("drew %d pixels, want 11 (6 + 5)", got)
	}

	// Out-of-frame coordinates are clipped rather than panicking: the
	// screens position content from a band's top and the frame is
	// shared with every other widget.
	SetPixel(frame, -1, -1, widget.PaperBlack)
	SetPixel(frame, 100, 100, widget.PaperBlack)
	if got := countIdx(frame, widget.PaperBlack); got != 11 {
		t.Errorf("an off-frame pixel was drawn: %d inked", got)
	}
}

func TestDrawTextAlignment(t *testing.T) {
	const w = 200
	text := "MON"
	width := TextWidth(BodyFace, text)

	// leftmost inked column, or -1.
	leftEdge := func(frame *image.Paletted) int {
		for x := range w {
			for y := range 30 {
				if frame.ColorIndexAt(x, y) != widget.PaperWhite {
					return x
				}
			}
		}
		return -1
	}

	left := newFrame(w, 30)
	DrawText(left, 10, 20, text, BodyFace, widget.PaperBlack)
	if got := leftEdge(left); got < 10 || got > 12 {
		t.Errorf("DrawText left edge = %d, want ~10", got)
	}

	centred := newFrame(w, 30)
	DrawTextCentered(centred, 0, w, 20, text, BodyFace, widget.PaperBlack)
	if want, got := (w-width)/2, leftEdge(centred); got < want || got > want+2 {
		t.Errorf("DrawTextCentered left edge = %d, want ~%d", got, want)
	}

	right := newFrame(w, 30)
	DrawTextRight(right, w, 20, text, BodyFace, widget.PaperBlack)
	if want, got := w-width, leftEdge(right); got < want || got > want+2 {
		t.Errorf("DrawTextRight left edge = %d, want ~%d", got, want)
	}
}

// Text is drawn in the index it is given and nothing else, so an
// inverted block can paint PaperWhite glyphs on PaperBlack.
func TestDrawTextUsesTheGivenIndex(t *testing.T) {
	frame := newFrame(100, 30)
	FillRect(frame, frame.Bounds(), widget.PaperBlack)
	DrawText(frame, 5, 20, "MON", BodyFace, widget.PaperWhite)

	if countIdx(frame, widget.PaperWhite) == 0 {
		t.Error("no white glyph pixels on the inverted block")
	}
}

// Scale and grow are separate axes that look like one. Scaling alone
// multiplies stem and counter together, so a big numeral would carry
// body text's stroke weight; Scaled pairs each scale with the dilation
// radius fonts.GrowFor prescribes for it.
func TestScaledPairsGrowWithScale(t *testing.T) {
	for _, scale := range []int{1, 2, 3} {
		d := Scaled(BodyBoldFace, scale, widget.PaperBlack)
		if d.Scale != scale {
			t.Errorf("Scale = %d, want %d", d.Scale, scale)
		}
		if want := fonts.GrowFor(scale); d.Grow != want {
			t.Errorf("scale %d: Grow = %d, want %d", scale, d.Grow, want)
		}
		if d.Index != widget.PaperBlack {
			t.Errorf("Index = %d, want PaperBlack", d.Index)
		}
	}

	// A bigger scale really does draw a bigger run.
	small := Scaled(BodyBoldFace, 1, widget.PaperBlack).Measure("16")
	large := Scaled(BodyBoldFace, 3, widget.PaperBlack).Measure("16")
	if large != 3*small {
		t.Errorf("3x measured %d, want 3 × %d", large, small)
	}
}
