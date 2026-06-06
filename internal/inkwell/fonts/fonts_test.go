package fonts

import (
	"strings"
	"testing"
)

func TestFace_Regular(t *testing.T) {
	f, err := Face(Regular, 10)
	if err != nil {
		t.Fatalf("Face(Regular, 10): %v", err)
	}
	if f == nil {
		t.Fatal("Face returned nil")
	}
}

func TestFace_SemiBold(t *testing.T) {
	f, err := Face(SemiBold, 10)
	if err != nil {
		t.Fatalf("Face(SemiBold, 10): %v", err)
	}
	if f == nil {
		t.Fatal("Face returned nil")
	}
}

func TestFace_Bold(t *testing.T) {
	f, err := Face(Bold, 16)
	if err != nil {
		t.Fatalf("Face(Bold, 16): %v", err)
	}
	if f == nil {
		t.Fatal("Face returned nil")
	}
}

func TestFace_MultipleSizes(t *testing.T) {
	sizes := []float64{9, 10, 12, 14, 16, 18}
	for _, sz := range sizes {
		f, err := Face(Regular, sz)
		if err != nil {
			t.Errorf("Face(Regular, %v): %v", sz, err)
		}
		if f == nil {
			t.Errorf("Face(Regular, %v) returned nil", sz)
		}
	}
}

// When the embedded TTF data fails to parse, Face must surface the
// parse error rather than silently returning a zero face. Swap in
// garbage data, force a re-parse, and assert the error propagates.
func TestFace_ParseError(t *testing.T) {
	restore := SwapDataForTest([]byte("not a font"), []byte("not a font"))
	defer restore()

	_, err := Face(Regular, 10)
	if err == nil {
		t.Fatal("expected parse error")
	}
	if want := "parse font 0"; !strings.Contains(err.Error(), want) {
		t.Errorf("error = %q, want it to mention %q", err.Error(), want)
	}

	// A second Face call must return the cached error rather than
	// re-running the (already broken) parse.
	if _, err2 := Face(SemiBold, 12); err2 == nil {
		t.Fatal("expected cached parse error on second call")
	}
}

// If only the second font (Bold) fails to parse, parseFonts must
// stop at index 1 and surface that index in the error message.
func TestFace_ParseError_SecondFont(t *testing.T) {
	restore := SwapDataForTest(terminusRegularTTF, []byte("not a font"))
	defer restore()

	_, err := Face(Bold, 16)
	if err == nil {
		t.Fatal("expected parse error")
	}
	if want := "parse font 1"; !strings.Contains(err.Error(), want) {
		t.Errorf("error = %q, want it to mention %q", err.Error(), want)
	}
}

func TestFace_HasDegreeSymbol(t *testing.T) {
	f, err := Face(Regular, 10)
	if err != nil {
		t.Fatalf("Face: %v", err)
	}
	for _, r := range "°" {
		_, _, ok := f.GlyphBounds(r)
		if !ok {
			t.Error("font does not contain degree symbol (°)")
		}
	}
}
