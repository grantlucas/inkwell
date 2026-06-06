package inkwell

import (
	"image"
	"image/color"
	"testing"
)

// --- Helpers ---

// solidImage returns a uniform image of the given color at the given size.
func solidImage(w, h int, c color.Color) image.Image {
	img := image.NewGray(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, c)
		}
	}
	return img
}

// bwTestProfile returns a 16x16 BW profile for buffer tests.
func bwTestProfile() *DisplayProfile {
	return &DisplayProfile{Name: "test", Width: 16, Height: 16, Color: BW}
}

// gray4TestProfile returns a 16x16 Gray4 profile for buffer tests.
func gray4TestProfile() *DisplayProfile {
	return &DisplayProfile{Name: "test", Width: 16, Height: 16, Color: Gray4}
}

// --- packBW tests ---

func TestPackBW_AllWhite(t *testing.T) {
	p := bwTestProfile()
	img := solidImage(16, 16, color.White)
	buf, err := PackImage(p, img)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(buf) != p.BufferSize() {
		t.Fatalf("buf len = %d, want %d", len(buf), p.BufferSize())
	}
	for i, b := range buf {
		if b != 0x00 {
			t.Fatalf("buf[%d] = 0x%02X, want 0x00 (all-white)", i, b)
		}
	}
}

func TestPackBW_AllBlack(t *testing.T) {
	p := bwTestProfile()
	img := solidImage(16, 16, color.Black)
	buf, err := PackImage(p, img)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for i, b := range buf {
		if b != 0xFF {
			t.Fatalf("buf[%d] = 0x%02X, want 0xFF (all-black)", i, b)
		}
	}
}

func TestPackBW_SinglePixelTopLeft(t *testing.T) {
	p := bwTestProfile()
	img := solidImage(16, 16, color.White)
	img.(*image.Gray).Set(0, 0, color.Black)
	buf, err := PackImage(p, img)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if buf[0] != 0x80 {
		t.Errorf("buf[0] = 0x%02X, want 0x80 (MSB set for pixel (0,0))", buf[0])
	}
	for i := 1; i < len(buf); i++ {
		if buf[i] != 0x00 {
			t.Errorf("buf[%d] = 0x%02X, want 0x00", i, buf[i])
		}
	}
}

func TestPackBW_SinglePixelByte0LSB(t *testing.T) {
	p := bwTestProfile()
	img := solidImage(16, 16, color.White)
	img.(*image.Gray).Set(7, 0, color.Black)
	buf, err := PackImage(p, img)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if buf[0] != 0x01 {
		t.Errorf("buf[0] = 0x%02X, want 0x01 (LSB set for pixel (7,0))", buf[0])
	}
	for i := 1; i < len(buf); i++ {
		if buf[i] != 0x00 {
			t.Errorf("buf[%d] = 0x%02X, want 0x00", i, buf[i])
		}
	}
}

func TestPackBW_Checkerboard(t *testing.T) {
	p := bwTestProfile()
	img := image.NewGray(image.Rect(0, 0, 16, 16))
	// Checkerboard: black on even columns, white on odd columns
	// Each byte: bits 7,5,3,1 set → 0b10101010 = 0xAA
	for y := range 16 {
		for x := range 16 {
			if x%2 == 0 {
				img.Set(x, y, color.Black)
			} else {
				img.Set(x, y, color.White)
			}
		}
	}
	buf, err := PackImage(p, img)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for i, b := range buf {
		if b != 0xAA {
			t.Errorf("buf[%d] = 0x%02X, want 0xAA (checkerboard)", i, b)
		}
	}
}

func TestPackBW_DitherProducesPattern_MidGray(t *testing.T) {
	// A solid mid-gray (Y=128) should produce a stipple pattern: about
	// half the pixels black, half white. Pure threshold would yield either
	// all-white or all-black. We expect *some* set bits and *some* clear
	// bits, and the count to be close to 50%.
	p := bwTestProfile()
	img := solidImage(16, 16, color.Gray{Y: 128})
	buf, err := PackImage(p, img)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var set int
	for _, b := range buf {
		for i := range 8 {
			if b&(1<<i) != 0 {
				set++
			}
		}
	}
	total := 16 * 16
	if set == 0 || set == total {
		t.Fatalf("mid-gray packed to %d/%d set bits — dithering not engaged", set, total)
	}
	// Bayer-4×4 with threshold = m*16 + 8 produces 8 of 16 cells with
	// threshold > 128, so a uniform Y=128 fill yields exactly 50% black.
	if set != total/2 {
		t.Errorf("Y=128 fill set bits = %d, want %d (50%%)", set, total/2)
	}
}

