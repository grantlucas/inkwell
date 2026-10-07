package widgets_test

import (
	"image"
	"net/http"
	"testing"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar"
	"github.com/grantlucas/inkwell/internal/inkwell/testutil"
	"github.com/grantlucas/inkwell/internal/inkwell/testutil/fakehttp"
	"github.com/grantlucas/inkwell/internal/inkwell/weather"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets"
)

// eventWidgets are the widgets that list events through the event list,
// each built from config as a dashboard builds it.
var eventWidgets = []string{"bold-five", "today-hero", "row-agenda", "event-list"}

// feedClient serves composeDeps' forecast, and the calendar feed with
// feed.
func feedClient(feed fakehttp.Reply) *fakehttp.Client {
	c := fakehttp.New()
	c.Set(composeFeed, feed)
	c.Handle(composeForecast, fakehttp.OpenMeteo{
		Now: composeNow, Site: composeNow.Location(), Sky: composeSky,
		Codes: []int{80, 63, 2, 45, 3, 0, 0, 0},
	}.Reply)
	return c
}

// renderFrom builds typeName over the whole panel on client, as the
// dashboard builds it, with its clock reading *clock, and returns what
// renders it: one widget, so its calendar cache lasts across renders.
func renderFrom(t *testing.T, typeName string, client *fakehttp.Client, clock *time.Time) func() *image.Paletted {
	t.Helper()
	now := func() time.Time { return *clock }
	deps := widget.Deps{
		Now:      now,
		Calendar: calendar.NewProvider(client, now),
		Weather: weather.NewProvider(client, time.Hour, now, weather.Settings{
			Location: weather.Location{Latitude: 43.25, Longitude: -79.87}, TempUnit: "C", Model: weather.ModelGEM,
		}),
	}
	panel := image.Rect(0, 0, 800, 480)
	w, err := widgets.NewDefaultRegistry().Create(typeName, panel, map[string]any{"feeds": []any{composeFeed}}, deps)
	if err != nil {
		t.Fatalf("Create %s: %v", typeName, err)
	}
	return func() *image.Paletted {
		frame := image.NewPaletted(panel, widget.PaperPalette)
		if err := w.Render(frame); err != nil {
			t.Fatalf("Render %s: %v", typeName, err)
		}
		return frame
	}
}

// Every widget that lists events tells a calendar it could not read from
// one with nothing on it: a feed down with nothing cached draws
// differently from a feed that answered with an empty calendar. A feed
// that fails with a good copy cached draws that copy exactly as if it had
// answered, and says nothing.
func TestEventWidgets_CalendarUnavailable(t *testing.T) {
	down := fakehttp.Reply{Status: http.StatusInternalServerError}
	empty := fakehttp.Reply{Body: "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nEND:VCALENDAR\r\n"}
	week := fakehttp.Reply{Body: composeWeek()}
	panel := image.Rect(0, 0, 800, 480)

	for _, typeName := range eventWidgets {
		t.Run(typeName+": down with nothing cached is not an empty calendar", func(t *testing.T) {
			clock := composeNow
			if testutil.SameIn(renderFrom(t, typeName, feedClient(down), &clock)(), renderFrom(t, typeName, feedClient(empty), &clock)(), panel) {
				t.Error("drew a calendar it could not read the same as an empty one")
			}
		})
		t.Run(typeName+": down with a cached copy draws the copy", func(t *testing.T) {
			clock := composeNow.Add(-time.Hour)
			client := feedClient(week)
			render := renderFrom(t, typeName, client, &clock)
			render()
			clock = composeNow
			client.Set(composeFeed, down)
			got := render()

			healthyClock := composeNow
			if !testutil.SameIn(got, renderFrom(t, typeName, feedClient(week), &healthyClock)(), panel) {
				t.Error("drew differently from the feed answering, want the cached copy drawn as if it had")
			}
		})
	}
}
