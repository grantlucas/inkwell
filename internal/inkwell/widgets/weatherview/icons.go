package weatherview

import (
	_ "embed"
	"fmt"
	"image"
	"image/color"

	"github.com/grantlucas/inkwell/internal/inkwell/weather"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

//go:embed weathericons.ttf
var weatherIconsTTF []byte

var conditionGlyphs = map[weather.Condition]rune{
	weather.Clear:        0xf00d, // wi_day_sunny
	weather.PartlyCloudy: 0xf002, // wi_day_cloudy
	weather.Cloudy:       0xf013, // wi_cloudy
	weather.Rain:         0xf019, // wi_rain
	weather.Snow:         0xf01b, // wi_snow
	weather.Thunderstorm: 0xf01e, // wi_thunderstorm
	weather.Fog:          0xf014, // wi_fog
	weather.Drizzle:      0xf01c, // wi_sprinkle
}

// fontData holds the raw TTF bytes. Tests can override this to inject
// invalid font data for error-path coverage.
var fontData = weatherIconsTTF

// DrawIcon renders a weather icon for the given condition at (x, y) with
// the specified pixel size. The icon is drawn centered horizontally and
// vertically within a size×size box at (x, y).
func DrawIcon(frame *image.Paletted, x, y, size int, cond weather.Condition) error {
	glyph, ok := conditionGlyphs[cond]
	if !ok {
		glyph = conditionGlyphs[weather.Clear]
	}

	face, err := iconFace(float64(size))
	if err != nil {
		return err
	}
	defer face.Close()

	advance, ok := face.GlyphAdvance(glyph)
	if !ok {
		return fmt.Errorf("glyph not found for condition %d", cond)
	}

	metrics := face.Metrics()
	glyphW := advance.Ceil()
	glyphH := (metrics.Ascent + metrics.Descent).Ceil()

	drawX := x + (size-glyphW)/2
	drawY := y + (size-glyphH)/2 + metrics.Ascent.Ceil()

	d := &font.Drawer{
		Dst:  frame,
		Src:  image.NewUniform(color.Black),
		Face: face,
		Dot:  fixed.P(drawX, drawY),
	}
	d.DrawBytes([]byte(string(glyph)))

	return nil
}

func iconFace(size float64) (font.Face, error) {
	tt, err := opentype.Parse(fontData)
	if err != nil {
		return nil, fmt.Errorf("parse weather icons font: %w", err)
	}

	face, err := opentype.NewFace(tt, &opentype.FaceOptions{
		Size:    max(size, 1),
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		return nil, fmt.Errorf("create weather icon face: %w", err) //nolint:goerr113 // unreachable with valid embedded font
	}

	return face, nil
}
