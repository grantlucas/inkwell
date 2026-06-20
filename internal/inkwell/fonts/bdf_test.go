package fonts

import (
	"bytes"
	"image"
	"strings"
	"testing"

	"golang.org/x/image/math/fixed"
)

// miniBDF is a hand-written 8px font with two glyphs (space and 'A') used to
// exercise the parser deterministically. 'A' is a 3x3 bitmap:
//
//	_X_   row 0x40
//	X_X   row 0xA0
//	XXX   row 0xE0
const miniBDF = `STARTFONT 2.1
FONT -test-Fixed-Medium-R-Normal--8-80-75-75-C-40-ISO8859-1
SIZE 8 75 75
FONTBOUNDINGBOX 4 8 0 -2
STARTPROPERTIES 2
FONT_ASCENT 6
FONT_DESCENT 2
ENDPROPERTIES
CHARS 2
STARTCHAR space
ENCODING 32
SWIDTH 500 0
DWIDTH 4 0
BBX 1 1 0 0
BITMAP
00
ENDCHAR
STARTCHAR A
ENCODING 65
SWIDTH 500 0
DWIDTH 4 0
BBX 3 3 0 0
BITMAP
40
A0
E0
ENDCHAR
ENDFONT
`

func TestParseBDF_MetricsAndAdvance(t *testing.T) {
	f, err := ParseBDF([]byte(miniBDF))
	if err != nil {
		t.Fatalf("ParseBDF: %v", err)
	}
	face := f.NewFace()
	m := face.Metrics()
	if got := m.Ascent; got != fixed.I(6) {
		t.Errorf("Ascent = %v, want %v", got, fixed.I(6))
	}
	if got := m.Descent; got != fixed.I(2) {
		t.Errorf("Descent = %v, want %v", got, fixed.I(2))
	}
	adv, ok := face.GlyphAdvance('A')
	if !ok || adv != fixed.I(4) {
		t.Errorf("GlyphAdvance('A') = %v,%v want %v,true", adv, ok, fixed.I(4))
	}
}

// bdfNoProps is a BDF without FONT_ASCENT/FONT_DESCENT, so ParseBDF must
// derive metrics from FONTBOUNDINGBOX (4 8 0 -2 → ascent 6, descent 2).
const bdfNoProps = `STARTFONT 2.1
FONTBOUNDINGBOX 4 8 0 -2
CHARS 1
STARTCHAR A
ENCODING 65
DWIDTH 4 0
BBX 3 3 0 0
BITMAP
40
A0
E0
ENDCHAR
ENDFONT
`

func TestParseBDF_FallbackMetricsFromBoundingBox(t *testing.T) {
	f, err := ParseBDF([]byte(bdfNoProps))
	if err != nil {
		t.Fatalf("ParseBDF: %v", err)
	}
	m := f.NewFace().Metrics()
	if m.Ascent != fixed.I(6) || m.Descent != fixed.I(2) {
		t.Errorf("metrics = asc %v desc %v, want 6/2", m.Ascent, m.Descent)
	}
}

// An ENCODING of -1 with an alternate codepoint uses the alternate; -1 alone
// is skipped. Here glyph 'B' is encoded as "-1 66" and must be reachable.
func TestParseBDF_EncodingAltAndUnencoded(t *testing.T) {
	const src = `STARTFONT 2.1
FONTBOUNDINGBOX 4 8 0 0
FONT_ASCENT 8
FONT_DESCENT 0
CHARS 2
STARTCHAR unencoded
ENCODING -1
DWIDTH 4 0
BBX 1 1 0 0
BITMAP
80
ENDCHAR
STARTCHAR B
ENCODING -1 66
DWIDTH 4 0
BBX 1 1 0 0
BITMAP
80
ENDCHAR
ENDFONT
`
	f, err := ParseBDF([]byte(src))
	if err != nil {
		t.Fatalf("ParseBDF: %v", err)
	}
	face := f.NewFace()
	if _, ok := face.GlyphAdvance('B'); !ok {
		t.Error("glyph 'B' (ENCODING -1 66) should be reachable")
	}
	if len(f.glyphs) != 1 {
		t.Errorf("unencoded glyph should be skipped; got %d glyphs", len(f.glyphs))
	}
}

