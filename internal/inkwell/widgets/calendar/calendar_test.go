package calendar

import (
	"fmt"
	"image"
	"testing"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/widget"
)

func TestFactory_ValidConfig(t *testing.T) {
	bounds := image.Rect(0, 0, 400, 300)
	config := map[string]any{
		"view": "today",
		"feeds": []any{
			"https://example.com/cal.ics",
		},
	}
	deps := widget.Deps{
		Now:         func() time.Time { return time.Date(2026, 4, 25, 9, 0, 0, 0, time.UTC) },
		DataSources: map[string]any{"http_client": &stubHTTPClient{}},
	}

	w, err := Factory(bounds, config, deps)
	if err != nil {
		t.Fatalf("Factory: %v", err)
	}
	if w.Bounds() != bounds {
		t.Errorf("Bounds = %v, want %v", w.Bounds(), bounds)
	}
}

func TestFactory_AllViewModes(t *testing.T) {
	for _, mode := range []string{"month", "week", "today", "upcoming"} {
		t.Run(mode, func(t *testing.T) {
			config := map[string]any{
				"view":  mode,
				"feeds": []any{"https://example.com/cal.ics"},
			}
			deps := widget.Deps{
				Now:         time.Now,
				DataSources: map[string]any{"http_client": &stubHTTPClient{}},
			}
			_, err := Factory(image.Rect(0, 0, 400, 300), config, deps)
			if err != nil {
				t.Fatalf("Factory(%q): %v", mode, err)
			}
		})
	}
}

func TestFactory_MissingView(t *testing.T) {
	config := map[string]any{
		"feeds": []any{"https://example.com/cal.ics"},
	}
	deps := widget.Deps{Now: time.Now}
	_, err := Factory(image.Rect(0, 0, 400, 300), config, deps)
	if err == nil {
		t.Fatal("expected error for missing view")
	}
}

func TestFactory_InvalidView(t *testing.T) {
	config := map[string]any{
		"view":  "quarterly",
		"feeds": []any{"https://example.com/cal.ics"},
	}
	deps := widget.Deps{Now: time.Now}
	_, err := Factory(image.Rect(0, 0, 400, 300), config, deps)
	if err == nil {
		t.Fatal("expected error for invalid view")
	}
}

func TestFactory_ViewWrongType(t *testing.T) {
	config := map[string]any{
		"view":  123,
		"feeds": []any{"https://example.com/cal.ics"},
	}
	deps := widget.Deps{Now: time.Now}
	_, err := Factory(image.Rect(0, 0, 400, 300), config, deps)
	if err == nil {
		t.Fatal("expected error for wrong type view")
	}
}

func TestFactory_MissingFeeds(t *testing.T) {
	config := map[string]any{
		"view": "today",
	}
	deps := widget.Deps{Now: time.Now}
	_, err := Factory(image.Rect(0, 0, 400, 300), config, deps)
	if err == nil {
		t.Fatal("expected error for missing feeds")
	}
}

func TestFactory_EmptyFeeds(t *testing.T) {
	config := map[string]any{
		"view":  "today",
		"feeds": []any{},
	}
	deps := widget.Deps{Now: time.Now}
	_, err := Factory(image.Rect(0, 0, 400, 300), config, deps)
	if err == nil {
		t.Fatal("expected error for empty feeds")
	}
}

func TestFactory_FeedsWrongType(t *testing.T) {
	config := map[string]any{
		"view":  "today",
		"feeds": "not-a-list",
	}
	deps := widget.Deps{Now: time.Now}
	_, err := Factory(image.Rect(0, 0, 400, 300), config, deps)
	if err == nil {
		t.Fatal("expected error for wrong type feeds")
	}
}

func TestFactory_FeedEntryWrongType(t *testing.T) {
	config := map[string]any{
		"view":  "today",
		"feeds": []any{123},
	}
	deps := widget.Deps{Now: time.Now}
	_, err := Factory(image.Rect(0, 0, 400, 300), config, deps)
	if err == nil {
		t.Fatal("expected error for non-string feed entry")
	}
}

func TestFactory_RefreshDuration(t *testing.T) {
	config := map[string]any{
		"view":    "today",
		"feeds":   []any{"https://example.com/cal.ics"},
		"refresh": "30m",
	}
	deps := widget.Deps{
		Now:         time.Now,
		DataSources: map[string]any{"http_client": &stubHTTPClient{}},
	}
	_, err := Factory(image.Rect(0, 0, 400, 300), config, deps)
	if err != nil {
		t.Fatalf("Factory with refresh: %v", err)
	}
}

