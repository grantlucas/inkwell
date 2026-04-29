package clock

import (
	"bytes"
	"image"
	"image/color"
	"testing"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/testutil"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
)

// fixedClock returns a time source that always returns t.
func fixedClock(t time.Time) func() time.Time {
	return func() time.Time { return t }
}

func TestWidget_BoundsReturnsConfiguredRect(t *testing.T) {
	bounds := image.Rect(10, 20, 110, 60)
	w := New(bounds, fixedClock(time.Time{}), "15:04")

	if got := w.Bounds(); got != bounds {
		t.Errorf("Bounds() = %v, want %v", got, bounds)
	}
}

func TestWidget_RenderDrawsNonBlankOutput(t *testing.T) {
	bounds := image.Rect(0, 0, 200, 50)
	clk := fixedClock(time.Date(2024, 1, 1, 14, 30, 0, 0, time.UTC))
	w := New(bounds, clk, "15:04")

	frame := image.NewPaletted(
		image.Rect(0, 0, 200, 50),
		color.Palette{color.White, color.Black},
	)
	if err := w.Render(frame); err != nil {
		t.Fatalf("Render: %v", err)
	}

	// At least one pixel should be black (rendered text).
	hasBlack := false
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if frame.ColorIndexAt(x, y) != 0 {
				hasBlack = true
				break
			}
		}
		if hasBlack {
			break
		}
	}
	if !hasBlack {
		t.Error("Render produced a blank frame; expected some black pixels for the clock text")
	}
}

func TestWidget_DifferentTimesProduceDifferentOutput(t *testing.T) {
	bounds := image.Rect(0, 0, 200, 50)
	palette := color.Palette{color.White, color.Black}

	render := func(hour, minute int) []uint8 {
		clk := fixedClock(time.Date(2024, 1, 1, hour, minute, 0, 0, time.UTC))
		w := New(bounds, clk, "15:04")
		frame := image.NewPaletted(image.Rect(0, 0, 200, 50), palette)
		if err := w.Render(frame); err != nil {
			t.Fatalf("Render(%d:%02d): %v", hour, minute, err)
		}
		out := make([]uint8, len(frame.Pix))
		copy(out, frame.Pix)
		return out
	}

	pix1430 := render(14, 30)
	pix0000 := render(0, 0)

	same := true
	for i := range pix1430 {
		if pix1430[i] != pix0000[i] {
			same = false
			break
		}
	}
	if same {
		t.Error("14:30 and 00:00 produced identical frames; expected different output")
	}
}

func TestFactory_DefaultFormat(t *testing.T) {
	fixedTime := time.Date(2024, 1, 1, 14, 30, 0, 0, time.UTC)
	deps := widget.Deps{Now: fixedClock(fixedTime)}
	bounds := image.Rect(0, 0, 200, 50)

	w, err := Factory(bounds, nil, deps)
	if err != nil {
		t.Fatalf("Factory: %v", err)
	}

	frame := image.NewPaletted(bounds, color.Palette{color.White, color.Black})
	if err := w.Render(frame); err != nil {
		t.Fatalf("Render: %v", err)
	}

	// Should render something (non-blank).
	hasBlack := false
	for _, px := range frame.Pix {
		if px != 0 {
			hasBlack = true
			break
		}
	}
	if !hasBlack {
		t.Error("Factory-created widget rendered blank frame")
	}
}

func TestFactory_CustomFormat(t *testing.T) {
	fixedTime := time.Date(2024, 1, 1, 14, 30, 0, 0, time.UTC)
	deps := widget.Deps{Now: fixedClock(fixedTime)}
	bounds := image.Rect(0, 0, 200, 50)
	palette := color.Palette{color.White, color.Black}

	w, err := Factory(bounds, map[string]any{"format": "3:04 PM"}, deps)
	if err != nil {
		t.Fatalf("Factory: %v", err)
	}

	frame := image.NewPaletted(bounds, palette)
	if err := w.Render(frame); err != nil {
		t.Fatalf("Render: %v", err)
	}

	// Verify custom format produces different output than default.
	defaultWidget, err := Factory(bounds, nil, deps)
	if err != nil {
		t.Fatalf("Factory default: %v", err)
	}
	defaultFrame := image.NewPaletted(bounds, palette)
	if err := defaultWidget.Render(defaultFrame); err != nil {
		t.Fatalf("Render default: %v", err)
	}
	if bytes.Equal(frame.Pix, defaultFrame.Pix) {
		t.Fatal("custom format rendered identically to the default format")
	}

	hasBlack := false
	for _, px := range frame.Pix {
		if px != 0 {
			hasBlack = true
			break
		}
	}
	if !hasBlack {
		t.Error("Factory-created widget with custom format rendered blank frame")
	}
}