func TestPackBW_DitherProducesPattern_LightGray(t *testing.T) {
	// PaperGray20 (Y=0xCC, 204) is used for the today-hour highlight band.
	// Without dithering it threshold-snaps to white and the band vanishes.
	// With Bayer-4×4 it should yield a sparse stipple (a handful of black
	// pixels per 4×4 cell).
	p := bwTestProfile()
	img := solidImage(16, 16, color.Gray{Y: 0xCC})
	buf, err := PackImage(p, img)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var set int
	for _, b := range buf {
		for i := range 8 {
			if b&(1<<i) != 0 {
				set++
			}
		}
	}
	if set == 0 {
		t.Fatal("PaperGray20 collapsed to pure white — band would vanish on device")
	}
	// Threshold = m*16+8 > 204 only for m ∈ {13, 14, 15} → 3 of 16 cells,
	// so a uniform fill yields 3/16 of the pixels black.
	want := 16 * 16 * 3 / 16
	if set != want {
		t.Errorf("Y=204 (Gray20) fill set bits = %d, want %d", set, want)
	}
}

func TestPackBW_DitherProducesPattern_DarkGray(t *testing.T) {
	// PaperGray80 (Y=0x33, 51) is used for the temperature polyline.
	// Plain threshold lands it at black (it's < 128 so it'd be 100%).
	// Bayer-4×4 should leave a small fraction of white pixels poking
	// through, giving the curve a slight gray weight.
	p := bwTestProfile()
	img := solidImage(16, 16, color.Gray{Y: 0x33})
	buf, err := PackImage(p, img)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var set int
	for _, b := range buf {
		for i := range 8 {
			if b&(1<<i) != 0 {
				set++
			}
		}
	}
	total := 16 * 16
	if set == total {
		t.Fatal("Gray80 collapsed to pure black — no halftone weight")
	}
	// Threshold > 51 for m ∈ {3..15} → 13 of 16 cells. 13/16 = ~81% black.
	want := total * 13 / 16
	if set != want {
		t.Errorf("Y=51 (Gray80) fill set bits = %d, want %d", set, want)
	}
}

func TestPackBW_BufferLength(t *testing.T) {
	p := bwTestProfile()
	img := solidImage(16, 16, color.White)
	buf, err := PackImage(p, img)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(buf) != p.BufferSize() {
		t.Errorf("len(buf) = %d, want %d", len(buf), p.BufferSize())
	}
}

func TestPackImage_UnsupportedColorDepth(t *testing.T) {
	p := &DisplayProfile{Name: "test", Width: 16, Height: 16, Color: Color7}
	img := solidImage(16, 16, color.White)
	_, err := PackImage(p, img)
	if err == nil {
		t.Fatal("expected error for unsupported color depth, got nil")
	}
}

// --- packGray4 tests ---

func TestPackGray4_AllWhite(t *testing.T) {
	p := gray4TestProfile()
	img := solidImage(16, 16, color.White)
	buf, err := PackImage(p, img)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(buf) != p.BufferSize() {
		t.Fatalf("buf len = %d, want %d", len(buf), p.BufferSize())
	}
	for i, b := range buf {
		if b != 0x00 {
			t.Fatalf("buf[%d] = 0x%02X, want 0x00 (all-white gray4)", i, b)
		}
	}
}

func TestPackGray4_AllBlack(t *testing.T) {
	p := gray4TestProfile()
	img := solidImage(16, 16, color.Black)
	buf, err := PackImage(p, img)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for i, b := range buf {
		if b != 0xFF {
			t.Fatalf("buf[%d] = 0x%02X, want 0xFF (all-black gray4)", i, b)
		}
	}
}

func TestPackGray4_MixedLevels(t *testing.T) {
	// 4 pixels in one byte: white(00), light-gray(01), dark-gray(10), black(11)
	// Expected byte: 0b00_01_10_11 = 0x1B
	p := &DisplayProfile{Name: "test", Width: 4, Height: 1, Color: Gray4}
	img := image.NewGray(image.Rect(0, 0, 4, 1))
	img.Set(0, 0, color.Gray{Y: 255}) // white  → 00
	img.Set(1, 0, color.Gray{Y: 192}) // light gray → 01
	img.Set(2, 0, color.Gray{Y: 128}) // dark gray  → 10
	img.Set(3, 0, color.Gray{Y: 0})   // black      → 11
	buf, err := PackImage(p, img)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(buf) != 1 {
		t.Fatalf("buf len = %d, want 1", len(buf))
	}
	if buf[0] != 0x1B {
		t.Errorf("buf[0] = 0x%02X (0b%08b), want 0x1B (0b00011011)", buf[0], buf[0])
	}
}

