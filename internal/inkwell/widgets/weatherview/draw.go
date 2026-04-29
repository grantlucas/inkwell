package weatherview

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
	f, err := fonts.Face(fonts.Regular, 11)
	if err != nil {
		panic("weatherview: load font: " + err.Error())
	}
	defaultFace = f
}

const (
	charWidth  = 7
	lineHeight = 13
)

func textWidth(f font.Face, text string) int {
	return font.MeasureString(f, text).Ceil()
}

func setPixel(frame *image.Paletted, x, y int, idx uint8) {
	if image.Pt(x, y).In(frame.Bounds()) {
		frame.SetColorIndex(x, y, idx)
	}
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

func drawDashedVLine(frame *image.Paletted, x, y1, y2, dash, gap int) {
	i := 0
	for y := y1; y < y2; y++ {
		if i%(dash+gap) < dash {
			setPixel(frame, x, y, 1)
		}
		i++
	}
}

func fillRect(frame *image.Paletted, r image.Rectangle, idx uint8) {
	c := color.White
	if idx == 1 {
		c = color.Black
	}
	draw.Draw(frame, r, image.NewUniform(c), image.Point{}, draw.Src)
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

func drawTextCentered(frame *image.Paletted, x1, x2, y int, text string) {
	tw := textWidth(defaultFace, text)
	x := x1 + (x2-x1-tw)/2
	drawText(frame, x, y, text)
}

// drawLine draws a line from (x1,y1) to (x2,y2) using Bresenham's algorithm.
func drawLine(frame *image.Paletted, x1, y1, x2, y2 int) {
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
		setPixel(frame, x1, y1, 1)
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

func truncateText(text string, maxChars int) string {
	if len(text) <= maxChars {
		return text
	}
	if maxChars <= 3 {
		return text[:maxChars]
	}
	return text[:maxChars-3] + "..."
}