func TestFactory_RefreshTooShort(t *testing.T) {
	config := map[string]any{
		"view":    "today",
		"feeds":   []any{"https://example.com/cal.ics"},
		"refresh": "30s",
	}
	deps := widget.Deps{Now: time.Now}
	_, err := Factory(image.Rect(0, 0, 400, 300), config, deps)
	if err == nil {
		t.Fatal("expected error for refresh < 1m")
	}
}

func TestFactory_RefreshWrongType(t *testing.T) {
	config := map[string]any{
		"view":    "today",
		"feeds":   []any{"https://example.com/cal.ics"},
		"refresh": 123,
	}
	deps := widget.Deps{Now: time.Now}
	_, err := Factory(image.Rect(0, 0, 400, 300), config, deps)
	if err == nil {
		t.Fatal("expected error for non-string refresh")
	}
}

func TestFactory_RefreshInvalid(t *testing.T) {
	config := map[string]any{
		"view":    "today",
		"feeds":   []any{"https://example.com/cal.ics"},
		"refresh": "not-a-duration",
	}
	deps := widget.Deps{Now: time.Now}
	_, err := Factory(image.Rect(0, 0, 400, 300), config, deps)
	if err == nil {
		t.Fatal("expected error for invalid refresh duration")
	}
}

func TestFactory_WeekStart(t *testing.T) {
	for _, ws := range []string{"monday", "sunday"} {
		t.Run(ws, func(t *testing.T) {
			config := map[string]any{
				"view":       "week",
				"feeds":      []any{"https://example.com/cal.ics"},
				"week_start": ws,
			}
			deps := widget.Deps{
				Now:         time.Now,
				DataSources: map[string]any{"http_client": &stubHTTPClient{}},
			}
			_, err := Factory(image.Rect(0, 0, 400, 300), config, deps)
			if err != nil {
				t.Fatalf("Factory(week_start=%q): %v", ws, err)
			}
		})
	}
}

func TestFactory_WeekStartInvalid(t *testing.T) {
	config := map[string]any{
		"view":       "week",
		"feeds":      []any{"https://example.com/cal.ics"},
		"week_start": "wednesday",
	}
	deps := widget.Deps{Now: time.Now}
	_, err := Factory(image.Rect(0, 0, 400, 300), config, deps)
	if err == nil {
		t.Fatal("expected error for invalid week_start")
	}
}

func TestFactory_WeekStartWrongType(t *testing.T) {
	config := map[string]any{
		"view":       "week",
		"feeds":      []any{"https://example.com/cal.ics"},
		"week_start": 1,
	}
	deps := widget.Deps{Now: time.Now}
	_, err := Factory(image.Rect(0, 0, 400, 300), config, deps)
	if err == nil {
		t.Fatal("expected error for non-string week_start")
	}
}

func TestFactory_MaxEvents(t *testing.T) {
	config := map[string]any{
		"view":       "today",
		"feeds":      []any{"https://example.com/cal.ics"},
		"max_events": 5,
	}
	deps := widget.Deps{
		Now:         time.Now,
		DataSources: map[string]any{"http_client": &stubHTTPClient{}},
	}
	_, err := Factory(image.Rect(0, 0, 400, 300), config, deps)
	if err != nil {
		t.Fatalf("Factory with max_events: %v", err)
	}
}

func TestFactory_MaxEventsInvalid(t *testing.T) {
	config := map[string]any{
		"view":       "today",
		"feeds":      []any{"https://example.com/cal.ics"},
		"max_events": 0,
	}
	deps := widget.Deps{Now: time.Now}
	_, err := Factory(image.Rect(0, 0, 400, 300), config, deps)
	if err == nil {
		t.Fatal("expected error for max_events = 0")
	}
}

func TestFactory_MaxEventsWrongType(t *testing.T) {
	config := map[string]any{
		"view":       "today",
		"feeds":      []any{"https://example.com/cal.ics"},
		"max_events": "five",
	}
	deps := widget.Deps{Now: time.Now}
	_, err := Factory(image.Rect(0, 0, 400, 300), config, deps)
	if err == nil {
		t.Fatal("expected error for non-int max_events")
	}
}

func TestFactory_NilNow(t *testing.T) {
	config := map[string]any{
		"view":  "today",
		"feeds": []any{"https://example.com/cal.ics"},
	}
	deps := widget.Deps{} // Now is nil
	_, err := Factory(image.Rect(0, 0, 400, 300), config, deps)
	if err != nil {
		t.Fatalf("Factory with nil Now: %v", err)
	}
}

func TestFactory_DefaultHTTPClient(t *testing.T) {
	config := map[string]any{
		"view":  "today",
		"feeds": []any{"https://example.com/cal.ics"},
	}
	// No DataSources — should use default http client.
	deps := widget.Deps{Now: time.Now}
	_, err := Factory(image.Rect(0, 0, 400, 300), config, deps)
	if err != nil {
		t.Fatalf("Factory without http_client: %v", err)
	}
}

