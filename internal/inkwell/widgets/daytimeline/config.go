package daytimeline

import (
	"fmt"
	"maps"
	"slices"

	"github.com/grantlucas/inkwell/internal/inkwell/weather"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daydata"
)

const (
	// widgetName prefixes every config error so a dashboard that fails
	// to load says which widget rejected it.
	widgetName = "day-timeline"

	// The default window runs from 7 am to 10 pm: the waking day, with
	// no setup.
	defaultStartHour = 7
	defaultEndHour   = 22

	// minSpan is the narrowest window accepted. Narrower than six hours
	// the grid is a few tall rows that say less than a list would, which
	// is more likely a typo than a choice.
	minSpan = 6
)

// spec declares day-timeline to the shared calendar-widget parser: the
// shared settings, and the window keys, which parseConfig reads itself.
// It lists no fixed number of events, so max_events is turned away with
// the reason rather than the bare list of accepted keys.
var spec = daydata.Spec{
	Widget: widgetName,
	Extra:  slices.Sorted(maps.Keys(ownKeys)),
	Rejected: map[string]string{
		"max_events": "every event in the window is placed at its time, and the rest are counted as earlier or later",
	},
}

// Config holds parsed day-timeline configuration: the settings every
// calendar widget shares, and the window.
type Config struct {
	daydata.Config
	Window Window
}

// Window is the span of today the grid shows, in whole hours of the
// display zone. EndHour is exclusive, and 24 is midnight at the end of
// the day.
type Window struct {
	StartHour int
	EndHour   int
}

// ownKeys are the settings only day-timeline takes, each with its parser.
// The shared parser accepts exactly these as the widget's own, so adding
// a key here is the whole change. Each checks its own value; whether the
// two make a window together is checked once both are read.
var ownKeys = map[string]func(*Config, any) error{
	"start_hour": func(c *Config, v any) (err error) {
		c.Window.StartHour, err = parseHour("start_hour", v)
		return err
	},
	"end_hour": func(c *Config, v any) (err error) {
		c.Window.EndHour, err = parseHour("end_hour", v)
		return err
	},
}

// parseHour reads one window edge: a whole hour from 0 to 24. YAML
// decodes 7 as an int and 7.5 as a float, so anything but an int is a
// fraction or not a number at all.
func parseHour(key string, v any) (int, error) {
	n, ok := v.(int)
	if !ok {
		return 0, fmt.Errorf("%s: %s must be a whole number of hours, got %#v", widgetName, key, v)
	}
	if n < 0 || n > 24 {
		return 0, fmt.Errorf("%s: %s must be from 0 to 24, got %d", widgetName, key, n)
	}
	return n, nil
}

// parseConfig validates and extracts config values: the shared settings
// through the shared parser, inheriting weather settings from inherit,
// then the window keys, in sorted order so the first error named is the
// same on every run, then the window as a whole.
func parseConfig(raw map[string]any, inherit *weather.Provider) (Config, error) {
	shared, err := daydata.ParseConfig(spec, raw, inherit)
	cfg := Config{
		Config: shared,
		Window: Window{StartHour: defaultStartHour, EndHour: defaultEndHour},
	}
	if err != nil {
		return cfg, err
	}
	for _, key := range spec.Extra {
		if v, ok := raw[key]; ok {
			if err := ownKeys[key](&cfg, v); err != nil {
				return cfg, err
			}
		}
	}
	return cfg, cfg.Window.validate()
}

// validate checks that the two edges make a window worth drawing.
func (w Window) validate() error {
	if w.EndHour <= w.StartHour {
		return fmt.Errorf("%s: end_hour (%d) must be after start_hour (%d)", widgetName, w.EndHour, w.StartHour)
	}
	if span := w.EndHour - w.StartHour; span < minSpan {
		return fmt.Errorf("%s: start_hour %d to end_hour %d is %d hours; the window must span at least %d",
			widgetName, w.StartHour, w.EndHour, span, minSpan)
	}
	return nil
}
