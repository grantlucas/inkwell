package weekly

import (
	"image"
	"slices"
	"testing"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar/ical"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
)

// inkBands counts the number of vertically separated horizontal bands that
// contain black ink at or right of column xMin. Each drawn text line forms one
// band, so it is a structural proxy for "how many text rows were rendered"
// without coupling to exact pixel positions. xMin is set past the left rule so
// the per-event rule stroke is not counted.
func inkBands(frame *image.Paletted, xMin int) int {
	b := frame.Bounds()
	bands := 0
	prev := false
	for y := b.Min.Y; y < b.Max.Y; y++ {
		row := false
		for x := xMin; x < b.Max.X; x++ {
			if frame.ColorIndexAt(x, y) == widget.PaperBlack {
				row = true
				break
			}
		}
		if row && !prev {
			bands++
		}
		prev = row
	}
	return bands
}

func TestWrapText(t *testing.T) {
	cases := []struct {
		label    string
		text     string
		maxChars int
		maxLines int
		want     []string
	}{
		{"word boundary", "Sprint Planning Meeting", 10, 3, []string{"Sprint", "Planning", "Meeting"}},
		{"fits on one line", "Lunch", 13, 3, []string{"Lunch"}},
		{"packs words up to width", "a bb ccc", 6, 3, []string{"a bb", "ccc"}},
		{"hard-breaks an over-long word", "Supercalifragilistic", 8, 3, []string{"Supercal", "ifragili", "stic"}},
		{"hard-breaks on rune boundaries, not bytes", "ααααββββ", 4, 3, []string{"αααα", "ββββ"}},
		{"hard-break then wrap remaining words", "Wordsmithery rules", 6, 3, []string{"Wordsm", "ithery", "rules"}},
		{"truncates past maxLines with ellipsis when room", "alpha beta gamma delta", 8, 2, []string{"alpha", "beta..."}},
		{"truncates without ellipsis when last line is full", "aaaaa bbbbb ccccc", 5, 2, []string{"aaaaa", "bbbbb"}},
		{"zero maxChars yields nil", "anything", 0, 3, nil},
		{"zero maxLines yields nil", "anything", 8, 0, nil},
		{"empty text yields nil", "   ", 8, 3, nil},
	}
	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			got := wrapText(tc.text, tc.maxChars, tc.maxLines)
			if len(got) != len(tc.want) {
				t.Fatalf("got %d lines %q, want %d %q", len(got), got, len(tc.want), tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Errorf("line %d = %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestLineCapacity(t *testing.T) {
	cases := []struct {
		label string
		w, h  int
		want  int
	}{
		{"tall column", 114, 200, 9},
		{"one slot", 114, 50, 1},
		{"two slots", 114, 70, 2},
		{"title clipped height", 114, 38, 1},
		{"no slots", 114, 10, 0},
	}
	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			if got := lineCapacity(image.Rect(0, 0, tc.w, tc.h)); got != tc.want {
				t.Errorf("lineCapacity(%dx%d) = %d, want %d", tc.w, tc.h, got, tc.want)
			}
		})
	}
}

func ev(summary string) ical.Event {
	return ical.Event{Summary: summary, Start: time.Date(2026, 4, 28, 9, 0, 0, 0, time.UTC)}
}

func evLoc(summary, loc string) ical.Event {
	e := ev(summary)
	e.Location = loc
	return e
}

