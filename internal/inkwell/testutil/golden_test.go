package testutil

import (
	"bytes"
	"image"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/grantlucas/inkwell/internal/inkwell/widget"
)

// goldenFixDir overrides GoldenDir for test isolation and restores it on cleanup.
func goldenFixDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	old := GoldenDir
	GoldenDir = dir
	t.Cleanup(func() { GoldenDir = old })
	return dir
}

// goldenTestPath returns the path AssertGoldenBuffer/PNG will use for the given
// test name and extension.
func goldenTestPath(dir, testName, ext string) string {
	safe := strings.ReplaceAll(testName, "/", "_")
	return filepath.Join(dir, safe+ext)
}

// spyT captures Errorf/Fatalf calls without failing the parent test.
type spyT struct {
	*testing.T
	errored bool
	fataled bool
}

func (s *spyT) Helper()      {}
func (s *spyT) Name() string { return s.T.Name() }
func (s *spyT) Errorf(_ string, _ ...any) {
	s.errored = true
}
func (s *spyT) Fatalf(_ string, _ ...any) {
	s.fataled = true
	runtime.Goexit()
}

// runSpy executes fn with a spyT in a separate goroutine so that
// runtime.Goexit (from spyT.Fatalf) terminates that goroutine rather than
// the test runner's goroutine.
func runSpy(t *testing.T, fn func(s *spyT)) *spyT {
	t.Helper()
	spy := &spyT{T: t}
	done := make(chan struct{})
	go func() {
		defer close(done)
		fn(spy)
	}()
	<-done
	return spy
}

func TestAssertGoldenBuffer_Match(t *testing.T) {
	dir := goldenFixDir(t)
	buf := []byte{0x01, 0x02, 0x03}

	// Pre-write matching golden file.
	p := goldenTestPath(dir, t.Name(), ".bin")
	if err := os.WriteFile(p, buf, 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	AssertGoldenBuffer(t, buf)
}

func TestAssertGoldenBuffer_Mismatch(t *testing.T) {
	dir := goldenFixDir(t)

	// Golden has different content.
	p := goldenTestPath(dir, t.Name(), ".bin")
	if err := os.WriteFile(p, []byte{0xAA, 0xBB}, 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	spy := runSpy(t, func(s *spyT) {
		AssertGoldenBuffer(s, []byte{0x11, 0x22})
	})
	if !spy.errored {
		t.Error("expected mismatch error, but none was reported")
	}
}

func TestAssertGoldenBuffer_Update(t *testing.T) {
	dir := goldenFixDir(t)

	oldUpdate := *Update
	*Update = true
	defer func() { *Update = oldUpdate }()

	buf := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	AssertGoldenBuffer(t, buf)

	p := goldenTestPath(dir, t.Name(), ".bin")
	got, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("golden file not written: %v", err)
	}
	if !bytes.Equal(got, buf) {
		t.Errorf("golden file content = %x, want %x", got, buf)
	}
}

func TestAssertGoldenBuffer_MissingFile(t *testing.T) {
	goldenFixDir(t) // empty dir — no golden file present

	spy := runSpy(t, func(s *spyT) {
		AssertGoldenBuffer(s, []byte{0x01})
	})
	if !spy.fataled {
		t.Error("expected fatal on missing file, but none occurred")
	}
}

func TestAssertGoldenPNG_Match(t *testing.T) {
	dir := goldenFixDir(t)

	img := image.NewPaletted(image.Rect(0, 0, 4, 4),
		widget.PaperPalette)

	p := goldenTestPath(dir, t.Name(), ".png")
	f, err := os.Create(p)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		t.Fatalf("setup: encode: %v", err)
	}
	f.Close()

	AssertGoldenPNG(t, img)
}

