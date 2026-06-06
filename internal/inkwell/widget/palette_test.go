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
