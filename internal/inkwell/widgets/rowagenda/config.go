package rowagenda

import (
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daygrid"
)

// Config defaults. There is no max_events here: the row's slot count
// is the cap, and it adapts to the day's load — three across the full
// width, or four in two columns. A separate setting could only
// contradict the geometry.
const (
	defaultRefresh = 15 * time.Minute

	// widgetName prefixes every config error so a dashboard that fails
	// to load says which widget rejected it.
	widgetName = "row-agenda"
)

// parseConfig validates and extracts config values. The accepted keys
// mirror weekly-calendar's so a screen can be swapped between the two.
// The feed list and the weather overrides are parsed by daygrid, which
// every calendar-plus-weather screen shares; what is left here is the
// handful of keys specific to this one.
func parseConfig(config map[string]any) (Config, error) {
	cfg := Config{Refresh: defaultRefresh}

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
	// Sorted, because ranging a map would name an arbitrary one of
	// several leftover keys per run — so removing them one at a time
	// would look like the error was wandering rather than counting
	// down.
	for _, key := range slices.Sorted(maps.Keys(unsupportedKeys)) {
		if _, ok := config[key]; ok {
			why := unsupportedKeys[key]
			return cfg, fmt.Errorf("row-agenda: %s is not supported: %s", key, why)
		}
	}

	return cfg, nil
}

// unsupportedKeys are weekly-calendar keys with no row-agenda meaning,
// each with the reason, so the error says what to do rather than just
// refusing.
var unsupportedKeys = map[string]string{
	"days":               "row-agenda is always five rows; the row height is what the date numeral's size sets",
	"max_events":         "the row's slot count is the cap, and it adapts to the day's load — three across the full width, or four in two columns",
	"week_start":         "the rows always start from today, so there is no week to start",
	"show_weather":       "the weather badge is part of the layout; a day with no forecast already draws nothing",
	"show_weather_label": "the badge has no condition label — the icon carries the condition",
	"highlight_hour":     "the now-marker follows the clock, on today's row only",
}
