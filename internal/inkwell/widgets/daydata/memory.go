package daydata

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
func InMemory(events []calendar.Event, forecast []weather.DailyForecast, opts ...MemoryOption) Source {
	m := memory{fetched: fetched{events: events, forecast: forecast, arrived: forecast != nil}}
	for _, opt := range opts {
		opt(&m)
	}
	return m
}

// MemoryOption adjusts what the in-memory adapter serves.
type MemoryOption func(*memory)

// CalendarDown serves the days as if a feed had nothing to draw from, so
// every render reports CalendarUnavailable. events are still served: they
// are the feeds that did answer.
func CalendarDown() MemoryOption {
	return func(m *memory) { m.calendarUnavailable = true }
}

type memory struct {
	fetched
}

func (m memory) Days(now time.Time, n int) Data {
	return assemble(daysFrom(now, n), m.fetched)
}
