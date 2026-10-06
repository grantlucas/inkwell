package weatherahead

import (
	"fmt"
	"image"

	"github.com/grantlucas/inkwell/internal/inkwell/weather"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daygrid"
)

// widgetName prefixes every config error so a dashboard that fails to
// load says which widget rejected it.
const widgetName = "weather-ahead"

const (
	// defaultDays is how many days after today the widget lists when
	// config doesn't say: the rest of a five-day span beside
	// today-weather.
	defaultDays = 4
	// maxDays is a week ahead. Today plus seven is eight forecast days,
	// which every weather model reaches; further out the forecast is
	// mostly guesswork anyway.
	maxDays = 7
)

// spec declares weather-ahead to the shared parser: the weather settings
// every day widget takes, no calendar, and days, which parseConfig reads
// itself.
var spec = daygrid.Spec{
	Widget:      widgetName,
	WeatherOnly: true,
	Extra:       []string{"days"},
}

// Config is weather-ahead's parsed configuration: the shared weather
// settings and how many days it lists.
type Config struct {
	daygrid.Config
	// Days is how many days after today the widget lists, one row each.
	Days int
}

// parseConfig reads the shared settings through the shared parser,
// inheriting weather settings from inherit, then days.
func parseConfig(raw map[string]any, inherit *weather.Provider) (Config, error) {
	shared, err := daygrid.ParseConfig(spec, raw, inherit)
	cfg := Config{Config: shared, Days: defaultDays}
	if err != nil {
		return cfg, err
	}
	v, ok := raw["days"]
	if !ok {
		return cfg, nil
	}
	n, ok := v.(int)
	if !ok {
		return cfg, fmt.Errorf("%s: days must be an integer, got %T", widgetName, v)
	}
	if n < 1 || n > maxDays {
		return cfg, fmt.Errorf("%s: days must be in [1, %d], got %d", widgetName, maxDays, n)
	}
	cfg.Days = n
	return cfg, nil
}

// Factory creates a weather-ahead Widget from config and dependencies. It
// doesn't use daygrid.Factory, which hands the widget only the shared
// settings, because days is the widget's own.
func Factory(bounds image.Rectangle, config map[string]any, deps widget.Deps) (widget.Widget, error) {
	cfg, err := parseConfig(config, deps.Weather)
	if err != nil {
		return nil, err
	}
	days, err := daygrid.New(widgetName, cfg.Config, deps)
	if err != nil {
		return nil, err
	}
	return New(bounds, days, deps.Now, cfg), nil
}
