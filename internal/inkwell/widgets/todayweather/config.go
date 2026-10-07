package todayweather

import "github.com/grantlucas/inkwell/internal/inkwell/widgets/daydata"

// widgetName prefixes every config error so a dashboard that fails to
// load says which widget rejected it.
const widgetName = "today-weather"

// spec declares today-weather to the shared parser. It draws no events, so
// it takes the weather settings every day widget takes and nothing from
// the calendar. weather-ahead's days is explained rather than listed as
// unknown, because the two sit side by side and a key copied between them
// is the likely mistake.
var spec = daydata.Spec{
	Widget: widgetName,
	Reads:  daydata.WeatherOnly,
	Rejected: map[string]string{
		"days": "today-weather always shows today; weather-ahead shows the days after it",
	},
}
