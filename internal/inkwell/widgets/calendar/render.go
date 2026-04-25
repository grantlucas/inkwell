package calendar

import (
	"image"
	"image/color"
	"image/draw"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

// face is the font used for all calendar rendering.
var face = basicfont.Face7x13

// charWidth and lineHeight are the dimensions of a single character.
var (
	charWidth  = 7
	lineHeight = 13
)

// drawText renders text at (x, y) where y is the baseline.
func drawText(frame *image.Paletted, x, y int, text string) {
	d := &font.Drawer{
		Dst:  frame,
		Src:  image.NewUniform(color.Black),
		Face: face,
		Dot:  fixed.P(x, y),
	}
	d.DrawString(text)
}

// drawTextCentered renders text horizontally centered within [x1, x2] at
// baseline y.
func drawTextCentered(frame *image.Paletted, x1, x2, y int, text string) {
	textW := len(text) * charWidth
	x := x1 + (x2-x1-textW)/2
	drawText(frame, x, y, text)
}

// drawTextRight renders text right-aligned at x (right edge) at baseline y.
func drawTextRight(frame *image.Paletted, x, y int, text string) {
	textW := len(text) * charWidth
	drawText(frame, x-textW, y, text)
}

// drawHLine draws a horizontal line from x1 to x2 at y.
func drawHLine(frame *image.Paletted, x1, x2, y int) {
	for x := x1; x < x2; x++ {
		frame.SetColorIndex(x, y, 1) // black
	}
}

// drawInvertedRect fills a rectangle with black.
func drawInvertedRect(frame *image.Paletted, r image.Rectangle) {
	draw.Draw(frame, r, image.NewUniform(color.Black), image.Point{}, draw.Src)
}

// drawTextInverted renders text in white on a black background at (x, y).
func drawTextInverted(frame *image.Paletted, x, y int, text string) {
	d := &font.Drawer{
		Dst:  frame,
		Src:  image.NewUniform(color.White),
		Face: face,
		Dot:  fixed.P(x, y),
	}
	d.DrawString(text)
}

// truncateText truncates text to maxChars, adding "..." if truncated.
func truncateText(text string, maxChars int) string {
	if len(text) <= maxChars {
		return text
	}
	if maxChars <= 3 {
		return text[:maxChars]
	}
	return text[:maxChars-3] + "..."
}

// fillWhite fills the bounds with white.
func fillWhite(frame *image.Paletted, bounds image.Rectangle) {
	draw.Draw(frame, bounds, image.NewUniform(color.White), image.Point{}, draw.Src)
}
