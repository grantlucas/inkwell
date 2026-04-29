package weekly

import (
	"image"
	"testing"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar/ical"
)

func TestRenderEvents_WithEvents(t *testing.T) {
	frame := newTestFrame(114, 200)
	events := []ical.Event{
		{
			UID:     "1",
			Summary: "Standup",
			Start:   time.Date(2026, 4, 28, 9, 0, 0, 0, time.UTC),
			End:     time.Date(2026, 4, 28, 9, 30, 0, 0, time.UTC),
		},
		{
			UID:     "2",
			Summary: "Lunch",
			Start:   time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC),
			End:     time.Date(2026, 4, 28, 13, 0, 0, 0, time.UTC),
		},
	}

	rendered := renderEvents(frame, image.Rect(0, 0, 114, 200), events, 5, false)
	if rendered != 2 {
		t.Errorf("rendered %d events, want 2", rendered)
	}

	hasBlack := false
	for _, px := range frame.Pix {
		if px != 0 {
			hasBlack = true
			break
		}
	}
	if !hasBlack {
		t.Error("events drew nothing")
	}
}

func TestRenderEvents_AllDay(t *testing.T) {
	frame := newTestFrame(114, 200)
	events := []ical.Event{
		{
			UID:     "1",
			Summary: "Holiday",
			Start:   time.Date(2026, 4, 28, 0, 0, 0, 0, time.UTC),
			End:     time.Date(2026, 4, 29, 0, 0, 0, 0, time.UTC),
			AllDay:  true,
		},
	}

	rendered := renderEvents(frame, image.Rect(0, 0, 114, 200), events, 5, false)
	if rendered != 1 {
		t.Errorf("rendered %d events, want 1", rendered)
	}
}

func TestRenderEvents_NoEvents(t *testing.T) {
	frame := newTestFrame(114, 200)
	rendered := renderEvents(frame, image.Rect(0, 0, 114, 200), nil, 5, false)
	if rendered != 0 {
		t.Errorf("rendered %d events, want 0", rendered)
	}

	hasBlack := false
	for _, px := range frame.Pix {
		if px != 0 {
			hasBlack = true
			break
		}
	}
	if !hasBlack {
		t.Error("no-events placeholder not drawn")
	}
}

func TestRenderEvents_MaxEventsLimit(t *testing.T) {
	frame := newTestFrame(114, 400)
	var events []ical.Event
	for i := range 10 {
		events = append(events, ical.Event{
			UID:     string(rune('A' + i)),
			Summary: "Event",
			Start:   time.Date(2026, 4, 28, 9+i, 0, 0, 0, time.UTC),
			End:     time.Date(2026, 4, 28, 10+i, 0, 0, 0, time.UTC),
		})
	}

	rendered := renderEvents(frame, image.Rect(0, 0, 114, 400), events, 3, false)
	if rendered != 3 {
		t.Errorf("rendered %d events, want 3", rendered)
	}
}

func TestRenderEvents_WithLocation(t *testing.T) {
	frame := newTestFrame(114, 200)
	events := []ical.Event{
		{
			UID:      "1",
			Summary:  "Meeting",
			Location: "Room 42",
			Start:    time.Date(2026, 4, 28, 10, 0, 0, 0, time.UTC),
			End:      time.Date(2026, 4, 28, 11, 0, 0, 0, time.UTC),
		},
	}

	rendered := renderEvents(frame, image.Rect(0, 0, 114, 200), events, 5, true)
	if rendered != 1 {
		t.Errorf("rendered %d events, want 1", rendered)
	}
}

func TestRenderEvents_TinyBounds(t *testing.T) {
	frame := newTestFrame(10, 10)
	events := []ical.Event{
		{
			UID:     "1",
			Summary: "Test",
			Start:   time.Date(2026, 4, 28, 9, 0, 0, 0, time.UTC),
			End:     time.Date(2026, 4, 28, 10, 0, 0, 0, time.UTC),
		},
	}
	rendered := renderEvents(frame, image.Rect(0, 0, 10, 10), events, 5, false)
	if rendered != 0 {
		t.Errorf("rendered %d events in tiny bounds, want 0", rendered)
	}
}

