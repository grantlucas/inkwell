package boldfive

import "github.com/grantlucas/inkwell/internal/inkwell/widgets/daygrid"

const (
	// defaultMaxEvents is four rather than weekly's five: the taller
	// line height this screen exists for costs one event per column.
	defaultMaxEvents = 4

	// widgetName prefixes every config error so a dashboard that fails
	// to load says which widget rejected it.
	widgetName = "bold-five"
)

// spec declares bold-five to the shared calendar-widget parser. It takes
// only the shared settings: it is always five columns from today, its
// weather band is part of the layout and its now-marker follows the clock,
// so weekly-calendar's days, week_start, show_weather, show_weather_label
// and highlight_hour have no meaning here and are rejected.
var spec = daygrid.Spec{Widget: widgetName, MaxEvents: defaultMaxEvents}
