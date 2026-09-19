package testutil

import (
	"bytes"
	"flag"
	"image"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Update controls whether AssertGoldenBuffer and AssertGoldenPNG update the
// golden files on disk instead of comparing against them. Enable with -update.
var Update = flag.Bool("update", false, "update golden files")

// GoldenDir is the base directory for golden files, relative to the package
// directory. Tests may override this for isolation.
var GoldenDir = "testdata"

// GoldenEncodePNG is the PNG encoder used by AssertGoldenPNG. Tests may
// replace it to simulate encode errors.
var GoldenEncodePNG = func(w io.Writer, m image.Image) error {
	return png.Encode(w, m)
}

// THelper is the subset of *testing.T used by the golden helpers.
type THelper interface {
	Helper()
	Errorf(format string, args ...any)
	Fatalf(format string, args ...any)
	Name() string
}

func goldenPath(t THelper, ext string) string {
	t.Helper()
	safe := strings.ReplaceAll(t.Name(), "/", "_")
	return filepath.Join(GoldenDir, safe+ext)
}

// AssertGoldenBuffer compares buf against the golden file
// testdata/<TestName>.bin. With -update, it writes buf to that file instead.
func AssertGoldenBuffer(t THelper, buf []byte) {
	t.Helper()
	path := goldenPath(t, ".bin")

	if *Update {
		if err := os.MkdirAll(GoldenDir, 0o750); err != nil {
			t.Fatalf("golden: mkdir %s: %v", GoldenDir, err)
			return
		}
		if err := os.WriteFile(path, buf, 0o644); err != nil {
			t.Fatalf("golden: write %s: %v", path, err)
		}
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("golden: read %s: %v (run with -update to create)", path, err)
		return
	}
	if !bytes.Equal(buf, want) {
		t.Errorf("golden: buffer mismatch for %s", path)
	}
}

// AssertGoldenPNG compares img against the golden PNG file
// testdata/<TestName>.png. With -update, it writes img as a PNG to that file
// instead.
//
// The comparison is pixel by pixel, not byte by byte. Go's PNG encoder is not
// byte-stable across releases (1.27 emits three fewer bytes than 1.26 for the
// clock widget's golden, pixel for pixel identical), so comparing the encoded
// bytes would pin the encoder rather than the render and fail on every
// toolchain bump. Golden files stay readable PNGs either way.
func AssertGoldenPNG(t THelper, img image.Image) {
	t.Helper()
	path := goldenPath(t, ".png")

	if *Update {
		if err := os.MkdirAll(GoldenDir, 0o750); err != nil {
			t.Fatalf("golden: mkdir %s: %v", GoldenDir, err)
			return
		}
		f, err := os.Create(path)
		if err != nil {
			t.Fatalf("golden: create %s: %v", path, err)
			return
		}
		defer f.Close()
		if err := GoldenEncodePNG(f, img); err != nil {
			t.Fatalf("golden: encode PNG %s: %v", path, err)
		}
		return
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("golden: read %s: %v (run with -update to create)", path, err)
		return
	}

	want, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("golden: decode %s: %v", path, err)
		return
	}

	if got, wantB := img.Bounds(), want.Bounds(); got != wantB {
		t.Errorf("golden: bounds %v, want %v for %s", got, wantB, path)
		return
	}
	if x, y, ok := firstPixelDiff(img, want); !ok {
		t.Errorf("golden: PNG mismatch for %s (first differing pixel at %d,%d)", path, x, y)
	}
}

// firstPixelDiff reports the first pixel where got and want disagree, scanning
// in row-major order. ok is true when every pixel matches. Colors are compared
// through RGBA() so a paletted image and the color model a decoded PNG happens
// to use still compare equal.
func firstPixelDiff(got, want image.Image) (x, y int, ok bool) {
	b := got.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			gr, gg, gb, ga := got.At(x, y).RGBA()
			wr, wg, wb, wa := want.At(x, y).RGBA()
			if gr != wr || gg != wg || gb != wb || ga != wa {
				return x, y, false
			}
		}
	}
	return 0, 0, true
}
