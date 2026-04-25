package calendar

import (
	"image"
	"image/color"
	"testing"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar"
	"github.com/grantlucas/inkwell/internal/inkwell/testutil"
)

var fixedTime = time.Date(2026, 4, 25, 9, 0, 0, 0, time.UTC) // Saturday

func fixedClock() time.Time { return fixedTime }

var testEvents = []calendar.Event{
	{
		UID:      "evt-1",
		Summary:  "Team Standup",
		Start:    time.Date(2026, 4, 25, 9, 0, 0, 0, time.UTC),
		End:      time.Date(2026, 4, 25, 10, 0, 0, 0, time.UTC),
		Location: "Room 42",
	},
	{
		UID:     "evt-2",
		Summary: "Sprint Planning",
		Start:   time.Date(2026, 4, 25, 14, 0, 0, 0, time.UTC),
		End:     time.Date(2026, 4, 25, 15, 30, 0, 0, time.UTC),
	},
	{
		UID:     "evt-allday",
		Summary: "Company Holiday",
		Start:   time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC),
		End:     time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC),
		AllDay:  true,
	},
}

var upcomingEvents = []calendar.Event{
	{
		UID:     "evt-today",
		Summary: "Standup",
		Start:   time.Date(2026, 4, 25, 9, 0, 0, 0, time.UTC),
		End:     time.Date(2026, 4, 25, 10, 0, 0, 0, time.UTC),
	},
	{
		UID:     "evt-tomorrow",
		Summary: "Lunch",
		Start:   time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC),
		End:     time.Date(2026, 4, 26, 13, 0, 0, 0, time.UTC),
	},
	{
		UID:     "evt-day3",
		Summary: "Farmers Market",
		Start:   time.Date(2026, 4, 27, 10, 0, 0, 0, time.UTC),
		End:     time.Date(2026, 4, 27, 12, 0, 0, 0, time.UTC),
	},
}

func newWidgetFrame(w, h int) *image.Paletted {
	return image.NewPaletted(
		image.Rect(0, 0, w, h),
		color.Palette{color.White, color.Black},
	)
}

func baseCfg() Config {
	return Config{
		MaxEvents: 10,
		WeekStart: time.Monday,
		now:       fixedClock,
	}
}

// --- Today View Tests ---

