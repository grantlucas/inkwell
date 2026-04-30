package separator

import (
	"image"
	"image/color"
	"testing"

	"github.com/grantlucas/inkwell/internal/inkwell/widget"
)

var palette = color.Palette{color.White, color.Black}

func TestWidget_Bounds(t *testing.T) {
	bounds := image.Rect(0, 50, 800, 52)
	w := New(bounds, 2)
	if got := w.Bounds(); got != bounds {
		t.Errorf("Bounds() = %v, want %v", got, bounds)
	}
}

func TestWidget_RenderDefault(t *testing.T) {
	bounds := image.Rect(0, 50, 800, 52)
	w := New(bounds, 2)

	frame := image.NewPaletted(image.Rect(0, 0, 800, 480), palette)
	if err := w.Render(frame); err != nil {
		t.Fatalf("Render: %v", err)
	}

	for x := bounds.Min.X; x < bounds.Max.X; x++ {
		if frame.ColorIndexAt(x, 50) != 1 {
			t.Fatalf("expected black at (%d, 50)", x)
		}
		if frame.ColorIndexAt(x, 51) != 1 {
			t.Fatalf("expected black at (%d, 51)", x)
		}
	}

	if frame.ColorIndexAt(0, 49) != 0 {
		t.Error("pixel above separator should be white")
	}
}

func TestWidget_RenderCustomThickness(t *testing.T) {
	bounds := image.Rect(0, 10, 100, 14)
	w := New(bounds, 3)

	frame := image.NewPaletted(image.Rect(0, 0, 100, 20), palette)
	if err := w.Render(frame); err != nil {
		t.Fatalf("Render: %v", err)
	}

	for y := 11; y <= 13; y++ {
		if frame.ColorIndexAt(50, y) != 1 {
			t.Errorf("expected black at (50, %d)", y)
		}
	}
	if frame.ColorIndexAt(50, 10) != 0 {
		t.Error("row 10 should be white (only 3 of 4 rows filled)")
	}
}

func TestWidget_RenderThicknessExceedsBounds(t *testing.T) {
	bounds := image.Rect(0, 0, 10, 2)
	w := New(bounds, 5)

	frame := image.NewPaletted(image.Rect(0, 0, 10, 10), palette)
	if err := w.Render(frame); err != nil {
		t.Fatalf("Render: %v", err)
	}

	for y := range 2 {
		if frame.ColorIndexAt(5, y) != 1 {
			t.Errorf("expected black at (5, %d)", y)
		}
	}
}

func TestFactory_Default(t *testing.T) {
	bounds := image.Rect(0, 50, 800, 52)
	w, err := Factory(bounds, nil, widget.Deps{})
	if err != nil {
		t.Fatalf("Factory: %v", err)
	}

	frame := image.NewPaletted(image.Rect(0, 0, 800, 480), palette)
	if err := w.Render(frame); err != nil {
		t.Fatalf("Render: %v", err)
	}
}

func TestFactory_CustomThicknessInt(t *testing.T) {
	_, err := Factory(image.Rect(0, 0, 100, 10), map[string]any{"thickness": 3}, widget.Deps{})
	if err != nil {
		t.Fatalf("Factory: %v", err)
	}
}

func TestFactory_CustomThicknessFloat(t *testing.T) {
	_, err := Factory(image.Rect(0, 0, 100, 10), map[string]any{"thickness": 3.0}, widget.Deps{})
	if err != nil {
		t.Fatalf("Factory: %v", err)
	}
}

func TestFactory_InvalidThicknessType(t *testing.T) {
	_, err := Factory(image.Rect(0, 0, 100, 10), map[string]any{"thickness": "big"}, widget.Deps{})
	if err == nil {
		t.Fatal("expected error for non-number thickness")
	}
}

func TestFactory_ZeroThickness(t *testing.T) {
	_, err := Factory(image.Rect(0, 0, 100, 10), map[string]any{"thickness": 0}, widget.Deps{})
	if err == nil {
		t.Fatal("expected error for zero thickness")
	}
}

func TestFactory_NegativeThickness(t *testing.T) {
	_, err := Factory(image.Rect(0, 0, 100, 10), map[string]any{"thickness": -1}, widget.Deps{})
	if err == nil {
		t.Fatal("expected error for negative thickness")
	}
}
