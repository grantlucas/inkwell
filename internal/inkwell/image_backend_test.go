package inkwell

import (
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// imageTestProfile returns a small 16x16 BW profile with command registers set,
// suitable for testing the image backend's command detection.
func imageTestProfile() *DisplayProfile {
	return &DisplayProfile{
		Name:         "test",
		Width:        16,
		Height:       16,
		Color:        BW,
		OldBufferCmd: 0x10,
		NewBufferCmd: 0x13,
		RefreshCmd:   0x12,
	}
}

func TestImageBackend_DisplayProducesPNG(t *testing.T) {
	dir := t.TempDir()
	p := imageTestProfile()
	backend := NewImageBackend(p, dir)

	buf := make([]byte, p.BufferSize())
	sendDisplaySequence(t, backend, p, buf)

	path := filepath.Join(dir, "frame_000.png")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected PNG file at %s: %v", path, err)
	}
}

func TestImageBackend_PNGMatchesBuffer(t *testing.T) {
	dir := t.TempDir()
	p := imageTestProfile()
	backend := NewImageBackend(p, dir)

	// Create a buffer with top-left pixel black (MSB of byte 0 = 1)
	buf := make([]byte, p.BufferSize())
	buf[0] = 0x80 // pixel (0,0) is black

	sendDisplaySequence(t, backend, p, buf)

	// Read the PNG back and verify pixel values
	f, err := os.Open(filepath.Join(dir, "frame_000.png"))
	if err != nil {
		t.Fatalf("open PNG: %v", err)
	}
	defer f.Close()

	img, err := png.Decode(f)
	if err != nil {
		t.Fatalf("decode PNG: %v", err)
	}

	// Pixel (0,0) should be black
	r, g, b, _ := img.At(0, 0).RGBA()
	if r != 0 || g != 0 || b != 0 {
		t.Errorf("pixel (0,0): got (%d,%d,%d), want black (0,0,0)", r, g, b)
	}

	// Pixel (1,0) should be white
	r, g, b, _ = img.At(1, 0).RGBA()
	wR, wG, wB, _ := color.White.RGBA()
	if r != wR || g != wG || b != wB {
		t.Errorf("pixel (1,0): got (%d,%d,%d), want white", r, g, b)
	}
}

func TestImageBackend_SequentialFilenames(t *testing.T) {
	dir := t.TempDir()
	p := imageTestProfile()
	backend := NewImageBackend(p, dir)

	buf := make([]byte, p.BufferSize())

	// Three display cycles
	for i := 0; i < 3; i++ {
		sendDisplaySequence(t, backend, p, buf)
	}

	for _, name := range []string{"frame_000.png", "frame_001.png", "frame_002.png"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("expected file %s: %v", name, err)
		}
	}
}

func TestImageBackend_WriteErrorOnBadDir(t *testing.T) {
	p := imageTestProfile()
	badPath := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(badPath, []byte("x"), 0o600); err != nil {
		t.Fatalf("create sentinel file: %v", err)
	}
	backend := NewImageBackend(p, badPath)

	buf := make([]byte, p.BufferSize())

	// Set up captured buffer via NewBufferCmd + SendData
	if err := backend.SendCommand(p.NewBufferCmd); err != nil {
		t.Fatalf("SendCommand(NewBufferCmd): %v", err)
	}
	if err := backend.SendData(buf); err != nil {
		t.Fatalf("SendData: %v", err)
	}

	// RefreshCmd should return an error since output path is a file, not a directory
	err := backend.SendCommand(p.RefreshCmd)
	if err == nil {
		t.Fatal("expected error writing to non-directory output path, got nil")
	}
}

func TestImageBackend_NilProfileReturnsError(t *testing.T) {
	backend := NewImageBackend(nil, t.TempDir())

	if err := backend.SendCommand(0x12); err == nil {
		t.Error("SendCommand with nil profile: expected error, got nil")
	}
	if err := backend.SendData([]byte{0x00}); err == nil {
		t.Error("SendData with nil profile: expected error, got nil")
	}
}

func TestImageBackend_BufferSizeMismatchReturnsError(t *testing.T) {
	p := imageTestProfile()
	backend := NewImageBackend(p, t.TempDir())

	// Capture a buffer that's the wrong size
	if err := backend.SendCommand(p.NewBufferCmd); err != nil {
		t.Fatalf("SendCommand(NewBufferCmd): %v", err)
	}
	if err := backend.SendData([]byte{0x00}); err != nil { // 1 byte, expected 32
		t.Fatalf("SendData: %v", err)
	}

	err := backend.SendCommand(p.RefreshCmd)
	if err == nil {
		t.Fatal("expected buffer size mismatch error, got nil")
	}
}

func TestImageBackend_ReadBusyResetClose(t *testing.T) {
	p := imageTestProfile()
	backend := NewImageBackend(p, t.TempDir())

	if !backend.ReadBusy() {
		t.Error("ReadBusy: got false, want true")
	}
	if err := backend.Reset(); err != nil {
		t.Errorf("Reset: %v", err)
	}
	if err := backend.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}

// sendDisplaySequence simulates the EPD Display() call sequence on a backend.
func sendDisplaySequence(t *testing.T, hw Hardware, p *DisplayProfile, buf []byte) {
	t.Helper()
	inverted := make([]byte, len(buf))
	for i, b := range buf {
		inverted[i] = ^b
	}
	if err := hw.SendCommand(p.OldBufferCmd); err != nil {
		t.Fatalf("SendCommand(OldBufferCmd): %v", err)
	}
	if err := hw.SendData(inverted); err != nil {
		t.Fatalf("SendData(old): %v", err)
	}
	if err := hw.SendCommand(p.NewBufferCmd); err != nil {
		t.Fatalf("SendCommand(NewBufferCmd): %v", err)
	}
	if err := hw.SendData(buf); err != nil {
		t.Fatalf("SendData(new): %v", err)
	}
	if err := hw.SendCommand(p.RefreshCmd); err != nil {
		t.Fatalf("SendCommand(RefreshCmd): %v", err)
	}
}
