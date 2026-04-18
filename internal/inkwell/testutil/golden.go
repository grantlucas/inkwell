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

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("golden: read %s: %v (run with -update to create)", path, err)
		return
	}

	var gotBuf bytes.Buffer
	if err := GoldenEncodePNG(&gotBuf, img); err != nil {
		t.Fatalf("golden: encode PNG: %v", err)
		return
	}
	if !bytes.Equal(gotBuf.Bytes(), want) {
		t.Errorf("golden: PNG mismatch for %s", path)
	}
}
