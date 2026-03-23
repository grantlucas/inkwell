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

// packBW packs an image into a 1-bit-per-pixel buffer (8 pixels per byte, MSB first).
// Convention: black pixel → bit 1, white pixel → bit 0.
func packBW(profile *DisplayProfile, img image.Image) []byte {
	buf := make([]byte, profile.BufferSize())
	w := profile.Width
	h := profile.Height
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			g := color.GrayModel.Convert(img.At(x, y)).(color.Gray)
			if g.Y < 128 { // dark → black → bit = 1
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
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
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

// UnpackBuffer converts a packed BW display buffer back to a paletted image.
// Bit 1 → black, bit 0 → white. Inverse of packBW.
func UnpackBuffer(profile *DisplayProfile, buf []byte) *image.Paletted {
	palette := color.Palette{color.White, color.Black}
	img := image.NewPaletted(image.Rect(0, 0, profile.Width, profile.Height), palette)
	w := profile.Width
	h := profile.Height
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			idx := (y*w + x) / 8
			bit := uint(7 - (x % 8))
			if buf[idx]>>bit&1 == 1 {
				img.SetColorIndex(x, y, 1) // black
			}
		}
	}
	return img
}
