package daygrid

import (
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar"
	"github.com/grantlucas/inkwell/internal/inkwell/weather"
)

// InMemory is the day data module's in-memory adapter: it serves fixed
// events and forecasts as days, built exactly as the production module
// builds them from what it fetched. Widget goldens draw from it, so they
// need no calendar, weather or HTTP stubs of their own.
//
// events are bucketed into whichever days are asked for; ones outside them
// are dropped. A nil forecast is a forecast that never arrived.
func InMemory(events []calendar.Event, forecast []weather.DailyForecast) Source {
	return memory{events: events, forecast: forecast}
}

type memory struct {
	events   []calendar.Event
	forecast []weather.DailyForecast
}

func (m memory) Days(now time.Time, n int) Data {
	return assemble(Days(now, n), m.events, m.forecast)
}
