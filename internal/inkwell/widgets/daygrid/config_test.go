package daygrid_test

import (
	"strings"
	"testing"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/weather"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daygrid"
)

// listing is a calendar widget that lists events, three by default.
var listing = daygrid.Spec{Widget: "test-widget", MaxEvents: 3}

// topLevel is the dashboard's top-level weather settings.
var topLevel = weather.NewProvider(nil, time.Hour, nil, weather.Settings{
	Location: weather.Location{Latitude: 43.25, Longitude: -79.87},
	TempUnit: "F",
	Model:    weather.ModelGEM,
})

// feedsAnd is the minimum valid config plus one key.
func feedsAnd(k string, v any) map[string]any {
	c := map[string]any{"feeds": []any{feedA}}
	if k != "" {
		c[k] = v
	}
	return c
}

// Every calendar widget's shared settings are parsed by one parser, so a
// setting is accepted the same way on every widget, and widget-level
// weather settings inherit from the top-level ones.
func TestParseConfig_Accepts(t *testing.T) {
	tests := []struct {
		label   string
		spec    daygrid.Spec
		raw     map[string]any
		inherit *weather.Provider
		check   func(*testing.T, daygrid.Config)
	}{
		{
			label: "defaults", spec: listing, raw: feedsAnd("", nil), inherit: topLevel,
			check: func(t *testing.T, c daygrid.Config) {
				if len(c.Feeds) != 1 || c.Feeds[0].URL != feedA {
					t.Errorf("Feeds = %+v", c.Feeds)
				}
				if c.Refresh != 15*time.Minute || c.MaxEvents != 3 || c.ShowLocation {
					t.Errorf("Refresh, MaxEvents, ShowLocation = %v, %d, %v", c.Refresh, c.MaxEvents, c.ShowLocation)
				}
			},
		},
		{
			label: "the top-level weather settings are inherited", spec: listing, raw: feedsAnd("", nil), inherit: topLevel,
			check: func(t *testing.T, c daygrid.Config) {
				w := c.Weather
				if w.Latitude != 43.25 || w.Longitude != -79.87 || w.TempUnit != "F" || w.Model != weather.ModelGEM {
					t.Errorf("Weather = %+v, want the top-level settings", w)
				}
			},
		},
		{
			label: "widget weather settings override the top level", spec: listing, inherit: topLevel,
			raw: map[string]any{
				"feeds": []any{feedA}, "latitude": 0.0, "longitude": 2.5, "temp_unit": "C", "weather_model": "ecmwf",
			},
			check: func(t *testing.T, c daygrid.Config) {
				w := c.Weather
				// Zero is a real latitude, so setting it overrides too.
				if w.Latitude != 0 || w.Longitude != 2.5 || w.TempUnit != "C" || w.Model != weather.ModelECMWF {
					t.Errorf("Weather = %+v, want the widget's own", w)
				}
			},
		},
		{
			// A blank unit would draw a bare number, which says less
			// than the wrong scale would.
			label: "Celsius with no top-level settings", spec: listing, raw: feedsAnd("", nil), inherit: nil,
			check: func(t *testing.T, c daygrid.Config) {
				if c.Weather.TempUnit != "C" {
					t.Errorf("TempUnit = %q, want C", c.Weather.TempUnit)
				}
			},
		},
		{
			label: "refresh", spec: listing, raw: feedsAnd("refresh", "30m"), inherit: topLevel,
			check: func(t *testing.T, c daygrid.Config) {
				if c.Refresh != 30*time.Minute {
					t.Errorf("Refresh = %v", c.Refresh)
				}
			},
		},
		{
			label: "max_events", spec: listing, raw: feedsAnd("max_events", 2), inherit: topLevel,
			check: func(t *testing.T, c daygrid.Config) {
				if c.MaxEvents != 2 {
					t.Errorf("MaxEvents = %d", c.MaxEvents)
				}
			},
		},
		{
			label: "show_location", spec: listing, raw: feedsAnd("show_location", true), inherit: topLevel,
			check: func(t *testing.T, c daygrid.Config) {
				if !c.ShowLocation {
					t.Error("ShowLocation = false")
				}
			},
		},
		{
			label: "a feed with rules", spec: listing, inherit: topLevel,
			raw: map[string]any{"feeds": []any{map[string]any{
				"url": feedA, "name": "Team", "rules": []any{map[string]any{"match": "^x", "exclude": true}},
			}}},
			check: func(t *testing.T, c daygrid.Config) {
				if len(c.Feeds) != 1 || c.Feeds[0].Name != "Team" || len(c.Feeds[0].Rules) != 1 {
					t.Errorf("Feeds = %+v", c.Feeds)
				}
			},
		},
		{
			label: "a key the widget declares as its own", inherit: topLevel,
			spec: daygrid.Spec{Widget: "test-widget", MaxEvents: 3, Extra: []string{"days"}},
			raw:  feedsAnd("days", 7),
			check: func(*testing.T, daygrid.Config) {
			},
		},
		{
			// The example config's bold-five screen.
			label: "an existing bold-five config", spec: daygrid.Spec{Widget: "bold-five", MaxEvents: 4}, inherit: topLevel,
			raw: map[string]any{
				"feeds": []any{"https://example.com/my-calendar.ics"}, "max_events": 3,
				"show_location": false, "refresh": "15m", "temp_unit": "F", "weather_model": "ecmwf",
			},
			check: func(t *testing.T, c daygrid.Config) {
				if c.MaxEvents != 3 || c.Weather.TempUnit != "F" || c.Weather.Model != weather.ModelECMWF {
					t.Errorf("Config = %+v", c)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			cfg, err := daygrid.ParseConfig(tt.spec, tt.raw, tt.inherit)
			if err != nil {
				t.Fatalf("ParseConfig: %v", err)
			}
			tt.check(t, cfg)
		})
	}
}

// Every rejection names the widget and says what was wrong, the same way
// on every calendar widget. A key the parser doesn't know, including a
// misspelling, is rejected rather than ignored.
func TestParseConfig_Rejects(t *testing.T) {
	const accepted = "(accepted: feeds, latitude, longitude, max_events, refresh, show_location, temp_unit, weather_model)"
	noMaxEvents := daygrid.Spec{Widget: "test-widget"}
	explains := daygrid.Spec{Widget: "test-widget", MaxEvents: 3, Rejected: map[string]string{
		"days": "always five columns",
	}}

	tests := []struct {
		label string
		spec  daygrid.Spec
		raw   map[string]any
		want  string
	}{
		{"feeds missing", listing, map[string]any{}, "feeds is required"},
		{"feeds not a list", listing, map[string]any{"feeds": feedA}, "feeds must be a list"},
		{"feeds empty", listing, map[string]any{"feeds": []any{}}, "feeds must not be empty"},
		{"refresh not a string", listing, feedsAnd("refresh", 5), "refresh must be a string"},
		{"refresh unparseable", listing, feedsAnd("refresh", "soon"), "invalid refresh"},
		{"refresh under a minute", listing, feedsAnd("refresh", "30s"), "refresh must be >= 1m"},
		{"max_events not an int", listing, feedsAnd("max_events", "four"), "max_events must be an integer"},
		{"max_events zero", listing, feedsAnd("max_events", 0), "max_events must be positive"},
		{"max_events negative", listing, feedsAnd("max_events", -1), "max_events must be positive"},
		{"show_location not a bool", listing, feedsAnd("show_location", "yes"), "show_location must be a bool"},
		{"latitude not a number", listing, feedsAnd("latitude", "45"), "latitude must be a number"},
		{"latitude too high", listing, feedsAnd("latitude", 91.0), "latitude must be in [-90, 90]"},
		{"latitude too low", listing, feedsAnd("latitude", -91.0), "latitude must be in [-90, 90]"},
		{"longitude not a number", listing, feedsAnd("longitude", "-75"), "longitude must be a number"},
		{"longitude too high", listing, feedsAnd("longitude", 181.0), "longitude must be in [-180, 180]"},
		{"longitude too low", listing, feedsAnd("longitude", -181.0), "longitude must be in [-180, 180]"},
		{"temp_unit not a string", listing, feedsAnd("temp_unit", 1), "temp_unit must be a string"},
		{"temp_unit unknown", listing, feedsAnd("temp_unit", "K"), "invalid temp_unit"},
		{"weather_model not a string", listing, feedsAnd("weather_model", 1), "weather_model must be a string"},
		{"weather_model unknown", listing, feedsAnd("weather_model", "nope"), "invalid weather_model"},
		{"a misspelt key", listing, feedsAnd("max_event", 3), `unsupported setting "max_event" ` + accepted},
		{"another widget's key", listing, feedsAnd("days", 7), `unsupported setting "days" ` + accepted},
		// A key the widget knows another calendar widget takes is
		// rejected with the widget's reason; anything else still gets
		// the accepted list.
		{"another widget's key, explained", explains, feedsAnd("days", 7), "days is not supported: always five columns"},
		{"a typo beside an explained key", explains, feedsAnd("max_event", 3), `unsupported setting "max_event" ` + accepted},
		{
			"max_events on a widget that doesn't list a fixed number", noMaxEvents, feedsAnd("max_events", 3),
			`unsupported setting "max_events" (accepted: feeds, latitude, longitude, refresh, show_location, temp_unit, weather_model)`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			_, err := daygrid.ParseConfig(tt.spec, tt.raw, topLevel)
			if err == nil {
				t.Fatal("expected an error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %q, want it to mention %q", err, tt.want)
			}
			if !strings.HasPrefix(err.Error(), "test-widget: ") {
				t.Errorf("error = %q, want it to name the widget", err)
			}
		})
	}
}
