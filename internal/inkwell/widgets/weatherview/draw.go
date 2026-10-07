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

// defaultFace is a package var rather than set in init so that vars
// derived from it, like precipLabelH, are initialised after it.
var defaultFace = mustLoadDefaultFace()

// mustLoadDefaultFace is extracted so the font-load panic branch is
// reachable from tests via fonts.SwapDataForTest.
func mustLoadDefaultFace() font.Face {
	f, err := fonts.Face(fonts.Regular, 10)
	if err != nil {
		panic("weatherview: load font: " + err.Error())
	}
	return f
}

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

func drawTextWithFace(frame *image.Paletted, x, y int, text string, f font.Face) {
	d := &font.Drawer{
		Dst:  frame,
		Src:  image.NewUniform(color.Black),
		Face: f,
		Dot:  fixed.P(x, y),
	}
	d.DrawString(text)
}

// walkLine visits every pixel of the Bresenham line from (x1,y1) to
// (x2,y2), both ends included, so a caller can decide each pixel's colour
// before any of them is written.
func walkLine(x1, y1, x2, y2 int, visit func(x, y int)) {
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
		visit(x1, y1)
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

// drawTextCenteredWithFace centers text between x1 and x2 using the given
// face. drawTextCentered measures with the package default face, which
// misplaces text whenever the caller supplies its own.
func drawTextCenteredWithFace(frame *image.Paletted, x1, x2, y int, text string, f font.Face) {
	tw := textWidth(f, text)
	x := x1 + (x2-x1-tw)/2
	drawTextWithFace(frame, x, y, text, f)
}
