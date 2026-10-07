package widgets_test

import (
	"image"
	"net/http"
	"testing"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/testutil"
	"github.com/grantlucas/inkwell/internal/inkwell/testutil/fakehttp"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets"
)

// eventWidgets are the widgets that list events through the event list,
// each built from config as a dashboard builds it.
var eventWidgets = []string{"bold-five", "today-hero", "row-agenda", "event-list"}

// renderFrom builds typeName over the whole panel on client, as the
// dashboard builds it, with its clock reading *clock, and returns what
// renders it: one widget, so its calendar cache lasts across renders.
func renderFrom(t *testing.T, typeName string, client *fakehttp.Client, clock *time.Time) func() *image.Paletted {
	t.Helper()
	deps := composeDeps(client, func() time.Time { return *clock })
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
// one with nothing on it: an unavailable feed draws differently from a
// feed that answered with an empty calendar. A feed that fails with a good
// copy cached draws that copy exactly as if it had answered, and says
// nothing.
func TestEventWidgets_CalendarUnavailable(t *testing.T) {
	failing := fakehttp.Reply{Status: http.StatusInternalServerError}
	empty := fakehttp.Reply{Body: "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nEND:VCALENDAR\r\n"}
	week := fakehttp.Reply{Body: composeWeek()}
	panel := image.Rect(0, 0, 800, 480)

	for _, typeName := range eventWidgets {
		t.Run(typeName+": unavailable is not an empty calendar", func(t *testing.T) {
			clock := composeNow
			if testutil.SameIn(renderFrom(t, typeName, composeClient(failing), &clock)(), renderFrom(t, typeName, composeClient(empty), &clock)(), panel) {
				t.Error("drew a calendar it could not read the same as an empty one")
			}
		})
		t.Run(typeName+": failing with a cached copy draws the copy", func(t *testing.T) {
			clock := composeNow.Add(-time.Hour)
			client := composeClient(week)
			render := renderFrom(t, typeName, client, &clock)
			render()
			clock = composeNow
			client.Set(composeFeed, failing)
			got := render()

			healthyClock := composeNow
			if !testutil.SameIn(got, renderFrom(t, typeName, composeClient(week), &healthyClock)(), panel) {
				t.Error("drew differently from the feed answering, want the cached copy drawn as if it had")
			}
		})
	}
}
