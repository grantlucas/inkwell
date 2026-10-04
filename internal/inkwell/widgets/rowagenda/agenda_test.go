package rowagenda

import (
	"image"
	"testing"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar"
	"github.com/grantlucas/inkwell/internal/inkwell/weather"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daygrid"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/weatherview"
)

// busyRow is the first row of a week whose first day carries n events
// and whose other days are empty, as the widget would lay it out.
func busyRow(n int) rowLayout {
	return planRows(panel, []int{n, 0, 0, 0, 0})[0]
}

func agendaRect() image.Rectangle { return busyRow(0).Agenda }

func nEvents(n int) []calendar.Event {
	out := make([]calendar.Event, n)
	for i := range out {
		start := time.Date(2026, 3, 16, 8+i, 0, 0, 0, time.UTC)
		out[i] = calendar.Event{Summary: "Event", Start: start, End: start.Add(time.Hour)}
	}
	return out
}

// Every row is one event column running the agenda's full width,
// however busy the day: width is what titles were starving for, and the
// row grows downward instead of splitting sideways.
func TestLayoutSlots_OneFullWidthColumn(t *testing.T) {
	for _, n := range []int{1, 3, 4, 7} {
		row := busyRow(n)
		full := row.Agenda.Dx() - 2*agendaPadX
		slots := layoutSlots(row.Agenda, row.Lines)
		if len(slots) != n {
			t.Fatalf("%d events: got %d slots, want one per event", n, len(slots))
		}
		for i, s := range slots {
			if s.width != full || s.x != slots[0].x {
				t.Errorf("%d events: slot %d is at x=%d, %d px wide; want x=%d, the full %d",
					n, i, s.x, s.width, slots[0].x, full)
			}
			if i > 0 && s.y != slots[i-1].y+daygrid.BodyLineH() {
				t.Errorf("%d events: slot %d is not the line below slot %d", n, i, i-1)
			}
		}
	}
}

// A row too narrow to carry a time and a title draws nothing rather
// than a column of ellipses.
func TestLayoutSlots_TooNarrow(t *testing.T) {
	if got := layoutSlots(image.Rect(0, 0, 40, 96), 2); got != nil {
		t.Errorf("got %d slots in a 40 px row, want none", len(got))
	}
}

// A row that lost lines to a crowded week gives its last visible line
// to "+N MORE", counting every event it could not show, and the marker
// takes its own line rather than overprinting an event.
func TestRenderAgenda_OverflowMarker(t *testing.T) {
	// A nine-event day that the week trimmed to eight lines: seven
	// events and a marker for the other five.
	row := busyRow(9)
	bounds, lines := row.Agenda, row.Lines
	events := nEvents(12)
	if lines != 8 {
		t.Fatalf("the busy row has %d lines, want 8", lines)
	}

	frame := newTestFrame(800, 480)
	drawn := renderAgenda(frame, bounds, events, lines, eventOptions{Location: time.UTC})
	if drawn != lines-1 {
		t.Fatalf("drew %d events into %d lines, want one fewer than the lines", drawn, lines)
	}

	slots := layoutSlots(bounds, lines)
	marker := slots[lines-1]
	markerBox := image.Rect(marker.x, marker.y-daygrid.BodyAscent(), marker.x+marker.width, marker.y+daygrid.BodyLineH()-daygrid.BodyAscent())

	// Draw the same events with the marker suppressed: if the marker
	// overprinted an event, its line would carry ink from both.
	eventsOnly := newTestFrame(800, 480)
	renderAgenda(eventsOnly, bounds, events[:drawn], lines, eventOptions{Location: time.UTC})
	if got := countIndexIn(eventsOnly, markerBox, widget.PaperBlack); got != 0 {
		t.Errorf("the marker's line already carries %d px of event ink — it is being overprinted", got)
	}

	// The marker counts the five events that did not make it.
	want := newTestFrame(800, 480)
	daygrid.DrawText(want, marker.x, marker.y, "+5 MORE", daygrid.BodyBoldFace, widget.PaperBlack)
	for y := markerBox.Min.Y; y < markerBox.Max.Y; y++ {
		for x := markerBox.Min.X; x < markerBox.Max.X; x++ {
			if frame.ColorIndexAt(x, y) != want.ColorIndexAt(x, y) {
				t.Fatalf("the marker line differs from \"+5 MORE\" at (%d,%d)", x, y)
			}
		}
	}
}

