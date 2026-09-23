package boldfive

import (
	"image"
	"testing"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daygrid"
)

func eventsRect() image.Rectangle {
	return image.Rect(0, 216, 160, 480)
}

func defaultEventOpts() eventOptions {
	return eventOptions{MaxEvents: defaultMaxEvents, Location: time.UTC}
}

func timedEvent(summary string, hour int) calendar.Event {
	start := time.Date(2026, 3, 16, hour, 0, 0, 0, time.UTC)
	return calendar.Event{Summary: summary, Start: start, End: start.Add(time.Hour)}
}

func TestWrapText(t *testing.T) {
	tests := []struct {
		label    string
		text     string
		maxChars int
		maxLines int
		want     []string
	}{
		{"short title stays on one line", "Standup", 14, 2, []string{"Standup"}},
		{"wraps at a word boundary", "Platform architecture review", 14, 2, []string{"Platform", "architecture…"}},
		{"exactly fills a line", "Car in for svc", 14, 2, []string{"Car in for svc"}},
		{"breaks an unbreakable word", "Supercalifragilistic", 10, 2, []string{"Supercalif", "ragilistic"}},
		{"ellipses past the line budget", "one two three four five six", 10, 2, []string{"one two", "three fou…"}},
		{"empty text draws nothing", "", 14, 2, nil},
		{"whitespace only draws nothing", "   ", 14, 2, nil},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			got := wrapText(tt.text, tt.maxChars, tt.maxLines)
			if len(got) != len(tt.want) {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("line %d = %q, want %q", i, got[i], tt.want[i])
				}
			}
			for i, l := range got {
				if len([]rune(l)) > tt.maxChars {
					t.Errorf("line %d (%q) is %d chars, over the %d budget", i, l, len([]rune(l)), tt.maxChars)
				}
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		label    string
		in       string
		maxChars int
		want     string
	}{
		{"fits", "09:00", 10, "09:00"},
		{"exactly fits", "09:00", 5, "09:00"},
		{"ellipsed", "Standup", 5, "Stan…"},
		{"no room for an ellipsis", "Standup", 1, "S"},
		{"zero budget", "Standup", 0, ""},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			if got := truncate(tt.in, tt.maxChars); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// An all-day event has no clock time to show, so it is labelled rather
// than given a meaningless 00:00.
func TestPlanEvent(t *testing.T) {
	toronto, err := time.LoadLocation("America/Toronto")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		label        string
		event        calendar.Event
		showLocation bool
		wantTime     string
		wantFirst    string
	}{
		{
			label:     "timed event shows its clock time",
			event:     timedEvent("Standup", 9),
			wantTime:  "09:00",
			wantFirst: "Standup",
		},
		{
			label:     "all-day event is labelled",
			event:     calendar.Event{Summary: "Holiday", AllDay: true},
			wantTime:  "ALL DAY",
			wantFirst: "Holiday",
		},
		{
			label:        "location is appended when configured",
			event:        calendar.Event{Summary: "Call", Location: "Room 2", Start: timedEvent("x", 9).Start},
			showLocation: true,
			wantTime:     "09:00",
			wantFirst:    "Call @ Room 2",
		},
		{
			label:     "location is omitted by default",
			event:     calendar.Event{Summary: "Call", Location: "Room 2", Start: timedEvent("x", 9).Start},
			wantTime:  "09:00",
			wantFirst: "Call",
		},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			opts := defaultEventOpts()
			opts.ShowLocation = tt.showLocation
			p := planEvent(tt.event, 14, opts)
			if p.timeLine != tt.wantTime {
				t.Errorf("timeLine = %q, want %q", p.timeLine, tt.wantTime)
			}
			if len(p.titleLines) == 0 || p.titleLines[0] != tt.wantFirst {
				t.Errorf("first title line = %q, want %q", p.titleLines, tt.wantFirst)
			}
		})
	}

	// The clock label is rendered in the dashboard's zone, not the
	// zone the feed happened to serialize the event with.
	utcEvent := timedEvent("Standup", 14)
	opts := defaultEventOpts()
	opts.Location = toronto
	if got := planEvent(utcEvent, 14, opts).timeLine; got != "10:00" {
		t.Errorf("timeLine = %q, want 10:00 (14:00Z in Toronto)", got)
	}
}

// An empty day says so rather than leaving a blank column that reads as
// a rendering fault.
func TestRenderEvents_EmptyDay(t *testing.T) {
	frame := newTestFrame(160, 480)
	if got := renderEvents(frame, eventsRect(), nil, defaultEventOpts()); got != 0 {
		t.Errorf("drawn = %d, want 0", got)
	}
	if countIndex(frame, widget.PaperBlack) == 0 {
		t.Error("nothing drawn at all — the empty marker is missing")
	}
}

// The rule along the top of the agenda is what separates it from the
// weather band; without it the two run together.
func TestRenderEvents_DrawsTopRule(t *testing.T) {
	frame := newTestFrame(160, 480)
	rect := eventsRect()
	renderEvents(frame, rect, nil, defaultEventOpts())

	for x := rect.Min.X; x < rect.Max.X; x++ {
		if frame.ColorIndexAt(x, rect.Min.Y) != widget.PaperBlack {
			t.Fatalf("no rule pixel at x=%d", x)
		}
	}
}

func TestRenderEvents_DrawsUpToMaxEvents(t *testing.T) {
	events := []calendar.Event{
		timedEvent("One", 9), timedEvent("Two", 10), timedEvent("Three", 11),
		timedEvent("Four", 12), timedEvent("Five", 13), timedEvent("Six", 14),
	}
	frame := newTestFrame(160, 480)
	if got := renderEvents(frame, eventsRect(), events, defaultEventOpts()); got != defaultMaxEvents {
		t.Errorf("drawn = %d, want %d", got, defaultMaxEvents)
	}
}

// A column too narrow to carry a title draws nothing rather than a
// stack of ellipses, which reads as a fault rather than as content.
func TestRenderEvents_TooNarrow(t *testing.T) {
	frame := newTestFrame(160, 480)
	got := renderEvents(frame, image.Rect(0, 216, 20, 480), []calendar.Event{timedEvent("Standup", 9)}, defaultEventOpts())
	if got != 0 {
		t.Errorf("drawn = %d, want 0", got)
	}
}

// An event whose title would be clipped off the bottom is not drawn at
// all: a time with no name under it reads as a broken row.
func TestRenderEvents_DoesNotClipAnEventInHalf(t *testing.T) {
	events := []calendar.Event{timedEvent("One", 9), timedEvent("Two", 10)}
	// Room for one event's three rows but not two.
	short := image.Rect(0, 216, 160, 216+eventsTopPad+3*daygrid.BodyLineH())
	frame := newTestFrame(160, 480)
	if got := renderEvents(frame, short, events, defaultEventOpts()); got != 1 {
		t.Errorf("drawn = %d, want 1", got)
	}
}
