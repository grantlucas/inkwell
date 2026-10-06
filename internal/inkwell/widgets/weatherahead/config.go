package weatherahead

import (
	"github.com/grantlucas/inkwell/internal/inkwell/weather"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daydata"
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
var spec = daydata.Spec{
	Widget:      widgetName,
	WeatherOnly: true,
	Extra:       []string{"days"},
}

// Config is weather-ahead's parsed configuration: the shared weather
// settings and how many days it lists.
type Config struct {
	daydata.Config
	// Days is how many days after today the widget lists, one row each.
	Days int
}

// parseConfig reads the shared settings through the shared parser,
// inheriting weather settings from inherit, then days.
func parseConfig(raw map[string]any, inherit *weather.Provider) (Config, error) {
	shared, err := daydata.ParseConfig(spec, raw, inherit)
	cfg := Config{Config: shared, Days: defaultDays}
	if err != nil {
		return cfg, err
	}
	keys := daydata.ReadKeys(widgetName, raw)
	keys.Int("days", 1, maxDays, &cfg.Days)
	return cfg, keys.Err()
}

// Factory creates a weather-ahead Widget from config and dependencies.
var Factory = daydata.Factory(widgetName, parseConfig, New)
