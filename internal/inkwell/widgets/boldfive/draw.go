package boldfive

import (
	"image"
	"image/color"
	"image/draw"

	"github.com/grantlucas/inkwell/internal/inkwell/fonts"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

// bodyFace is the 20 px Tamzen tier, which is the floor for body text on
// this panel at distance (2.1 mm caps). Everything larger is this same
// face run through a fonts.ScaledDrawer rather than a bigger font, so
// every glyph stays a 1-bit mask that survives both packers.
var (
	bodyFace     font.Face
	bodyBoldFace font.Face
)

func init() {
	bodyFace = mustLoadFace(fonts.Regular, 16, "body")
	bodyBoldFace = mustLoadFace(fonts.Bold, 16, "body bold")
}

// mustLoadFace is extracted so the load-failure branch stays reachable
// from tests via fonts.SwapDataForTest. The role names the face in the
// panic so a failure says which one.
func mustLoadFace(weight fonts.Weight, size float64, role string) font.Face {
	f, err := fonts.Face(weight, size)
	if err != nil {
		panic("boldfive: load " + role + " font: " + err.Error())
	}
	return f
}

// bodyAscent is the baseline offset of the 20 px tier, the unit every
// vertical measurement in this package is built from.
func bodyAscent() int { return bodyFace.Metrics().Ascent.Ceil() }

// bodyLineH is one line of body text.
func bodyLineH() int { return bodyFace.Metrics().Height.Ceil() }

// bodyAdvance is the fixed advance of the monospaced tier, used to turn
// a pixel width into a character budget.
func bodyAdvance() int {
	adv, _ := bodyFace.GlyphAdvance('0')
	return adv.Ceil()
}

// scaled builds a drawer for the body face at the given integer scale,
// dilated by the radius fonts.GrowFor prescribes for it. Scale and grow
// travel together everywhere in this package: scaling alone multiplies
// stem and counter equally, so a 60 px numeral would carry body text's
// stroke weight and read thin from across the room.
func scaled(face font.Face, scale int) fonts.ScaledDrawer {
	return fonts.ScaledDrawer{
		Face:  face,
		Scale: scale,
		Grow:  fonts.GrowFor(scale),
		Index: widget.PaperBlack,
	}
}

func fillWhite(frame *image.Paletted, r image.Rectangle) {
	draw.Draw(frame, r, image.NewUniform(color.White), image.Point{}, draw.Src)
}

// drawText draws body text in solid PaperBlack. Every glyph is a 1-bit
// mask, so black paints solid-black pixels that read on both the BW
// threshold and the Gray4 buckets; hierarchy comes from weight and size,
// never from colour (see CLAUDE.md).
func drawText(frame *image.Paletted, x, y int, text string, f font.Face) {
	d := &font.Drawer{
		Dst:  frame,
		Src:  image.NewUniform(widget.PaperPalette[widget.PaperBlack]),
		Face: f,
		Dot:  fixed.P(x, y),
	}
	d.DrawString(text)
}

func textWidth(f font.Face, text string) int {
	return font.MeasureString(f, text).Ceil()
}

func drawTextCentered(frame *image.Paletted, x1, x2, y int, text string, f font.Face) {
	drawText(frame, x1+(x2-x1-textWidth(f, text))/2, y, text, f)
}

func drawTextRight(frame *image.Paletted, xRight, y int, text string, f font.Face) {
	drawText(frame, xRight-textWidth(f, text), y, text, f)
}

func drawHLine(frame *image.Paletted, x1, x2, y int, idx uint8) {
	for x := x1; x < x2; x++ {
		setPixel(frame, x, y, idx)
	}
}

func drawVLine(frame *image.Paletted, x, y1, y2 int, idx uint8) {
	for y := y1; y < y2; y++ {
		setPixel(frame, x, y, idx)
	}
}

func setPixel(frame *image.Paletted, x, y int, idx uint8) {
	if image.Pt(x, y).In(frame.Bounds()) {
		frame.SetColorIndex(x, y, idx)
	}
}