// When every event has a line, they are all drawn and there is no
// marker.
func TestRenderAgenda_DrawsEveryEventThatFits(t *testing.T) {
	tests := []struct {
		label  string
		events int
		lines  int
	}{
		{"exact fit", 5, 5},
		{"room to spare", 1, 3},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			frame := newTestFrame(800, 480)
			drawn := renderAgenda(frame, busyRow(tt.events).Agenda, nEvents(tt.events), tt.lines, eventOptions{Location: time.UTC})
			if drawn != tt.events {
				t.Errorf("drew %d events, want all %d", drawn, tt.events)
			}
		})
	}
}

// An empty day says so; a blank strip reads as a fault.
func TestRenderAgenda_EmptyDay(t *testing.T) {
	frame := newTestFrame(800, 480)
	if drawn := renderAgenda(frame, agendaRect(), nil, 1, eventOptions{Location: time.UTC}); drawn != 0 {
		t.Errorf("drew %d events, want 0", drawn)
	}
	if countIndexIn(frame, agendaRect(), widget.PaperBlack) == 0 {
		t.Error("an empty day drew nothing at all")
	}
}

func TestRenderAgenda_TooNarrow(t *testing.T) {
	frame := newTestFrame(800, 480)
	if drawn := renderAgenda(frame, image.Rect(0, 0, 40, 96), nEvents(2), 2, eventOptions{Location: time.UTC}); drawn != 0 {
		t.Errorf("drew %d events into a 40 px row, want 0", drawn)
	}
}

// A slot with room for the time but not for a title draws the time
// alone rather than an ellipsis where the title should be.
func TestDrawEventInSlot_NoRoomForATitle(t *testing.T) {
	frame := newTestFrame(800, 480)
	timeW := daygrid.TextWidth(daygrid.BodyFace, "ALL DAY ")
	s := slot{x: 0, y: 30, width: timeW + daygrid.BodyAdvance()}
	drawEventInSlot(frame, s, nEvents(1)[0], eventOptions{Location: time.UTC})

	// The time is drawn; nothing is drawn past where a title would go.
	if countIndexIn(frame, image.Rect(0, 0, timeW, 480), widget.PaperBlack) == 0 {
		t.Error("the time was not drawn")
	}
}

