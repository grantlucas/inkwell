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