// A short bitmap row (fewer hex digits than the BBX width needs) leaves the
// uncovered columns blank rather than erroring.
func TestParseBDF_ShortBitmapRow(t *testing.T) {
	const src = `STARTFONT 2.1
FONTBOUNDINGBOX 8 8 0 0
FONT_ASCENT 8
FONT_DESCENT 0
CHARS 1
STARTCHAR wide
ENCODING 87
DWIDTH 8 0
BBX 8 1 0 0
BITMAP
8
ENDCHAR
ENDFONT
`
	f, err := ParseBDF([]byte(src))
	if err != nil {
		t.Fatalf("ParseBDF: %v", err)
	}
	_, mask, mp, _, ok := f.NewFace().Glyph(fixed.P(0, 8), 'W')
	if !ok {
		t.Fatal("glyph not ok")
	}
	// Column 0 set (hex "8" = 1000), columns 4-7 have no hex → blank.
	if _, _, _, a := mask.At(mp.X+0, mp.Y).RGBA(); a == 0 {
		t.Error("column 0 should be set")
	}
	if _, _, _, a := mask.At(mp.X+5, mp.Y).RGBA(); a != 0 {
		t.Error("column 5 should be blank (no hex provided)")
	}
}

func TestParseBDF_Errors(t *testing.T) {
	const head = "STARTFONT 2.1\nFONTBOUNDINGBOX 4 8 0 0\nFONT_ASCENT 8\nFONT_DESCENT 0\nCHARS 1\n"
	cases := []struct {
		label string
		src   string
		want  string
	}{
		{"no glyphs", "STARTFONT 2.1\nFONTBOUNDINGBOX 4 8 0 0\nENDFONT\n", "no glyphs"},
		{"bad bitmap hex", head + "STARTCHAR x\nENCODING 120\nDWIDTH 4 0\nBBX 3 1 0 0\nBITMAP\nZZ\nENDCHAR\nENDFONT\n", "bad bitmap hex"},
		{"truncated bitmap", head + "STARTCHAR x\nENCODING 120\nDWIDTH 4 0\nBBX 3 3 0 0\nBITMAP\n40\n", "bitmap truncated"},
		{"missing endchar", head + "STARTCHAR x\nENCODING 120\nDWIDTH 4 0\nBBX 3 1 0 0\n", "missing ENDCHAR"},
	}
	for _, c := range cases {
		t.Run(c.label, func(t *testing.T) {
			_, err := ParseBDF([]byte(c.src))
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("err = %v, want it to contain %q", err, c.want)
			}
		})
	}
}

func TestParseBDF_ScannerError(t *testing.T) {
	// A single token longer than the scanner's max buffer (1 MiB) makes
	// Scan fail with bufio.ErrTooLong, which ParseBDF surfaces.
	huge := bytes.Repeat([]byte("A"), (1<<20)+16)
	if _, err := ParseBDF(huge); err == nil || !strings.Contains(err.Error(), "read bdf") {
		t.Errorf("err = %v, want a read-bdf error", err)
	}
}

func TestBDFFace_MissingGlyphAndTrivialMethods(t *testing.T) {
	f, err := ParseBDF([]byte(miniBDF))
	if err != nil {
		t.Fatalf("ParseBDF: %v", err)
	}
	face := f.NewFace()
	if _, ok := face.GlyphAdvance('Z'); ok {
		t.Error("GlyphAdvance('Z') should be false for a missing glyph")
	}
	if _, _, ok := face.GlyphBounds('Z'); ok {
		t.Error("GlyphBounds('Z') should be false for a missing glyph")
	}
	if _, _, _, _, ok := face.Glyph(fixed.P(0, 0), 'Z'); ok {
		t.Error("Glyph('Z') should be false for a missing glyph")
	}
	if k := face.Kern('A', 'A'); k != 0 {
		t.Errorf("Kern = %v, want 0", k)
	}
	if err := face.Close(); err != nil {
		t.Errorf("Close = %v, want nil", err)
	}
}

func TestParseBDF_GlyphBitmap(t *testing.T) {
	f, err := ParseBDF([]byte(miniBDF))
	if err != nil {
		t.Fatalf("ParseBDF: %v", err)
	}
	face := f.NewFace()
	// Draw 'A' with the baseline at (0, 8); BBX offset is (0,0) so the 3x3
	// bitmap sits with its bottom on the baseline → top at y=5.
	dr, mask, maskp, _, ok := face.Glyph(fixed.P(0, 8), 'A')
	if !ok {
		t.Fatal("Glyph('A') not ok")
	}
	want := image.Rect(0, 5, 3, 8)
	if dr != want {
		t.Errorf("dest rect = %v, want %v", dr, want)
	}
	// Expected on-pixels (col,row within the bitmap): _X_ / X_X / XXX
	on := map[image.Point]bool{
		{1, 0}: true,
		{0, 1}: true, {2, 1}: true,
		{0, 2}: true, {1, 2}: true, {2, 2}: true,
	}
	for y := range 3 {
		for x := range 3 {
			_, _, _, a := mask.At(maskp.X+x, maskp.Y+y).RGBA()
			gotOn := a > 0
			if gotOn != on[image.Point{x, y}] {
				t.Errorf("pixel (%d,%d) on=%v, want %v", x, y, gotOn, on[image.Point{x, y}])
			}
		}
	}
}
