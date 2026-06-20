package fonts

import (
	"strings"
	"testing"

	"golang.org/x/image/math/fixed"
)

func TestFace_LoadsAllWeightsAndSizes(t *testing.T) {
	cases := []struct {
		label  string
		weight Weight
		sizePt float64
	}{
		{"regular small", Regular, 10},
		{"semibold small", SemiBold, 10},
		{"regular body", Regular, 12},
		{"semibold body", SemiBold, 12},
		{"bold heading", Bold, 16},
		{"odd size 9pt", Regular, 9},
		{"odd size 14pt", SemiBold, 14},
		{"odd size 18pt", Bold, 18},
	}
	for _, c := range cases {
		t.Run(c.label, func(t *testing.T) {
			f, err := Face(c.weight, c.sizePt)
			if err != nil {
				t.Fatalf("Face(%v, %v): %v", c.weight, c.sizePt, err)
			}
			if f == nil {
				t.Fatal("Face returned nil")
			}
		})
	}
}

// Point sizes snap to the nearest embedded Tamzen pixel size (height == px at
// 96 DPI), which pins both the mapping and the nearestTier tie-break.
func TestFace_SnapsPointSizeToNearestPixelTier(t *testing.T) {
	cases := []struct {
		label    string
		sizePt   float64
		wantPxHt int
	}{
		{"10pt → 12px", 10, 12}, // 13px → nearest 12
		{"12pt → 16px", 12, 16}, // 16px → exact
		{"16pt → 20px", 16, 20}, // 21px → nearest 20
		{"9pt → 12px", 9, 12},   // 12px → exact, smallest tier
		{"tie 13.5pt rounds to 18px→20", 15, 20},
	}
	for _, c := range cases {
		t.Run(c.label, func(t *testing.T) {
			f, err := Face(Regular, c.sizePt)
			if err != nil {
				t.Fatalf("Face: %v", err)
			}
			if got := f.Metrics().Height; got != fixed.I(c.wantPxHt) {
				t.Errorf("height = %v, want %v (px tier %d)", got, fixed.I(c.wantPxHt), c.wantPxHt)
			}
		})
	}
}

func TestNearestTier_TieAndBounds(t *testing.T) {
	cases := []struct {
		px     int
		wantPx int
	}{
		{0, 12},   // far below smallest → smallest
		{12, 12},  // exact
		{14, 12},  // |14-12|=2 vs |14-16|=2 tie → first (12)
		{15, 16},  // closer to 16
		{18, 16},  // |18-16|=2 vs |18-20|=2 tie → 16 (earlier)
		{19, 20},  // closer to 20
		{100, 20}, // far above largest → largest
	}
	for _, c := range cases {
		if got := nearestTier(c.px).px; got != c.wantPx {
			t.Errorf("nearestTier(%d).px = %d, want %d", c.px, got, c.wantPx)
		}
	}
}

// A malformed BDF must surface as a parse error from Face, for both the
// regular and bold paths (each widget relies on this to fail loudly).
func TestFace_ParseError(t *testing.T) {
	restore := SwapDataForTest([]byte("not a font"), []byte("not a font"))
	defer restore()

	for _, w := range []Weight{Regular, Bold} {
		if _, err := Face(w, 12); err == nil || !strings.Contains(err.Error(), "parse font") {
			t.Errorf("Face(%v): err = %v, want a parse-font error", w, err)
		}
	}
}

// Overriding only one weight leaves the other reading its embedded data, so a
// bad bold override fails bold while regular still loads.
func TestFace_ParseError_BoldOnly(t *testing.T) {
	restore := SwapDataForTest(nil, []byte("not a font"))
	defer restore()

	if _, err := Face(Bold, 16); err == nil {
		t.Error("expected bold parse error")
	}
	if _, err := Face(Regular, 16); err != nil {
		t.Errorf("regular should still load, got %v", err)
	}
}

func TestFace_HasDegreeSymbol(t *testing.T) {
	f, err := Face(Regular, 10)
	if err != nil {
		t.Fatalf("Face: %v", err)
	}
	for _, r := range "°" {
		if _, _, ok := f.GlyphBounds(r); !ok {
			t.Error("font does not contain degree symbol (°)")
		}
	}
}
