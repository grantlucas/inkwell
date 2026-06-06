package inkwell

import (
	"fmt"
	"image"
	"image/color"
)

// PackImage converts a Go image into a packed display buffer for the given profile.
// Returns an error if the profile's ColorDepth is not supported.
func PackImage(profile *DisplayProfile, img image.Image) ([]byte, error) {
	switch profile.Color {
	case BW:
		return packBW(profile, img), nil
	case Gray4:
		return packGray4(profile, img), nil
	default:
		return nil, fmt.Errorf("unsupported color depth: %v", profile.Color)
	}
}

// bayer4x4 is the standard 4×4 ordered-dither threshold matrix. Values are
// in [0, 15] and get scaled into the [0, 255] luminance range when used as
// per-pixel thresholds. The matrix produces visually-stable halftone patterns
// (no error-propagation snake artifacts) which suits a dashboard's flat
// gray fills better than a Floyd-Steinberg style approach.
var bayer4x4 = [4][4]int{
	{0, 8, 2, 10},
	{12, 4, 14, 6},
	{3, 11, 1, 9},
	{15, 7, 13, 5},
}

// packBW packs an image into a 1-bit-per-pixel buffer (8 pixels per byte, MSB
// first). Convention: black pixel → bit 1, white pixel → bit 0.
//
// The Waveshare 7.5" V2 panel is fundamentally 1-bit, so any intermediate
// grays the compositor produced (anti-aliased glyph edges, today highlight
// tint, hour-band, soft separators, precip-bar fills) would otherwise snap
// to pure black or white at the device — wiping out the visual hierarchy.
// To preserve gradations on a 1-bit panel we apply Bayer-4×4 ordered
// dithering: each output pixel uses a position-dependent threshold from
// the matrix, so a gray fill turns into a structured stipple pattern the
// eye reads as a continuous tone. Pure black (Y=0) and pure white (Y=255)
// are unaffected because they fall outside the threshold range, keeping
// solid type and pure-white backgrounds crisp.
func packBW(profile *DisplayProfile, img image.Image) []byte {
	buf := make([]byte, profile.BufferSize())
	w := profile.Width
	h := profile.Height
	for y := range h {
		for x := range w {
			g := color.GrayModel.Convert(img.At(x, y)).(color.Gray)
			// Matrix value 0..15 → threshold 8, 24, 40, … 248.
			threshold := bayer4x4[y%4][x%4]*16 + 8
			if int(g.Y) < threshold { // dark → black → bit = 1
				idx := (y*w + x) / 8
				bit := uint(7 - (x % 8)) // MSB first
				buf[idx] |= 1 << bit
			}
		}
	}
	return buf
}

// packGray4 packs an image into a 2-bit-per-pixel buffer (4 pixels per byte, MSB first).
// Luminance mapping: white=00, light-gray=01, dark-gray=10, black=11.
func packGray4(profile *DisplayProfile, img image.Image) []byte {
	buf := make([]byte, profile.BufferSize())
	w := profile.Width
	h := profile.Height
	for y := range h {
		for x := range w {
			g := color.GrayModel.Convert(img.At(x, y)).(color.Gray)
			var level byte
			switch {
			case g.Y > 192:
				level = 0b00 // white  (Y=193..255)
			case g.Y > 128:
				level = 0b01 // light gray (Y=129..192)
			case g.Y > 64:
				level = 0b10 // dark gray  (Y=65..128)
			default:
				level = 0b11 // black      (Y=0..64)
			}
			p := y*w + x
			buf[p/4] |= level << uint(6-(p%4)*2)
		}
	}
	return buf
}

// splitGray4Planes converts Inkwell's 2bpp Gray4 buffer into the two 1bpp
// planes the Waveshare 7.5" V2 controller expects (one written to
// OldBufferCmd, one to NewBufferCmd). Inkwell's pixel encoding —
// white=00, light=01, dark=10, black=11 — makes the split fall out as
// two bit projections: plane A carries the low bit of each pixel,
// plane B the high bit. Together they encode the 4 shades the panel
// physically supports in its Init4Gray mode.
//
// The mapping was derived from the upstream Python reference
// (epd7in5_V2.py EPD_4Gray_Display) by inverting Waveshare's
// truth table for our opposite-polarity encoding. Verified per-shade
// in TestSplitGray4Planes.
func splitGray4Planes(buf []byte) (planeA, planeB []byte) {
	planeA = make([]byte, len(buf)/2)
	planeB = make([]byte, len(buf)/2)
	for i := range planeA {
		// One output byte covers 8 pixels = 2 input bytes (4 pixels each).
		// First input byte holds the left 4 pixels, MSB-first.
		var a, b byte
		for n := range 8 {
			src := buf[i*2+(n>>2)]    // bytes 0..1 within the pair
			shift := uint(6 - 2*(n&3)) // 6, 4, 2, 0 — pixel position
			pix := (src >> shift) & 0b11
			outShift := uint(7 - n) // MSB-first packing in the plane byte
			a |= (pix & 0b01) << outShift
			b |= ((pix >> 1) & 0b01) << outShift
		}
		planeA[i] = a
		planeB[i] = b
	}
	return planeA, planeB
}

// UnpackBuffer converts a packed BW display buffer back to a paletted image.
// Bit 1 → black, bit 0 → white. Inverse of packBW.
func UnpackBuffer(profile *DisplayProfile, buf []byte) *image.Paletted {
	palette := color.Palette{color.White, color.Black}
	img := image.NewPaletted(image.Rect(0, 0, profile.Width, profile.Height), palette)
	w := profile.Width
	h := profile.Height
	for y := range h {
		for x := range w {
			idx := (y*w + x) / 8
			bit := uint(7 - (x % 8))
			if buf[idx]>>bit&1 == 1 {
				img.SetColorIndex(x, y, 1) // black
			}
		}
	}
	return img
}