// A golden PNG that holds the same pixels but different bytes must still
// match. Go's PNG encoder is not byte-stable across releases — 1.27 emits 229
// bytes where 1.26 emitted 232 for the clock widget's golden, pixel for pixel
// identical — so comparing encoded bytes pins the encoder rather than the
// render, and every toolchain bump falsely fails.
func TestAssertGoldenPNG_MatchesDespiteDifferentEncoding(t *testing.T) {
	dir := goldenFixDir(t)

	img := image.NewPaletted(image.Rect(0, 0, 4, 4), widget.PaperPalette)
	img.SetColorIndex(1, 1, 11)
	img.SetColorIndex(2, 3, 5)

	p := goldenTestPath(dir, t.Name(), ".png")
	f, err := os.Create(p)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	// A different compression level stands in for a different encoder
	// version: same pixels, different bytes on disk.
	enc := png.Encoder{CompressionLevel: png.NoCompression}
	if err := enc.Encode(f, img); err != nil {
		f.Close()
		t.Fatalf("setup: encode: %v", err)
	}
	f.Close()

	var reference bytes.Buffer
	if err := png.Encode(&reference, img); err != nil {
		t.Fatalf("setup: re-encode: %v", err)
	}
	golden, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("setup: read back: %v", err)
	}
	if bytes.Equal(golden, reference.Bytes()) {
		t.Fatal("setup: the two encodings are byte-identical, so this test proves nothing")
	}

	AssertGoldenPNG(t, img)
}

func TestAssertGoldenPNG_Mismatch(t *testing.T) {
	dir := goldenFixDir(t)

	golden := image.NewPaletted(image.Rect(0, 0, 4, 4),
		widget.PaperPalette)

	p := goldenTestPath(dir, t.Name(), ".png")
	f, err := os.Create(p)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := png.Encode(f, golden); err != nil {
		f.Close()
		t.Fatalf("setup: encode: %v", err)
	}
	f.Close()

	// One black pixel — differs from the all-white golden.
	different := image.NewPaletted(image.Rect(0, 0, 4, 4),
		widget.PaperPalette)
	different.SetColorIndex(0, 0, 1)

	spy := runSpy(t, func(s *spyT) {
		AssertGoldenPNG(s, different)
	})
	if !spy.errored {
		t.Error("expected mismatch error, but none was reported")
	}
}

func TestAssertGoldenPNG_Update(t *testing.T) {
	dir := goldenFixDir(t)

	oldUpdate := *Update
	*Update = true
	defer func() { *Update = oldUpdate }()

	img := image.NewPaletted(image.Rect(0, 0, 2, 2),
		widget.PaperPalette)
	img.SetColorIndex(1, 1, 1)

	AssertGoldenPNG(t, img)

	p := goldenTestPath(dir, t.Name(), ".png")
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("golden PNG not written: %v", err)
	}
	decoded, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode golden PNG: %v", err)
	}
	// Verify pixels round-trip.
	for y := range 2 {
		for x := range 2 {
			gr, gg, gb, _ := decoded.At(x, y).RGBA()
			wr, wg, wb, _ := img.At(x, y).RGBA()
			if gr != wr || gg != wg || gb != wb {
				t.Errorf("pixel (%d,%d): decoded %v, want %v", x, y, decoded.At(x, y), img.At(x, y))
			}
		}
	}
}

func TestAssertGoldenPNG_MissingFile(t *testing.T) {
	goldenFixDir(t) // empty dir

	img := image.NewPaletted(image.Rect(0, 0, 2, 2),
		widget.PaperPalette)

	spy := runSpy(t, func(s *spyT) {
		AssertGoldenPNG(s, img)
	})
	if !spy.fataled {
		t.Error("expected fatal on missing file, but none occurred")
	}
}

// --- error-path tests for 100% coverage ---

