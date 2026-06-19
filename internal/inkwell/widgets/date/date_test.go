package date

import (
	"image"
	"strings"
	"testing"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/fonts"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
)

func fixedClock(t time.Time) func() time.Time {
	return func() time.Time { return t }
}

func TestWidget_Bounds(t *testing.T) {
	bounds := image.Rect(0, 0, 800, 50)
	w := New(bounds, fixedClock(time.Now()), "Monday, January 2")
	if got := w.Bounds(); got != bounds {
		t.Errorf("Bounds() = %v, want %v", got, bounds)
	}
}

func TestWidget_Render(t *testing.T) {
	bounds := image.Rect(0, 0, 800, 50)
	clk := fixedClock(time.Date(2026, 4, 27, 14, 30, 0, 0, time.UTC))
	w := New(bounds, clk, "Monday, January 2")

	frame := image.NewPaletted(bounds, widget.PaperPalette)
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

func TestFactory_Default(t *testing.T) {
	clk := fixedClock(time.Date(2026, 4, 27, 14, 30, 0, 0, time.UTC))
	deps := widget.Deps{Now: clk}
	bounds := image.Rect(0, 0, 800, 50)

	w, err := Factory(bounds, nil, deps)
	if err != nil {
		t.Fatalf("Factory: %v", err)
	}

	frame := image.NewPaletted(bounds, widget.PaperPalette)
	if err := w.Render(frame); err != nil {
		t.Fatalf("Render: %v", err)
	}
}

func TestFactory_CustomFormat(t *testing.T) {
	deps := widget.Deps{Now: fixedClock(time.Now())}
	_, err := Factory(image.Rect(0, 0, 800, 50), map[string]any{"format": "Jan 2"}, deps)
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

func TestFactory_NilNow(t *testing.T) {
	deps := widget.Deps{}
	w, err := Factory(image.Rect(0, 0, 800, 50), nil, deps)
	if err != nil {
		t.Fatalf("Factory: %v", err)
	}
	frame := image.NewPaletted(image.Rect(0, 0, 800, 50), widget.PaperPalette)
	if err := w.Render(frame); err != nil {
		t.Fatalf("Render: %v", err)
	}
}

// mustLoadDateFace runs at package init with valid embedded fonts.
// Pin its panic branch by swapping in bad TTF data and re-invoking
// it directly.
func TestMustLoadDateFace_PanicsOnFontError(t *testing.T) {
	restore := fonts.SwapDataForTest([]byte("bad"), []byte("bad"))
	defer restore()

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic from mustLoadDateFace")
		}
		msg, ok := r.(string)
		if !ok || !strings.Contains(msg, "date: load font") {
			t.Errorf("panic = %v, want a string mentioning 'date: load font'", r)
		}
	}()
	_ = mustLoadDateFace()
}

// Direct callers of New must also survive a nil clock; the constructor
// defaults to time.Now rather than crashing the first Render call.
func TestNew_NilNow(t *testing.T) {
	w := New(image.Rect(0, 0, 800, 50), nil, "Monday")
	frame := image.NewPaletted(image.Rect(0, 0, 800, 50), widget.PaperPalette)
	if err := w.Render(frame); err != nil {
		t.Fatalf("Render: %v", err)
	}
}

func TestWidget_RefreshEvery(t *testing.T) {
	w := New(image.Rect(0, 0, 10, 10), time.Now, "Monday")
	if got := w.RefreshEvery(); got != 24*time.Hour {
		t.Errorf("RefreshEvery() = %v, want 24h", got)
	}
}
