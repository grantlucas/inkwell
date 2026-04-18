package widgets_test

import (
	"image"
	"testing"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets"
)

func TestDefaultRegistry_ContainsClock(t *testing.T) {
	r := widgets.NewDefaultRegistry()
	deps := widget.Deps{Now: func() time.Time {
		return time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	}}

	w, err := r.Create("clock", image.Rect(0, 0, 200, 50), nil, deps)
	if err != nil {
		t.Fatalf("Create clock: %v", err)
	}
	if w == nil {
		t.Fatal("expected non-nil widget")
	}
}
