package separator

import (
	"image"
	"testing"

	"github.com/grantlucas/inkwell/internal/inkwell/widget"
)

var palette = widget.PaperPalette

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

	// All separator rows are solid PaperBlack now that the device path
	// is pure threshold — there's no halftone interior to render any
	// gray as a "soft" hairline, so the whole band is black.
	for x := bounds.Min.X; x < bounds.Max.X; x++ {
		for _, y := range []int{50, 51} {
			if got := frame.ColorIndexAt(x, y); got != widget.PaperBlack {
				t.Fatalf("row pixel (%d, %d): got %d, want %d (PaperBlack)", x, y, got, widget.PaperBlack)
			}
		}
	}

	if frame.ColorIndexAt(0, 49) != widget.PaperWhite {
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

	for _, y := range []int{11, 12, 13} {
		if got := frame.ColorIndexAt(50, y); got != widget.PaperBlack {
			t.Errorf("row (50, %d): got %d, want %d (PaperBlack)", y, got, widget.PaperBlack)
		}
	}
	if frame.ColorIndexAt(50, 10) != widget.PaperWhite {
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

	// Within bounds (rows 0 and 1) every row is solid PaperBlack.
	for _, y := range []int{0, 1} {
		if got := frame.ColorIndexAt(5, y); got != widget.PaperBlack {
			t.Errorf("row (5,%d): got %d, want %d (PaperBlack)", y, got, widget.PaperBlack)
		}
	}
}

func TestWidget_RenderSingleRowUsesBlack(t *testing.T) {
	bounds := image.Rect(0, 5, 10, 6)
	w := New(bounds, 1)

	frame := image.NewPaletted(image.Rect(0, 0, 10, 10), palette)
	if err := w.Render(frame); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if got := frame.ColorIndexAt(5, 5); got != widget.PaperBlack {
		t.Errorf("hairline pixel (5,5): got %d, want %d (PaperBlack)", got, widget.PaperBlack)
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
