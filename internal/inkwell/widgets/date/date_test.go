package date

import (
	"image"
	"image/color"
	"testing"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/widget"
)

func fixedClock(t time.Time) func() time.Time {
	return func() time.Time { return t }
}

func TestWidget_Bounds(t *testing.T) {
	bounds := image.Rect(0, 0, 800, 52)
	w := New(bounds, fixedClock(time.Now()), "Monday, January 2", true)
	if got := w.Bounds(); got != bounds {
		t.Errorf("Bounds() = %v, want %v", got, bounds)
	}
}

func TestWidget_Render(t *testing.T) {
	bounds := image.Rect(0, 0, 800, 52)
	clk := fixedClock(time.Date(2026, 4, 27, 14, 30, 0, 0, time.UTC))
	w := New(bounds, clk, "Monday, January 2", true)

	frame := image.NewPaletted(bounds, color.Palette{color.White, color.Black})
	if err := w.Render(frame); err != nil {
		t.Fatalf("Render: %v", err)
	}

	hasBlack := false
	for _, px := range frame.Pix {
		if px != 0 {
			hasBlack = true
			break
		}
	}
	if !hasBlack {
		t.Error("rendered blank frame")
	}
}

func TestWidget_RenderNoBorder(t *testing.T) {
	bounds := image.Rect(0, 0, 800, 52)
	clk := fixedClock(time.Date(2026, 4, 27, 14, 30, 0, 0, time.UTC))
	w := New(bounds, clk, "Monday, January 2", false)

	frame := image.NewPaletted(bounds, color.Palette{color.White, color.Black})
	if err := w.Render(frame); err != nil {
		t.Fatalf("Render: %v", err)
	}

	bottomBlack := false
	for x := bounds.Min.X; x < bounds.Max.X; x++ {
		if frame.ColorIndexAt(x, bounds.Max.Y-1) == 1 {
			bottomBlack = true
			break
		}
	}
	if bottomBlack {
		t.Error("border drawn when border=false")
	}
}

func TestFactory_Default(t *testing.T) {
	clk := fixedClock(time.Date(2026, 4, 27, 14, 30, 0, 0, time.UTC))
	deps := widget.Deps{Now: clk}
	bounds := image.Rect(0, 0, 800, 52)

	w, err := Factory(bounds, nil, deps)
	if err != nil {
		t.Fatalf("Factory: %v", err)
	}

	frame := image.NewPaletted(bounds, color.Palette{color.White, color.Black})
	if err := w.Render(frame); err != nil {
		t.Fatalf("Render: %v", err)
	}
}

func TestFactory_CustomFormat(t *testing.T) {
	deps := widget.Deps{Now: fixedClock(time.Now())}
	_, err := Factory(image.Rect(0, 0, 800, 52), map[string]any{"format": "Jan 2"}, deps)
	if err != nil {
		t.Fatalf("Factory: %v", err)
	}
}

func TestFactory_InvalidFormat(t *testing.T) {
	deps := widget.Deps{}
	_, err := Factory(image.Rect(0, 0, 100, 50), map[string]any{"format": 123}, deps)
	if err == nil {
		t.Fatal("expected error for non-string format")
	}
}

func TestFactory_EmptyFormat(t *testing.T) {
	deps := widget.Deps{}
	_, err := Factory(image.Rect(0, 0, 100, 50), map[string]any{"format": ""}, deps)
	if err == nil {
		t.Fatal("expected error for empty format")
	}
}

func TestFactory_BorderConfig(t *testing.T) {
	deps := widget.Deps{Now: fixedClock(time.Now())}
	_, err := Factory(image.Rect(0, 0, 800, 52), map[string]any{"border": false}, deps)
	if err != nil {
		t.Fatalf("Factory: %v", err)
	}
}

func TestFactory_BorderInvalidType(t *testing.T) {
	deps := widget.Deps{}
	_, err := Factory(image.Rect(0, 0, 100, 50), map[string]any{"border": "yes"}, deps)
	if err == nil {
		t.Fatal("expected error for non-bool border")
	}
}

func TestFactory_NilNow(t *testing.T) {
	deps := widget.Deps{}
	w, err := Factory(image.Rect(0, 0, 800, 52), nil, deps)
	if err != nil {
		t.Fatalf("Factory: %v", err)
	}
	frame := image.NewPaletted(image.Rect(0, 0, 800, 52), color.Palette{color.White, color.Black})
	if err := w.Render(frame); err != nil {
		t.Fatalf("Render: %v", err)
	}
}
