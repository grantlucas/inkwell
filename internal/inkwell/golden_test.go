package inkwell

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// goldenFixDir overrides goldenDir for test isolation and restores it on cleanup.
func goldenFixDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	old := goldenDir
	goldenDir = dir
	t.Cleanup(func() { goldenDir = old })
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

func (s *spyT) Helper() {}
func (s *spyT) Name() string { return s.T.Name() }
func (s *spyT) Errorf(_ string, _ ...any) {
	s.errored = true
}
func (s *spyT) Fatalf(_ string, _ ...any) {
	s.fataled = true
	// The implementation uses `return` after every Fatalf call, so we don't
	// need runtime.Goexit() here.
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

	spy := &spyT{T: t}
	AssertGoldenBuffer(spy, []byte{0x11, 0x22})
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

	spy := &spyT{T: t}
	AssertGoldenBuffer(spy, []byte{0x01})
	if !spy.fataled {
		t.Error("expected fatal on missing file, but none occurred")
	}
}

func TestAssertGoldenPNG_Match(t *testing.T) {
	dir := goldenFixDir(t)

	img := image.NewPaletted(image.Rect(0, 0, 4, 4),
		color.Palette{color.White, color.Black})

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

func TestAssertGoldenPNG_Mismatch(t *testing.T) {
	dir := goldenFixDir(t)

	golden := image.NewPaletted(image.Rect(0, 0, 4, 4),
		color.Palette{color.White, color.Black})

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
		color.Palette{color.White, color.Black})
	different.SetColorIndex(0, 0, 1)

	spy := &spyT{T: t}
	AssertGoldenPNG(spy, different)
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
		color.Palette{color.White, color.Black})
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
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
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
		color.Palette{color.White, color.Black})

	spy := &spyT{T: t}
	AssertGoldenPNG(spy, img)
	if !spy.fataled {
		t.Error("expected fatal on missing file, but none occurred")
	}
}

// --- error-path tests for 100% coverage ---

func TestAssertGoldenBuffer_UpdateMkdirError(t *testing.T) {
	// Point goldenDir at a path that cannot be created (a file blocks it).
	base := t.TempDir()
	blocker := filepath.Join(base, "blocker")
	if err := os.WriteFile(blocker, []byte{}, 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	old := goldenDir
	goldenDir = filepath.Join(blocker, "subdir") // blocker is a file, not a dir
	defer func() { goldenDir = old }()

	oldUpdate := *Update
	*Update = true
	defer func() { *Update = oldUpdate }()

	spy := &spyT{T: t}
	AssertGoldenBuffer(spy, []byte{0x01})
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

	spy := &spyT{T: t}
	AssertGoldenBuffer(spy, []byte{0x01})
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
	old := goldenDir
	goldenDir = filepath.Join(blocker, "subdir")
	defer func() { goldenDir = old }()

	oldUpdate := *Update
	*Update = true
	defer func() { *Update = oldUpdate }()

	img := image.NewPaletted(image.Rect(0, 0, 2, 2),
		color.Palette{color.White, color.Black})

	spy := &spyT{T: t}
	AssertGoldenPNG(spy, img)
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
		color.Palette{color.White, color.Black})

	spy := &spyT{T: t}
	AssertGoldenPNG(spy, img)
	if !spy.fataled {
		t.Error("expected fatal on Create error, but none occurred")
	}
}

func TestAssertGoldenPNG_UpdateEncodeError(t *testing.T) {
	goldenFixDir(t)

	oldUpdate := *Update
	*Update = true
	defer func() { *Update = oldUpdate }()

	oldEncode := goldenEncodePNG
	goldenEncodePNG = func(_ io.Writer, _ image.Image) error {
		return os.ErrInvalid
	}
	defer func() { goldenEncodePNG = oldEncode }()

	img := image.NewPaletted(image.Rect(0, 0, 2, 2),
		color.Palette{color.White, color.Black})

	spy := &spyT{T: t}
	AssertGoldenPNG(spy, img)
	if !spy.fataled {
		t.Error("expected fatal on encode error, but none occurred")
	}
}

func TestAssertGoldenPNG_CompareEncodeError(t *testing.T) {
	dir := goldenFixDir(t)

	// Write a golden PNG so ReadFile succeeds.
	img := image.NewPaletted(image.Rect(0, 0, 2, 2),
		color.Palette{color.White, color.Black})
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

	oldEncode := goldenEncodePNG
	goldenEncodePNG = func(_ io.Writer, _ image.Image) error {
		return os.ErrInvalid
	}
	defer func() { goldenEncodePNG = oldEncode }()

	spy := &spyT{T: t}
	AssertGoldenPNG(spy, img)
	if !spy.fataled {
		t.Error("expected fatal on encode error during comparison, but none occurred")
	}
}
