package weatherview

import (
	_ "embed"
	"fmt"
	"image"
	"image/color"
	"unicode/utf8"

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

// newOpenTypeFace is the indirection over opentype.NewFace that tests
// override to exercise the otherwise-unreachable "create face" error
// branch. Production calls go straight through.
var newOpenTypeFace = opentype.NewFace

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
	// Close on an in-memory opentype face is documented to always
	// return nil; explicitly discard so errcheck doesn't flag the
	// deferred call, and skip the dead error-handling branch.
	defer func() { _ = face.Close() }()

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
	// utf8.EncodeRune avoids the [] byte (string(glyph)) double-conversion
	// allocation. Worst case a 4-byte buffer is plenty for any valid
	// rune.
	var buf [utf8.UTFMax]byte
	n := utf8.EncodeRune(buf[:], glyph)
	d.DrawBytes(buf[:n])

	return nil
}

func iconFace(size float64) (font.Face, error) {
	tt, err := opentype.Parse(fontData)
	if err != nil {
		return nil, fmt.Errorf("parse weather icons font: %w", err)
	}

	// HintingNone for icon glyphs: weather symbols are circular/curved shapes
	// rather than pixel-aligned stems, so full hinting was producing chunky
	// staircased edges. Without hinting the glyph anti-aliases against the
	// grayscale palette and the icon edges read as smooth curves.
	face, err := newOpenTypeFace(tt, &opentype.FaceOptions{
		Size:    max(size, 1),
		DPI:     72,
		Hinting: font.HintingNone,
	})
	if err != nil {
		return nil, fmt.Errorf("create weather icon face: %w", err) //nolint:goerr113 // only reachable via test override
	}

	return face, nil
}
