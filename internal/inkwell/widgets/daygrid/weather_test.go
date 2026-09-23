package daygrid

import (
	"strings"
	"testing"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/weather"
)

func TestParseWeatherKeys_Accepted(t *testing.T) {
	tests := []struct {
		label  string
		config map[string]any
		check  func(*testing.T, WeatherConfig)
	}{
		{"latitude", map[string]any{"latitude": 43.25}, func(t *testing.T, c WeatherConfig) {
			if c.Latitude != 43.25 || !c.LatSet {
				t.Errorf("Latitude = %v, set = %v", c.Latitude, c.LatSet)
			}
		}},
		{"longitude", map[string]any{"longitude": -79.87}, func(t *testing.T, c WeatherConfig) {
			if c.Longitude != -79.87 || !c.LonSet {
				t.Errorf("Longitude = %v, set = %v", c.Longitude, c.LonSet)
			}
		}},
		{"temp_unit C", map[string]any{"temp_unit": "C"}, func(t *testing.T, c WeatherConfig) {
			if c.TempUnit != "C" || !c.UnitSet {
				t.Errorf("TempUnit = %q, set = %v", c.TempUnit, c.UnitSet)
			}
		}},
		{"temp_unit F", map[string]any{"temp_unit": "F"}, func(t *testing.T, c WeatherConfig) {
			if c.TempUnit != "F" || !c.UnitSet {
				t.Errorf("TempUnit = %q, set = %v", c.TempUnit, c.UnitSet)
			}
		}},
		{"weather_model", map[string]any{"weather_model": "gem"}, func(t *testing.T, c WeatherConfig) {
			if c.Model != weather.ModelGEM || !c.ModelSet {
				t.Errorf("Model = %v, set = %v", c.Model, c.ModelSet)
			}
		}},
		// Zero is a legal latitude, which is exactly why absence cannot
		// be inferred from the value and the set flags have to exist.
		{"zero latitude still counts as set", map[string]any{"latitude": 0.0}, func(t *testing.T, c WeatherConfig) {
			if !c.LatSet {
				t.Error("LatSet = false for an explicit 0")
			}
		}},
		{"nothing set", map[string]any{}, func(t *testing.T, c WeatherConfig) {
			if c.LatSet || c.LonSet || c.UnitSet || c.ModelSet {
				t.Errorf("something was marked set: %+v", c)
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			var cfg WeatherConfig
			if err := ParseWeatherKeys(testWidget, tt.config, &cfg); err != nil {
				t.Fatalf("ParseWeatherKeys: %v", err)
			}
			tt.check(t, cfg)
		})
	}
}

func TestParseWeatherKeys_Rejections(t *testing.T) {
	tests := []struct {
		label  string
		config map[string]any
		want   string
	}{
		{"latitude not a number", map[string]any{"latitude": "45"}, "latitude must be a number"},
		{"latitude too high", map[string]any{"latitude": 91.0}, "latitude must be in [-90, 90]"},
		{"latitude too low", map[string]any{"latitude": -91.0}, "latitude must be in [-90, 90]"},
		{"longitude not a number", map[string]any{"longitude": "-75"}, "longitude must be a number"},
		{"longitude too high", map[string]any{"longitude": 181.0}, "longitude must be in [-180, 180]"},
		{"longitude too low", map[string]any{"longitude": -181.0}, "longitude must be in [-180, 180]"},
		{"temp_unit not a string", map[string]any{"temp_unit": 1}, "temp_unit must be a string"},
		{"temp_unit unknown", map[string]any{"temp_unit": "K"}, "invalid temp_unit"},
		{"weather_model not a string", map[string]any{"weather_model": 1}, "weather_model must be a string"},
		{"weather_model unknown", map[string]any{"weather_model": "nope"}, "invalid weather_model"},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			var cfg WeatherConfig
			err := ParseWeatherKeys(testWidget, tt.config, &cfg)
			if err == nil {
				t.Fatal("expected an error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %q, want it to mention %q", err, tt.want)
			}
			// The widget name is what tells an operator which entry in
			// their dashboard to go and look at.
			if !strings.HasPrefix(err.Error(), testWidget+":") {
				t.Errorf("error = %q, want it prefixed with the widget name", err)
			}
		})
	}
}

// A widget that sets nothing inherits location, unit and model from the
// shared provider, so a dashboard configures them once at the top level.
func TestResolveDefaults(t *testing.T) {
	provider := weather.NewProvider(nil, time.Hour, time.Now, weather.Settings{
		Location: weather.Location{Latitude: 43.25, Longitude: -79.87},
		TempUnit: "F",
		Model:    weather.ModelGEM,
	})

	t.Run("inherits everything unset", func(t *testing.T) {
		var cfg WeatherConfig
		ResolveDefaults(&cfg, provider)
		if cfg.Latitude != 43.25 || cfg.Longitude != -79.87 {
			t.Errorf("location = %v,%v", cfg.Latitude, cfg.Longitude)
		}
		if cfg.TempUnit != "F" {
			t.Errorf("TempUnit = %q", cfg.TempUnit)
		}
		if cfg.Model != weather.ModelGEM {
			t.Errorf("Model = %v", cfg.Model)
		}
	})

	t.Run("keeps what the widget set", func(t *testing.T) {
		cfg := WeatherConfig{
			Latitude: 1, LatSet: true,
			Longitude: 2, LonSet: true,
			TempUnit: "C", UnitSet: true,
			Model: weather.ModelECMWF, ModelSet: true,
		}
		ResolveDefaults(&cfg, provider)
		if cfg.Latitude != 1 || cfg.Longitude != 2 || cfg.TempUnit != "C" {
			t.Errorf("overrides lost: %+v", cfg)
		}
		if cfg.Model != weather.ModelECMWF {
			t.Errorf("Model = %v, want the override", cfg.Model)
		}
	})

	// Without a provider there is nothing to inherit, but a unit is
	// still needed: a blank one renders a bare number, which says less
	// than the wrong scale would.
	t.Run("falls back to Celsius without a provider", func(t *testing.T) {
		var cfg WeatherConfig
		ResolveDefaults(&cfg, nil)
		if cfg.TempUnit != "C" {
			t.Errorf("TempUnit = %q, want C", cfg.TempUnit)
		}
	})
}

func TestWeatherConfig_Location(t *testing.T) {
	cfg := WeatherConfig{Latitude: 43.25, Longitude: -79.87}
	got := cfg.Location()
	if got.Latitude != 43.25 || got.Longitude != -79.87 {
		t.Errorf("Location() = %+v", got)
	}
}