func TestFitTo(t *testing.T) {
	adv := daygrid.BodyAdvance()
	tests := []struct {
		label string
		in    string
		width int
		want  string
	}{
		{"fits", "Standup", 10 * adv, "Standup"},
		{"exactly fits", "Standup", 7 * adv, "Standup"},
		{"ellipsed", "Design review", 7 * adv, "Design»"},
		{"one character of room", "Design", 1 * adv, "D"},
		{"no room at all", "Design", 0, ""},
		{"trims surrounding space", "  Standup  ", 10 * adv, "Standup"},
		// The budget is in characters, so the cut is in runes: byte
		// slicing would halve a multi-byte glyph.
		{"multi-byte title", "日本語のミーティング", 5 * adv, "日本語の»"},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			if got := fitTo(tt.in, tt.width); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTimeLineAndTitle(t *testing.T) {
	toronto, err := time.LoadLocation("America/Toronto")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 3, 16, 20, 15, 0, 0, time.UTC)
	timed := calendar.Event{Summary: "Design review", Location: "Room 2", Start: start, End: start.Add(time.Hour)}
	allDay := calendar.Event{Summary: "Conference", AllDay: true}

	utcOpts := eventOptions{Location: time.UTC}
	if got := timeLineFor(timed, utcOpts); got != "20:15" {
		t.Errorf("timeLineFor = %q, want 20:15", got)
	}
	if got := timeLineFor(allDay, utcOpts); got != "ALL DAY" {
		t.Errorf("all-day timeLineFor = %q, want ALL DAY", got)
	}
	// Labelled in the dashboard's zone, not the feed's.
	if got := timeLineFor(timed, eventOptions{Location: toronto}); got != "16:15" {
		t.Errorf("timeLineFor in Toronto = %q, want 16:15", got)
	}

	if got := titleFor(timed, utcOpts); got != "Design review" {
		t.Errorf("titleFor = %q", got)
	}
	withLoc := eventOptions{Location: time.UTC, ShowLocation: true}
	if got := titleFor(timed, withLoc); got != "Design review @ Room 2" {
		t.Errorf("titleFor with location = %q", got)
	}
	if got := titleFor(allDay, withLoc); got != "Conference" {
		t.Errorf("titleFor with no location = %q", got)
	}
}

// A day the forecast never covered draws no badge: a zero
// DailyForecast is indistinguishable from a real 0°/0° reading.
func TestRenderBadge_MissingForecast(t *testing.T) {
	frame := newTestFrame(800, 480)
	renderBadge(frame, busyRow(0).Badge,
		weather.DailyForecast{}, "C", true, true, 14, weatherview.TempRange{Min: 0, Max: 25})
	if got := countIndexIn(frame, frame.Bounds(), widget.PaperBlack); got != 0 {
		t.Errorf("drew %d px for a day with no forecast", got)
	}
}

// Every row carries the combined chart, so a dry day still draws its
// temperature line rather than leaving the chart blank, and two days on
// one shared range sit at different heights when one is colder.
func TestRenderBadge_CombinedChart(t *testing.T) {
	badge := busyRow(0).Badge
	chart := image.Rect(badge.Min.X+chartDX, badge.Min.Y, badge.Min.X+chartDX+chartW, badge.Max.Y)
	rng := weatherview.TempRange{Min: -10, Max: 30}

	dryDay := func(temp float64) weather.DailyForecast {
		var hourly []weather.HourlyPoint
		for h := range 24 {
			hourly = append(hourly, weather.HourlyPoint{Hour: h, Temperature: temp})
		}
		return weather.DailyForecast{
			Date: time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC),
			High: temp, Low: temp, Hourly: hourly,
		}
	}

	// topInk is the first row of the chart carrying ink above its
	// baseline, which on a dry day is the temperature line.
	topInk := func(frame *image.Paletted) int {
		for y := chart.Min.Y; y < chart.Max.Y; y++ {
			if countIndexIn(frame, image.Rect(chart.Min.X, y, chart.Max.X, y+1), widget.PaperBlack) > 0 {
				return y
			}
		}
		return -1
	}

	cold, warm := newTestFrame(800, 480), newTestFrame(800, 480)
	renderBadge(cold, badge, dryDay(-5), "C", false, false, 14, rng)
	renderBadge(warm, badge, dryDay(25), "C", false, false, 14, rng)

	coldY, warmY := topInk(cold), topInk(warm)
	if coldY < 0 || warmY < 0 {
		t.Fatal("a dry day drew a blank chart")
	}
	if warmY >= coldY {
		t.Errorf("the warm day's line (y=%d) is not above the cold day's (y=%d)", warmY, coldY)
	}
}

// A condition glyph that will not load must not take the temperatures
// with it.
func TestRenderBadge_IconFailure(t *testing.T) {
	orig := drawIcon
	defer func() { drawIcon = orig }()
	drawIcon = func(*image.Paletted, int, int, int, weather.Condition) error { return errIcon{} }

	frame := newTestFrame(800, 480)
	renderBadge(frame, busyRow(0).Badge,
		weather.DailyForecast{
			Date: time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC),
			High: 14, Low: 3,
		}, "C", true, true, 14, weatherview.TempRange{Min: 0, Max: 25})
	if countIndexIn(frame, frame.Bounds(), widget.PaperBlack) == 0 {
		t.Error("nothing drawn after the icon failed")
	}
}

