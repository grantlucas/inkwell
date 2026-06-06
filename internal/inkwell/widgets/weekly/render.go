package weekly

import (
	"image"
	"image/color"
	"image/draw"

	"github.com/grantlucas/inkwell/internal/inkwell/fonts"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

var defaultFace font.Face

func init() {
	f, err := fonts.Face(fonts.Regular, 12)
	if err != nil {
		panic("weekly: load font: " + err.Error())
	}
	defaultFace = f
}

func fillWhite(frame *image.Paletted, r image.Rectangle) {
	draw.Draw(frame, r, image.NewUniform(color.White), image.Point{}, draw.Src)
}

func drawText(frame *image.Paletted, x, y int, text string) {
	drawTextWithFace(frame, x, y, text, defaultFace)
}

func drawTextWithFace(frame *image.Paletted, x, y int, text string, f font.Face) {
	d := &font.Drawer{
		Dst:  frame,
		Src:  image.NewUniform(color.Black),
		Face: f,
		Dot:  fixed.P(x, y),
	}
	d.DrawString(text)
}

// drawTextGray draws text in the supplied gray index. Useful for secondary
// labels (times, day-of-week tags) that should sit visually below the
// primary content.
func drawTextGray(frame *image.Paletted, x, y int, text string, idx uint8) {
	drawTextGrayWithFace(frame, x, y, text, defaultFace, idx)
}

func drawTextGrayWithFace(frame *image.Paletted, x, y int, text string, f font.Face, idx uint8) {
	d := &font.Drawer{
		Dst:  frame,
		Src:  image.NewUniform(widget.PaperPalette[idx]),
		Face: f,
		Dot:  fixed.P(x, y),
	}
	d.DrawString(text)
}

func textWidth(f font.Face, text string) int {
	return font.MeasureString(f, text).Ceil()
}

func drawTextCentered(frame *image.Paletted, x1, x2, y int, text string) {
	drawTextCenteredWithFace(frame, x1, x2, y, text, defaultFace)
}

func drawTextCenteredWithFace(frame *image.Paletted, x1, x2, y int, text string, f font.Face) {
	tw := textWidth(f, text)
	x := x1 + (x2-x1-tw)/2
	drawTextWithFace(frame, x, y, text, f)
}

func drawTextCenteredGrayWithFace(frame *image.Paletted, x1, x2, y int, text string, f font.Face, idx uint8) {
	tw := textWidth(f, text)
	x := x1 + (x2-x1-tw)/2
	drawTextGrayWithFace(frame, x, y, text, f, idx)
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

func fillRect(frame *image.Paletted, r image.Rectangle, idx uint8) {
	draw.Draw(frame, r, image.NewUniform(widget.PaperPalette[idx]), image.Point{}, draw.Src)
}

func truncateText(text string, maxChars int) string {
	if len(text) <= maxChars {
		return text
	}
	if maxChars <= 3 {
		return text[:maxChars]
	}
	return text[:maxChars-3] + "..."
}
