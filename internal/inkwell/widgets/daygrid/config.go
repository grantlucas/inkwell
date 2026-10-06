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

// defaultRefresh is how fresh a calendar widget wants its feeds when it
// doesn't set config.refresh.
const defaultRefresh = 15 * time.Minute

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
	// Day is which day a one-day widget draws: 0 is today, 1 tomorrow,
	// up to MaxDay. A widget drawing several days leaves it 0.
	Day int
}

// MaxDay is the furthest day a one-day widget can be placed on: a week
// out. The forecast every widget shares reaches a day past that (see
// weather.ForecastHorizon), so a chart can still range over the whole
// week it sits in.
const MaxDay = 6

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
	// Rejected explains, key by key, why another calendar widget's
	// setting means nothing here. Calendar configs are meant to be
	// swappable, so a leftover key is a reasonable thing to find in a
	// pasted config, and the reason says what to do about it. Any other
	// unknown key gets the list of accepted ones.
	Rejected map[string]string
	// WeatherOnly is a widget that draws the forecast and no events, so
	// it reads no calendar. feeds is then not required but rejected,
	// along with every other calendar setting (refresh, show_location,
	// max_events), with that as the reason; MaxEvents is ignored. Its
	// weather settings parse and inherit exactly as a calendar widget's
	// do.
	WeatherOnly bool
	// CalendarOnly is a widget that lists events and draws no forecast,
	// so it reads no weather. Its weather settings are rejected with that
	// as the reason.
	CalendarOnly bool
	// OneDay is a widget placed on a single day rather than drawing
	// several, so it accepts day: which day, from today, it draws.
	OneDay bool
}

// calendarKeys are the shared settings that configure a widget's
// calendar, which a weather-only widget doesn't have.
var calendarKeys = []string{"feeds", "refresh", "show_location"}

// weatherKeys are the shared settings that say where and how a widget's
// forecast is fetched and shown. Every day widget accepts them.
var weatherKeys = []string{"latitude", "longitude", "temp_unit", "weather_model"}

// ParseConfig parses the settings every calendar widget shares, the same
// way for each, and rejects any key neither shared nor the widget's own,
// so a misspelt setting fails loudly rather than being ignored. Weather
// settings the widget doesn't set are inherited from the top-level ones
// inherit carries; inherit may be nil.
func ParseConfig(spec Spec, raw map[string]any, inherit *weather.Provider) (Config, error) {
	cfg := Config{Refresh: defaultRefresh, MaxEvents: spec.MaxEvents}
	name := spec.Widget

	if err := rejectUnknown(spec, raw); err != nil {
		return cfg, err
	}

	// A weather-only widget has had every calendar key rejected above,
	// so only the weather settings are left to parse.
	if !spec.WeatherOnly {
		f, ok := raw["feeds"]
		if !ok {
			return cfg, fmt.Errorf("%s: feeds is required", name) //nolint:goerr113 // config validation message
		}
		feeds, err := parseFeeds(name, f)
		if err != nil {
			return cfg, err
		}
		cfg.Feeds = feeds
	}

	var err error
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

	if v, ok := raw["day"]; ok {
		n, ok := v.(int)
		if !ok {
			return cfg, fmt.Errorf("%s: day must be an integer, got %T", name, v)
		}
		if n < 0 || n > MaxDay {
			return cfg, fmt.Errorf("%s: day must be in [0, %d], got %d", name, MaxDay, n)
		}
		cfg.Day = n
	}

	if v, ok := raw["show_location"]; ok {
		b, ok := v.(bool)
		if !ok {
			return cfg, fmt.Errorf("%s: show_location must be a bool, got %T", name, v)
		}
		cfg.ShowLocation = b
	}

	if err := parseWeatherKeys(name, raw, &cfg.Weather); err != nil {
		return cfg, err
	}
	resolveDefaults(&cfg.Weather, inherit)
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
// doesn't accept, with the widget's reason when it gave one. Sorted,
// because ranging a map would name an arbitrary one of several bad keys
// per run, so fixing them one at a time would look like the error was
// wandering rather than counting down.
func rejectUnknown(spec Spec, raw map[string]any) error {
	accepted := slices.Clone(spec.Extra)
	if !spec.CalendarOnly {
		accepted = append(accepted, weatherKeys...)
	}
	if spec.OneDay {
		accepted = append(accepted, "day")
	}
	if !spec.WeatherOnly {
		accepted = append(accepted, calendarKeys...)
		if spec.MaxEvents > 0 {
			accepted = append(accepted, "max_events")
		}
	}
	slices.Sort(accepted)
	for _, key := range slices.Sorted(maps.Keys(raw)) {
		if slices.Contains(accepted, key) {
			continue
		}
		if why, ok := spec.Rejected[key]; ok {
			return fmt.Errorf("%s: %s is not supported: %s", spec.Widget, key, why)
		}
		if spec.WeatherOnly && (slices.Contains(calendarKeys, key) || key == "max_events") {
			return fmt.Errorf("%s: %s is not supported: %s shows only the weather, so it reads no calendar", spec.Widget, key, spec.Widget)
		}
		if spec.CalendarOnly && slices.Contains(weatherKeys, key) {
			return fmt.Errorf("%s: %s is not supported: %s shows only events, so it reads no forecast", spec.Widget, key, spec.Widget)
		}
		return fmt.Errorf("%s: unsupported setting %q (accepted: %s)", spec.Widget, key, strings.Join(accepted, ", "))
	}
	return nil
}
