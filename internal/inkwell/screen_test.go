package inkwell

import (
	"image"
	"testing"

	"github.com/grantlucas/inkwell/internal/inkwell/widget"
)

type stubWidget struct {
	bounds image.Rectangle
}

func (s *stubWidget) Bounds() image.Rectangle        { return s.bounds }
func (s *stubWidget) Render(_ *image.Paletted) error  { return nil }

func TestScreen_NameAndWidgets(t *testing.T) {
	w1 := &stubWidget{bounds: image.Rect(0, 0, 100, 50)}
	w2 := &stubWidget{bounds: image.Rect(100, 0, 200, 50)}
	s := NewScreen("main", []widget.Widget{w1, w2})

	if s.Name != "main" {
		t.Errorf("Name = %q, want main", s.Name)
	}
	widgets := s.Widgets()
	if len(widgets) != 2 {
		t.Fatalf("len(Widgets) = %d, want 2", len(widgets))
	}
	if widgets[0].Bounds() != w1.bounds {
		t.Errorf("Widgets[0].Bounds() = %v, want %v", widgets[0].Bounds(), w1.bounds)
	}
}
