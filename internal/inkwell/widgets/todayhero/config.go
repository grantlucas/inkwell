package todayhero

import "github.com/grantlucas/inkwell/internal/inkwell/widgets/daydata"

const (
	// defaultMaxEvents is three: the hero agenda spends its height on a
	// scaled time and a wrapped title, so three is what fits before the
	// "+N MORE" line.
	defaultMaxEvents = 3

	// widgetName prefixes every config error so a dashboard that fails
	// to load says which widget rejected it.
	widgetName = "today-hero"
)

// spec declares today-hero to the shared calendar-widget parser. It takes
// only the shared settings. The weekly-calendar keys it has no equivalent
// for are rejected with the reason rather than ignored: the docs promise
// the config is swappable between the two, so a leftover key is a
// reasonable thing to find in a pasted config, and silently dropping
// show_weather: false would draw a weather band the operator explicitly
// turned off, which looks like a bug in the widget rather than a key that
// did not carry over.
var spec = daydata.Spec{
	Widget:    widgetName,
	MaxEvents: defaultMaxEvents,
	Rejected: map[string]string{
		"days":               "today-hero is always today plus four rows; the split is what makes the hero column readable",
		"week_start":         "the panel always starts from today, so there is no week to start",
		"show_weather":       "the weather band is part of the layout; a day with no forecast already draws nothing",
		"show_weather_label": "the condition label is part of the hero block and has no narrow-cell equivalent to turn off",
		"highlight_hour":     "the now-marker follows the clock, on today's chart only",
	},
}
