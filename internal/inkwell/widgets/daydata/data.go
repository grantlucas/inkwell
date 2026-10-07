// Package daydata is the day data module every day widget draws from. A
// widget parses its settings with ParseConfig, builds its module with New,
// and on each render asks it for n days from now: each day's date, whether
// it is Today, its events and its forecast, plus the shared temperature
// range. The concurrent calendar and weather fetch under one deadline, the
// all-day convention, matching forecasts to days and the range all happen
// inside it, so a widget is only layout and drawing.
//
// The part that really must not be copied is the all-day bucketing in
// filterEventsForDay: an iCal VALUE=DATE is anchored to UTC midnight by
// the parser while days are built in the viewer's local zone, so comparing
// them as instants leaks an all-day event into the previous local day in
// any negative-UTC zone. Independent copies of that would drift, and the
// failure is silent and off by one day.
//
// Every error message takes a widget name, so a dashboard that fails to
// load still says which widget rejected the config. The text and rule
// helpers the day widgets draw with are drawkit's.
package daydata

import (
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar"
	"github.com/grantlucas/inkwell/internal/inkwell/weather"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/weatherview"
)

// Source is the day data module's interface: given the current time and a
// number of days, it returns everything a widget needs to draw them.
//
// It has two adapters. New is the production one, fetching through the
// dashboard's shared calendar module and weather provider. InMemory serves
// fixed events and forecasts, for widget tests.
type Source interface {
	Days(now time.Time, n int) Data
}

// Data is one render's days, as one widget sees them.
type Data struct {
	Days []Day
	// TempRange is the shared temperature range across the days that
	// have a forecast, so every chart a widget draws compares on one
	// scale. With no forecast at all it is GlobalTempRange's fallback.
	TempRange weatherview.TempRange
	// ForecastArrived is whether a forecast came back at all, even one
	// that reaches none of the days. Whether a day has weather is its own
	// Forecast; this is for a layout that sizes itself on the forecast's
	// arrival, as weekly-calendar's band does, so a 200 response with no
	// daily data doesn't reflow the screen for one cycle.
	ForecastArrived bool
	// CalendarUnavailable is whether any of the widget's feeds had nothing
	// to draw from on this render: its fetch failed and there was no
	// earlier good copy. The days then carry only the other feeds' events,
	// and a day with none would read as a free day, so a widget says the
	// calendar is missing (NoCalendar) rather than draw it empty. A feed
	// that failed while it had a copy cached served that copy, and is not
	// unavailable.
	CalendarUnavailable bool
}

// NoCalendar is what a widget says when its calendar is unavailable,
// rather than let an empty day stand for one it couldn't read.
const NoCalendar = "CALENDAR UNAVAILABLE"

// New builds a widget's day data module from its configuration and the
// dependencies the app hands every widget. A missing dependency is a
// wiring fault, reported with widgetName.
func New(widgetName string, cfg Config, deps widget.Deps, opts ...Option) (Source, error) {
	if err := requireDeps(widgetName, deps); err != nil {
		return nil, err
	}
	m := &module{
		widget:   widgetName,
		cal:      deps.Calendar.Source(cfg.Feeds, cfg.Refresh),
		weather:  deps.Weather.SourceForModel(cfg.Weather.Model),
		location: cfg.Weather.location(),
	}
	for _, opt := range opts {
		opt(m)
	}
	return m, nil
}

// Option adjusts how New builds a widget's module.
type Option func(*module)

// WithoutWeather leaves the forecast unfetched, so every day's Forecast is
// nil and ForecastArrived is false. weekly-calendar passes it for
// show_weather: false, so a screen that hides its weather doesn't fetch a
// forecast to throw away.
func WithoutWeather() Option {
	return func(m *module) { m.weather = nil }
}

// module is the production adapter.
type module struct {
	widget   string
	cal      calendar.Source
	weather  weather.Source
	location weather.Location
}

func (m *module) Days(now time.Time, n int) Data {
	days := daysFrom(now, n)
	ctx, cancel := fetchContext()
	defer cancel()
	got := fetch(ctx, m.widget, m.cal, m.weather, days, m.location)
	return assemble(days, got)
}

// assemble gives each day its events and its forecast, and takes the
// shared temperature range across the days that have one.
func assemble(days []Day, got fetched) Data {
	var known []weather.DailyForecast
	for i := range days {
		days[i].Events = filterEventsForDay(got.events, days[i])
		if f := forecastFor(got.forecast, days[i]); f != nil {
			days[i].Forecast = f
			known = append(known, *f)
		}
	}
	return Data{
		Days:                days,
		TempRange:           weatherview.GlobalTempRange(known),
		ForecastArrived:     got.arrived,
		CalendarUnavailable: got.calendarUnavailable,
	}
}