func TestRenderEvents_HeightClipping(t *testing.T) {
	frame := newTestFrame(114, 50)
	var events []ical.Event
	for i := range 5 {
		events = append(events, ical.Event{
			UID:     string(rune('A' + i)),
			Summary: "Event",
			Start:   time.Date(2026, 4, 28, 9+i, 0, 0, 0, time.UTC),
			End:     time.Date(2026, 4, 28, 10+i, 0, 0, 0, time.UTC),
		})
	}

	rendered := renderEvents(frame, image.Rect(0, 0, 114, 50), events, 10, false)
	if rendered >= 5 {
		t.Errorf("rendered %d events in 50px, expected clipping", rendered)
	}
}

func TestFilterEventsForDay(t *testing.T) {
	events := []ical.Event{
		{
			UID:     "before",
			Summary: "Yesterday",
			Start:   time.Date(2026, 4, 27, 9, 0, 0, 0, time.UTC),
			End:     time.Date(2026, 4, 27, 10, 0, 0, 0, time.UTC),
		},
		{
			UID:     "during",
			Summary: "Today event",
			Start:   time.Date(2026, 4, 28, 14, 0, 0, 0, time.UTC),
			End:     time.Date(2026, 4, 28, 15, 0, 0, 0, time.UTC),
		},
		{
			UID:     "allday",
			Summary: "All Day",
			Start:   time.Date(2026, 4, 28, 0, 0, 0, 0, time.UTC),
			End:     time.Date(2026, 4, 29, 0, 0, 0, 0, time.UTC),
			AllDay:  true,
		},
		{
			UID:     "after",
			Summary: "Tomorrow",
			Start:   time.Date(2026, 4, 29, 9, 0, 0, 0, time.UTC),
			End:     time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC),
		},
	}

	dayStart := time.Date(2026, 4, 28, 0, 0, 0, 0, time.UTC)
	dayEnd := dayStart.AddDate(0, 0, 1)
	filtered := filterEventsForDay(events, dayStart, dayEnd)

	if len(filtered) != 2 {
		t.Fatalf("got %d events, want 2", len(filtered))
	}
	if !filtered[0].AllDay {
		t.Error("all-day event should sort first")
	}
	if filtered[1].Summary != "Today event" {
		t.Errorf("second event = %q, want 'Today event'", filtered[1].Summary)
	}
}

func TestRenderEvents_TitleClippedByHeight(t *testing.T) {
	// Bounds just tall enough for time line but not title.
	frame := newTestFrame(114, 30)
	events := []ical.Event{
		{
			UID:     "1",
			Summary: "Meeting",
			Start:   time.Date(2026, 4, 28, 9, 0, 0, 0, time.UTC),
			End:     time.Date(2026, 4, 28, 10, 0, 0, 0, time.UTC),
		},
	}
	rendered := renderEvents(frame, image.Rect(0, 0, 114, 30), events, 5, false)
	if rendered != 1 {
		t.Errorf("rendered %d events, want 1", rendered)
	}
}

func TestFilterEventsForDay_SortByStart(t *testing.T) {
	events := []ical.Event{
		{
			UID:     "late",
			Summary: "Late",
			Start:   time.Date(2026, 4, 28, 15, 0, 0, 0, time.UTC),
			End:     time.Date(2026, 4, 28, 16, 0, 0, 0, time.UTC),
		},
		{
			UID:     "early",
			Summary: "Early",
			Start:   time.Date(2026, 4, 28, 9, 0, 0, 0, time.UTC),
			End:     time.Date(2026, 4, 28, 10, 0, 0, 0, time.UTC),
		},
	}
	dayStart := time.Date(2026, 4, 28, 0, 0, 0, 0, time.UTC)
	dayEnd := dayStart.AddDate(0, 0, 1)
	filtered := filterEventsForDay(events, dayStart, dayEnd)
	if len(filtered) != 2 {
		t.Fatalf("got %d events, want 2", len(filtered))
	}
	if filtered[0].Summary != "Early" {
		t.Errorf("first event = %q, want 'Early'", filtered[0].Summary)
	}
}

func TestFilterEventsForDay_Empty(t *testing.T) {
	dayStart := time.Date(2026, 4, 28, 0, 0, 0, 0, time.UTC)
	dayEnd := dayStart.AddDate(0, 0, 1)
	filtered := filterEventsForDay(nil, dayStart, dayEnd)
	if len(filtered) != 0 {
		t.Errorf("got %d events, want 0", len(filtered))
	}
}