func TestRenderToday_WithEvents(t *testing.T) {
	frame := newWidgetFrame(400, 200)
	cfg := baseCfg()
	cfg.View = ViewToday

	err := renderToday(frame, frame.Bounds(), testEvents, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !hasBlackPixels(frame, frame.Bounds()) {
		t.Error("rendered blank frame")
	}
	testutil.AssertGoldenPNG(t, frame)
}

func TestRenderToday_Empty(t *testing.T) {
	frame := newWidgetFrame(400, 200)
	cfg := baseCfg()
	cfg.View = ViewToday

	err := renderToday(frame, frame.Bounds(), nil, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !hasBlackPixels(frame, frame.Bounds()) {
		t.Error("rendered blank frame")
	}
	testutil.AssertGoldenPNG(t, frame)
}

func TestRenderToday_WithTitle(t *testing.T) {
	frame := newWidgetFrame(400, 200)
	cfg := baseCfg()
	cfg.View = ViewToday
	cfg.Title = "Work Calendar"

	err := renderToday(frame, frame.Bounds(), testEvents, cfg)
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertGoldenPNG(t, frame)
}

func TestRenderToday_ShowLocation(t *testing.T) {
	frame := newWidgetFrame(400, 200)
	cfg := baseCfg()
	cfg.View = ViewToday
	cfg.ShowLocation = true

	err := renderToday(frame, frame.Bounds(), testEvents, cfg)
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertGoldenPNG(t, frame)
}

func TestRenderToday_MaxEvents(t *testing.T) {
	frame := newWidgetFrame(400, 200)
	cfg := baseCfg()
	cfg.View = ViewToday
	cfg.MaxEvents = 1

	err := renderToday(frame, frame.Bounds(), testEvents, cfg)
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertGoldenPNG(t, frame)
}

func TestRenderToday_SingleEvent(t *testing.T) {
	frame := newWidgetFrame(400, 200)
	cfg := baseCfg()
	cfg.View = ViewToday

	events := []calendar.Event{testEvents[0]}
	err := renderToday(frame, frame.Bounds(), events, cfg)
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertGoldenPNG(t, frame)
}

// --- Upcoming View Tests ---

func TestRenderUpcoming_WithEvents(t *testing.T) {
	frame := newWidgetFrame(400, 300)
	cfg := baseCfg()
	cfg.View = ViewUpcoming

	err := renderUpcoming(frame, frame.Bounds(), upcomingEvents, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !hasBlackPixels(frame, frame.Bounds()) {
		t.Error("rendered blank frame")
	}
	testutil.AssertGoldenPNG(t, frame)
}

func TestRenderUpcoming_Empty(t *testing.T) {
	frame := newWidgetFrame(400, 300)
	cfg := baseCfg()
	cfg.View = ViewUpcoming

	err := renderUpcoming(frame, frame.Bounds(), nil, cfg)
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertGoldenPNG(t, frame)
}

func TestRenderUpcoming_ShowLocation(t *testing.T) {
	frame := newWidgetFrame(400, 300)
	cfg := baseCfg()
	cfg.View = ViewUpcoming
	cfg.ShowLocation = true

	events := []calendar.Event{
		{
			UID:      "loc",
			Summary:  "Meeting",
			Start:    time.Date(2026, 4, 25, 9, 0, 0, 0, time.UTC),
			End:      time.Date(2026, 4, 25, 10, 0, 0, 0, time.UTC),
			Location: "Room 1",
		},
	}
	err := renderUpcoming(frame, frame.Bounds(), events, cfg)
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertGoldenPNG(t, frame)
}

func TestRenderUpcoming_AllDayEvent(t *testing.T) {
	frame := newWidgetFrame(400, 300)
	cfg := baseCfg()
	cfg.View = ViewUpcoming

	events := []calendar.Event{
		{
			UID:     "allday",
			Summary: "Holiday",
			Start:   time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC),
			End:     time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC),
			AllDay:  true,
		},
	}
	err := renderUpcoming(frame, frame.Bounds(), events, cfg)
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertGoldenPNG(t, frame)
}

func TestRenderUpcoming_WithTitle(t *testing.T) {
	frame := newWidgetFrame(400, 300)
	cfg := baseCfg()
	cfg.View = ViewUpcoming
	cfg.Title = "Team Schedule"

	err := renderUpcoming(frame, frame.Bounds(), upcomingEvents, cfg)
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertGoldenPNG(t, frame)
}

// --- Week View Tests ---

func TestRenderWeek_WithEvents(t *testing.T) {
	frame := newWidgetFrame(400, 480)
	cfg := baseCfg()
	cfg.View = ViewWeek

	err := renderWeek(frame, frame.Bounds(), testEvents, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !hasBlackPixels(frame, frame.Bounds()) {
		t.Error("rendered blank frame")
	}
	testutil.AssertGoldenPNG(t, frame)
}

func TestRenderWeek_Empty(t *testing.T) {
	frame := newWidgetFrame(400, 480)
	cfg := baseCfg()
	cfg.View = ViewWeek

	err := renderWeek(frame, frame.Bounds(), nil, cfg)
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertGoldenPNG(t, frame)
}

func TestRenderWeek_SundayStart(t *testing.T) {
	frame := newWidgetFrame(400, 480)
	cfg := baseCfg()
	cfg.View = ViewWeek
	cfg.WeekStart = time.Sunday

	err := renderWeek(frame, frame.Bounds(), testEvents, cfg)
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertGoldenPNG(t, frame)
}

func TestRenderWeek_WithTitle(t *testing.T) {
	frame := newWidgetFrame(400, 480)
	cfg := baseCfg()
	cfg.View = ViewWeek
	cfg.Title = "My Week"

	err := renderWeek(frame, frame.Bounds(), testEvents, cfg)
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertGoldenPNG(t, frame)
}

func TestRenderWeek_ShowLocation(t *testing.T) {
	frame := newWidgetFrame(400, 480)
	cfg := baseCfg()
	cfg.View = ViewWeek
	cfg.ShowLocation = true

	err := renderWeek(frame, frame.Bounds(), testEvents, cfg)
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertGoldenPNG(t, frame)
}

// --- Month View Tests ---

func TestRenderMonth_WithEvents(t *testing.T) {
	frame := newWidgetFrame(400, 300)
	cfg := baseCfg()
	cfg.View = ViewMonth

	err := renderMonth(frame, frame.Bounds(), testEvents, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !hasBlackPixels(frame, frame.Bounds()) {
		t.Error("rendered blank frame")
	}
	testutil.AssertGoldenPNG(t, frame)
}

func TestRenderMonth_Empty(t *testing.T) {
	frame := newWidgetFrame(400, 300)
	cfg := baseCfg()
	cfg.View = ViewMonth

	err := renderMonth(frame, frame.Bounds(), nil, cfg)
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertGoldenPNG(t, frame)
}

func TestRenderMonth_SundayStart(t *testing.T) {
	frame := newWidgetFrame(400, 300)
	cfg := baseCfg()
	cfg.View = ViewMonth
	cfg.WeekStart = time.Sunday

	err := renderMonth(frame, frame.Bounds(), testEvents, cfg)
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertGoldenPNG(t, frame)
}

func TestRenderMonth_WithTitle(t *testing.T) {
	frame := newWidgetFrame(400, 300)
	cfg := baseCfg()
	cfg.View = ViewMonth
	cfg.Title = "April Schedule"

	err := renderMonth(frame, frame.Bounds(), testEvents, cfg)
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertGoldenPNG(t, frame)
}

func TestRenderMonth_SingleEvent(t *testing.T) {
	frame := newWidgetFrame(400, 300)
	cfg := baseCfg()
	cfg.View = ViewMonth

	events := []calendar.Event{testEvents[0]}
	err := renderMonth(frame, frame.Bounds(), events, cfg)
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertGoldenPNG(t, frame)
}

// --- Small frame / overflow tests ---

func TestRenderToday_SmallFrame(t *testing.T) {
	// Frame too small to fit all events — should not panic.
	frame := newWidgetFrame(200, 50)
	cfg := baseCfg()
	err := renderToday(frame, frame.Bounds(), testEvents, cfg)
	if err != nil {
		t.Fatal(err)
	}
}

func TestRenderUpcoming_SmallFrame(t *testing.T) {
	frame := newWidgetFrame(200, 30)
	cfg := baseCfg()
	err := renderUpcoming(frame, frame.Bounds(), upcomingEvents, cfg)
	if err != nil {
		t.Fatal(err)
	}
}

func TestRenderUpcoming_MaxEvents(t *testing.T) {
	// Multiple events on same day, maxEvents=1 should truncate.
	manyEvents := []calendar.Event{
		{UID: "a", Summary: "First", Start: fixedTime, End: fixedTime.Add(time.Hour)},
		{UID: "b", Summary: "Second", Start: fixedTime.Add(2 * time.Hour), End: fixedTime.Add(3 * time.Hour)},
	}
	frame := newWidgetFrame(400, 300)
	cfg := baseCfg()
	cfg.MaxEvents = 1
	err := renderUpcoming(frame, frame.Bounds(), manyEvents, cfg)
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertGoldenPNG(t, frame)
}

func TestRenderWeek_SmallFrame(t *testing.T) {
	frame := newWidgetFrame(200, 30)
	cfg := baseCfg()
	err := renderWeek(frame, frame.Bounds(), testEvents, cfg)
	if err != nil {
		t.Fatal(err)
	}
}

func TestRenderWeek_EventOverflowsFrame(t *testing.T) {
	// Events on Monday (day 1 of week), frame tall enough for header + day
	// header but not events. This triggers the y+lineHeight > bounds.Max.Y
	// break inside the event loop.
	mondayTime := time.Date(2026, 4, 20, 9, 0, 0, 0, time.UTC) // Monday
	events := []calendar.Event{
		{UID: "a", Summary: "First", Start: mondayTime, End: mondayTime.Add(time.Hour)},
		{UID: "b", Summary: "Second", Start: mondayTime.Add(2 * time.Hour), End: mondayTime.Add(3 * time.Hour)},
		{UID: "c", Summary: "Third", Start: mondayTime.Add(4 * time.Hour), End: mondayTime.Add(5 * time.Hour)},
	}
	// Height: header(13) + sep(4) + lineH(13) + dayHeader(13) = 43, need ~56 for one event.
	// With height=60 we can fit header + day header + 1 event but not 3.
	frame := newWidgetFrame(400, 60)
	cfg := baseCfg()
	cfg.now = func() time.Time { return mondayTime }
	cfg.MaxEvents = 10
	err := renderWeek(frame, frame.Bounds(), events, cfg)
	if err != nil {
		t.Fatal(err)
	}
}

func TestRenderWeek_MaxEvents(t *testing.T) {
	// testEvents has 3 events on Apr 25 (Sat), maxEvents=1 should truncate.
	frame := newWidgetFrame(400, 480)
	cfg := baseCfg()
	cfg.MaxEvents = 1
	err := renderWeek(frame, frame.Bounds(), testEvents, cfg)
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertGoldenPNG(t, frame)
}

func TestRenderMonth_SmallFrame(t *testing.T) {
	frame := newWidgetFrame(200, 50)
	cfg := baseCfg()
	err := renderMonth(frame, frame.Bounds(), testEvents, cfg)
	if err != nil {
		t.Fatal(err)
	}
}

func TestRenderMonth_EventDotIndicator(t *testing.T) {
	// Test the dot indicator for days with events (not today).
	// Use a time on day 1, with events on day 15.
	earlyTime := time.Date(2026, 4, 1, 9, 0, 0, 0, time.UTC)
	frame := newWidgetFrame(400, 300)
	cfg := baseCfg()
	cfg.now = func() time.Time { return earlyTime }
	events := []calendar.Event{
		{
			UID:     "mid-month",
			Summary: "Meeting",
			Start:   time.Date(2026, 4, 15, 9, 0, 0, 0, time.UTC),
			End:     time.Date(2026, 4, 15, 10, 0, 0, 0, time.UTC),
		},
	}
	err := renderMonth(frame, frame.Bounds(), events, cfg)
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertGoldenPNG(t, frame)
}

func TestRenderMonth_MultiDayEvent(t *testing.T) {
	frame := newWidgetFrame(400, 300)
	cfg := baseCfg()
	events := []calendar.Event{
		{
			UID:     "multi-day",
			Summary: "Conference",
			Start:   time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC),
			End:     time.Date(2026, 4, 13, 0, 0, 0, 0, time.UTC),
			AllDay:  true,
		},
	}
	err := renderMonth(frame, frame.Bounds(), events, cfg)
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertGoldenPNG(t, frame)
}

// --- viewTimeRange Tests ---

func TestViewTimeRange_Today(t *testing.T) {
	start, end := viewTimeRange(ViewToday, fixedTime, time.Monday)
	wantStart := time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC)
	wantEnd := time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)
	if !start.Equal(wantStart) {
		t.Errorf("start = %v, want %v", start, wantStart)
	}
	if !end.Equal(wantEnd) {
		t.Errorf("end = %v, want %v", end, wantEnd)
	}
}