func TestAssertGoldenBuffer_UpdateMkdirError(t *testing.T) {
	// Point GoldenDir at a path that cannot be created (a file blocks it).
	base := t.TempDir()
	blocker := filepath.Join(base, "blocker")
	if err := os.WriteFile(blocker, []byte{}, 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	old := GoldenDir
	GoldenDir = filepath.Join(blocker, "subdir") // blocker is a file, not a dir
	defer func() { GoldenDir = old }()

	oldUpdate := *Update
	*Update = true
	defer func() { *Update = oldUpdate }()

	spy := runSpy(t, func(s *spyT) {
		AssertGoldenBuffer(s, []byte{0x01})
	})
	if !spy.fataled {
		t.Error("expected fatal on MkdirAll error, but none occurred")
	}
}

func TestAssertGoldenBuffer_UpdateWriteError(t *testing.T) {
	dir := goldenFixDir(t)

	// Place a directory at the target file path so WriteFile fails.
	safe := strings.ReplaceAll(t.Name(), "/", "_")
	targetPath := filepath.Join(dir, safe+".bin")
	if err := os.MkdirAll(targetPath, 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}

	oldUpdate := *Update
	*Update = true
	defer func() { *Update = oldUpdate }()

	spy := runSpy(t, func(s *spyT) {
		AssertGoldenBuffer(s, []byte{0x01})
	})
	if !spy.fataled {
		t.Error("expected fatal on WriteFile error, but none occurred")
	}
}

func TestAssertGoldenPNG_UpdateMkdirError(t *testing.T) {
	base := t.TempDir()
	blocker := filepath.Join(base, "blocker")
	if err := os.WriteFile(blocker, []byte{}, 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	old := GoldenDir
	GoldenDir = filepath.Join(blocker, "subdir")
	defer func() { GoldenDir = old }()

	oldUpdate := *Update
	*Update = true
	defer func() { *Update = oldUpdate }()

	img := image.NewPaletted(image.Rect(0, 0, 2, 2),
		widget.PaperPalette)

	spy := runSpy(t, func(s *spyT) {
		AssertGoldenPNG(s, img)
	})
	if !spy.fataled {
		t.Error("expected fatal on MkdirAll error, but none occurred")
	}
}

func TestAssertGoldenPNG_UpdateCreateError(t *testing.T) {
	dir := goldenFixDir(t)

	// Place a directory at the target file path so os.Create fails.
	safe := strings.ReplaceAll(t.Name(), "/", "_")
	targetPath := filepath.Join(dir, safe+".png")
	if err := os.MkdirAll(targetPath, 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}

	oldUpdate := *Update
	*Update = true
	defer func() { *Update = oldUpdate }()

	img := image.NewPaletted(image.Rect(0, 0, 2, 2),
		widget.PaperPalette)

	spy := runSpy(t, func(s *spyT) {
		AssertGoldenPNG(s, img)
	})
	if !spy.fataled {
		t.Error("expected fatal on Create error, but none occurred")
	}
}

func TestAssertGoldenPNG_UpdateEncodeError(t *testing.T) {
	goldenFixDir(t)

	oldUpdate := *Update
	*Update = true
	defer func() { *Update = oldUpdate }()

	oldEncode := GoldenEncodePNG
	GoldenEncodePNG = func(_ io.Writer, _ image.Image) error {
		return os.ErrInvalid
	}
	defer func() { GoldenEncodePNG = oldEncode }()

	img := image.NewPaletted(image.Rect(0, 0, 2, 2),
		widget.PaperPalette)

	spy := runSpy(t, func(s *spyT) {
		AssertGoldenPNG(s, img)
	})
	if !spy.fataled {
		t.Error("expected fatal on encode error, but none occurred")
	}
}

func TestAssertGoldenPNG_UndecodableGolden(t *testing.T) {
	dir := goldenFixDir(t)

	// A golden file that is not a PNG at all — a truncated or corrupted
	// checkout, say — is a setup failure, not a render mismatch.
	p := goldenTestPath(dir, t.Name(), ".png")
	if err := os.WriteFile(p, []byte("not a png"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	img := image.NewPaletted(image.Rect(0, 0, 2, 2), widget.PaperPalette)
	spy := runSpy(t, func(s *spyT) {
		AssertGoldenPNG(s, img)
	})
	if !spy.fataled {
		t.Error("expected fatal on undecodable golden, but none occurred")
	}
}

func TestAssertGoldenPNG_BoundsMismatch(t *testing.T) {
	dir := goldenFixDir(t)

	golden := image.NewPaletted(image.Rect(0, 0, 4, 4), widget.PaperPalette)
	p := goldenTestPath(dir, t.Name(), ".png")
	f, err := os.Create(p)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := png.Encode(f, golden); err != nil {
		f.Close()
		t.Fatalf("setup: encode: %v", err)
	}
	f.Close()

	// A resized widget must report the size change rather than scanning
	// pixels off the end of the golden.
	resized := image.NewPaletted(image.Rect(0, 0, 8, 4), widget.PaperPalette)
	spy := runSpy(t, func(s *spyT) {
		AssertGoldenPNG(s, resized)
	})
	if !spy.errored {
		t.Error("expected bounds-mismatch error, but none was reported")
	}
}
