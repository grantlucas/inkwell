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

	// Multi-row separator: top edge is crisp PaperBlack so the 1-px
	// stroke survives dithering, and the interior fills in with
	// PaperGray40 so the band reads as a soft hairline overall.
	for x := bounds.Min.X; x < bounds.Max.X; x++ {
		if got := frame.ColorIndexAt(x, 51); got != widget.PaperGray40 {
			t.Fatalf("base row pixel (%d, 51): got %d, want %d (PaperGray40)", x, got, widget.PaperGray40)
		}
		if got := frame.ColorIndexAt(x, 50); got != widget.PaperBlack {
			t.Fatalf("top row pixel (%d, 50): got %d, want %d (PaperBlack)", x, got, widget.PaperBlack)
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

	// Interior rows render at PaperGray40; only the topmost row gets
	// the device-safe PaperBlack accent.
	for _, y := range []int{12, 13} {
		if got := frame.ColorIndexAt(50, y); got != widget.PaperGray40 {
			t.Errorf("base row (50, %d): got %d, want %d (PaperGray40)", y, got, widget.PaperGray40)
		}
	}
	if got := frame.ColorIndexAt(50, 11); got != widget.PaperBlack {
		t.Errorf("top row (50, 11): got %d, want %d (PaperBlack)", got, widget.PaperBlack)
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

	// Within bounds (rows 0 and 1) the separator still draws as
	// black top edge + gray interior.
	if got := frame.ColorIndexAt(5, 1); got != widget.PaperGray40 {
		t.Errorf("base row (5,1): got %d, want %d (PaperGray40)", got, widget.PaperGray40)
	}
	if got := frame.ColorIndexAt(5, 0); got != widget.PaperBlack {
		t.Errorf("top row (5,0): got %d, want %d (PaperBlack)", got, widget.PaperBlack)
	}
}

func TestWidget_RenderSingleRowUsesBlack(t *testing.T) {
	// A 1-px separator must be PaperBlack — flat gray dithers into a
	// dashed dotted line on the e-paper device.
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
