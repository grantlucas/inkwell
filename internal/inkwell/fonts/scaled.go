package fonts

import (
	"image"

	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

// ScaledDrawer draws bitmap text far larger than any embedded Tamzen tier,
// with an independent weight axis. Size and weight are separate settings and
// conflating them is what makes big text look wrong:
//
//   - Scale blows each glyph's 1-bit mask up by an integer factor. Nearest-
//     neighbour scaling of a 1-bit mask is another 1-bit mask, so there is no
//     anti-aliased fringe for packBW's threshold to drop and nothing for
//     Gray4 to bucket wrongly. Nothing caps the factor but panel real estate.
//   - Grow dilates the scaled mask. Scaling multiplies stem and counter
//     together, so Tamzen Bold's stem ratio (2 px in a 10 px cell, 20%) is
//     invariant under it: a 60 px numeral carries the stroke weight of body
//     text and reads thin from across the room, exactly like a text face
//     blown up on a photocopier. Dilation is what turns it into a display
//     weight. See GrowFor for the radius to use at each scale.
//
// Every inked pixel is set to Index and nothing else is touched, so the
// result stays a two-value image — blank paper plus one palette entry.
type ScaledDrawer struct {
	Face  font.Face // the bitmap face to take glyph masks from
	Scale int       // integer pixel multiplier; anything below 1 draws at 1x
	Grow  int       // dilation radius in output pixels; negative counts as 0
	Index uint8     // palette index every inked pixel is set to
}

// Draw draws s with the pen at (x, baseline), clipping to dst, and returns
// the pen advance in output pixels. A rune the face has no glyph for draws
// nothing but still takes whatever advance the face reports for it.
func (d ScaledDrawer) Draw(dst *image.Paletted, x, baseline int, s string) int {
	return d.walk(s, func(r rune, pen int) {
		dr, mask, maskp, _, ok := d.Face.Glyph(fixed.P(0, 0), r)
		if !ok {
			return
		}
		d.blitGlyph(dst, x+pen, baseline, dr, mask, maskp)
	})
}

// DrawCentered draws s centred between x1 and x2 on the given baseline and
// returns the pen advance. A run wider than the box overhangs it evenly.
func (d ScaledDrawer) DrawCentered(dst *image.Paletted, x1, x2, baseline int, s string) int {
	return d.Draw(dst, x1+(x2-x1-d.Measure(s))/2, baseline, s)
}

// DrawRight draws s so its advance ends at xRight and returns that advance.
func (d ScaledDrawer) DrawRight(dst *image.Paletted, xRight, baseline int, s string) int {
	return d.Draw(dst, xRight-d.Measure(s), baseline, s)
}

// Measure returns the pen advance Draw would report for s, in output pixels,
// without drawing anything. Dilation does not move the pen — it can spill up
// to Grow px past the advance on either side — so the width depends only on
// the face and the scale.
func (d ScaledDrawer) Measure(s string) int {
	return d.walk(s, nil)
}

// walk advances the pen across s, calling visit (when non-nil) with each rune
// and the output-pixel offset its glyph starts at, and returns the total
// advance in output pixels.
//
// Draw and Measure both go through here so a run can never be placed by one
// set of rules and measured by another — that would misplace every centred
// and right-aligned label. The accumulation matches font.MeasureString: the
// kerning pair is added between runes, and a rune the face has no glyph for
// still takes the advance the face reports for it. Tamzen kerns at zero and
// reports a zero advance for a rune it lacks, so for the embedded faces this
// is the plain sum of integer advances.
func (d ScaledDrawer) walk(s string, visit func(r rune, penX int)) int {
	scale := d.scale()
	advance, prev := fixed.Int26_6(0), rune(-1)
	for _, r := range s {
		if prev >= 0 {
			advance += d.Face.Kern(prev, r)
		}
		if visit != nil {
			visit(r, advance.Ceil()*scale)
		}
		a, _ := d.Face.GlyphAdvance(r)
		advance += a
		prev = r
	}
	return advance.Ceil() * scale
}

// GrowFor returns the dilation radius that gives a display weight at the
// given scale, from the weight table measured on Tamzen Bold 10x20 (2 px stem
// against a 3 px counter at 1x): 2 from 3x up, 1 at 2x, 0 at 1x.
//
// Dilation adds its radius to the stem on each side and takes the same off
// the counter, so the counter is what runs out first. At 1x there are only
// 3 px of it to spend: a radius of 1 leaves a 1 px counter and fills in 8, 0
// and e. Body text therefore has no weight axis at all, and hierarchy below
// 2x has to come from the Bold cut rather than from dilation.
func GrowFor(scale int) int {
	switch {
	case scale >= 3:
		return 2
	case scale == 2:
		return 1
	default:
		return 0
	}
}

// blitGlyph paints one glyph mask, scaled and dilated, with its origin at the
// pen position (pen, baseline).
func (d ScaledDrawer) blitGlyph(
	dst *image.Paletted, pen, baseline int, dr image.Rectangle, mask image.Image, maskp image.Point,
) {
	scale, grow := d.scale(), d.grow()
	for gy := dr.Min.Y; gy < dr.Max.Y; gy++ {
		for gx := dr.Min.X; gx < dr.Max.X; gx++ {
			_, _, _, a := mask.At(maskp.X+gx-dr.Min.X, maskp.Y+gy-dr.Min.Y).RGBA()
			if a < 0x8000 {
				continue
			}
			// The union of every inked pixel's expanded rect is exactly the
			// dilation of the glyph's pixel set, so the structuring element
			// costs one extra term in this blit rather than a second pass.
			d.fill(dst, image.Rect(
				pen+gx*scale-grow, baseline+gy*scale-grow,
				pen+gx*scale+scale+grow, baseline+gy*scale+scale+grow,
			))
		}
	}
}

// fill sets the part of r that lands inside dst to the drawer's index.
func (d ScaledDrawer) fill(dst *image.Paletted, r image.Rectangle) {
	r = r.Intersect(dst.Bounds())
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			dst.SetColorIndex(x, y, d.Index)
		}
	}
}

func (d ScaledDrawer) scale() int {
	if d.Scale < 1 {
		return 1
	}
	return d.Scale
}

func (d ScaledDrawer) grow() int {
	if d.Grow < 0 {
		return 0
	}
	return d.Grow
}
