// Package fonts loads the bitmap typeface Inkwell renders text with.
//
// Inkwell drives a 1-bit (and 4-level) e-paper panel. Outline fonts rendered
// through x/image/opentype are always anti-aliased, and the AA fringe of a
// thin stem straddles the BW threshold and drops out — small text fragments
// on the device (see inkwell-5yh). To avoid that at the source we render a
// true bitmap font (Tamzen) whose glyphs are pure 1-bit masks: every pixel is
// black or white, so there is no fringe for the threshold to lose. Bitmap
// fonts only exist at fixed pixel sizes, so Face snaps the requested point
// size to the nearest embedded Tamzen size.
package fonts

import (
	_ "embed"
	"fmt"

	"golang.org/x/image/font"
)

// Tamzen ships a Regular and Bold weight at each pixel size. We embed three
// sizes that cover Inkwell's hierarchy: 12px (small labels), 16px (body), and
// 20px (headings). Tamzen is distributed under a permissive "use, copy,
// modify, and distribute as you see fit" license (see Tamzen-LICENSE.txt).
//
//go:embed Tamzen6x12r.bdf
var tamzen6x12r []byte

//go:embed Tamzen6x12b.bdf
var tamzen6x12b []byte

//go:embed Tamzen8x16r.bdf
var tamzen8x16r []byte

//go:embed Tamzen8x16b.bdf
var tamzen8x16b []byte

//go:embed Tamzen10x20r.bdf
var tamzen10x20r []byte

//go:embed Tamzen10x20b.bdf
var tamzen10x20b []byte

// Weight selects the typeface weight. Tamzen has no SemiBold, so SemiBold maps
// to Bold — callers use it to mean "heavier than body" and get the Bold cut.
type Weight int

const (
	Regular  Weight = iota
	SemiBold        // maps to Bold (Tamzen has no SemiBold)
	Bold
)

// sizeTier is one embedded Tamzen size, in ascending pixel height.
type sizeTier struct {
	px            int
	regular, bold []byte
}

var sizeTiers = []sizeTier{
	{px: 12, regular: tamzen6x12r, bold: tamzen6x12b},
	{px: 16, regular: tamzen8x16r, bold: tamzen8x16b},
	{px: 20, regular: tamzen10x20r, bold: tamzen10x20b},
}

// Test overrides for the embedded BDF data. When set (non-nil) they replace
// the font bytes at every size so a test can force a parse failure across all
// of a widget's faces regardless of which size it asks for.
var (
	overrideRegular []byte
	overrideBold    []byte
)

// SwapDataForTest replaces the BDF bytes the parser reads for the regular and
// bold weights. Returns a restore function that puts the originals back.
// Test-only: callers must always defer the restore.
func SwapDataForTest(regular, bold []byte) (restore func()) {
	origR, origB := overrideRegular, overrideBold
	overrideRegular, overrideBold = regular, bold
	return func() { overrideRegular, overrideBold = origR, origB }
}

// Face returns a bitmap font.Face for the given weight and point size. The
// point size is converted to pixels at 96 DPI and snapped to the nearest
// embedded Tamzen size, so the returned glyphs are pixel-perfect with no
// anti-aliasing. The current tiers (12/16/20px) cover ~9pt-15pt+ cleanly:
// 10pt→12px, 12pt→16px, 16pt→20px.
func Face(weight Weight, sizePt float64) (font.Face, error) {
	px := int(sizePt*96.0/72.0 + 0.5)
	t := nearestTier(px)
	f, err := ParseBDF(dataFor(t, weight >= SemiBold))
	if err != nil {
		return nil, fmt.Errorf("parse font: %w", err)
	}
	return f.NewFace(), nil
}

// dataFor resolves the BDF bytes for a tier and weight, honoring test
// overrides when present.
func dataFor(t sizeTier, bold bool) []byte {
	if bold {
		if overrideBold != nil {
			return overrideBold
		}
		return t.bold
	}
	if overrideRegular != nil {
		return overrideRegular
	}
	return t.regular
}

// nearestTier returns the embedded size whose pixel height is closest to px.
func nearestTier(px int) sizeTier {
	best := sizeTiers[0]
	bestD := absInt(px - best.px)
	for _, t := range sizeTiers[1:] {
		if d := absInt(px - t.px); d < bestD {
			best, bestD = t, d
		}
	}
	return best
}

func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
