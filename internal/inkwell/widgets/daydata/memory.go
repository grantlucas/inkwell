package daydata

import (
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar"
	"github.com/grantlucas/inkwell/internal/inkwell/weather"
)

// Memory is the day data module's in-memory adapter: it serves fixed
// events and forecasts as days, built exactly as the production module
// builds them from what it fetched. Widget goldens draw from it, so they
// need no calendar, weather or HTTP stubs of their own.
//
// Events are bucketed into whichever days are asked for; ones outside them
// are dropped. A nil Forecast is a forecast that never arrived.
type Memory struct {
	Events   []calendar.Event
	Forecast []weather.DailyForecast
	// CalendarUnavailable serves the days as if a feed were unavailable,
	// so every render reports it. Events are still served: they are the
	// feeds that did answer.
	CalendarUnavailable bool
}

// InMemory serves events and forecast from a Memory whose feeds all
// answered.
func InMemory(events []calendar.Event, forecast []weather.DailyForecast) Source {
	return Memory{Events: events, Forecast: forecast}
}

// Days builds the n days starting with now's from what m holds.
func (m Memory) Days(now time.Time, n int) Data {
	return assemble(daysFrom(now, n), fetched{
		events:              m.Events,
		calendarUnavailable: m.CalendarUnavailable,
		forecast:            m.Forecast,
		arrived:             m.Forecast != nil,
	})
}