func TestPackGray4_BufferLength(t *testing.T) {
	p := gray4TestProfile()
	img := solidImage(16, 16, color.White)
	buf, err := PackImage(p, img)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(buf) != p.BufferSize() {
		t.Errorf("len(buf) = %d, want %d", len(buf), p.BufferSize())
	}
}

// --- splitGray4Planes tests ---

// TestSplitGray4Planes walks the upstream Waveshare display_4Gray plane
// derivation across all four shades using Inkwell's encoding (white=00,
// light=01, dark=10, black=11). In this encoding the plane bits fall out
// trivially: plane A (0x10) carries the low bit of each pixel; plane B
// (0x13) carries the high bit. Locking each shade independently — plus
// the mixed and multi-byte cases — guards against a "levels swapped"
// regression that buffer-level tests can't catch (packGray4 is a host-
// side function and never runs on device).
func TestSplitGray4Planes(t *testing.T) {
	cases := []struct {
		label string
		in    []byte
		wantA []byte
		wantB []byte
	}{
		{
			label: "all white (00) → A=0 B=0",
			in:    []byte{0x00, 0x00},
			wantA: []byte{0x00},
			wantB: []byte{0x00},
		},
		{
			label: "all black (11) → A=1 B=1",
			in:    []byte{0xFF, 0xFF},
			wantA: []byte{0xFF},
			wantB: []byte{0xFF},
		},
		{
			label: "all light gray (01) → A=1 B=0",
			in:    []byte{0x55, 0x55},
			wantA: []byte{0xFF},
			wantB: []byte{0x00},
		},
		{
			label: "all dark gray (10) → A=0 B=1",
			in:    []byte{0xAA, 0xAA},
			wantA: []byte{0x00},
			wantB: []byte{0xFF},
		},
		{
			label: "mixed w,l,d,b, b,d,l,w",
			// byte0 = 0b00_01_10_11 = 0x1B
			// byte1 = 0b11_10_01_00 = 0xE4
			in:    []byte{0x1B, 0xE4},
			wantA: []byte{0x5A}, // 0,1,0,1, 1,0,1,0
			wantB: []byte{0x3C}, // 0,0,1,1, 1,1,0,0
		},
		{
			label: "two output bytes: mixed then light+dark",
			in:    []byte{0x1B, 0xE4, 0x55, 0xAA},
			wantA: []byte{0x5A, 0xF0},
			wantB: []byte{0x3C, 0x0F},
		},
	}
	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			gotA, gotB := splitGray4Planes(tc.in)
			if len(gotA) != len(tc.wantA) || len(gotB) != len(tc.wantB) {
				t.Fatalf("plane lengths = (%d, %d), want (%d, %d)",
					len(gotA), len(gotB), len(tc.wantA), len(tc.wantB))
			}
			for i := range tc.wantA {
				if gotA[i] != tc.wantA[i] {
					t.Errorf("planeA[%d] = 0x%02X (0b%08b), want 0x%02X (0b%08b)",
						i, gotA[i], gotA[i], tc.wantA[i], tc.wantA[i])
				}
				if gotB[i] != tc.wantB[i] {
					t.Errorf("planeB[%d] = 0x%02X (0b%08b), want 0x%02X (0b%08b)",
						i, gotB[i], gotB[i], tc.wantB[i], tc.wantB[i])
				}
			}
		})
	}
}

// --- joinGray4Planes tests ---

// TestJoinGray4Planes covers the inverse of splitGray4Planes. Joining
// the two 1bpp planes recovers the original 2bpp buffer because plane A
// is the low bit and plane B is the high bit of each pixel — the
// projection is information-preserving. The same shade table as
// TestSplitGray4Planes runs here to make the symmetry visible.
func TestJoinGray4Planes(t *testing.T) {
	cases := []struct {
		label  string
		planeA []byte
		planeB []byte
		want   []byte
	}{
		{
			label:  "all white",
			planeA: []byte{0x00},
			planeB: []byte{0x00},
			want:   []byte{0x00, 0x00},
		},
		{
			label:  "all black",
			planeA: []byte{0xFF},
			planeB: []byte{0xFF},
			want:   []byte{0xFF, 0xFF},
		},
		{
			label:  "all light gray (low bit set)",
			planeA: []byte{0xFF},
			planeB: []byte{0x00},
			want:   []byte{0x55, 0x55},
		},
		{
			label:  "all dark gray (high bit set)",
			planeA: []byte{0x00},
			planeB: []byte{0xFF},
			want:   []byte{0xAA, 0xAA},
		},
		{
			label:  "mixed w,l,d,b, b,d,l,w",
			planeA: []byte{0x5A},
			planeB: []byte{0x3C},
			want:   []byte{0x1B, 0xE4},
		},
	}
	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			got := joinGray4Planes(tc.planeA, tc.planeB)
			if len(got) != len(tc.want) {
				t.Fatalf("len = %d, want %d", len(got), len(tc.want))
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Errorf("byte[%d] = 0x%02X, want 0x%02X", i, got[i], tc.want[i])
				}
			}
		})
	}
}

