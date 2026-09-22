package boldfive

import (
	"fmt"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/weather"
)

// Config defaults. maxEvents is four rather than weekly's five: the
// taller line height this screen exists for costs one event per column.
const (
	defaultRefresh   = 15 * time.Minute
	defaultMaxEvents = 4
)

// parseConfig validates and extracts config values. The accepted keys
// mirror weekly-calendar's so a screen can be swapped between the two.
//
// Like feeds.go, this duplicates weekly's parser; issue #92 extracts the
// shared scaffolding once a second screen shows where the seams are.
func parseConfig(config map[string]any) (Config, error) {
	cfg := Config{
		Refresh:   defaultRefresh,
		MaxEvents: defaultMaxEvents,
	}

	f, ok := config["feeds"]
	if !ok {
		return cfg, fmt.Errorf("bold-five: feeds is required") //nolint:goerr113 // config validation message
	}
	feeds, err := parseFeeds(f)
	if err != nil {
		return cfg, err
	}
	cfg.Feeds = feeds

	// refresh here is the calendar cache TTL, not the widget's render
	// cadence — that is the required top-level `refresh:` LoadConfig
	// validates. Same name, different axis (see CLAUDE.md).
	if v, ok := config["refresh"]; ok {
		s, ok := v.(string)
		if !ok {
			return cfg, fmt.Errorf("bold-five: refresh must be a string, got %T", v)
		}
		d, err := time.ParseDuration(s)
		if err != nil {
			return cfg, fmt.Errorf("bold-five: invalid refresh %q: %w", s, err)
		}
		if d < time.Minute {
			return cfg, fmt.Errorf("bold-five: refresh must be >= 1m, got %v", d)
		}
		cfg.Refresh = d
	}

	if v, ok := config["max_events"]; ok {
		n, ok := v.(int)
		if !ok {
			return cfg, fmt.Errorf("bold-five: max_events must be an integer, got %T", v)
		}
		if n <= 0 {
			return cfg, fmt.Errorf("bold-five: max_events must be positive, got %d", n)
		}
		cfg.MaxEvents = n
	}

	if v, ok := config["show_location"]; ok {
		b, ok := v.(bool)
		if !ok {
			return cfg, fmt.Errorf("bold-five: show_location must be a bool, got %T", v)
		}
		cfg.ShowLocation = b
	}

	if v, ok := config["latitude"]; ok {
		f, ok := v.(float64)
		if !ok {
			return cfg, fmt.Errorf("bold-five: latitude must be a number, got %T", v)
		}
		if f < -90 || f > 90 {
			return cfg, fmt.Errorf("bold-five: latitude must be in [-90, 90], got %v", f)
		}
		cfg.Latitude, cfg.latSet = f, true
	}

	if v, ok := config["longitude"]; ok {
		f, ok := v.(float64)
		if !ok {
			return cfg, fmt.Errorf("bold-five: longitude must be a number, got %T", v)
		}
		if f < -180 || f > 180 {
			return cfg, fmt.Errorf("bold-five: longitude must be in [-180, 180], got %v", f)
		}
		cfg.Longitude, cfg.lonSet = f, true
	}

	if v, ok := config["temp_unit"]; ok {
		s, ok := v.(string)
		if !ok {
			return cfg, fmt.Errorf("bold-five: temp_unit must be a string, got %T", v)
		}
		switch s {
		case "C", "F":
			cfg.TempUnit, cfg.unitSet = s, true
		default:
			return cfg, fmt.Errorf("bold-five: invalid temp_unit %q (must be C or F)", s)
		}
	}

	if v, ok := config["weather_model"]; ok {
		s, ok := v.(string)
		if !ok {
			return cfg, fmt.Errorf("bold-five: weather_model must be a string, got %T", v)
		}
		m, err := weather.ParseModel(s)
		if err != nil {
			return cfg, fmt.Errorf("bold-five: invalid weather_model: %w", err)
		}
		cfg.WeatherModel, cfg.modelSet = m, true
	}

	return cfg, nil
}
