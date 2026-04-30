package weekly

import (
	"image"
	"image/color"
	"image/draw"

	"github.com/grantlucas/inkwell/internal/inkwell/fonts"
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

func drawTextWhite(frame *image.Paletted, x, y int, text string) {
	drawTextWhiteWithFace(frame, x, y, text, defaultFace)
}

func drawTextWhiteWithFace(frame *image.Paletted, x, y int, text string, f font.Face) {
	d := &font.Drawer{
		Dst:  frame,
		Src:  image.NewUniform(color.White),
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

func drawTextCenteredWhite(frame *image.Paletted, x1, x2, y int, text string) {
	drawTextCenteredWhiteWithFace(frame, x1, x2, y, text, defaultFace)
}

func drawTextCenteredWhiteWithFace(frame *image.Paletted, x1, x2, y int, text string, f font.Face) {
	tw := textWidth(f, text)
	x := x1 + (x2-x1-tw)/2
	drawTextWhiteWithFace(frame, x, y, text, f)
}

func drawHLine(frame *image.Paletted, x1, x2, y int) {
	for x := x1; x < x2; x++ {
		setPixel(frame, x, y, 1)
	}
}

func drawVLine(frame *image.Paletted, x, y1, y2 int) {
	for y := y1; y < y2; y++ {
		setPixel(frame, x, y, 1)
	}
}

func setPixel(frame *image.Paletted, x, y int, idx uint8) {
	if image.Pt(x, y).In(frame.Bounds()) {
		frame.SetColorIndex(x, y, idx)
	}
}

func fillRect(frame *image.Paletted, r image.Rectangle, idx uint8) {
	c := color.White
	if idx == 1 {
		c = color.Black
	}
	draw.Draw(frame, r, image.NewUniform(c), image.Point{}, draw.Src)
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