func TestViewTimeRange_Upcoming(t *testing.T) {
	start, end := viewTimeRange(ViewUpcoming, fixedTime, time.Monday)
	wantStart := time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC)
	wantEnd := time.Date(2026, 4, 28, 0, 0, 0, 0, time.UTC)
	if !start.Equal(wantStart) {
		t.Errorf("start = %v, want %v", start, wantStart)
	}
	if !end.Equal(wantEnd) {
		t.Errorf("end = %v, want %v", end, wantEnd)
	}
}

func TestViewTimeRange_Week_Monday(t *testing.T) {
	start, end := viewTimeRange(ViewWeek, fixedTime, time.Monday)
	// 2026-04-25 is Saturday. Monday before is Apr 20.
	wantStart := time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC)
	wantEnd := time.Date(2026, 4, 27, 0, 0, 0, 0, time.UTC)
	if !start.Equal(wantStart) {
		t.Errorf("start = %v, want %v", start, wantStart)
	}
	if !end.Equal(wantEnd) {
		t.Errorf("end = %v, want %v", end, wantEnd)
	}
}

func TestViewTimeRange_Week_Sunday(t *testing.T) {
	start, end := viewTimeRange(ViewWeek, fixedTime, time.Sunday)
	// 2026-04-25 is Saturday. Sunday before is Apr 19.
	wantStart := time.Date(2026, 4, 19, 0, 0, 0, 0, time.UTC)
	wantEnd := time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)
	if !start.Equal(wantStart) {
		t.Errorf("start = %v, want %v", start, wantStart)
	}
	if !end.Equal(wantEnd) {
		t.Errorf("end = %v, want %v", end, wantEnd)
	}
}