func TestFactory_Title(t *testing.T) {
	config := map[string]any{
		"view":  "today",
		"feeds": []any{"https://example.com/cal.ics"},
		"title": "My Calendar",
	}
	deps := widget.Deps{
		Now:         time.Now,
		DataSources: map[string]any{"http_client": &stubHTTPClient{}},
	}
	_, err := Factory(image.Rect(0, 0, 400, 300), config, deps)
	if err != nil {
		t.Fatalf("Factory with title: %v", err)
	}
}

func TestFactory_TitleWrongType(t *testing.T) {
	config := map[string]any{
		"view":  "today",
		"feeds": []any{"https://example.com/cal.ics"},
		"title": 123,
	}
	deps := widget.Deps{Now: time.Now}
	_, err := Factory(image.Rect(0, 0, 400, 300), config, deps)
	if err == nil {
		t.Fatal("expected error for non-string title")
	}
}

func TestFactory_ShowLocation(t *testing.T) {
	config := map[string]any{
		"view":          "today",
		"feeds":         []any{"https://example.com/cal.ics"},
		"show_location": true,
	}
	deps := widget.Deps{
		Now:         time.Now,
		DataSources: map[string]any{"http_client": &stubHTTPClient{}},
	}
	_, err := Factory(image.Rect(0, 0, 400, 300), config, deps)
	if err != nil {
		t.Fatalf("Factory with show_location: %v", err)
	}
}

func TestWidget_RenderToday(t *testing.T) {
	frame := newTestFrame(400, 200)
	w := &Widget{
		bounds: frame.Bounds(),
		source: &staticSource{events: testEvents},
		now:    fixedClock,
		config: Config{
			View:      ViewToday,
			MaxEvents: 10,
			now:       fixedClock,
		},
	}
	if err := w.Render(frame); err != nil {
		t.Fatal(err)
	}
	if !hasBlackPixels(frame, frame.Bounds()) {
		t.Error("rendered blank frame")
	}
}

func TestWidget_RenderUpcoming(t *testing.T) {
	frame := newTestFrame(400, 300)
	w := &Widget{
		bounds: frame.Bounds(),
		source: &staticSource{events: upcomingEvents},
		now:    fixedClock,
		config: Config{
			View:      ViewUpcoming,
			MaxEvents: 10,
			WeekStart: time.Monday,
			now:       fixedClock,
		},
	}
	if err := w.Render(frame); err != nil {
		t.Fatal(err)
	}
}

func TestWidget_RenderWeek(t *testing.T) {
	frame := newTestFrame(400, 480)
	w := &Widget{
		bounds: frame.Bounds(),
		source: &staticSource{events: testEvents},
		now:    fixedClock,
		config: Config{
			View:      ViewWeek,
			MaxEvents: 10,
			WeekStart: time.Monday,
			now:       fixedClock,
		},
	}
	if err := w.Render(frame); err != nil {
		t.Fatal(err)
	}
}

func TestWidget_RenderMonth(t *testing.T) {
	frame := newTestFrame(400, 300)
	w := &Widget{
		bounds: frame.Bounds(),
		source: &staticSource{events: testEvents},
		now:    fixedClock,
		config: Config{
			View:      ViewMonth,
			MaxEvents: 10,
			WeekStart: time.Monday,
			now:       fixedClock,
		},
	}
	if err := w.Render(frame); err != nil {
		t.Fatal(err)
	}
}

func TestWidget_RenderUnknownView(t *testing.T) {
	frame := newTestFrame(400, 200)
	w := &Widget{
		bounds: frame.Bounds(),
		source: &staticSource{},
		now:    fixedClock,
		config: Config{
			View:      "invalid",
			MaxEvents: 10,
			now:       fixedClock,
		},
	}
	err := w.Render(frame)
	if err == nil {
		t.Fatal("expected error for unknown view")
	}
}

func TestWidget_RenderWithSourceError(t *testing.T) {
	frame := newTestFrame(400, 200)
	w := &Widget{
		bounds: frame.Bounds(),
		source: &staticSource{events: testEvents, err: fmt.Errorf("fetch failed")},
		now:    fixedClock,
		config: Config{
			View:      ViewToday,
			MaxEvents: 10,
			now:       fixedClock,
		},
	}
	// Should render stale data even on error.
	if err := w.Render(frame); err != nil {
		t.Fatal(err)
	}
}

func TestFactory_ShowLocationWrongType(t *testing.T) {
	config := map[string]any{
		"view":          "today",
		"feeds":         []any{"https://example.com/cal.ics"},
		"show_location": "yes",
	}
	deps := widget.Deps{Now: time.Now}
	_, err := Factory(image.Rect(0, 0, 400, 300), config, deps)
	if err == nil {
		t.Fatal("expected error for non-bool show_location")
	}
}
