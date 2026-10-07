package daytimeline

import (
	"fmt"

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

// spec declares day-timeline to the shared parser: the shared settings,
// and the window keys, which parseConfig reads itself. It lists no fixed
// number of events, so max_events is turned away with the reason rather
// than the bare list of accepted keys.
var spec = daydata.Spec{
	Widget: widgetName,
	Extra:  []string{"end_hour", "start_hour"},
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

// parseConfig validates and extracts config values: the shared settings
// through the shared parser, inheriting weather settings from inherit,
// then each edge of the window, then the window as a whole.
func parseConfig(raw map[string]any, inherit *weather.Provider) (Config, error) {
	shared, err := daydata.ParseConfig(spec, raw, inherit)
	cfg := Config{
		Config: shared,
		Window: Window{StartHour: defaultStartHour, EndHour: defaultEndHour},
	}
	if err != nil {
		return cfg, err
	}
	keys := daydata.ReadKeys(widgetName, raw)
	keys.Int("start_hour", 0, 24, &cfg.Window.StartHour)
	keys.Int("end_hour", 0, 24, &cfg.Window.EndHour)
	if err := keys.Err(); err != nil {
		return cfg, err
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
