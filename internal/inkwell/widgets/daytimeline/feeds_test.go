package daytimeline

import (
	"strings"
	"testing"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar"
	"github.com/grantlucas/inkwell/internal/inkwell/testutil"
	"github.com/grantlucas/inkwell/internal/inkwell/testutil/fakehttp"
	"github.com/grantlucas/inkwell/internal/inkwell/weather"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
)

// The feeds seam: the widget built by its Factory, fetching through the
// real calendar module from a fake HTTP client, as the dashboard builds
// it. The in-memory day data the other tests use doesn't collapse
// duplicates; the calendar module does, once, for every widget.

const (
	workFeed   = "https://work.example/cal.ics"
	familyFeed = "https://family.example/cal.ics"
)

// vcalendar wraps VEVENT blocks in a VCALENDAR.
func vcalendar(events ...string) string {
	return "BEGIN:VCALENDAR\r\n" + strings.Join(events, "") + "END:VCALENDAR\r\n"
}

// vevent is a timed event today, its times in UTC.
func vevent(uid, summary string, from, to time.Time) string {
	const f = "20060102T150405Z"
	return "BEGIN:VEVENT\r\nUID:" + uid + "\r\nDTSTART:" + from.UTC().Format(f) + "\r\nDTEND:" + to.UTC().Format(f) +
		"\r\nSUMMARY:" + summary + "\r\nEND:VEVENT\r\n"
}

// fromFeeds renders the widget built by Factory, its feeds served by tr.
func fromFeeds(t *testing.T, tr *fakehttp.Client, feeds ...any) *Widget {
	t.Helper()
	clock := fixedClock(testTime)
	deps := widget.Deps{
		Now:      clock,
		Calendar: calendar.NewProvider(tr, clock),
		Weather:  weather.NewProvider(tr, time.Hour, clock, weather.Settings{}),
	}
	w, err := Factory(testBounds, map[string]any{"feeds": feeds}, deps)
	if err != nil {
		t.Fatalf("Factory: %v", err)
	}
	return w.(*Widget)
}

// One event carried by two feeds, as when a meeting is on a work calendar
// and shared to a family one, is one block the column's width: not two
// events clashing side by side. The copies needn't share a UID, only a
// title, start and end.
func TestWidget_DuplicatesFromTwoFeedsAreOneBlock(t *testing.T) {
	tr := fakehttp.New()
	tr.Serve(workFeed, vcalendar(
		vevent("work-1", "Parent-teacher interview", at(16, 0), at(17, 0)),
		vevent("work-2", "Standup", at(9, 0), at(9, 30)),
	))
	tr.Serve(familyFeed, vcalendar(
		vevent("family-9", "Parent-teacher interview", at(16, 0), at(17, 0)),
		vevent("family-3", "Swim lessons", at(18, 30), at(19, 15)),
	))
	frame := renderToFrame(t, fromFeeds(t, tr, workFeed, familyFeed))

	l, tl := gridOf(testBounds, defaultConfig().Window)
	y := tl.y(at(16, 30))
	for _, x := range []int{l.Events.Min.X, l.Events.Min.X + l.Events.Dx()/2, l.Events.Max.X - 1} {
		if frame.ColorIndexAt(x, y) != widget.PaperBlack {
			t.Errorf("paper at x=%d across the 16:00 block; want one block the column's width", x)
		}
	}
	testutil.AssertGoldenPNG(t, frame)
}