// TestSplitJoinGray4PlanesRoundTrip exercises both helpers as inverses
// using a packGray4 output as input — the most realistic source of
// 2bpp buffers in production. If either function develops a bit-order
// regression, the round-trip catches it without needing a hardware
// re-derivation.
func TestSplitJoinGray4PlanesRoundTrip(t *testing.T) {
	p := &DisplayProfile{Name: "rt", Width: 16, Height: 4, Color: Gray4}
	img := image.NewGray(image.Rect(0, 0, 16, 4))
	// Sprinkle all four shades so the buffer has non-trivial bits set.
	shades := []uint8{0xFF, 0xC0, 0x80, 0x00}
	for y := range 4 {
		for x := range 16 {
			img.Set(x, y, color.Gray{Y: shades[(x+y)%4]})
		}
	}
	orig, err := PackImage(p, img)
	if err != nil {
		t.Fatalf("PackImage: %v", err)
	}

	a, b := splitGray4Planes(orig)
	back := joinGray4Planes(a, b)
	if len(back) != len(orig) {
		t.Fatalf("round-trip length = %d, want %d", len(back), len(orig))
	}
	for i := range orig {
		if back[i] != orig[i] {
			t.Errorf("byte[%d] = 0x%02X, want 0x%02X (round-trip lost)",
				i, back[i], orig[i])
		}
	}
}

// --- UnpackBuffer tests ---

func TestUnpackBuffer_RoundTrip(t *testing.T) {
	p := bwTestProfile()
	// Create a non-trivial input image
	orig := image.NewGray(image.Rect(0, 0, 16, 16))
	for y := range 16 {
		for x := range 16 {
			if (x+y)%3 == 0 {
				orig.Set(x, y, color.Black)
			} else {
				orig.Set(x, y, color.White)
			}
		}
	}
	buf, err := PackImage(p, orig)
	if err != nil {
		t.Fatalf("PackImage: %v", err)
	}
	got := UnpackBuffer(p, buf)
	for y := range 16 {
		for x := range 16 {
			origGray := color.GrayModel.Convert(orig.At(x, y)).(color.Gray)
			gotGray := color.GrayModel.Convert(got.At(x, y)).(color.Gray)
			origBlack := origGray.Y < 128
			gotBlack := gotGray.Y < 128
			if origBlack != gotBlack {
				t.Errorf("pixel (%d,%d): origBlack=%v gotBlack=%v", x, y, origBlack, gotBlack)
			}
		}
	}
}

func TestUnpackBuffer_KnownBuffer(t *testing.T) {
	p := &DisplayProfile{Name: "test", Width: 8, Height: 1, Color: BW}
	// 0x80 = 0b10000000: only first pixel (MSB) is black
	buf := []byte{0x80}
	img := UnpackBuffer(p, buf)
	// Pixel (0,0) should be black
	c0 := color.GrayModel.Convert(img.At(0, 0)).(color.Gray)
	if c0.Y >= 128 {
		t.Errorf("pixel (0,0): Y=%d, want black (<128)", c0.Y)
	}
	// All other pixels should be white
	for x := 1; x < 8; x++ {
		c := color.GrayModel.Convert(img.At(x, 0)).(color.Gray)
		if c.Y < 128 {
			t.Errorf("pixel (%d,0): Y=%d, want white (>=128)", x, c.Y)
		}
	}
}

// TestUnpackBuffer_Gray4 validates the inverse of packGray4: a 2bpp
// buffer decodes back to a 4-colour paletted image with the four
// canonical luminances. The pixel codes in Inkwell's encoding
// (white=00, light=01, dark=10, black=11) align with the palette
// indices so the byte 0x1B = 0b00_01_10_11 unpacks to
// white,light,dark,black left-to-right.
func TestUnpackBuffer_Gray4(t *testing.T) {
	p := &DisplayProfile{Name: "test", Width: 4, Height: 1, Color: Gray4}
	buf := []byte{0x1B} // w, l, d, b
	img := UnpackBuffer(p, buf)

	wantY := []uint8{0xFF, 0xC0, 0x80, 0x00}
	for x, want := range wantY {
		got := color.GrayModel.Convert(img.At(x, 0)).(color.Gray)
		if got.Y != want {
			t.Errorf("pixel (%d,0): Y=0x%02X, want 0x%02X", x, got.Y, want)
		}
	}
	// The palette must have exactly 4 entries so PNG encoding picks the
	// right bit depth and clients see four distinct shades.
	if len(img.Palette) != 4 {
		t.Errorf("palette len = %d, want 4", len(img.Palette))
	}
}

