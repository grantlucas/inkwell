package inkwell

import (
	"image"
	"image/color"
	"testing"
	"time"
)

// fixedClock returns a time source that always returns t.
func fixedClock(t time.Time) func() time.Time {
	return func() time.Time { return t }
}

func clockTestProfile() *DisplayProfile {
	return &DisplayProfile{
		Name:   "clock-test",
		Width:  200,
		Height: 100,
		Color:  BW,
	}
}

func TestClockWidget_BoundsReturnsConfiguredRect(t *testing.T) {
	bounds := image.Rect(10, 20, 110, 60)
	w := NewClockWidget(bounds, fixedClock(time.Time{}))

	if got := w.Bounds(); got != bounds {
		t.Errorf("Bounds() = %v, want %v", got, bounds)
	}
}

func TestClockWidget_RenderDrawsNonBlankOutput(t *testing.T) {
	bounds := image.Rect(0, 0, 200, 50)
	clk := fixedClock(time.Date(2024, 1, 1, 14, 30, 0, 0, time.UTC))
	w := NewClockWidget(bounds, clk)

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

func TestClockWidget_DifferentTimesProduceDifferentOutput(t *testing.T) {
	bounds := image.Rect(0, 0, 200, 50)
	palette := color.Palette{color.White, color.Black}

	render := func(hour, minute int) []uint8 {
		clk := fixedClock(time.Date(2024, 1, 1, hour, minute, 0, 0, time.UTC))
		w := NewClockWidget(bounds, clk)
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

func TestClockWidget_GoldenFile(t *testing.T) {
	bounds := image.Rect(0, 0, 200, 50)
	clk := fixedClock(time.Date(2024, 1, 1, 14, 30, 0, 0, time.UTC))
	w := NewClockWidget(bounds, clk)

	frame := image.NewPaletted(
		image.Rect(0, 0, 200, 50),
		color.Palette{color.White, color.Black},
	)
	if err := w.Render(frame); err != nil {
		t.Fatalf("Render: %v", err)
	}

	AssertGoldenPNG(t, frame)
}
