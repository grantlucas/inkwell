package rowagenda

import "github.com/grantlucas/inkwell/internal/inkwell/widgets/daydata"

// widgetName prefixes every config error so a dashboard that fails to
// load says which widget rejected it.
const widgetName = "row-agenda"

// spec declares row-agenda to the shared calendar-widget parser. It takes
// only the shared settings, and not max_events: a row grows to show every
// event the week has room for, and when the week is too full the busiest
// rows give up lines first, so a separate cap could only contradict that.
//
// max_events and the weekly-calendar keys this screen has no equivalent
// for are rejected with the reason rather than ignored. The docs promise
// the config is swappable between calendar widgets, so a leftover key is a
// reasonable thing to find in a pasted config, and silently dropping
// show_weather: false would draw a weather badge the operator explicitly
// turned off, which looks like a bug in the widget rather than a key that
// did not carry over.
var spec = daydata.Spec{
	Widget: widgetName,
	Rejected: map[string]string{
		"days":               "row-agenda is always five rows, so the screen shows the same span of days however full the week is",
		"max_events":         "each row grows to fit its events, and when the week is too full the busiest rows give up lines first",
		"week_start":         "the rows always start from today, so there is no week to start",
		"show_weather":       "the weather badge is part of the layout; a day with no forecast already draws nothing",
		"show_weather_label": "the badge has no condition label — the icon carries the condition",
		"highlight_hour":     "the now-marker follows the clock, on today's row only",
	},
}
