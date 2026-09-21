package fonts

import (
	"fmt"
	"image"
	"image/color"
	"testing"

	"github.com/grantlucas/inkwell/internal/inkwell/testutil"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

// twoIndexPalette is the minimum palette a test frame needs: index 0 white
// (the zero value of Paletted.Pix, i.e. blank paper) and index 1 black.
var twoIndexPalette = color.Palette{color.Gray{Y: 0xFF}, color.Gray{Y: 0x00}}

func newFrame(w, h int) *image.Paletted {
	return image.NewPaletted(image.Rect(0, 0, w, h), twoIndexPalette)
}

func testFace(t *testing.T) font.Face {
	t.Helper()
	f, err := Face(Bold, 16) // the 10x20 tier
	if err != nil {
		t.Fatalf("Face: %v", err)
	}
	return f
}

// inkedAt reports the set of inked pixels, keyed by point.
func inkedAt(m *image.Paletted) map[image.Point]bool {
	got := map[image.Point]bool{}
	b := m.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if m.ColorIndexAt(x, y) != 0 {
				got[image.Pt(x, y)] = true
			}
		}
	}
	return got
}

// At 1x with no dilation the drawer must deposit exactly the pixels
// font.Drawer would, so it is a drop-in for plain bitmap text.
func TestScaledDrawer_UnscaledMatchesFontDrawer(t *testing.T) {
	face := testFace(t)
	const text = "80"

	want := newFrame(80, 40)
	d := &font.Drawer{
		Dst:  want,
		Src:  image.NewUniform(color.Black),
		Face: face,
		Dot:  fixed.P(4, 28),
	}
	d.DrawString(text)

	got := newFrame(80, 40)
	adv := ScaledDrawer{Face: face, Index: 1}.Draw(got, 4, 28, text)

	if wantAdv := font.MeasureString(face, text).Ceil(); adv != wantAdv {
		t.Errorf("advance = %d, want %d", adv, wantAdv)
	}
	wantInk, gotInk := inkedAt(want), inkedAt(got)
	if len(gotInk) == 0 {
		t.Fatal("nothing was drawn")
	}
	for p := range wantInk {
		if !gotInk[p] {
			t.Fatalf("pixel %v inked by font.Drawer but not by ScaledDrawer", p)
		}
	}
	for p := range gotInk {
		if !wantInk[p] {
			t.Fatalf("pixel %v inked by ScaledDrawer but not by font.Drawer", p)
		}
	}
}

// Scaling is nearest-neighbour on the 1-bit mask: every pixel inked at 1x
// becomes a solid scale x scale block and nothing else is touched. That is
// what keeps the result a pure 1-bit mask with no fringe for packBW to drop.
func TestScaledDrawer_ScalesEachPixelToASolidBlock(t *testing.T) {
	face := testFace(t)
	const text = "80"

	for _, scale := range []int{2, 3, 4} {
		t.Run(string(rune('0'+scale))+"x", func(t *testing.T) {
			base := image.NewPaletted(image.Rect(-8, -32, 48, 16), twoIndexPalette)
			ScaledDrawer{Face: face, Index: 1}.Draw(base, 0, 0, text)

			big := image.NewPaletted(
				image.Rect(-8*scale, -32*scale, 48*scale, 16*scale), twoIndexPalette)
			ScaledDrawer{Face: face, Scale: scale, Index: 1}.Draw(big, 0, 0, text)

			b := base.Bounds()
			for y := b.Min.Y; y < b.Max.Y; y++ {
				for x := b.Min.X; x < b.Max.X; x++ {
					want := base.ColorIndexAt(x, y)
					for j := range scale {
						for i := range scale {
							px, py := x*scale+i, y*scale+j
							if got := big.ColorIndexAt(px, py); got != want {
								t.Fatalf("scale %d: pixel (%d,%d) = %d, want %d from 1x (%d,%d)",
									scale, px, py, got, want, x, y)
							}
						}
					}
				}
			}
		})
	}
}

