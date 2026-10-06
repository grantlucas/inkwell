package widgets_test

import (
	"image"
	"slices"
	"testing"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar"
	"github.com/grantlucas/inkwell/internal/inkwell/calendar/testcal"
	"github.com/grantlucas/inkwell/internal/inkwell/testutil/fakehttp"
	"github.com/grantlucas/inkwell/internal/inkwell/weather"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets"
)

// typedDeps is what the app hands every widget: one calendar module and one
// weather provider, both fetching through tr.
func typedDeps(tr *fakehttp.Client, now func() time.Time) widget.Deps {
	return widget.Deps{
		Now:      now,
		Calendar: calendar.NewProvider(tr, now),
		Weather:  weather.NewProvider(tr, time.Hour, now, weather.Settings{TempUnit: "C"}),
	}
}

// Every calendar widget showing one feed, on however many screens, causes
// one upstream request for it: the calendar module is shared, and no
// factory builds a cache of its own.
func TestDefaultRegistry_CalendarWidgetsShareOneFetchPerFeed(t *testing.T) {
	const url = "https://example.com/a.ics"
	now := func() time.Time { return time.Date(2026, 3, 16, 12, 0, 0, 0, time.UTC) }
	tr := fakehttp.New()
	tr.Serve(url, testcal.Generate(now()))
	deps := typedDeps(tr, now)
	feeds := map[string]any{"feeds": []any{url}}

	r := widgets.NewDefaultRegistry()
	for _, typeName := range []string{"bold-five", "day-timeline", "row-agenda", "today-hero", "weekly-calendar"} {
		bounds := image.Rect(0, 0, 800, 480)
		w, err := r.Create(typeName, bounds, feeds, deps)
		if err != nil {
			t.Fatalf("Create %s: %v", typeName, err)
		}
		if err := w.Render(image.NewPaletted(bounds, widget.PaperPalette)); err != nil {
			t.Fatalf("Render %s: %v", typeName, err)
		}
	}

	if got := tr.Requests(url); got != 1 {
		t.Errorf("upstream requests for the feed = %d, want 1", got)
	}
}

// Every registered widget builds from the typed dependencies the app hands
// out. The table must name every registered type, so a new widget can't be
// registered without proving it builds here.
func TestDefaultRegistry_BuildsEveryWidgetFromTypedDeps(t *testing.T) {
	now := func() time.Time { return time.Date(2026, 3, 16, 12, 0, 0, 0, time.UTC) }
	deps := typedDeps(fakehttp.New(), now)
	feeds := map[string]any{"feeds": []any{"https://example.com/a.ics"}}

	tests := []struct {
		typeName string
		bounds   image.Rectangle
		config   map[string]any
	}{
		{typeName: "bold-five", bounds: image.Rect(0, 0, 800, 480), config: feeds},
		{typeName: "clock", bounds: image.Rect(0, 0, 200, 50)},
		{typeName: "date", bounds: image.Rect(0, 0, 800, 52)},
		{typeName: "day-timeline", bounds: image.Rect(0, 48, 500, 480), config: feeds},
		{typeName: "fuzzy_clock", bounds: image.Rect(0, 0, 800, 50)},
		{typeName: "row-agenda", bounds: image.Rect(0, 0, 800, 480), config: feeds},
		{typeName: "separator", bounds: image.Rect(0, 0, 800, 2)},
		{typeName: "today-hero", bounds: image.Rect(0, 0, 800, 480), config: feeds},
		{typeName: "today-weather", bounds: image.Rect(534, 48, 800, 208)},
		{typeName: "weather-ahead", bounds: image.Rect(534, 208, 800, 480)},
		{typeName: "weekly-calendar", bounds: image.Rect(0, 52, 800, 480), config: feeds},
	}

	r := widgets.NewDefaultRegistry()
	var covered []string
	for _, tt := range tests {
		covered = append(covered, tt.typeName)
		t.Run(tt.typeName, func(t *testing.T) {
			w, err := r.Create(tt.typeName, tt.bounds, tt.config, deps)
			if err != nil {
				t.Fatalf("Create %s: %v", tt.typeName, err)
			}
			if got := w.Bounds(); got != tt.bounds {
				t.Errorf("Bounds = %v, want %v", got, tt.bounds)
			}
		})
	}

	if registered := r.Types(); !slices.Equal(covered, registered) {
		t.Errorf("table covers %v, registry has %v", covered, registered)
	}
}
