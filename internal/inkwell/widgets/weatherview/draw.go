package weatherview

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
	defaultFace = mustLoadDefaultFace()
}

// mustLoadDefaultFace is extracted so the font-load panic branch is
// reachable from tests via fonts.SwapDataForTest.
func mustLoadDefaultFace() font.Face {
	f, err := fonts.Face(fonts.Regular, 10)
	if err != nil {
		panic("weatherview: load font: " + err.Error())
	}
	return f
}

const (
	charWidth  = 7
	lineHeight = 15
)

func textWidth(f font.Face, text string) int {
	return font.MeasureString(f, text).Ceil()
}

func setPixel(frame *image.Paletted, x, y int, idx uint8) {
	if image.Pt(x, y).In(frame.Bounds()) {
		frame.SetColorIndex(x, y, idx)
	}
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

func fillRect(frame *image.Paletted, r image.Rectangle, idx uint8) {
	draw.Draw(frame, r, image.NewUniform(widget.PaperPalette[idx]), image.Point{}, draw.Src)
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

// drawTextGrayWithFace draws text using a given gray palette index. The
// font.Drawer's alpha mask is blended against the supplied solid color, so
// coverage values between 0 and 255 land on the nearest available palette
// entry — which gives free anti-aliased grays around glyph edges.
func drawTextGrayWithFace(frame *image.Paletted, x, y int, text string, f font.Face, idx uint8) {
	d := &font.Drawer{
		Dst:  frame,
		Src:  image.NewUniform(widget.PaperPalette[idx]),
		Face: f,
		Dot:  fixed.P(x, y),
	}
	d.DrawString(text)
}

func drawTextCentered(frame *image.Paletted, x1, x2, y int, text string) {
	tw := textWidth(defaultFace, text)
	x := x1 + (x2-x1-tw)/2
	drawText(frame, x, y, text)
}

func drawTextCenteredGray(frame *image.Paletted, x1, x2, y int, text string, idx uint8) {
	tw := textWidth(defaultFace, text)
	x := x1 + (x2-x1-tw)/2
	drawTextGrayWithFace(frame, x, y, text, defaultFace, idx)
}

// drawLine draws a line from (x1,y1) to (x2,y2) using Bresenham's algorithm.
func drawLine(frame *image.Paletted, x1, y1, x2, y2 int, idx uint8) {
	dx := abs(x2 - x1)
	dy := abs(y2 - y1)
	sx := 1
	if x1 > x2 {
		sx = -1
	}
	sy := 1
	if y1 > y2 {
		sy = -1
	}
	err := dx - dy

	for {
		setPixel(frame, x1, y1, idx)
		if x1 == x2 && y1 == y2 {
			break
		}
		e2 := 2 * err
		if e2 > -dy {
			err -= dy
			x1 += sx
		}
		if e2 < dx {
			err += dx
			y1 += sy
		}
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// truncateText shortens text to at most maxChars runes, appending an
// ellipsis when truncation occurs. Operating on runes (rather than
// bytes) avoids slicing through the middle of a multi-byte UTF-8
// sequence and producing invalid output.
func truncateText(text string, maxChars int) string {
	runes := []rune(text)
	if len(runes) <= maxChars {
		return text
	}
	if maxChars <= 3 {
		return string(runes[:maxChars])
	}
	return string(runes[:maxChars-3]) + "..."
}
