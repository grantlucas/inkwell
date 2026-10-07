package daydata

import (
	"sort"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar"
	"github.com/grantlucas/inkwell/internal/inkwell/weather"
)

// Day is one column or row of the grid: the local midnight it starts
// at, and whether it is today.
//
// The end is a method rather than a field so it cannot be left out. A
// Day literal missing its end would make the timed-event branch of
// filterEventsForDay compare against the zero time, which is never
// after anything — the column would come back with its all-day events
// and none of its timed ones, which looks like a quiet day rather than
// a bug. daysFrom is the intended producer, but the type is exported and
// tests build literals, so the trap is worth closing in the type.
type Day struct {
	Start   time.Time
	IsToday bool

	// Events are the day's events, all-day first and then by start, as
	// filterEventsForDay buckets them.
	Events []calendar.Event
	// Forecast is the day's forecast, or nil when the forecast does not
	// reach it. Nil is the absence: a zero DailyForecast would read as a
	// clear day at 0°C.
	Forecast *weather.DailyForecast
}

// NoForecast is what a widget says for a day the forecast doesn't reach,
// rather than draw numbers nobody forecast.
const NoForecast = "NO FORECAST"

// End is the local midnight the day runs to, exclusive.
func (d Day) End() time.Time { return d.Start.AddDate(0, 0, 1) }

// daysFrom builds n consecutive days starting from the local midnight of
// now, in now's own location.
//
// The clock is injected rather than read here, and the zone comes from
// the clock rather than being re-resolved: the dashboard hands every
// widget a time already in the display zone, so a widget that called
// time.Now().In(somewhere) would quietly disagree with the rest of the
// panel about which day it is.
func daysFrom(now time.Time, n int) []Day {
	loc := now.Location()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)

	days := make([]Day, 0, max(n, 0))
	for i := range n {
		days = append(days, Day{
			Start:   today.AddDate(0, 0, i),
			IsToday: i == 0,
		})
	}
	return days
}

// window returns the span the day list covers, which is what a calendar
// or forecast fetch should ask for.
func window(days []Day) (start, end time.Time) {
	if len(days) == 0 {
		return time.Time{}, time.Time{}
	}
	return days[0].Start, days[len(days)-1].End()
}

// filterEventsForDay returns the events overlapping the day, all-day
// first and then by start time.
//
// All-day events are calendar date labels, not instants. An iCal
// VALUE=DATE is anchored to UTC midnight by the parser, but day columns
// are built in the viewer's local zone, so comparing the two as
// instants leaks an all-day event into the previous local day in any
// negative-UTC zone — a Thursday trip showing up on Wednesday in
// America/Toronto. All-day events are therefore bucketed by their date
// components alone, zone-independently, and instant overlap is reserved
// for timed events.
//
// This is the function issue #92 exists for: three independent copies
// would drift, and the failure is silent and off by one day.
func filterEventsForDay(events []calendar.Event, day Day) []calendar.Event {
	col := dateOnly(day.Start)
	var out []calendar.Event
	for _, e := range events {
		var overlaps bool
		if e.AllDay {
			// DTEND is exclusive, so the column's date must satisfy
			// start <= col < end.
			overlaps = !col.Before(dateOnly(e.Start)) && col.Before(dateOnly(e.End))
		} else {
			overlaps = e.Start.Before(day.End()) && e.End.After(day.Start)
		}
		if overlaps {
			out = append(out, e)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		// All-day events sort first: they apply to the whole day, so
		// slotting them among the timed events by instant would put
		// them at an arbitrary point in the list.
		if out[i].AllDay != out[j].AllDay {
			return out[i].AllDay
		}
		return out[i].Start.Before(out[j].Start)
	})
	return out
}

// dateOnly strips the clock and zone from t, returning a comparable
// midnight-UTC anchor of its calendar date.
func dateOnly(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// forecastFor returns a copy of the day's forecast, or nil when the
// forecast does not reach that far. Matching is by calendar date rather
// than by instant, for the same zone reason as the all-day bucketing
// above.
func forecastFor(days []weather.DailyForecast, day Day) *weather.DailyForecast {
	for _, d := range days {
		if d.Date.Year() == day.Start.Year() && d.Date.YearDay() == day.Start.YearDay() {
			return &d
		}
	}
	return nil
}
