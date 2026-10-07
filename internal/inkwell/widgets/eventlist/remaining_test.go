package eventlist_test

import (
	"testing"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/eventlist"
)

// An event that finished is history. An event still running is the one
// you most want to see, so it counts as remaining; an all-day event
// applies to the whole day and never finishes partway through it. A
// timed VEVENT with neither DTEND nor DURATION is parsed with
// End == Start, so an exclusive comparison would drop it at the very
// minute it fires — and if it were the last one, a list hiding finished
// events would say the day is done over an event happening now.
func TestRemaining(t *testing.T) {
	at := func(h, m int) time.Time { return time.Date(2026, 3, 16, h, m, 0, 0, time.UTC) }
	events := []calendar.Event{
		{Summary: "Finished", Start: at(9, 0), End: at(10, 0)},
		{Summary: "Running", Start: at(14, 0), End: at(16, 0)},
		{Summary: "Later", Start: at(18, 0), End: at(19, 0)},
		{Summary: "All day", AllDay: true},
	}
	reminder := []calendar.Event{{Summary: "Pick up parcel", Start: at(16, 0), End: at(16, 0)}}

	tests := []struct {
		label  string
		events []calendar.Event
		now    time.Time
		want   []string
	}{
		{"finished events are dropped, running ones kept", events, at(15, 0), []string{"Running", "Later", "All day"}},
		{"late in the day only the all-day event is left", events, at(23, 0), []string{"All day"}},
		{"a reminder is kept before it fires", reminder, at(15, 45), []string{"Pick up parcel"}},
		{"a reminder is kept at the minute it fires", reminder, at(16, 0), []string{"Pick up parcel"}},
		{"a reminder is dropped a minute later", reminder, at(16, 1), nil},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			got := eventlist.Remaining(tt.events, tt.now)
			if len(got) != len(tt.want) {
				t.Fatalf("got %d events, want %v", len(got), tt.want)
			}
			for i := range tt.want {
				if got[i].Summary != tt.want[i] {
					t.Errorf("event %d = %q, want %q", i, got[i].Summary, tt.want[i])
				}
			}
		})
	}
}
