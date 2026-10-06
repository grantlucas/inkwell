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
// only the shared settings. The weekly-calendar keys it has no equivalent
// for are rejected with the reason rather than ignored: silently dropping
// show_weather: false would draw a weather band the operator explicitly
// turned off, which looks like a bug in the widget rather than a key that
// did not carry over.
var spec = daygrid.Spec{
	Widget:    widgetName,
	MaxEvents: defaultMaxEvents,
	Rejected: map[string]string{
		"days":               "bold-five is always five columns; every type size is derived from a 160 px column",
		"week_start":         "columns always start from today, so there is no week to start",
		"show_weather":       "the weather band is part of the layout; a day with no forecast already draws nothing",
		"show_weather_label": "there is no condition label to show — the icon carries the condition",
		"highlight_hour":     "the now-marker follows the clock, on today's column only",
	},
}
