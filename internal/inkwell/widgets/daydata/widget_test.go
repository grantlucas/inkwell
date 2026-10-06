package daydata_test

import (
	"image"
	"testing"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daydata"
)

// dayWidget is the smallest day widget: the shared Base and nothing drawn.
type dayWidget struct{ daydata.Base[daydata.Config] }

func (dayWidget) Render(*image.Paletted) error { return nil }

func newDayWidget(bounds image.Rectangle, days daydata.Source, now func() time.Time, cfg daydata.Config) *dayWidget {
	return &dayWidget{daydata.NewBase(bounds, days, now, cfg)}
}

// A day widget's factory parses the shared settings against its spec,
// binds its day data to the dashboard's calendar module and weather
// provider, and hands it the dashboard's clock. A bad setting or a missing
// dependency, the clock included, stops it, naming the widget.
func TestFactory(t *testing.T) {
	now := time.Date(2026, 10, 5, 8, 0, 0, 0, mustZone(t, "America/Toronto"))
	s := newSeam(t, now)
	s.tr.Serve(feedA, ics(weeklySeries))
	noCalendar := s.deps
	noCalendar.Calendar = nil
	noClock := s.deps
	noClock.Now = nil
	factory := daydata.Factory("test-widget", daydata.Parser(daydata.Spec{Widget: "test-widget", MaxEvents: 3}), newDayWidget)
	bounds := image.Rect(10, 20, 410, 260)
	feeds := map[string]any{"feeds": []any{feedA}}

	tests := []struct {
		label   string
		raw     map[string]any
		deps    widget.Deps
		wantErr string
	}{
		{label: "builds from the dashboard's dependencies", raw: feeds, deps: s.deps},
		{label: "a bad setting", raw: map[string]any{}, deps: s.deps, wantErr: "test-widget: feeds is required"},
		{label: "a missing dependency", raw: feeds, deps: noCalendar, wantErr: "test-widget: no calendar module"},
		// A widget quietly reading the wall clock would disagree with the
		// rest of the panel about the display zone.
		{label: "no clock", raw: feeds, deps: noClock, wantErr: "test-widget: no clock"},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			w, err := factory(bounds, tt.raw, tt.deps)
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("err = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("factory: %v", err)
			}
			got := w.(*dayWidget)
			if got.Bounds() != bounds {
				t.Errorf("Bounds = %v, want %v", got.Bounds(), bounds)
			}
			if got.Config.MaxEvents != 3 || got.Config.Weather.TempUnit != "C" {
				t.Errorf("Config = %+v, want the spec's cap and the top-level unit", got.Config)
			}
			if !got.Now().Equal(now) {
				t.Errorf("Now() = %v, want the dashboard's clock", got.Now())
			}
			if g := dayEvents(got.Days.Days(now, 1))[0]; g != "Weekly Sync" {
				t.Errorf("Today lists %q, want the dashboard calendar's events", g)
			}
		})
	}
}

// A widget's factory builds its day data with the options the widget
// asks for, so a widget that draws no forecast fetches none.
func TestFactory_PassesItsOptions(t *testing.T) {
	now := time.Date(2026, 10, 5, 8, 0, 0, 0, mustZone(t, "America/Toronto"))
	s := newSeam(t, now)
	s.tr.Serve(feedA, ics(weeklySeries))
	spec := daydata.Parser(daydata.Spec{Widget: "test-widget", MaxEvents: 3})
	raw := map[string]any{"feeds": []any{feedA}}

	for _, tt := range []struct {
		label       string
		opts        []daydata.Option
		wantArrived bool
	}{
		{label: "no options fetch the forecast", wantArrived: true},
		{label: "without weather fetches none", opts: []daydata.Option{daydata.WithoutWeather()}},
	} {
		t.Run(tt.label, func(t *testing.T) {
			w, err := daydata.Factory("test-widget", spec, newDayWidget, tt.opts...)(image.Rect(0, 0, 10, 10), raw, s.deps)
			if err != nil {
				t.Fatalf("factory: %v", err)
			}
			if got := w.(*dayWidget).Days.Days(now, 1).ForecastArrived; got != tt.wantArrived {
				t.Errorf("ForecastArrived = %v, want %v", got, tt.wantArrived)
			}
		})
	}
}
