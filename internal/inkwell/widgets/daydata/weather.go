package daydata

import (
	"fmt"

	"github.com/grantlucas/inkwell/internal/inkwell/weather"
)

// WeatherConfig is the weather half of a screen's config: the resolved
// values plus which of them the widget actually set.
//
// The "set" flags are what make inheritance work. A widget that omits
// latitude should take the dashboard's, but zero is a legal latitude —
// so absence cannot be inferred from the value and has to be recorded
// while parsing.
type WeatherConfig struct {
	Latitude  float64
	Longitude float64
	TempUnit  string
	Model     weather.Model

	latSet   bool
	lonSet   bool
	unitSet  bool
	modelSet bool
}

// parseWeatherKeys reads the weather override keys a screen may carry —
// latitude, longitude, temp_unit and weather_model — recording which
// were present. Absent keys are left for resolveDefaults.
func parseWeatherKeys(widgetName string, config map[string]any, out *WeatherConfig) error {
	if v, ok := config["latitude"]; ok {
		f, ok := v.(float64)
		if !ok {
			return fmt.Errorf("%s: latitude must be a number, got %T", widgetName, v)
		}
		if f < -90 || f > 90 {
			return fmt.Errorf("%s: latitude must be in [-90, 90], got %v", widgetName, f)
		}
		out.Latitude, out.latSet = f, true
	}

	if v, ok := config["longitude"]; ok {
		f, ok := v.(float64)
		if !ok {
			return fmt.Errorf("%s: longitude must be a number, got %T", widgetName, v)
		}
		if f < -180 || f > 180 {
			return fmt.Errorf("%s: longitude must be in [-180, 180], got %v", widgetName, f)
		}
		out.Longitude, out.lonSet = f, true
	}

	if v, ok := config["temp_unit"]; ok {
		s, ok := v.(string)
		if !ok {
			return fmt.Errorf("%s: temp_unit must be a string, got %T", widgetName, v)
		}
		switch s {
		case "C", "F":
			out.TempUnit, out.unitSet = s, true
		default:
			return fmt.Errorf("%s: invalid temp_unit %q (must be C or F)", widgetName, s)
		}
	}

	if v, ok := config["weather_model"]; ok {
		s, ok := v.(string)
		if !ok {
			return fmt.Errorf("%s: weather_model must be a string, got %T", widgetName, v)
		}
		m, err := weather.ParseModel(s)
		if err != nil {
			return fmt.Errorf("%s: invalid weather_model: %w", widgetName, err)
		}
		out.Model, out.modelSet = m, true
	}

	return nil
}

// resolveDefaults fills any field the widget did not set from the
// shared Provider's defaults, so a dashboard configures location, model
// and unit once at the top level and any widget may override them.
//
// TempUnit falls back to "C" when no provider supplies one: a blank
// unit would render a bare number, which says less than the wrong
// scale would.
func resolveDefaults(cfg *WeatherConfig, provider *weather.Provider) {
	var def weather.Settings
	if provider != nil {
		def = provider.Defaults()
	}
	if !cfg.latSet {
		cfg.Latitude = def.Location.Latitude
	}
	if !cfg.lonSet {
		cfg.Longitude = def.Location.Longitude
	}
	if !cfg.unitSet {
		cfg.TempUnit = def.TempUnit
	}
	if cfg.TempUnit == "" {
		cfg.TempUnit = "C"
	}
	if !cfg.modelSet {
		cfg.Model = def.Model
	}
}

// location is the point the forecast is fetched for.
func (c WeatherConfig) location() weather.Location {
	return weather.Location{Latitude: c.Latitude, Longitude: c.Longitude}
}
