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

// On the 1-bit panel, the BW packer threshold-snaps each pixel at Y<128.
// That means PaperGray05..PaperGray50 (Y=0x80..0xF2) all collapse to pure
// white and PaperGray60..PaperGray90 (Y=0x1A..0x66) all collapse to pure
// black — they only stay visually distinct on the Gray4 path and in the
// source-design preview. Pin the bucket boundary so a palette shuffle
// that quietly drops a "subtle" shade across the Y=128 line shows up in
// CI rather than only on hardware. Anything a widget needs to be visible
// on the BW threshold path has to land at PaperGray60 or darker.
func TestPaperPalette_BWBucket(t *testing.T) {
	cases := []struct {
		idx       uint8
		wantBlack bool
	}{
		{PaperWhite, false},
		{PaperGray05, false},
		{PaperGray10, false},
		{PaperGray20, false},
		{PaperGray30, false},
		{PaperGray40, false},
		{PaperGray50, false}, // Y=0x80 is exactly the threshold; treated as white
		{PaperGray60, true},
		{PaperGray70, true},
		{PaperGray80, true},
		{PaperGray90, true},
		{PaperBlack, true},
	}
	for _, c := range cases {
		g := PaperPalette[c.idx].(color.Gray)
		gotBlack := g.Y < 128
		if gotBlack != c.wantBlack {
			t.Errorf("palette[%d] Y=0x%02X gotBlack=%v wantBlack=%v",
				c.idx, g.Y, gotBlack, c.wantBlack)
		}
	}
}
