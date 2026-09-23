package daygrid

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar"
	"github.com/grantlucas/inkwell/internal/inkwell/weather"
)

// FetchTimeout bounds a screen's calendar and weather fetches
// *together*, so a slow upstream on either side cannot stall the render
// loop for longer than this in total. The loop is what the budget
// protects, and it does not care which of the two was slow — giving
// each its own budget would double the worst-case stall.
const FetchTimeout = 10 * time.Second

// Fetch gets a screen's calendar events and forecast, concurrently,
// under one shared deadline.
//
// Concurrently because sequentially they starve each other: a calendar
// fetch that ate the whole budget handed the forecast an already
// expired context, and that render fell back to the weather cache or
// drew no weather at all (issue #111). Nothing about the forecast
// depends on the events, so there is no ordering to preserve, and
// running them together keeps the total bound at FetchTimeout rather
// than widening it.
//
// Neither failure is returned. A fetch failure must not blank the
// panel, so each side logs — named with widgetName, so the line says
// which screen — and the caller renders with whatever arrived. A nil
// weather source is not a failure: it means the screen was configured
// without weather.
func Fetch(ctx context.Context, widgetName string, cal calendar.Source, ws weather.Source, days []Day, loc weather.Location) ([]calendar.Event, []weather.DailyForecast) {
	start, end := Window(days)

	var (
		wg       sync.WaitGroup
		events   []calendar.Event
		forecast []weather.DailyForecast
	)

	wg.Go(func() {
		got, err := cal.Events(ctx, start, end)
		if err != nil {
			log.Printf("%s: fetch calendar events: %v", widgetName, err)
		}
		// Kept even alongside an error: a partial result is still
		// worth drawing, and the sources return what they managed.
		events = got
	})

	if ws != nil {
		wg.Go(func() {
			f, err := ws.Forecast(ctx, loc, len(days))
			if err != nil {
				log.Printf("%s: fetch weather forecast: %v", widgetName, err)
			}
			if f != nil {
				forecast = f.Days
			}
		})
	}

	wg.Wait()
	return events, forecast
}

// FetchContext returns the render-scope context the screens share, and
// its cancel. Split out so every screen spells the budget the same way.
func FetchContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), FetchTimeout)
}
