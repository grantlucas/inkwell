package fonts

import (
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
