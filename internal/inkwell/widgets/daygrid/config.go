package daygrid

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar"
	"github.com/grantlucas/inkwell/internal/inkwell/weather"
)

// DefaultRefresh is how fresh a calendar widget wants its feeds when it
// doesn't set config.refresh.
const DefaultRefresh = 15 * time.Minute

// Config is the configuration every calendar widget shares: its feeds,
// how fresh it wants them, where its forecast is for, and how its events
// are listed.
type Config struct {
	Feeds []calendar.Feed
	// Refresh is the calendar cache duration, the nested config.refresh,
	// not the widget's render cadence (the required top-level refresh:
	// LoadConfig validates). Same name, different axis.
	Refresh time.Duration
	// Weather is resolved: whatever the widget didn't set is inherited
	// from the top-level weather settings.
	Weather      WeatherConfig
	MaxEvents    int
	ShowLocation bool
}

// Spec is what a calendar widget tells the shared parser about itself.
type Spec struct {
	// Widget names the widget in every error, so a dashboard that fails
	// to load says which widget rejected it.
	Widget string
	// MaxEvents is the widget's default max_events. Zero means the
	// widget doesn't list a fixed number of events, so the key is
	// rejected.
	MaxEvents int
	// Extra are the widget's own keys: accepted here, and left in the
	// raw config for the widget to parse.
	Extra []string
}

// sharedKeys are the settings every calendar widget accepts.
var sharedKeys = []string{
	"feeds", "refresh", "show_location",
	"latitude", "longitude", "temp_unit", "weather_model",
}

// ParseConfig parses the settings every calendar widget shares, the same
// way for each, and rejects any key neither shared nor the widget's own,
// so a misspelt setting fails loudly rather than being ignored. Weather
// settings the widget doesn't set are inherited from the top-level ones
// inherit carries; inherit may be nil.
func ParseConfig(spec Spec, raw map[string]any, inherit *weather.Provider) (Config, error) {
	cfg := Config{Refresh: DefaultRefresh, MaxEvents: spec.MaxEvents}
	name := spec.Widget

	if err := rejectUnknown(spec, raw); err != nil {
		return cfg, err
	}

	f, ok := raw["feeds"]
	if !ok {
		return cfg, fmt.Errorf("%s: feeds is required", name) //nolint:goerr113 // config validation message
	}
	feeds, err := ParseFeeds(name, f)
	if err != nil {
		return cfg, err
	}
	cfg.Feeds = feeds

	if v, ok := raw["refresh"]; ok {
		if cfg.Refresh, err = parseRefresh(name, v); err != nil {
			return cfg, err
		}
	}

	if v, ok := raw["max_events"]; ok {
		n, ok := v.(int)
		if !ok {
			return cfg, fmt.Errorf("%s: max_events must be an integer, got %T", name, v)
		}
		if n <= 0 {
			return cfg, fmt.Errorf("%s: max_events must be positive, got %d", name, n)
		}
		cfg.MaxEvents = n
	}

	if v, ok := raw["show_location"]; ok {
		b, ok := v.(bool)
		if !ok {
			return cfg, fmt.Errorf("%s: show_location must be a bool, got %T", name, v)
		}
		cfg.ShowLocation = b
	}

	if err := ParseWeatherKeys(name, raw, &cfg.Weather); err != nil {
		return cfg, err
	}
	ResolveDefaults(&cfg.Weather, inherit)
	return cfg, nil
}

// parseRefresh reads config.refresh, the calendar cache duration.
func parseRefresh(name string, v any) (time.Duration, error) {
	s, ok := v.(string)
	if !ok {
		return 0, fmt.Errorf("%s: refresh must be a string, got %T", name, v)
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("%s: invalid refresh %q: %w", name, s, err)
	}
	if d < time.Minute {
		return 0, fmt.Errorf("%s: refresh must be >= 1m, got %v", name, d)
	}
	return d, nil
}

// rejectUnknown fails on the first key, in sorted order, that the widget
// doesn't accept. Sorted, because ranging a map would name an arbitrary
// one of several bad keys per run, so fixing them one at a time would
// look like the error was wandering rather than counting down.
func rejectUnknown(spec Spec, raw map[string]any) error {
	accepted := slices.Concat(sharedKeys, spec.Extra)
	if spec.MaxEvents > 0 {
		accepted = append(accepted, "max_events")
	}
	slices.Sort(accepted)
	for _, key := range slices.Sorted(maps.Keys(raw)) {
		if !slices.Contains(accepted, key) {
			return fmt.Errorf("%s: unsupported setting %q (accepted: %s)", spec.Widget, key, strings.Join(accepted, ", "))
		}
	}
	return nil
}