func TestViewTimeRange_Month(t *testing.T) {
	start, end := viewTimeRange(ViewMonth, fixedTime, time.Monday)
	wantStart := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	wantEnd := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	if !start.Equal(wantStart) {
		t.Errorf("start = %v, want %v", start, wantStart)
	}
	if !end.Equal(wantEnd) {
		t.Errorf("end = %v, want %v", end, wantEnd)
	}
}

func TestViewTimeRange_Default(t *testing.T) {
	start, end := viewTimeRange("invalid", fixedTime, time.Monday)
	wantStart := time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC)
	wantEnd := time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)
	if !start.Equal(wantStart) {
		t.Errorf("start = %v, want %v", start, wantStart)
	}
	if !end.Equal(wantEnd) {
		t.Errorf("end = %v, want %v", end, wantEnd)
	}
}

// --- Filter helper test ---

func TestFilterEventsForDay(t *testing.T) {
	day := time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC)
	dayEnd := day.AddDate(0, 0, 1)

	all := []calendar.Event{
		{UID: "in", Start: day.Add(9 * time.Hour), End: day.Add(10 * time.Hour)},
		{UID: "out", Start: day.AddDate(0, 0, 1), End: day.AddDate(0, 0, 2)},
	}
	got := filterEventsForDay(all, day, dayEnd)
	if len(got) != 1 {
		t.Fatalf("got %d events, want 1", len(got))
	}
	if got[0].UID != "in" {
		t.Errorf("got UID %q, want %q", got[0].UID, "in")
	}
}

// --- weekdayAbbreviations test ---

func TestWeekdayAbbreviations_Monday(t *testing.T) {
	got := weekdayAbbreviations(time.Monday)
	want := [7]string{"Mo", "Tu", "We", "Th", "Fr", "Sa", "Su"}
	if got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestWeekdayAbbreviations_Sunday(t *testing.T) {
	got := weekdayAbbreviations(time.Sunday)
	want := [7]string{"Su", "Mo", "Tu", "We", "Th", "Fr", "Sa"}
	if got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}