// The advance scales with the glyphs, and Measure reports it without drawing.
func TestScaledDrawer_MeasureMatchesDrawAdvance(t *testing.T) {
	face := testFace(t)
	cases := []struct {
		label string
		text  string
		scale int
	}{
		{"empty", "", 3},
		{"digits 1x", "80", 1},
		{"digits 3x", "80", 3},
		{"mixed 2x", "MON 21", 2},
		{"zero scale draws at 1x", "80", 0},
	}
	for _, c := range cases {
		t.Run(c.label, func(t *testing.T) {
			d := ScaledDrawer{Face: face, Scale: c.scale, Index: 1}
			frame := newFrame(400, 120)
			if got, want := d.Draw(frame, 10, 100, c.text), d.Measure(c.text); got != want {
				t.Errorf("Draw advance = %d, Measure = %d", got, want)
			}
			scale := max(c.scale, 1)
			if want := font.MeasureString(face, c.text).Ceil() * scale; d.Measure(c.text) != want {
				t.Errorf("Measure = %d, want %d", d.Measure(c.text), want)
			}
		})
	}
}

// Grow is a morphological dilation of the *scaled* mask by a (2*grow+1)
// square, so stems thicken on every side and counters close symmetrically.
// Weight is therefore a separate axis from size: scaling alone preserves
// Tamzen Bold's 20% stem ratio however large the glyph gets.
func TestScaledDrawer_GrowDilatesTheScaledMask(t *testing.T) {
	face := testFace(t)
	const text = "80"

	cases := []struct{ scale, grow int }{{1, 1}, {2, 1}, {2, 2}, {3, 1}, {3, 2}, {3, 3}}
	for _, c := range cases {
		t.Run(label(c.scale, c.grow), func(t *testing.T) {
			bounds := image.Rect(-40, -120, 200, 60)
			plain := image.NewPaletted(bounds, twoIndexPalette)
			ScaledDrawer{Face: face, Scale: c.scale, Index: 1}.Draw(plain, 0, 0, text)

			grown := image.NewPaletted(bounds, twoIndexPalette)
			ScaledDrawer{Face: face, Scale: c.scale, Grow: c.grow, Index: 1}.Draw(grown, 0, 0, text)

			want := dilate(inkedAt(plain), c.grow)
			got := inkedAt(grown)
			for p := range want {
				if !got[p] && p.In(bounds) {
					t.Fatalf("pixel %v should be inked by dilation radius %d", p, c.grow)
				}
			}
			for p := range got {
				if !want[p] {
					t.Fatalf("pixel %v inked beyond dilation radius %d", p, c.grow)
				}
			}
		})
	}
}

// dilate expands a pixel set by a square structuring element of radius grow.
func dilate(ink map[image.Point]bool, grow int) map[image.Point]bool {
	out := map[image.Point]bool{}
	for p := range ink {
		for dy := -grow; dy <= grow; dy++ {
			for dx := -grow; dx <= grow; dx++ {
				out[image.Pt(p.X+dx, p.Y+dy)] = true
			}
		}
	}
	return out
}

func label(scale, grow int) string {
	return fmt.Sprintf("%dx_grow%d", scale, grow)
}

// GrowFor encodes the weight table from the design pass: 2 from 3x up, 1 at
// 2x, and 0 at 1x — body text has no weight axis, because a grow of 1 at 1x
// takes the stem to 4 px against a 1 px counter and fills every bowl.
func TestGrowFor(t *testing.T) {
	cases := []struct{ scale, want int }{
		{-1, 0}, {0, 0}, {1, 0}, {2, 1}, {3, 2}, {4, 2}, {8, 2},
	}
	for _, c := range cases {
		if got := GrowFor(c.scale); got != c.want {
			t.Errorf("GrowFor(%d) = %d, want %d", c.scale, got, c.want)
		}
	}
}

