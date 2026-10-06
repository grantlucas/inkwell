package daygrid_test

import (
	"slices"
	"testing"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar"
	"github.com/grantlucas/inkwell/internal/inkwell/weather"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daygrid"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/weatherview"
)

// The in-memory adapter hands a widget fixed events and forecasts as days,
// built the way the production module builds them: events bucketed into
// their days, forecasts matched by date, and the range across the days
// that have one.
func TestInMemory(t *testing.T) {
	now := time.Date(2026, 3, 16, 14, 30, 0, 0, time.UTC)
	at := func(day, hour int) time.Time { return time.Date(2026, 3, day, hour, 0, 0, 0, time.UTC) }
	events := []calendar.Event{
		{Summary: "Standup", Start: at(16, 9), End: at(16, 10)},
		{Summary: "Dentist", Start: at(18, 10), End: at(18, 11)},
		{Summary: "Next week", Start: at(25, 10), End: at(25, 11)},
	}
	forecast := []weather.DailyForecast{
		{Date: at(16, 0), High: 14, Low: 6},
		{Date: at(17, 0), High: 9, Low: -2},
	}

	got := daygrid.InMemory(events, forecast).Days(now, 3)

	if g := dayEvents(got); !slices.Equal(g, []string{"Standup", "", "Dentist"}) {
		t.Errorf("events by day = %q", g)
	}
	for i, want := range []float64{14, 9} {
		if f := got.Days[i].Forecast; f == nil || f.High != want {
			t.Errorf("day %d forecast = %+v, want high %v", i, f, want)
		}
	}
	if got.Days[2].Forecast != nil {
		t.Errorf("day 2 forecast = %+v, want none", got.Days[2].Forecast)
	}
	if want := (weatherview.TempRange{Min: -2, Max: 14}); got.TempRange != want {
		t.Errorf("TempRange = %+v, want %+v", got.TempRange, want)
	}
	if !got.Days[0].IsToday || got.Days[1].IsToday {
		t.Error("only the first day is Today")
	}
}
