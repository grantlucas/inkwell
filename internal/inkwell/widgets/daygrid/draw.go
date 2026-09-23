package daygrid

import (
	"image"
	"image/color"
	"image/draw"

	"github.com/grantlucas/inkwell/internal/inkwell/fonts"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

// The screens all draw the same way, because the panel only really
// supports one way of drawing. Text is the 20 px Tamzen tier, which is
// the floor for body text at distance (2.1 mm caps); anything larger is
// that same face run through a fonts.ScaledDrawer rather than a bigger
// font, so every glyph stays a 1-bit mask that survives both packers.
//
// These are thin wrappers, but sharing them is what stops each screen
// re-deciding what colour body text is. The answer is always solid
// PaperBlack (or PaperWhite on an inverted block): a gray source has its
// anti-aliased fringe chopped by the BW threshold and vanishes into
// Gray4's light bucket, so hierarchy comes from weight and size. See
// CLAUDE.md.

// BodyFace and BodyBoldFace are the 20 px tier in both weights.
var (
	BodyFace     font.Face
	BodyBoldFace font.Face
)

func init() {
	BodyFace = MustLoadFace(fonts.Regular, 16, "body")
	BodyBoldFace = MustLoadFace(fonts.Bold, 16, "body bold")
}

// MustLoadFace loads a face or panics. It is a separate function so the
// failure branch stays reachable from tests via fonts.SwapDataForTest;
// role names the face in the panic so a failure says which one.
func MustLoadFace(weight fonts.Weight, size float64, role string) font.Face {
	f, err := fonts.Face(weight, size)
	if err != nil {
		panic("daygrid: load " + role + " font: " + err.Error())
	}
	return f
}

// BodyAscent is the baseline offset of the body tier, the unit the
// screens' vertical measurements are built from.
func BodyAscent() int { return BodyFace.Metrics().Ascent.Ceil() }

// BodyLineH is one line of body text.
func BodyLineH() int { return BodyFace.Metrics().Height.Ceil() }

// BodyAdvance is the fixed advance of the monospaced tier, used to turn
// a pixel width into a character budget.
func BodyAdvance() int {
	adv, _ := BodyFace.GlyphAdvance('0')
	return adv.Ceil()
}

// Scaled builds a drawer for face at the given integer scale, dilated by
// the radius fonts.GrowFor prescribes for it, inking in idx.
//
// Scale and grow travel together because they are separate axes that
// look like one: scaling multiplies stem and counter equally, so a 60 px
// numeral would carry body text's stroke weight and read thin from
// across the room. Dilation is what turns size into weight.
func Scaled(face font.Face, scale int, idx uint8) fonts.ScaledDrawer {
	return fonts.ScaledDrawer{
		Face:  face,
		Scale: scale,
		Grow:  fonts.GrowFor(scale),
		Index: idx,
	}
}

// FillWhite clears a region to paper.
func FillWhite(frame *image.Paletted, r image.Rectangle) {
	draw.Draw(frame, r, image.NewUniform(color.White), image.Point{}, draw.Src)
}

// FillRect floods a region with one palette index.
func FillRect(frame *image.Paletted, r image.Rectangle, idx uint8) {
	draw.Draw(frame, r, image.NewUniform(widget.PaperPalette[idx]), image.Point{}, draw.Src)
}

// DrawText draws body text with its baseline at (x, y) in the given
// palette index.
func DrawText(frame *image.Paletted, x, y int, text string, f font.Face, idx uint8) {
	d := &font.Drawer{
		Dst:  frame,
		Src:  image.NewUniform(widget.PaperPalette[idx]),
		Face: f,
		Dot:  fixed.P(x, y),
	}
	d.DrawString(text)
}

// TextWidth is the advance text would take in f.
func TextWidth(f font.Face, text string) int {
	return font.MeasureString(f, text).Ceil()
}

// DrawTextCentered centres text between x1 and x2.
func DrawTextCentered(frame *image.Paletted, x1, x2, y int, text string, f font.Face, idx uint8) {
	DrawText(frame, x1+(x2-x1-TextWidth(f, text))/2, y, text, f, idx)
}

// DrawTextRight ends text's advance at xRight.
func DrawTextRight(frame *image.Paletted, xRight, y int, text string, f font.Face, idx uint8) {
	DrawText(frame, xRight-TextWidth(f, text), y, text, f, idx)
}

// DrawHLine draws a horizontal rule.
func DrawHLine(frame *image.Paletted, x1, x2, y int, idx uint8) {
	for x := x1; x < x2; x++ {
		SetPixel(frame, x, y, idx)
	}
}

// DrawVLine draws a vertical rule.
func DrawVLine(frame *image.Paletted, x, y1, y2 int, idx uint8) {
	for y := y1; y < y2; y++ {
		SetPixel(frame, x, y, idx)
	}
}

// SetPixel inks one pixel, clipping to the frame.
func SetPixel(frame *image.Paletted, x, y int, idx uint8) {
	if image.Pt(x, y).In(frame.Bounds()) {
		frame.SetColorIndex(x, y, idx)
	}
}
