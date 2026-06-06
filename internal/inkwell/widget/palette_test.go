package widget

import (
	"image/color"
	"testing"
)

func TestPaperPalette_StableIndices(t *testing.T) {
	cases := []struct {
		label string
		idx   uint8
		wantY uint8
	}{
		{"PaperWhite", PaperWhite, 0xFF},
		{"PaperBlack", PaperBlack, 0x00},
		{"PaperGray05", PaperGray05, 0xF2},
		{"PaperGray10", PaperGray10, 0xE6},
		{"PaperGray20", PaperGray20, 0xCC},
		{"PaperGray30", PaperGray30, 0xB3},
		{"PaperGray40", PaperGray40, 0x99},
		{"PaperGray50", PaperGray50, 0x80},
		{"PaperGray60", PaperGray60, 0x66},
		{"PaperGray70", PaperGray70, 0x4D},
		{"PaperGray80", PaperGray80, 0x33},
		{"PaperGray90", PaperGray90, 0x1A},
	}

	if int(PaperGray90)+1 != len(PaperPalette) {
		t.Fatalf("palette length %d does not match named indices", len(PaperPalette))
	}

	for _, c := range cases {
		t.Run(c.label, func(t *testing.T) {
			g, ok := PaperPalette[c.idx].(color.Gray)
			if !ok {
				t.Fatalf("palette[%d] is not color.Gray", c.idx)
			}
			if g.Y != c.wantY {
				t.Errorf("palette[%d] Y = 0x%02X, want 0x%02X", c.idx, g.Y, c.wantY)
			}
		})
	}
}

func TestPaperPalette_MonotonicGrayRamp(t *testing.T) {
	// The gray ramp from PaperGray05 upward should be strictly darker as
	// the index increases — this keeps "more ink" semantically meaningful.
	prev := uint8(0xFF)
	for i := int(PaperGray05); i <= int(PaperGray90); i++ {
		g := PaperPalette[i].(color.Gray)
		if g.Y >= prev {
			t.Errorf("palette[%d] Y=0x%02X is not darker than previous 0x%02X", i, g.Y, prev)
		}
		prev = g.Y
	}
}

// On the 1-bit panel, each PaperGrayNN must produce a *distinct* halftone
// stipple after Bayer-4×4 ordered dithering — otherwise the entry is
// palette bloat with no on-device benefit. Simulate the dither across a
// 32×32 tile (large enough to express every threshold in the matrix
// multiple times) and assert that the number of "on" pixels strictly
// increases from PaperGray05 to PaperGray90.
func TestPaperPalette_PostDitherDistinctness(t *testing.T) {
	// Same threshold table used by packBW in internal/inkwell/buffer.go.
	// Kept in sync by convention; if packBW's matrix changes, update here.
	bayer := [4][4]int{
		{0, 8, 2, 10},
		{12, 4, 14, 6},
		{3, 11, 1, 9},
		{15, 7, 13, 5},
	}
	onPixels := func(y uint8) int {
		count := 0
		for py := range 32 {
			for px := range 32 {
				threshold := bayer[py%4][px%4]*16 + 8
				if int(y) < threshold {
					count++
				}
			}
		}
		return count
	}

	prev := -1
	for i := int(PaperGray05); i <= int(PaperGray90); i++ {
		g := PaperPalette[i].(color.Gray)
		n := onPixels(g.Y)
		if n <= prev {
			t.Errorf("palette[%d] (Y=0x%02X) on-pixel count %d is not greater than previous %d — entries collapse to the same stipple on the device", i, g.Y, n, prev)
		}
		prev = n
	}
}
