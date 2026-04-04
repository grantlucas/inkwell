package inkwell

import (
	"errors"
	"image"
"testing"
)

// fillWidget fills its bounds with black (palette index 1).
type fillWidget struct {
	bounds image.Rectangle
}

func (w *fillWidget) Bounds() image.Rectangle { return w.bounds }

func (w *fillWidget) Render(frame *image.Paletted) error {
	for y := w.bounds.Min.Y; y < w.bounds.Max.Y; y++ {
		for x := w.bounds.Min.X; x < w.bounds.Max.X; x++ {
			frame.SetColorIndex(x, y, 1) // black
		}
	}
	return nil
}

// errorWidget always returns an error from Render.
type errorWidget struct {
	bounds image.Rectangle
	err    error
}

func (w *errorWidget) Bounds() image.Rectangle { return w.bounds }

func (w *errorWidget) Render(_ *image.Paletted) error { return w.err }

func TestCompositor_ZeroWidgets(t *testing.T) {
	p := imageTestProfile()
	comp := NewCompositor(p)

	frame, err := comp.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	// Entire frame should be white (palette index 0)
	for y := 0; y < p.Height; y++ {
		for x := 0; x < p.Width; x++ {
			if ci := frame.ColorIndexAt(x, y); ci != 0 {
				t.Fatalf("pixel (%d,%d): got index %d, want 0 (white)", x, y, ci)
			}
		}
	}

	// Verify dimensions
	if frame.Bounds().Dx() != p.Width || frame.Bounds().Dy() != p.Height {
		t.Errorf("frame size = %dx%d, want %dx%d",
			frame.Bounds().Dx(), frame.Bounds().Dy(), p.Width, p.Height)
	}

	// Verify palette
	if len(frame.Palette) != 2 {
		t.Fatalf("palette length = %d, want 2", len(frame.Palette))
	}
	r, g, b, _ := frame.Palette[0].RGBA()
	if r != 0xFFFF || g != 0xFFFF || b != 0xFFFF {
		t.Errorf("palette[0] = (%d,%d,%d), want white", r, g, b)
	}
}

func TestCompositor_OneWidget(t *testing.T) {
	p := imageTestProfile()
	comp := NewCompositor(p)

	widgetBounds := image.Rect(2, 2, 6, 6) // 4x4 block
	comp.AddWidget(&fillWidget{bounds: widgetBounds})

	frame, err := comp.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	// Inside widget bounds should be black
	for y := 2; y < 6; y++ {
		for x := 2; x < 6; x++ {
			if ci := frame.ColorIndexAt(x, y); ci != 1 {
				t.Errorf("pixel (%d,%d): got index %d, want 1 (black)", x, y, ci)
			}
		}
	}

	// Outside widget bounds should be white
	if ci := frame.ColorIndexAt(0, 0); ci != 0 {
		t.Errorf("pixel (0,0): got index %d, want 0 (white)", ci)
	}
	if ci := frame.ColorIndexAt(7, 7); ci != 0 {
		t.Errorf("pixel (7,7): got index %d, want 0 (white)", ci)
	}
}

func TestCompositor_TwoWidgets(t *testing.T) {
	p := imageTestProfile()
	comp := NewCompositor(p)

	// Two non-overlapping widgets
	comp.AddWidget(&fillWidget{bounds: image.Rect(0, 0, 4, 4)})   // top-left
	comp.AddWidget(&fillWidget{bounds: image.Rect(8, 8, 12, 12)}) // bottom-right

	frame, err := comp.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	// First widget region should be black
	if ci := frame.ColorIndexAt(0, 0); ci != 1 {
		t.Errorf("widget1 pixel (0,0): got index %d, want 1", ci)
	}
	if ci := frame.ColorIndexAt(3, 3); ci != 1 {
		t.Errorf("widget1 pixel (3,3): got index %d, want 1", ci)
	}

	// Second widget region should be black
	if ci := frame.ColorIndexAt(8, 8); ci != 1 {
		t.Errorf("widget2 pixel (8,8): got index %d, want 1", ci)
	}
	if ci := frame.ColorIndexAt(11, 11); ci != 1 {
		t.Errorf("widget2 pixel (11,11): got index %d, want 1", ci)
	}

	// Gap between widgets should be white
	if ci := frame.ColorIndexAt(5, 5); ci != 0 {
		t.Errorf("gap pixel (5,5): got index %d, want 0 (white)", ci)
	}
}

func TestCompositor_ErrorPropagation(t *testing.T) {
	p := imageTestProfile()
	comp := NewCompositor(p)

	expectedErr := errors.New("widget render failed")
	comp.AddWidget(&errorWidget{
		bounds: image.Rect(0, 0, 4, 4),
		err:    expectedErr,
	})

	_, err := comp.Render()
	if err == nil {
		t.Fatal("expected error from Render, got nil")
	}
	if !errors.Is(err, expectedErr) {
		t.Errorf("error = %v, want %v", err, expectedErr)
	}
}
