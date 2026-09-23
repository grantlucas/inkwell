package rowagenda

import (
	"strings"
	"testing"
	"time"
)

// feedsOnly is the minimum valid config — feeds is the only required key.
func feedsOnly() map[string]any {
	return map[string]any{"feeds": []any{"https://example.com/a.ics"}}
}

func withKey(k string, v any) map[string]any {
	c := feedsOnly()
	c[k] = v
	return c
}

func TestParseConfig_Defaults(t *testing.T) {
	cfg, err := parseConfig(feedsOnly())
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	if cfg.Refresh != defaultRefresh {
		t.Errorf("Refresh = %v, want %v", cfg.Refresh, defaultRefresh)
	}
	if cfg.ShowLocation {
		t.Error("ShowLocation defaults to true, want false")
	}
	if len(cfg.Feeds) != 1 {
		t.Errorf("Feeds = %v, want one entry", cfg.Feeds)
	}
}

// Every rejection carries the widget name and says which key was wrong,
// because a dashboard failing to load is otherwise a guessing game.
func TestParseConfig_Rejections(t *testing.T) {
	tests := []struct {
		label  string
		config map[string]any
		want   string
	}{
		{"feeds missing", map[string]any{}, "feeds is required"},
		{"feeds not a list", map[string]any{"feeds": "https://example.com/a.ics"}, "feeds must be a list"},
		{"feeds empty", map[string]any{"feeds": []any{}}, "feeds must not be empty"},
		{"refresh not a string", withKey("refresh", 5), "refresh must be a string"},
		{"refresh unparseable", withKey("refresh", "soon"), "invalid refresh"},
		{"refresh under a minute", withKey("refresh", "30s"), "refresh must be >= 1m"},
		{"show_location not a bool", withKey("show_location", "yes"), "show_location must be a bool"},
		{"latitude not a number", withKey("latitude", "45"), "latitude must be a number"},
		{"latitude out of range", withKey("latitude", 91.0), "latitude must be in [-90, 90]"},
		{"longitude not a number", withKey("longitude", "-75"), "longitude must be a number"},
		{"longitude out of range", withKey("longitude", -181.0), "longitude must be in [-180, 180]"},
		{"temp_unit not a string", withKey("temp_unit", 1), "temp_unit must be a string"},
		{"temp_unit unknown", withKey("temp_unit", "K"), "invalid temp_unit"},
		{"weather_model not a string", withKey("weather_model", 1), "weather_model must be a string"},
		{"weather_model unknown", withKey("weather_model", "nope"), "invalid weather_model"},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			_, err := parseConfig(tt.config)
			if err == nil {
				t.Fatal("expected an error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %q, want it to mention %q", err, tt.want)
			}
			if !strings.HasPrefix(err.Error(), "row-agenda:") {
				t.Errorf("error = %q, want it to name the widget", err)
			}
		})
	}
}

func TestParseConfig_Accepted(t *testing.T) {
	tests := []struct {
		label  string
		config map[string]any
		check  func(*testing.T, Config)
	}{
		{"refresh", withKey("refresh", "30m"), func(t *testing.T, c Config) {
			if c.Refresh != 30*time.Minute {
				t.Errorf("Refresh = %v", c.Refresh)
			}
		}},
		{"show_location", withKey("show_location", true), func(t *testing.T, c Config) {
			if !c.ShowLocation {
				t.Error("ShowLocation = false")
			}
		}},
		{"latitude", withKey("latitude", 43.25), func(t *testing.T, c Config) {
			if c.Weather.Latitude != 43.25 || !c.Weather.LatSet {
				t.Errorf("Latitude = %v, set = %v", c.Weather.Latitude, c.Weather.LatSet)
			}
		}},
		{"longitude", withKey("longitude", -79.87), func(t *testing.T, c Config) {
			if c.Weather.Longitude != -79.87 || !c.Weather.LonSet {
				t.Errorf("Longitude = %v, set = %v", c.Weather.Longitude, c.Weather.LonSet)
			}
		}},
		{"temp_unit", withKey("temp_unit", "F"), func(t *testing.T, c Config) {
			if c.Weather.TempUnit != "F" || !c.Weather.UnitSet {
				t.Errorf("TempUnit = %q, set = %v", c.Weather.TempUnit, c.Weather.UnitSet)
			}
		}},
		{"weather_model", withKey("weather_model", "gem"), func(t *testing.T, c Config) {
			if !c.Weather.ModelSet {
				t.Error("ModelSet = false")
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			cfg, err := parseConfig(tt.config)
			if err != nil {
				t.Fatalf("parseConfig: %v", err)
			}
			tt.check(t, cfg)
		})
	}
}

// The docs promise the config is swappable with weekly-calendar, so a
// leftover key is a reasonable thing to find in a pasted config.
// Silently dropping show_weather: false would draw a weather band the
// operator explicitly turned off, which reads as a bug in the widget
// rather than a key that did not carry over — so every weekly key with
// no row-agenda meaning is rejected, with the reason.
func TestParseConfig_RejectsWeeklyOnlyKeys(t *testing.T) {
	for key := range unsupportedKeys {
		t.Run(key, func(t *testing.T) {
			_, err := parseConfig(withKey(key, true))
			if err == nil {
				t.Fatalf("%s was accepted silently", key)
			}
			if !strings.Contains(err.Error(), key) {
				t.Errorf("error = %q, want it to name the key", err)
			}
			if !strings.Contains(err.Error(), "not supported") {
				t.Errorf("error = %q, want it to say the key is unsupported", err)
			}
		})
	}
}
