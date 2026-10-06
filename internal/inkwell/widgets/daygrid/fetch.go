package daygrid

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar"
	"github.com/grantlucas/inkwell/internal/inkwell/weather"
)

// fetchTimeout bounds a screen's calendar and weather fetches
// *together*, so a slow upstream on either side cannot stall the render
// loop for longer than this in total. The loop is what the budget
// protects, and it does not care which of the two was slow — giving
// each its own budget would double the worst-case stall.
const fetchTimeout = 10 * time.Second

// fetch gets a screen's calendar events and forecast, concurrently,
// under one shared deadline.
//
// Concurrently because sequentially they starve each other: a calendar
// fetch that ate the whole budget handed the forecast an already
// expired context, and that render fell back to the weather cache or
// drew no weather at all (issue #111). Nothing about the forecast
// depends on the events, so there is no ordering to preserve, and
// running them together keeps the total bound at fetchTimeout rather
// than widening it.
//
// Neither failure is returned. A fetch failure must not blank the
// panel, so each side logs — named with widgetName, so the line says
// which screen — and the caller renders with whatever arrived. A nil
// weather source is not a failure: it means the screen was configured
// without weather.
func fetch(ctx context.Context, widgetName string, cal calendar.Source, ws weather.Source, days []Day, loc weather.Location) fetched {
	start, end := window(days)

	var (
		wg  sync.WaitGroup
		out fetched
	)

	wg.Go(func() {
		got, err := cal.Events(ctx, start, end)
		if err != nil {
			log.Printf("%s: fetch calendar events: %v", widgetName, err)
		}
		// Kept even alongside an error: a partial result is still
		// worth drawing, and the sources return what they managed.
		out.events = got
	})

	if ws != nil {
		wg.Go(func() {
			f, err := ws.Forecast(ctx, loc, len(days))
			if err != nil {
				log.Printf("%s: fetch weather forecast: %v", widgetName, err)
			}
			if f != nil {
				out.forecast, out.arrived = f.Days, true
			}
		})
	}

	wg.Wait()
	return out
}

// fetched is what one render's fetch produced: the events, the
// forecast's days, and whether a forecast arrived at all. A 200 response
// with no daily data arrived, carrying no days.
type fetched struct {
	events   []calendar.Event
	forecast []weather.DailyForecast
	arrived  bool
}

// fetchContext returns the render-scope context the screens share, and
// its cancel. Split out so every screen spells the budget the same way.
func fetchContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), fetchTimeout)
}