// The alignment wrappers place the same run of text by its measured width,
// so a caller never has to redo the scale arithmetic.
func TestScaledDrawer_Alignment(t *testing.T) {
	face := testFace(t)
	const text = "80"
	d := ScaledDrawer{Face: face, Scale: 2, Grow: 1, Index: 1}
	w := d.Measure(text)

	cases := []struct {
		label string
		draw  func(dst *image.Paletted) int
		wantX int
	}{
		{"centered", func(dst *image.Paletted) int {
			return d.DrawCentered(dst, 100, 300, 80, text)
		}, 100 + (200-w)/2},
		{"centered in a box narrower than the text", func(dst *image.Paletted) int {
			return d.DrawCentered(dst, 100, 100+w-10, 80, text)
		}, 100 - 5},
		{"right aligned", func(dst *image.Paletted) int {
			return d.DrawRight(dst, 300, 80, text)
		}, 300 - w},
	}
	for _, c := range cases {
		t.Run(c.label, func(t *testing.T) {
			got := newFrame(400, 120)
			if adv := c.draw(got); adv != w {
				t.Errorf("advance = %d, want %d", adv, w)
			}
			want := newFrame(400, 120)
			d.Draw(want, c.wantX, 80, text)
			if x, y, ok := firstIndexDiff(got, want); !ok {
				t.Errorf("placement differs from Draw at x=%d, want pen x=%d (first diff %d,%d)",
					c.wantX, c.wantX, x, y)
			}
		})
	}
}

// firstIndexDiff reports the first pixel where two paletted frames disagree.
func firstIndexDiff(got, want *image.Paletted) (x, y int, ok bool) {
	b := got.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if got.ColorIndexAt(x, y) != want.ColorIndexAt(x, y) {
				return x, y, false
			}
		}
	}
	return 0, 0, true
}

// The output must hold nothing but blank paper and the requested index — no
// intermediate grays. That property is what makes the technique safe on both
// packer paths (threshold for BW, 4 buckets for Gray4), so it has to fail
// loudly if anyone later routes this through an anti-aliased path.
func TestScaledDrawer_InksOnlyTheRequestedIndex(t *testing.T) {
	face := testFace(t)
	for _, idx := range []uint8{widget.PaperBlack, widget.PaperGray70, widget.PaperWhite} {
		t.Run(fmt.Sprintf("index%d", idx), func(t *testing.T) {
			frame := image.NewPaletted(image.Rect(0, 0, 300, 120), widget.PaperPalette)
			ScaledDrawer{Face: face, Scale: 3, Grow: 2, Index: idx}.Draw(frame, 10, 100, "80")
			for _, got := range frame.Pix {
				if got != widget.PaperWhite && got != idx {
					t.Fatalf("frame contains index %d, want only %d (paper) or %d",
						got, widget.PaperWhite, idx)
				}
			}
		})
	}
}

// Ink may spill past the advance by the dilation radius and no further, so a
// caller can lay runs out on the measured width and know what overlaps.
func TestScaledDrawer_InkStaysWithinAdvancePlusGrow(t *testing.T) {
	face := testFace(t)
	const (
		text = "80"
		penX = 60
	)
	cases := []struct{ scale, grow int }{{1, 0}, {2, 1}, {3, 2}, {3, 3}}
	for _, c := range cases {
		t.Run(label(c.scale, c.grow), func(t *testing.T) {
			frame := newFrame(400, 200)
			d := ScaledDrawer{Face: face, Scale: c.scale, Grow: c.grow, Index: 1}
			adv := d.Draw(frame, penX, 150, text)

			loX, hiX := penX-c.grow, penX+adv+c.grow
			for p := range inkedAt(frame) {
				if p.X < loX || p.X >= hiX {
					t.Fatalf("ink at x=%d outside [%d,%d) for advance %d grow %d",
						p.X, loX, hiX, adv, c.grow)
				}
			}
		})
	}
}

// A rune the face does not carry is skipped entirely: no ink, no advance —
// the same rule font.MeasureString follows, so Draw and Measure stay in step.
func TestScaledDrawer_SkipsMissingGlyphs(t *testing.T) {
	face := testFace(t)
	const missing = '漢'
	if _, _, ok := face.GlyphBounds(missing); ok {
		t.Fatalf("test needs a rune Tamzen lacks; %q is present", missing)
	}

	d := ScaledDrawer{Face: face, Scale: 2, Grow: 1, Index: 1}
	with := newFrame(300, 120)
	advWith := d.Draw(with, 10, 100, "8"+string(missing)+"0")
	without := newFrame(300, 120)
	advWithout := d.Draw(without, 10, 100, "80")

	if advWith != advWithout {
		t.Errorf("advance with missing glyph = %d, want %d", advWith, advWithout)
	}
	if x, y, ok := firstIndexDiff(with, without); !ok {
		t.Errorf("missing glyph changed the render at (%d,%d)", x, y)
	}
}