func TestFactory_InvalidFormatType(t *testing.T) {
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

func TestFactory_NilNowUsesDefault(t *testing.T) {
	deps := widget.Deps{} // Now is nil
	bounds := image.Rect(0, 0, 200, 50)

	w, err := Factory(bounds, nil, deps)
	if err != nil {
		t.Fatalf("Factory: %v", err)
	}
	// Just verify it doesn't panic on render.
	frame := image.NewPaletted(bounds, color.Palette{color.White, color.Black})
	if err := w.Render(frame); err != nil {
		t.Fatalf("Render: %v", err)
	}
}

func TestFactory_AlignRight(t *testing.T) {
	deps := widget.Deps{Now: fixedClock(time.Date(2024, 1, 1, 14, 30, 0, 0, time.UTC))}
	bounds := image.Rect(0, 0, 200, 50)

	w, err := Factory(bounds, map[string]any{"align": "right"}, deps)
	if err != nil {
		t.Fatalf("Factory: %v", err)
	}

	frame := image.NewPaletted(bounds, color.Palette{color.White, color.Black})
	if err := w.Render(frame); err != nil {
		t.Fatalf("Render: %v", err)
	}

	rightHalf := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Dx() / 2; x < bounds.Max.X; x++ {
			if frame.ColorIndexAt(x, y) == 1 {
				rightHalf++
			}
		}
	}
	if rightHalf == 0 {
		t.Error("right-aligned clock has no pixels in right half")
	}
}

func TestFactory_AlignLeft(t *testing.T) {
	deps := widget.Deps{Now: fixedClock(time.Date(2024, 1, 1, 14, 30, 0, 0, time.UTC))}
	bounds := image.Rect(0, 0, 200, 50)

	w, err := Factory(bounds, map[string]any{"align": "left"}, deps)
	if err != nil {
		t.Fatalf("Factory: %v", err)
	}

	frame := image.NewPaletted(bounds, color.Palette{color.White, color.Black})
	if err := w.Render(frame); err != nil {
		t.Fatalf("Render: %v", err)
	}

	leftHalf := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Dx()/2; x++ {
			if frame.ColorIndexAt(x, y) == 1 {
				leftHalf++
			}
		}
	}
	if leftHalf == 0 {
		t.Error("left-aligned clock has no pixels in left half")
	}
}

func TestFactory_AlignInvalid(t *testing.T) {
	deps := widget.Deps{}
	_, err := Factory(image.Rect(0, 0, 100, 50), map[string]any{"align": "middle"}, deps)
	if err == nil {
		t.Fatal("expected error for invalid align")
	}
}

func TestFactory_AlignCenter(t *testing.T) {
	deps := widget.Deps{Now: fixedClock(time.Date(2024, 1, 1, 14, 30, 0, 0, time.UTC))}
	bounds := image.Rect(0, 0, 200, 50)

	w, err := Factory(bounds, map[string]any{"align": "center"}, deps)
	if err != nil {
		t.Fatalf("Factory: %v", err)
	}

	frame := image.NewPaletted(bounds, color.Palette{color.White, color.Black})
	if err := w.Render(frame); err != nil {
		t.Fatalf("Render: %v", err)
	}
}

func TestFactory_AlignWrongType(t *testing.T) {
	deps := widget.Deps{}
	_, err := Factory(image.Rect(0, 0, 100, 50), map[string]any{"align": 42}, deps)
	if err == nil {
		t.Fatal("expected error for non-string align")
	}
}

func TestWidget_GoldenFile(t *testing.T) {
	bounds := image.Rect(0, 0, 200, 50)
	clk := fixedClock(time.Date(2024, 1, 1, 14, 30, 0, 0, time.UTC))
	w := New(bounds, clk, "15:04")

	frame := image.NewPaletted(
		image.Rect(0, 0, 200, 50),
		color.Palette{color.White, color.Black},
	)
	if err := w.Render(frame); err != nil {
		t.Fatalf("Render: %v", err)
	}

	testutil.AssertGoldenPNG(t, frame)
}
