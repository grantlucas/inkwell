package boldfive

import (
	"fmt"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daygrid"
)

// Config defaults. maxEvents is four rather than weekly's five: the
// taller line height this screen exists for costs one event per column.
const (
	defaultRefresh   = 15 * time.Minute
	defaultMaxEvents = 4

	// widgetName prefixes every config error so a dashboard that fails
	// to load says which widget rejected it.
	widgetName = "bold-five"
)

// parseConfig validates and extracts config values. The accepted keys
// mirror weekly-calendar's so a screen can be swapped between the two.
// The feed list and the weather overrides are parsed by daygrid, which
// every calendar-plus-weather screen shares; what is left here is the
// handful of keys specific to this one.
func parseConfig(config map[string]any) (Config, error) {
	cfg := Config{
		Refresh:   defaultRefresh,
		MaxEvents: defaultMaxEvents,
	}

	f, ok := config["feeds"]
	if !ok {
		return cfg, fmt.Errorf("%s: feeds is required", widgetName) //nolint:goerr113 // config validation message
	}
	feeds, err := daygrid.ParseFeeds(widgetName, f)
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
			return cfg, fmt.Errorf("%s: refresh must be a string, got %T", widgetName, v)
		}
		d, err := time.ParseDuration(s)
		if err != nil {
			return cfg, fmt.Errorf("%s: invalid refresh %q: %w", widgetName, s, err)
		}
		if d < time.Minute {
			return cfg, fmt.Errorf("%s: refresh must be >= 1m, got %v", widgetName, d)
		}
		cfg.Refresh = d
	}

	if v, ok := config["max_events"]; ok {
		n, ok := v.(int)
		if !ok {
			return cfg, fmt.Errorf("%s: max_events must be an integer, got %T", widgetName, v)
		}
		if n <= 0 {
			return cfg, fmt.Errorf("%s: max_events must be positive, got %d", widgetName, n)
		}
		cfg.MaxEvents = n
	}

	if v, ok := config["show_location"]; ok {
		b, ok := v.(bool)
		if !ok {
			return cfg, fmt.Errorf("%s: show_location must be a bool, got %T", widgetName, v)
		}
		cfg.ShowLocation = b
	}

	// Location, unit and model are shared with every other
	// calendar-plus-weather screen, so they are parsed once in daygrid
	// rather than restated here.
	if err := daygrid.ParseWeatherKeys(widgetName, config, &cfg.Weather); err != nil {
		return cfg, err
	}

	// weekly-calendar keys this screen has no equivalent for are
	// rejected rather than ignored. The docs promise the config is
	// swappable between the two, so a leftover key is a reasonable
	// thing to find in a pasted config — and silently dropping
	// show_weather: false would draw a weather band the operator
	// explicitly turned off, which looks like a bug in the widget
	// rather than a key that did not carry over.
	for key, why := range unsupportedKeys {
		if _, ok := config[key]; ok {
			return cfg, fmt.Errorf("bold-five: %s is not supported: %s", key, why)
		}
	}

	return cfg, nil
}

// unsupportedKeys are weekly-calendar keys with no bold-five meaning,
// each with the reason, so the error says what to do rather than just
// refusing.
var unsupportedKeys = map[string]string{
	"days":               "bold-five is always five columns; every type size is derived from a 160 px column",
	"week_start":         "columns always start from today, so there is no week to start",
	"show_weather":       "the weather band is part of the layout; a day with no forecast already draws nothing",
	"show_weather_label": "there is no condition label to show — the icon carries the condition",
	"highlight_hour":     "the now-marker follows the clock, on today's column only",
}