func TestPlanEvents(t *testing.T) {
	const long = "A Very Long Event Title That Overflows"
	cases := []struct {
		label        string
		events       []ical.Event
		maxEvents    int
		capacity     int
		maxChars     int
		showLocation bool
		wantBudgets  []int      // titleBudget per planned event
		wantTitles   []bool     // drawTitle per planned event
		wantLocs     []bool     // drawLocation per planned event
		wantLines    [][]string // titleLines per planned event (nil = skip)
	}{
		{
			label:       "full column keeps every title at one line",
			events:      []ical.Event{ev("Standup"), ev("Lunch"), ev("Sync"), ev("Review"), ev("Demo")},
			maxEvents:   5,
			capacity:    10,
			maxChars:    13,
			wantBudgets: []int{1, 1, 1, 1, 1},
			wantTitles:  []bool{true, true, true, true, true},
			wantLocs:    []bool{false, false, false, false, false},
		},
		{
			label:       "sparse overflowing title gets extra lines",
			events:      []ical.Event{ev(long)},
			maxEvents:   5,
			capacity:    10,
			maxChars:    13,
			wantBudgets: []int{3},
			wantTitles:  []bool{true},
			wantLocs:    []bool{false},
			wantLines:   [][]string{{"A Very Long", "Event Title", "That..."}},
		},
		{
			label:       "full column truncates an overflowing title at one line",
			events:      []ical.Event{ev("Standup"), ev(long), ev("Sync"), ev("Review"), ev("Demo")},
			maxEvents:   5,
			capacity:    10, // 5 events x 2 lines, no leftover to wrap with
			maxChars:    13,
			wantBudgets: []int{1, 1, 1, 1, 1},
			wantTitles:  []bool{true, true, true, true, true},
			wantLocs:    []bool{false, false, false, false, false},
			wantLines:   [][]string{{"Standup"}, {"A Very Lon..."}, {"Sync"}, {"Review"}, {"Demo"}},
		},
		{
			label:       "title with collapsible whitespace is shown in full, not truncated",
			events:      []ical.Event{ev("ab   cd")},
			maxEvents:   5,
			capacity:    10,
			maxChars:    6,
			wantBudgets: []int{1},
			wantTitles:  []bool{true},
			wantLocs:    []bool{false},
			wantLines:   [][]string{{"ab cd"}},
		},
		{
			label:       "sparse title that fits stays at one line",
			events:      []ical.Event{ev("Lunch")},
			maxEvents:   5,
			capacity:    10,
			maxChars:    13,
			wantBudgets: []int{1},
			wantTitles:  []bool{true},
			wantLocs:    []bool{false},
		},
		{
			label:       "leftover split across two overflowing titles",
			events:      []ical.Event{ev(long), ev(long)},
			maxEvents:   5,
			capacity:    10,
			maxChars:    13,
			wantBudgets: []int{3, 3},
			wantTitles:  []bool{true, true},
			wantLocs:    []bool{false, false},
		},
		{
			label:       "scarce leftover favors the earlier overflowing title",
			events:      []ical.Event{ev(long), ev(long)},
			maxEvents:   5,
			capacity:    5, // 2 events cost 4 lines, leaving just 1 to hand out
			maxChars:    13,
			wantBudgets: []int{2, 1},
			wantTitles:  []bool{true, true},
			wantLocs:    []bool{false, false},
		},
		{
			label:       "maxEvents caps the selection",
			events:      []ical.Event{ev("a"), ev("b"), ev("c"), ev("d")},
			maxEvents:   2,
			capacity:    20,
			maxChars:    13,
			wantBudgets: []int{1, 1},
			wantTitles:  []bool{true, true},
			wantLocs:    []bool{false, false},
		},
		{
			label:       "capacity leaves a time-only event at the bottom",
			events:      []ical.Event{ev("First"), ev("Second")},
			maxEvents:   5,
			capacity:    3,
			maxChars:    13,
			wantBudgets: []int{1, 1},
			wantTitles:  []bool{true, false},
			wantLocs:    []bool{false, false},
		},
		{
			label:        "location consumes a slot when shown",
			events:       []ical.Event{evLoc("Meeting", "Room 42")},
			maxEvents:    5,
			capacity:     10,
			maxChars:     13,
			showLocation: true,
			wantBudgets:  []int{1},
			wantTitles:   []bool{true},
			wantLocs:     []bool{true},
		},
	}
	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			plan := planEvents(tc.events, tc.maxEvents, tc.capacity, tc.maxChars, tc.showLocation)
			if len(plan) != len(tc.wantBudgets) {
				t.Fatalf("plan has %d events, want %d", len(plan), len(tc.wantBudgets))
			}
			for i := range plan {
				if plan[i].titleBudget != tc.wantBudgets[i] {
					t.Errorf("event %d titleBudget = %d, want %d", i, plan[i].titleBudget, tc.wantBudgets[i])
				}
				if plan[i].drawTitle != tc.wantTitles[i] {
					t.Errorf("event %d drawTitle = %v, want %v", i, plan[i].drawTitle, tc.wantTitles[i])
				}
				if plan[i].drawLocation != tc.wantLocs[i] {
					t.Errorf("event %d drawLocation = %v, want %v", i, plan[i].drawLocation, tc.wantLocs[i])
				}
				if tc.wantLines != nil {
					if got, want := plan[i].titleLines, tc.wantLines[i]; !slices.Equal(got, want) {
						t.Errorf("event %d titleLines = %q, want %q", i, got, want)
					}
				}
			}
		})
	}
}

func TestRenderEvents_WrappedTitleWithLocation(t *testing.T) {
	// A sparse day with a long title AND a location: the title wraps to
	// multiple lines and the location still draws below it, and the left rule
	// spans the whole multi-line event.
	frame := newTestFrame(114, 200)
	events := []ical.Event{evLoc("A Very Long Event Title That Overflows", "Conference Room")}
	rendered := renderEvents(frame, image.Rect(0, 0, 114, 200), events, 5, true)
	if rendered != 1 {
		t.Fatalf("rendered %d events, want 1", rendered)
	}
	// time + multiple title lines + location => well more than the 3 bands a
	// single-line event would produce.
	if bands := inkBands(frame, 8); bands < 4 {
		t.Errorf("expected >=4 ink bands (time + wrapped title + location), got %d", bands)
	}
}

