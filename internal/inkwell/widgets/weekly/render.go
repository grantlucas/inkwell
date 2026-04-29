package weekly

import (
	"image"
	"image/color"
	"image/draw"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

var face = basicfont.Face7x13

func fillWhite(frame *image.Paletted, r image.Rectangle) {
	draw.Draw(frame, r, image.NewUniform(color.White), image.Point{}, draw.Src)
}

func drawText(frame *image.Paletted, x, y int, text string) {
	d := &font.Drawer{
		Dst:  frame,
		Src:  image.NewUniform(color.Black),
		Face: face,
		Dot:  fixed.P(x, y),
	}
	d.DrawString(text)
}

func drawTextWhite(frame *image.Paletted, x, y int, text string) {
	d := &font.Drawer{
		Dst:  frame,
		Src:  image.NewUniform(color.White),
		Face: face,
		Dot:  fixed.P(x, y),
	}
	d.DrawString(text)
}

func drawTextCentered(frame *image.Paletted, x1, x2, y int, text string) {
	textW := len(text) * charWidth
	x := x1 + (x2-x1-textW)/2
	drawText(frame, x, y, text)
}

func drawTextCenteredWhite(frame *image.Paletted, x1, x2, y int, text string) {
	textW := len(text) * charWidth
	x := x1 + (x2-x1-textW)/2
	drawTextWhite(frame, x, y, text)
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