// Drawing off the edge clips instead of panicking, and the pixels that do
// land are the same ones an unclipped frame would get.
func TestScaledDrawer_ClipsToDestination(t *testing.T) {
	face := testFace(t)
	d := ScaledDrawer{Face: face, Scale: 3, Grow: 2, Index: 1}

	cases := []struct {
		label  string
		x, y   int
		wantNo bool // true when the run falls entirely outside the frame
	}{
		{"over the left edge", -40, 100, false},
		{"over the right edge", 180, 100, false},
		{"over the top edge", 20, 10, false},
		{"over the bottom edge", 20, 140, false},
		{"entirely off frame", -400, 100, true},
	}
	for _, c := range cases {
		t.Run(c.label, func(t *testing.T) {
			clipped := newFrame(200, 120)
			d.Draw(clipped, c.x, c.y, "80")

			roomy := image.NewPaletted(image.Rect(-500, -200, 500, 400), twoIndexPalette)
			d.Draw(roomy, c.x, c.y, "80")

			ink := inkedAt(clipped)
			if c.wantNo != (len(ink) == 0) {
				t.Fatalf("inked %d pixels, wantNone = %v", len(ink), c.wantNo)
			}
			b := clipped.Bounds()
			for y := b.Min.Y; y < b.Max.Y; y++ {
				for x := b.Min.X; x < b.Max.X; x++ {
					if clipped.ColorIndexAt(x, y) != roomy.ColorIndexAt(x, y) {
						t.Fatalf("clipped pixel (%d,%d) differs from the unclipped render", x, y)
					}
				}
			}
		})
	}
}

// The weight table from the design pass, pinned as golden renders. Each row
// is a scale/grow pair with its measured stem and counter on Tamzen Bold
// 10x20 (2 px stem, 3 px counter at 1x); 8, 0 and the colon are the limiting
// glyphs, so those are what the card draws. The degenerate 1x/grow-1 row is
// here on purpose — it is the "do not do this" boundary, and a golden is the
// only thing that keeps it from being quietly adopted as a default.
func TestScaledDrawer_WeightTable(t *testing.T) {
	face := testFace(t)
	cases := []struct {
		scale, grow int
		note        string
	}{
		{1, 0, "body weight — the only usable setting at 1x"},
		{1, 1, "unusable: 4 px stem against a 1 px counter, every bowl fills"},
		{2, 1, "display weight at 2x: 6 px stem, 4 px counter"},
		{2, 2, "too far at 2x: clogs zeros and the colon"},
		{3, 0, "text weight: 20% stem ratio survives the scale, reads thin"},
		{3, 1, "safe display weight: 8 px stem, 7 px counter"},
		{3, 2, "poster weight: 10 px stem, 5 px counter"},
		{3, 3, "too far at 3x: 12 px stem, 3 px counter — 8 and 0 fill in"},
	}
	for _, c := range cases {
		t.Run(label(c.scale, c.grow), func(t *testing.T) {
			frame := image.NewPaletted(image.Rect(0, 0, 220, 90), widget.PaperPalette)
			d := ScaledDrawer{Face: face, Scale: c.scale, Grow: c.grow, Index: widget.PaperBlack}
			d.Draw(frame, 8, 74, "10:08")
			testutil.AssertGoldenPNG(t, frame)
		})
	}
}

