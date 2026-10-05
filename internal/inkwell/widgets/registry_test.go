package widgets_test

import (
	"context"
	"image"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/weather"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets"
)

// offlineTransport answers every request with an error, so building a
// widget can never reach the network.
type offlineTransport struct{}

func (offlineTransport) Do(*http.Request) (*http.Response, error) {
	return nil, context.DeadlineExceeded
}

// Every registered widget builds from the typed dependencies the app hands
// out. The table must name every registered type, so a new widget can't be
// registered without proving it builds here.
func TestDefaultRegistry_BuildsEveryWidgetFromTypedDeps(t *testing.T) {
	now := func() time.Time { return time.Date(2026, 3, 16, 12, 0, 0, 0, time.UTC) }
	client := offlineTransport{}
	deps := widget.Deps{
		Now:        now,
		HTTPClient: client,
		Weather:    weather.NewProvider(client, time.Hour, now, weather.Settings{TempUnit: "C"}),
	}
	feeds := map[string]any{"feeds": []any{"https://example.com/a.ics"}}

	tests := []struct {
		typeName string
		bounds   image.Rectangle
		config   map[string]any
	}{
		{typeName: "bold-five", bounds: image.Rect(0, 0, 800, 480), config: feeds},
		{typeName: "clock", bounds: image.Rect(0, 0, 200, 50)},
		{typeName: "date", bounds: image.Rect(0, 0, 800, 52)},
		{typeName: "fuzzy_clock", bounds: image.Rect(0, 0, 800, 50)},
		{typeName: "row-agenda", bounds: image.Rect(0, 0, 800, 480), config: feeds},
		{typeName: "separator", bounds: image.Rect(0, 0, 800, 2)},
		{typeName: "today-hero", bounds: image.Rect(0, 0, 800, 480), config: feeds},
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
