package widget

import "image/color"

// PaperPalette is the rendering palette used by the compositor for 1-bit
// monochrome displays. It is intentionally richer than the 2-color BW palette
// so that font drawing, anti-aliased edges, and subtle tonal accents render
// into intermediate grays instead of being snapped to pure black or white.
// The packer collapses the gray range to the device's actual color depth at
// the very end (threshold for BW, 4-bucket mapping for Gray4), which means
// the e-ink controller still sees a clean signal while the on-screen preview
// shows the higher-fidelity source.
//
// Index conventions:
//
//   - 0          → pure white (matches Go's zero-initialized Pix array)
//   - 1          → pure black (matches existing widget code that used `1`)
//   - PaperGray* → intermediate gray ramp from light to dark
//
// Indices 0 and 1 are stable; widgets that hardcoded `0` or `1` continue to
// work. New widgets should prefer the named constants for clarity.
var PaperPalette = color.Palette{
	color.Gray{Y: 0xFF}, // 0  White
	color.Gray{Y: 0x00}, // 1  Black
	color.Gray{Y: 0xF2}, // 2  Gray05  (very faint)
	color.Gray{Y: 0xE6}, // 3  Gray10
	color.Gray{Y: 0xCC}, // 4  Gray20
	color.Gray{Y: 0xB3}, // 5  Gray30
	color.Gray{Y: 0x99}, // 6  Gray40
	color.Gray{Y: 0x80}, // 7  Gray50  (mid)
	color.Gray{Y: 0x66}, // 8  Gray60
	color.Gray{Y: 0x4D}, // 9  Gray70
	color.Gray{Y: 0x33}, // 10 Gray80
	color.Gray{Y: 0x1A}, // 11 Gray90
}

// Stable palette indices. Use these in widgets in preference to magic numbers.
const (
	PaperWhite  uint8 = 0
	PaperBlack  uint8 = 1
	PaperGray05 uint8 = 2  // 95% lightness — backdrop tints
	PaperGray10 uint8 = 3  // 90% lightness — subtle fills
	PaperGray20 uint8 = 4  // 80% lightness — soft separators, today-highlight fill
	PaperGray30 uint8 = 5  // 70% lightness — chart axes, dim grid
	PaperGray40 uint8 = 6  // 60% lightness
	PaperGray50 uint8 = 7  // 50% lightness — medium accents
	PaperGray60 uint8 = 8  // 40% lightness — column dividers
	PaperGray70 uint8 = 9  // 30% lightness — accent text
	PaperGray80 uint8 = 10 // 20% lightness — strong accents
	PaperGray90 uint8 = 11 // 10% lightness — near-black accents
)
