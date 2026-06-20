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

// packBW packs an image into a 1-bit-per-pixel buffer (8 pixels per byte,
// MSB first). Convention: black pixel → bit 1, white pixel → bit 0.
//
// The Waveshare 7.5" V2 panel is fundamentally 1-bit. We threshold each
// pixel at Y <= 128 → black so the device sees pure black or pure white —
// no halftone stipple. Anti-aliased glyph edges from the font drawer (and
// any other intermediate grays) collapse along the same threshold, which
// keeps text crisp without leaving halftone dots scattered through what
// should be solid backgrounds. Widgets that need a soft accent must draw
// it as a solid PaperBlack stroke (a line, outline, or tick) rather than
// a gray fill; gray fills below the threshold render solid black and
// gray fills above it disappear, so they read as "all-or-nothing" in BW.
//
// The cutoff is "at least half covered" (Y <= 128), not "strictly more
// than half" (Y < 128): a thin anti-aliased stem straddling two columns
// deposits ~50% ink in each, which the compositor quantises to
// PaperGray50 (Y=128). A strict Y < 128 dropped exactly those stem
// centres, so 1px features (the colon in times, the 1/i/l/j stems,
// Terminus Regular body text) disconnected and faded out on the panel
// while reading fine in the grayscale source preview. Inking the 50%
// midpoint keeps them whole (inkwell-5yh). Gray4 is unaffected — it has
// its own bucket mapping in packGray4.
func packBW(profile *DisplayProfile, img image.Image) []byte {
	buf := make([]byte, profile.BufferSize())
	w := profile.Width
	h := profile.Height
	for y := range h {
		for x := range w {
			g := color.GrayModel.Convert(img.At(x, y)).(color.Gray)
			if g.Y <= 128 { // at least half covered → black → bit = 1
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

// joinGray4Planes reverses splitGray4Planes: given the two 1bpp planes
// the controller saw on the wire, reconstruct the original 2bpp buffer.
// Plane A holds the low bit of each pixel and plane B the high bit, so
// the join is a per-bit OR with the right shifts. Used by capture
// backends (WebPreview, ImageBackend) that observe the SPI traffic and
// want to render what the panel would show.
func joinGray4Planes(planeA, planeB []byte) []byte {
	buf := make([]byte, len(planeA)*2)
	for i := range planeA {
		a, b := planeA[i], planeB[i]
		var b0, b1 byte
		for n := range 8 {
			inShift := uint(7 - n)
			pixA := (a >> inShift) & 1
			pixB := (b >> inShift) & 1
			pix := (pixB << 1) | pixA
			// Pixels 0..3 → b0 (bits 7-6, 5-4, 3-2, 1-0); pixels 4..7 → b1.
			outShift := uint(6 - 2*(n&3))
			if n < 4 {
				b0 |= pix << outShift
			} else {
				b1 |= pix << outShift
			}
		}
		buf[i*2] = b0
		buf[i*2+1] = b1
	}
	return buf
}

// UnpackBuffer converts a packed display buffer back into a paletted
// image. The decoding tracks the profile's ColorDepth:
//
//   - BW: bit 1 → black, bit 0 → white (inverse of packBW).
//   - Gray4: each 2-bit code maps to one of four canonical luminances
//     (white=00, light=01, dark=10, black=11), inverse of packGray4.
//
// The returned image's palette indices line up with the pixel codes so
// downstream encoders (PNG, scaling) preserve the four distinct shades.
func UnpackBuffer(profile *DisplayProfile, buf []byte) *image.Paletted {
	if profile.Color == Gray4 {
		return unpackGray4(profile, buf)
	}
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

// gray4Palette maps Inkwell's 2-bit pixel codes to canonical luminances.
// Index = pixel code, so SetColorIndex(x, y, code) does the right thing.
var gray4Palette = color.Palette{
	color.Gray{Y: 0xFF}, // 00 white
	color.Gray{Y: 0xC0}, // 01 light gray
	color.Gray{Y: 0x80}, // 10 dark gray
	color.Gray{Y: 0x00}, // 11 black
}

func unpackGray4(profile *DisplayProfile, buf []byte) *image.Paletted {
	img := image.NewPaletted(image.Rect(0, 0, profile.Width, profile.Height), gray4Palette)
	w := profile.Width
	h := profile.Height
	for y := range h {
		for x := range w {
			p := y*w + x
			shift := uint(6 - (p%4)*2)
			code := (buf[p/4] >> shift) & 0b11
			img.SetColorIndex(x, y, code)
		}
	}
	return img
}

// reconstructFrame turns the two on-wire planes captured by a Hardware
// backend back into a viewable image. The behaviour depends on
// profile.Color:
//
//   - BW: the "old" plane is just ~"new" so we ignore it and unpack the
//     new plane directly.
//   - Gray4: the buffer was split into two 1bpp planes for the wire;
//     join them to recover the 2bpp buffer, then unpack.
//
// Returns an error if the plane sizes don't match the profile or the
// color depth isn't supported by capture backends.
func reconstructFrame(profile *DisplayProfile, planeOld, planeNew []byte) (*image.Paletted, error) {
	switch profile.Color {
	case BW:
		if expected := profile.BufferSize(); len(planeNew) != expected {
			return nil, fmt.Errorf("BW buffer size %d does not match expected %d", len(planeNew), expected)
		}
		return UnpackBuffer(profile, planeNew), nil
	case Gray4:
		expected := profile.BufferSize() / 2
		if len(planeOld) != expected || len(planeNew) != expected {
			return nil, fmt.Errorf("Gray4 plane sizes (%d, %d) do not match expected (%d, %d)",
				len(planeOld), len(planeNew), expected, expected)
		}
		return UnpackBuffer(profile, joinGray4Planes(planeOld, planeNew)), nil
	default:
		return nil, fmt.Errorf("reconstructFrame: unsupported color depth %v", profile.Color)
	}
}
