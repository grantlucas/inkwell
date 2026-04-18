package widget_test

import (
	"image"
	"testing"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/widget"
)

// stubWidget implements widget.Widget for testing.
type stubWidget struct {
	bounds image.Rectangle
}

func (s *stubWidget) Bounds() image.Rectangle            { return s.bounds }
func (s *stubWidget) Render(_ *image.Paletted) error     { return nil }

func TestRegistry_EmptyNamePanics(t *testing.T) {
	r := widget.NewRegistry()
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on empty type name")
		}
	}()
	r.Register("", func(image.Rectangle, map[string]any, widget.Deps) (widget.Widget, error) {
		return nil, nil
	})
}

func TestRegistry_NilFactoryPanics(t *testing.T) {
	r := widget.NewRegistry()
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on nil factory")
		}
	}()
	r.Register("test", nil)
}

func TestRegistry_DuplicateRegisterPanics(t *testing.T) {
	r := widget.NewRegistry()
	dummy := widget.Factory(func(image.Rectangle, map[string]any, widget.Deps) (widget.Widget, error) {
		return nil, nil
	})
	r.Register("dup", dummy)

	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on duplicate registration")
		}
	}()
	r.Register("dup", dummy)
}

func TestRegistry_CreateUnknownType(t *testing.T) {
	r := widget.NewRegistry()
	_, err := r.Create("nonexistent", image.Rectangle{}, nil, widget.Deps{})
	if err == nil {
		t.Fatal("expected error for unknown widget type")
	}
	want := `unknown widget type: "nonexistent"`
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

func TestRegistry_RegisterAndCreate(t *testing.T) {
	r := widget.NewRegistry()
	r.Register("stub", func(bounds image.Rectangle, _ map[string]any, _ widget.Deps) (widget.Widget, error) {
		return &stubWidget{bounds: bounds}, nil
	})

	bounds := image.Rect(10, 20, 100, 200)
	w, err := r.Create("stub", bounds, nil, widget.Deps{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := w.Bounds(); got != bounds {
		t.Errorf("bounds = %v, want %v", got, bounds)
	}
}

func TestRegistry_CreatePassesConfigAndDeps(t *testing.T) {
	r := widget.NewRegistry()

	fixedTime := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	var gotConfig map[string]any
	var gotDeps widget.Deps
	var gotBounds image.Rectangle

	r.Register("spy", func(bounds image.Rectangle, config map[string]any, deps widget.Deps) (widget.Widget, error) {
		gotBounds = bounds
		gotConfig = config
		gotDeps = deps
		return &stubWidget{bounds: bounds}, nil
	})

	wantBounds := image.Rect(5, 10, 50, 60)
	wantConfig := map[string]any{"key": "value"}
	wantDeps := widget.Deps{Now: func() time.Time { return fixedTime }}

	_, err := r.Create("spy", wantBounds, wantConfig, wantDeps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotBounds != wantBounds {
		t.Errorf("bounds = %v, want %v", gotBounds, wantBounds)
	}
	if gotConfig["key"] != "value" {
		t.Errorf("config[key] = %v, want %q", gotConfig["key"], "value")
	}
	if gotDeps.Now == nil {
		t.Fatal("deps.Now was nil")
	}
	if got := gotDeps.Now(); got != fixedTime {
		t.Errorf("deps.Now() = %v, want %v", got, fixedTime)
	}
}