// The stem and counter widths the weight table is built on, measured off the
// render rather than asserted in prose. Dilation adds grow px to the stem on
// each side and takes the same off the counter, which is the whole reason the
// table stops where it does: at 1x there are only 3 px of counter to spend.
func TestScaledDrawer_StemAndCounterMatchTheWeightTable(t *testing.T) {
	face := testFace(t)
	cases := []struct {
		scale, grow   int
		stem, counter int
	}{
		{1, 0, 2, 3},
		{1, 1, 4, 1}, // unusable — one pixel of counter left
		{2, 0, 4, 6},
		{2, 1, 6, 4},
		{2, 2, 8, 2},
		{3, 0, 6, 9},
		{3, 1, 8, 7},
		{3, 2, 10, 5},
		{3, 3, 12, 3},
	}
	for _, c := range cases {
		t.Run(label(c.scale, c.grow), func(t *testing.T) {
			frame := newFrame(120, 120)
			ScaledDrawer{Face: face, Scale: c.scale, Grow: c.grow, Index: 1}.Draw(frame, 20, 100, "8")
			stem, counter := stemAndCounter(frame)
			if stem != c.stem || counter != c.counter {
				t.Errorf("stem %d counter %d, want stem %d counter %d", stem, counter, c.stem, c.counter)
			}
		})
	}
}

// stemAndCounter measures a glyph's left stem and its widest counter: it
// looks for the row holding the widest gap enclosed by ink on both sides and
// reports that gap along with the ink run to its left.
func stemAndCounter(m *image.Paletted) (stem, counter int) {
	b := m.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		inkRun, gapRun, sawInk := 0, 0, false
		for x := b.Min.X; x <= b.Max.X; x++ {
			inked := x < b.Max.X && m.ColorIndexAt(x, y) != 0
			switch {
			case inked && gapRun > 0: // gap closed by ink on the right
				if sawInk && gapRun > counter {
					stem, counter = inkRun, gapRun
				}
				inkRun, gapRun = 1, 0
			case inked:
				inkRun++
				sawInk = true
			case sawInk:
				gapRun++
			}
		}
	}
	return stem, counter
}

// Tamzen's zero is slashed, so its counter is split in two and it fills in
// one step before the 8 does: at 3x the 8 still holds a 3 px counter at grow
// 3 while the 0 has closed completely. That makes the zero the limiting
// glyph, and grow 2 the last radius where a numeral still reads as a numeral.
func TestScaledDrawer_ZeroIsTheLimitingGlyph(t *testing.T) {
	face := testFace(t)
	draw := func(glyph string, grow int) int {
		frame := newFrame(120, 120)
		ScaledDrawer{Face: face, Scale: 3, Grow: grow, Index: 1}.Draw(frame, 20, 100, glyph)
		_, counter := stemAndCounter(frame)
		return counter
	}
	if got := draw("0", 2); got != 5 {
		t.Errorf("zero counter at grow 2 = %d, want 5 (still open)", got)
	}
	if got := draw("0", 3); got != 0 {
		t.Errorf("zero counter at grow 3 = %d, want 0 (filled in)", got)
	}
	if got := draw("8", 3); got == 0 {
		t.Error("eight should still hold a counter at grow 3 — the zero fills first")
	}
}

// The zero value is a plain 1x drawer, and out-of-range settings fall back to
// it rather than erroring — a widget that computes a scale or grow from
// layout arithmetic still renders something legible.
func TestScaledDrawer_OutOfRangeSettingsFallBackToPlainText(t *testing.T) {
	face := testFace(t)
	cases := []struct {
		label string
		d     ScaledDrawer
	}{
		{"zero value scale", ScaledDrawer{Face: face, Index: 1}},
		{"negative scale", ScaledDrawer{Face: face, Scale: -2, Index: 1}},
		{"negative grow", ScaledDrawer{Face: face, Grow: -3, Index: 1}},
	}
	want := newFrame(200, 120)
	ScaledDrawer{Face: face, Scale: 1, Grow: 0, Index: 1}.Draw(want, 10, 100, "80")

	for _, c := range cases {
		t.Run(c.label, func(t *testing.T) {
			got := newFrame(200, 120)
			c.d.Draw(got, 10, 100, "80")
			if x, y, ok := firstIndexDiff(got, want); !ok {
				t.Errorf("render differs from plain 1x text at (%d,%d)", x, y)
			}
		})
	}
}
