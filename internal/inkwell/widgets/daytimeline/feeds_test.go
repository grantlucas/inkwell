package daytimeline

import (
	"image"
	"maps"
	"net/http"
	"slices"
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

// fromFeeds is the widget built by Factory, its feeds served by tr.
func fromFeeds(t *testing.T, tr *fakehttp.Client, feeds ...any) *Widget {
	t.Helper()
	return fromFeedsAt(t, tr, fixedClock(testTime), feeds...)
}

// fromFeedsAt is fromFeeds on the dashboard clock clock, which a test can
// move on between renders.
func fromFeedsAt(t *testing.T, tr *fakehttp.Client, clock func() time.Time, feeds ...any) *Widget {
	t.Helper()
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

// A feed with nothing to draw from, its fetch failed with no earlier good
// copy, is said on the panel: an empty grid would read as a free day. The
// feeds that did answer still draw. A feed that fails with a good copy
// cached draws that copy, exactly as if it had answered, and says nothing.
func TestWidget_UnavailableCalendar(t *testing.T) {
	work := fakehttp.Reply{Body: vcalendar(
		vevent("work-1", "Standup", at(9, 0), at(9, 30)),
		vevent("work-2", "Design review", at(15, 0), at(16, 0)),
	)}
	family := fakehttp.Reply{Body: vcalendar(vevent("family-1", "Swim lessons", at(18, 30), at(19, 15)))}
	down := fakehttp.Reply{Status: http.StatusInternalServerError}

	tests := []struct {
		label string
		// earlier is what the feeds served on a render an hour before,
		// past the refresh setting; nil for no earlier render.
		earlier  map[string]fakehttp.Reply
		replies  map[string]fakehttp.Reply
		wantNote bool
	}{
		{label: "unavailable feed", replies: map[string]fakehttp.Reply{workFeed: down}, wantNote: true},
		{
			label:   "feed failing with a cached copy",
			earlier: map[string]fakehttp.Reply{workFeed: work},
			replies: map[string]fakehttp.Reply{workFeed: down},
		},
		{label: "one of two feeds unavailable", replies: map[string]fakehttp.Reply{workFeed: down, familyFeed: family}, wantNote: true},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			feeds := slices.Sorted(maps.Keys(tt.replies))
			healthy := fakehttp.New()
			for _, url := range feeds {
				healthy.Set(url, map[string]fakehttp.Reply{workFeed: work, familyFeed: family}[url])
			}

			tr := fakehttp.New()
			now := testTime
			w := fromFeedsAt(t, tr, func() time.Time { return now }, toAny(feeds)...)
			if tt.earlier != nil {
				now = testTime.Add(-time.Hour)
				for url, r := range tt.earlier {
					tr.Set(url, r)
				}
				renderToFrame(t, w)
				now = testTime
			}
			for url, r := range tt.replies {
				tr.Set(url, r)
			}
			frame := renderToFrame(t, w)

			// The top of the events column, where an ordinary day with
			// nothing early draws nothing.
			l, _ := gridOf(testBounds, defaultConfig().Window)
			band := image.Rect(l.Events.Min.X, testBounds.Min.Y, l.Events.Max.X, testBounds.Min.Y+noteH())
			if got := testutil.Inked(frame, band); got != tt.wantNote {
				t.Errorf("note band inked = %v, want %v", got, tt.wantNote)
			}
			if !tt.wantNote && !testutil.SameIn(frame, renderToFrame(t, fromFeeds(t, healthy, toAny(feeds)...)), testBounds) {
				t.Error("drew differently from every feed answering, want the cached copy drawn as if it had")
			}
			testutil.AssertGoldenPNG(t, frame)
		})
	}
}

// toAny is urls as the feeds a widget's config lists.
func toAny(urls []string) []any {
	out := make([]any, len(urls))
	for i, u := range urls {
		out[i] = u
	}
	return out
}