func TestRenderEvents_SparseTitleWraps(t *testing.T) {
	render := func(summary string) *image.Paletted {
		f := newTestFrame(114, 200)
		renderEvents(f, image.Rect(0, 0, 114, 200), []ical.Event{ev(summary)}, 5, false)
		return f
	}
	// textX past the 2px rule + gaps: eventPadX(4)+eventRuleW(2)+eventGap(2).
	const textX = 8
	long := inkBands(render("A Very Long Event Title That Overflows"), textX)
	short := inkBands(render("Hi"), textX)

	if long <= short {
		t.Errorf("long title produced %d ink bands, short title %d — long should wrap to more", long, short)
	}
	if long < 3 {
		t.Errorf("expected >=3 ink bands (time + multi-line title), got %d", long)
	}
}

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

// All-day events are calendar date labels, not instants. A multi-day
// all-day event (e.g. a trip starting Thursday) must land in exactly the
// columns for the dates it spans, even when the viewer's day columns are
// built in a negative-UTC timezone. Parsed all-day dates are anchored to
// UTC midnight, so an instant-overlap comparison against local-zone columns
// leaks the event into the previous local day. This reproduces inkwell-9f0:
// the trip showed up a day early in America/Toronto (UTC-4/5).
func TestFilterEventsForDay_AllDayMultiDayNegativeTimezone(t *testing.T) {
	// DTSTART;VALUE=DATE:20260625 / DTEND;VALUE=DATE:20260628 (exclusive)
	// parses to UTC midnight, matching ical.parseDateTime.
	trip := ical.Event{
		UID:     "trip",
		Summary: "Winnipeg",
		Start:   time.Date(2026, 6, 25, 0, 0, 0, 0, time.UTC),
		End:     time.Date(2026, 6, 28, 0, 0, 0, 0, time.UTC),
		AllDay:  true,
	}
	loc := time.FixedZone("UTC-5", -5*60*60)

	cases := []struct {
		label string
		day   time.Time
		want  bool
	}{
		{"day before start", time.Date(2026, 6, 24, 0, 0, 0, 0, loc), false},
		{"first day (Thursday)", time.Date(2026, 6, 25, 0, 0, 0, 0, loc), true},
		{"middle day", time.Date(2026, 6, 26, 0, 0, 0, 0, loc), true},
		{"last spanned day", time.Date(2026, 6, 27, 0, 0, 0, 0, loc), true},
		{"exclusive end day", time.Date(2026, 6, 28, 0, 0, 0, 0, loc), false},
	}
	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			filtered := filterEventsForDay([]ical.Event{trip}, tc.day, tc.day.AddDate(0, 0, 1))
			got := len(filtered) == 1
			if got != tc.want {
				t.Errorf("trip present on %s = %v, want %v", tc.label, got, tc.want)
			}
		})
	}
}

func TestRenderEvents_TitleClippedByHeight(t *testing.T) {
	// Bounds just tall enough for time line but not title.
	frame := newTestFrame(114, 38)
	events := []ical.Event{
		{
			UID:     "1",
			Summary: "Meeting",
			Start:   time.Date(2026, 4, 28, 9, 0, 0, 0, time.UTC),
			End:     time.Date(2026, 4, 28, 10, 0, 0, 0, time.UTC),
		},
	}
	rendered := renderEvents(frame, image.Rect(0, 0, 114, 38), events, 5, false)
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

// When the column runs out of vertical room *before* the next event's
// time line can fit, the loop must stop via the pre-event y-overflow
// break (line 35 in events.go). With lineHeight=18 and bounds height
// 70, event 1 completes (time line at y=38, summary at y=58) and
// event 2's pre-event check `58 > 70-18=52` fires.
func TestRenderEvents_StopsAtPreEventYOverflow(t *testing.T) {
	frame := newTestFrame(114, 70)
	events := []ical.Event{
		{UID: "1", Summary: "First", Start: time.Date(2026, 4, 28, 9, 0, 0, 0, time.UTC)},
		{UID: "2", Summary: "Second", Start: time.Date(2026, 4, 28, 10, 0, 0, 0, time.UTC)},
		{UID: "3", Summary: "Third", Start: time.Date(2026, 4, 28, 11, 0, 0, 0, time.UTC)},
	}

	rendered := renderEvents(frame, image.Rect(0, 0, 114, 70), events, 10, false)
	if rendered != 1 {
		t.Errorf("rendered %d events, want 1 (pre-event y-overflow should have stopped the loop after event 1)", rendered)
	}
}