type errIcon struct{}

func (errIcon) Error() string { return "no glyph" }

// A row shorter than its slots is not a configuration this screen
// offers, but the widget is positioned by the dashboard and the draw
// helpers clip to the frame rather than to the row — so the slot list
// stops at the row's bottom edge rather than running past it and
// painting into the next day.
func TestLayoutSlots_ShortRowDropsSlots(t *testing.T) {
	wide := agendaRect().Dx()
	// Tall enough for one line and no more.
	short := image.Rect(0, 0, wide, agendaPadY+daygrid.BodyLineH()+1)

	slots := layoutSlots(short, 3)
	if len(slots) != 1 {
		t.Fatalf("got %d slots in a one-line row, want 1", len(slots))
	}
	// Asserted against the glyph's *descender*, not the baseline. The
	// code used to branch on the baseline alone, and the test asserted
	// the same predicate — so it could not fail, and it said nothing
	// about the claim in its own name.
	descent := daygrid.BodyFace.Metrics().Descent.Ceil()
	if s := slots[0]; s.y+descent >= short.Max.Y {
		t.Errorf("slot at baseline %d descends past the row's %d", s.y, short.Max.Y)
	}
}

// The temperature block and the chart share the badge, and the chart is
// drawn second — so an overlap does not read as a layout slip, it reads
// as bars painted through the digits. The shipped goldens cannot catch
// it because their rain sits in hours 12-17, well right of the label.
func TestRenderBadge_TemperatureNeverReachesTheChart(t *testing.T) {
	badge := busyRow(0).Badge

	// Morning rain, so the leftmost bars land where the label would
	// overrun if it could.
	var hourly []weather.HourlyPoint
	for h := range 24 {
		prob := 0.0
		if h >= 6 && h <= 9 {
			prob = 0.9
		}
		hourly = append(hourly, weather.HourlyPoint{Hour: h, PrecipitationProb: prob})
	}

	// The widest readings the block has to hold.
	tests := []struct {
		label  string
		hi, lo float64
		unit   string
	}{
		{"negative celsius", -15, -22, "C"},
		{"three-digit fahrenheit", 37.8, 20, "F"}, // 100°F
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			frame := newTestFrame(800, 480)
			renderBadge(frame, badge, weather.DailyForecast{
				Date: time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC),
				High: tt.hi, Low: tt.lo, Hourly: hourly,
			}, tt.unit, true, true, 8, weatherview.TempRange{Min: -30, Max: 40})

			// The widest reading the block can hold must end before
			// the chart's left edge.
			if end := hiDX + tempMaxChars*daygrid.BodyAdvance(); end > chartDX {
				t.Errorf("temperature block can reach %d, chart starts at %d — they overlap", end, chartDX)
			}
			// And this day's actual reading must not have drawn into
			// the chart's first bar slot either.
			if daygrid.TextWidth(daygrid.BodyBoldFace, "-100°F") > tempMaxChars*daygrid.BodyAdvance() {
				t.Error("tempMaxChars no longer covers the widest reading")
			}
		})
	}
}

// The empty-day message is a fixed string, so at the narrowest bounds
// the widget accepts it has to be fitted like every other one — an
// unfitted 170 px message in a 100 px slot paints straight over
// whatever shares the frame.
func TestRenderAgenda_EmptyMessageStaysInBounds(t *testing.T) {
	bounds := image.Rect(0, 0, minWidth, 96)
	frame := newTestFrame(800, 480)
	renderAgenda(frame, image.Rect(agendaX, 0, minWidth, 96), nil, 1, eventOptions{Location: time.UTC})

	for y := range 480 {
		for x := bounds.Max.X; x < 800; x++ {
			if frame.ColorIndexAt(x, y) != widget.PaperWhite {
				t.Fatalf("drew at (%d,%d), past the widget's %d px edge", x, y, bounds.Max.X)
			}
		}
	}
}