func TestUnpackBuffer_Gray4_RoundTrip(t *testing.T) {
	p := &DisplayProfile{Name: "rt", Width: 16, Height: 4, Color: Gray4}
	img := image.NewGray(image.Rect(0, 0, 16, 4))
	shades := []uint8{0xFF, 0xC0, 0x80, 0x00}
	for y := range 4 {
		for x := range 16 {
			img.Set(x, y, color.Gray{Y: shades[(x+y)%4]})
		}
	}
	buf, err := PackImage(p, img)
	if err != nil {
		t.Fatalf("PackImage: %v", err)
	}
	got := UnpackBuffer(p, buf)
	for y := range 4 {
		for x := range 16 {
			origY := color.GrayModel.Convert(img.At(x, y)).(color.Gray).Y
			gotY := color.GrayModel.Convert(got.At(x, y)).(color.Gray).Y
			if origY != gotY {
				t.Errorf("pixel (%d,%d): orig=0x%02X got=0x%02X", x, y, origY, gotY)
			}
		}
	}
}

// --- reconstructFrame tests ---
//
// reconstructFrame is the shared helper that capture backends
// (WebPreview, ImageBackend) use to turn the two on-wire planes back
// into a viewable image. It dispatches on profile.Color so each
// backend's per-frame handling stays a one-liner.

func TestReconstructFrame_BW(t *testing.T) {
	p := bwTestProfile()
	// Single black pixel at (0,0); rest white.
	pack := make([]byte, p.BufferSize())
	pack[0] = 0x80
	// The "old" plane is whatever Display sent — by convention ~pack —
	// but reconstructFrame for BW must ignore it and use only pack.
	old := make([]byte, p.BufferSize())
	for i, b := range pack {
		old[i] = ^b
	}

	img, err := reconstructFrame(p, old, pack)
	if err != nil {
		t.Fatalf("reconstructFrame: %v", err)
	}
	c := color.GrayModel.Convert(img.At(0, 0)).(color.Gray)
	if c.Y >= 128 {
		t.Errorf("pixel (0,0) Y=%d, want black", c.Y)
	}
}

func TestReconstructFrame_Gray4(t *testing.T) {
	// 16 pixels = 4 bytes 2bpp → 2 bytes per plane.
	p := &DisplayProfile{Name: "test", Width: 16, Height: 1, Color: Gray4}
	full := []byte{0x1B, 0xE4, 0x55, 0xAA} // w,l,d,b, b,d,l,w, then 4 light, 4 dark
	planeA, planeB := splitGray4Planes(full)

	img, err := reconstructFrame(p, planeA, planeB)
	if err != nil {
		t.Fatalf("reconstructFrame: %v", err)
	}
	wantY := []uint8{0xFF, 0xC0, 0x80, 0x00} // first 4 pixels: w, l, d, b
	for x, want := range wantY {
		got := color.GrayModel.Convert(img.At(x, 0)).(color.Gray).Y
		if got != want {
			t.Errorf("pixel (%d,0) Y=0x%02X, want 0x%02X", x, got, want)
		}
	}
}

func TestReconstructFrame_SizeMismatch(t *testing.T) {
	cases := []struct {
		label   string
		profile *DisplayProfile
		old     []byte
		new     []byte
	}{
		{
			label:   "BW: new plane wrong size",
			profile: bwTestProfile(),
			old:     make([]byte, 32),
			new:     make([]byte, 31),
		},
		{
			label:   "Gray4: planes mismatched",
			profile: &DisplayProfile{Name: "t", Width: 16, Height: 2, Color: Gray4},
			old:     make([]byte, 4),
			new:     make([]byte, 3),
		},
	}
	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			if _, err := reconstructFrame(tc.profile, tc.old, tc.new); err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

func TestReconstructFrame_UnsupportedColorDepth(t *testing.T) {
	p := &DisplayProfile{Name: "c7", Width: 16, Height: 2, Color: Color7}
	if _, err := reconstructFrame(p, nil, nil); err == nil {
		t.Error("expected error for Color7, got nil")
	}
}
