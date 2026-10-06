// Package daygrid is the day data module every day widget draws from. A
// widget parses its settings with ParseConfig, builds its module with New,
// and on each render asks it for n days from now: each day's date, whether
// it is Today, its events and its forecast, plus the shared temperature
// range. The concurrent calendar and weather fetch under one deadline, the
// all-day convention, matching forecasts to days and the range all happen
// inside it, so a widget is only layout and drawing.
//
// The part that really must not be copied is the all-day bucketing in
// FilterEventsForDay: an iCal VALUE=DATE is anchored to UTC midnight by
// the parser while days are built in the viewer's local zone, so comparing
// them as instants leaks an all-day event into the previous local day in
// any negative-UTC zone. Independent copies of that would drift, and the
// failure is silent and off by one day.
//
// It also holds the drawing helpers the day widgets share. Every error
// message takes a widget name, so a dashboard that fails to load still
// says which widget rejected the config.
package daygrid

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
}

// New builds a widget's day data module from its configuration and the
// dependencies the app hands every widget. A missing dependency is a
// wiring fault, reported with widgetName.
func New(widgetName string, cfg Config, deps widget.Deps) (Source, error) {
	if err := RequireDeps(widgetName, deps); err != nil {
		return nil, err
	}
	return &module{
		widget:   widgetName,
		cal:      deps.Calendar.Source(cfg.Feeds, cfg.Refresh),
		weather:  deps.Weather.SourceForModel(cfg.Weather.Model),
		location: cfg.Weather.Location(),
	}, nil
}

// module is the production adapter.
type module struct {
	widget   string
	cal      calendar.Source
	weather  weather.Source
	location weather.Location
}

func (m *module) Days(now time.Time, n int) Data {
	days := Days(now, n)
	ctx, cancel := FetchContext()
	defer cancel()
	fetched := Fetch(ctx, m.widget, m.cal, m.weather, days, m.location)
	return assemble(days, fetched.Events, fetched.Days())
}

// assemble gives each day its events and its forecast, and takes the
// shared temperature range across the days that have one.
func assemble(days []Day, events []calendar.Event, forecast []weather.DailyForecast) Data {
	var known []weather.DailyForecast
	for i := range days {
		days[i].Events = FilterEventsForDay(events, days[i])
		if f := forecastFor(forecast, days[i]); f != nil {
			days[i].Forecast = f
			known = append(known, *f)
		}
	}
	return Data{Days: days, TempRange: weatherview.GlobalTempRange(known)}
}
