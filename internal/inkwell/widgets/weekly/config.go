package weekly

import (
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/weather"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daygrid"
)

// widgetName prefixes every config error so a dashboard that fails to
// load says which widget rejected it.
const widgetName = "weekly-calendar"

// spec declares weekly-calendar to the shared calendar-widget parser: the
// shared settings, a default of five events a column, and the keys only
// this widget takes, which parseConfig reads itself.
var spec = daygrid.Spec{
	Widget:    widgetName,
	MaxEvents: 5,
	Extra:     slices.Sorted(maps.Keys(ownKeys)),
}

// Config holds parsed weekly-calendar configuration: the settings every
// calendar widget shares, and this widget's own.
type Config struct {
	daygrid.Config

	WeekStart        time.Weekday
	Days             int
	ShowWeather      bool
	ShowWeatherLabel bool
	HighlightHour    int
}

// ownKeys are the settings only weekly-calendar takes, each with its
// parser. The shared parser accepts exactly these as the widget's own, so
// adding a key here is the whole change.
var ownKeys = map[string]func(*Config, any) error{
	"week_start": func(c *Config, v any) error {
		s, ok := v.(string)
		if !ok {
			return fmt.Errorf("weekly-calendar: week_start must be a string, got %T", v)
		}
		switch s {
		case "monday":
			c.WeekStart = time.Monday
		case "sunday":
			c.WeekStart = time.Sunday
		default:
			return fmt.Errorf("weekly-calendar: invalid week_start %q (must be monday or sunday)", s)
		}
		return nil
	},
	"days": func(c *Config, v any) error {
		n, ok := v.(int)
		if !ok {
			return fmt.Errorf("weekly-calendar: days must be an integer, got %T", v)
		}
		if n < 1 || n > defaultDays {
			return fmt.Errorf("weekly-calendar: days must be in [1, %d], got %d", defaultDays, n)
		}
		c.Days = n
		return nil
	},
	"show_weather": func(c *Config, v any) error {
		b, ok := v.(bool)
		if !ok {
			return fmt.Errorf("weekly-calendar: show_weather must be a bool, got %T", v)
		}
		c.ShowWeather = b
		return nil
	},
	"show_weather_label": func(c *Config, v any) error {
		b, ok := v.(bool)
		if !ok {
			return fmt.Errorf("weekly-calendar: show_weather_label must be a bool, got %T", v)
		}
		c.ShowWeatherLabel = b
		return nil
	},
	"highlight_hour": func(c *Config, v any) error {
		n, ok := v.(int)
		if !ok {
			return fmt.Errorf("weekly-calendar: highlight_hour must be an integer, got %T", v)
		}
		if n < 0 || n > 23 {
			return fmt.Errorf("weekly-calendar: highlight_hour must be in [0, 23], got %d", n)
		}
		c.HighlightHour = n
		return nil
	},
}

// parseConfig validates and extracts config values: the shared settings
// through the shared parser, inheriting weather settings from inherit,
// then this widget's own keys, in sorted order so the first error named
// is the same on every run.
func parseConfig(config map[string]any, inherit *weather.Provider) (Config, error) {
	shared, err := daygrid.ParseConfig(spec, config, inherit)
	cfg := Config{
		Config:           shared,
		WeekStart:        time.Monday,
		Days:             defaultDays,
		ShowWeather:      true,
		ShowWeatherLabel: true,
		HighlightHour:    15,
	}
	if err != nil {
		return cfg, err
	}
	for _, key := range spec.Extra {
		if v, ok := config[key]; ok {
			if err := ownKeys[key](&cfg, v); err != nil {
				return cfg, err
			}
		}
	}
	return cfg, nil
}
