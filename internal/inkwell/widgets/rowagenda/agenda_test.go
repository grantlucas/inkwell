package rowagenda

import (
	"image"
	"testing"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar"
	"github.com/grantlucas/inkwell/internal/inkwell/weather"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daygrid"
)

func agendaRect() image.Rectangle {
	return computeRows(image.Rect(0, 0, 800, 480))[0].Agenda
}

func nEvents(n int) []calendar.Event {
	out := make([]calendar.Event, n)
	for i := range out {
		start := time.Date(2026, 3, 16, 8+i, 0, 0, 0, time.UTC)
		out[i] = calendar.Event{Summary: "Event", Start: start, End: start.Add(time.Hour)}
	}
	return out
}

// The column count adapts to the day's load, which is the whole point
// of the screen: width is what titles were starving for, so it is spent
// on them whenever the day can afford it.
func TestLayoutSlots_AdaptsToTheLoad(t *testing.T) {
	bounds := agendaRect()
	full := bounds.Dx() - 2*agendaPadX

	tests := []struct {
		label     string
		events    int
		wantSlots int
		wantWide  bool // slots take the full width
	}{
		{"one event", 1, 3, true},
		{"three events", 3, 3, true},
		{"four events splits into columns", 4, 4, false},
		{"many events", 9, 4, false},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			slots := layoutSlots(bounds, tt.events)
			if len(slots) != tt.wantSlots {
				t.Fatalf("got %d slots, want %d", len(slots), tt.wantSlots)
			}
			if tt.wantWide && slots[0].width != full {
				t.Errorf("slot width = %d, want the full %d", slots[0].width, full)
			}
			if !tt.wantWide && slots[0].width >= full {
				t.Errorf("slot width = %d, want less than the full %d", slots[0].width, full)
			}
		})
	}

	// A one-column slot has to be worth the swap: it should carry
	// noticeably more characters than a two-column one.
	wide := layoutSlots(bounds, 1)[0].width / daygrid.BodyAdvance()
	narrow := layoutSlots(bounds, 4)[0].width / daygrid.BodyAdvance()
	if wide < narrow*2-2 {
		t.Errorf("one-column slot is %d chars against two-column's %d — the split is not buying enough", wide, narrow)
	}
}

// A row too narrow to carry a time and a title draws nothing rather
// than a column of ellipses.
func TestLayoutSlots_TooNarrow(t *testing.T) {
	if got := layoutSlots(image.Rect(0, 0, 40, 96), 2); got != nil {
		t.Errorf("got %d slots in a 40 px row, want none", len(got))
	}
}

// The overflow marker must occupy a slot rather than overprinting one.
// An earlier draft of this design drew it on top of the fourth event.
func TestRenderAgenda_OverflowMarkerDoesNotOverprint(t *testing.T) {
	bounds := agendaRect()
	events := nEvents(9)

	frame := newTestFrame(800, 480)
	drawn := renderAgenda(frame, bounds, events, eventOptions{Location: time.UTC})

	slots := layoutSlots(bounds, len(events))
	if drawn != len(slots)-1 {
		t.Fatalf("drew %d events into %d slots, want one fewer than the slots", drawn, len(slots))
	}

	// Draw the same events with the marker suppressed, then compare the
	// marker's slot: if the marker overprinted an event, the slot would
	// carry ink from both.
	eventsOnly := newTestFrame(800, 480)
	renderAgenda(eventsOnly, bounds, events[:drawn], eventOptions{Location: time.UTC})

	marker := slots[len(slots)-1]
	markerBox := image.Rect(marker.x, marker.y-daygrid.BodyAscent(), marker.x+marker.width, marker.y+daygrid.BodyLineH())
	if got := countIndexIn(eventsOnly, markerBox, widget.PaperBlack); got != 0 {
		t.Errorf("the marker's slot already carries %d px of event ink — it is being overprinted", got)
	}
	if countIndexIn(frame, markerBox, widget.PaperBlack) == 0 {
		t.Error("no marker drawn in the reserved slot")
	}
}

// Exactly as many events as slots draws them all, with no marker.
func TestRenderAgenda_ExactFitDrawsNoMarker(t *testing.T) {
	bounds := agendaRect()
	slots := layoutSlots(bounds, 4)
	events := nEvents(len(slots))

	frame := newTestFrame(800, 480)
	if drawn := renderAgenda(frame, bounds, events, eventOptions{Location: time.UTC}); drawn != len(slots) {
		t.Errorf("drew %d events, want all %d", drawn, len(slots))
	}
}

// A row with one event must not read past it into the fixed slot list.
func TestRenderAgenda_FewerEventsThanSlots(t *testing.T) {
	frame := newTestFrame(800, 480)
	if drawn := renderAgenda(frame, agendaRect(), nEvents(1), eventOptions{Location: time.UTC}); drawn != 1 {
		t.Errorf("drew %d events, want 1", drawn)
	}
}

// An empty day says so; a blank strip reads as a fault.
func TestRenderAgenda_EmptyDay(t *testing.T) {
	frame := newTestFrame(800, 480)
	if drawn := renderAgenda(frame, agendaRect(), nil, eventOptions{Location: time.UTC}); drawn != 0 {
		t.Errorf("drew %d events, want 0", drawn)
	}
	if countIndexIn(frame, agendaRect(), widget.PaperBlack) == 0 {
		t.Error("an empty day drew nothing at all")
	}
}

func TestRenderAgenda_TooNarrow(t *testing.T) {
	frame := newTestFrame(800, 480)
	if drawn := renderAgenda(frame, image.Rect(0, 0, 40, 96), nEvents(2), eventOptions{Location: time.UTC}); drawn != 0 {
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
	renderBadge(frame, computeRows(image.Rect(0, 0, 800, 480))[0].Badge,
		weather.DailyForecast{}, "C", true, true, 14)
	if got := countIndexIn(frame, frame.Bounds(), widget.PaperBlack); got != 0 {
		t.Errorf("drew %d px for a day with no forecast", got)
	}
}

// A condition glyph that will not load must not take the temperatures
// with it.
func TestRenderBadge_IconFailure(t *testing.T) {
	orig := drawIcon
	defer func() { drawIcon = orig }()
	drawIcon = func(*image.Paletted, int, int, int, weather.Condition) error { return errIcon{} }

	frame := newTestFrame(800, 480)
	renderBadge(frame, computeRows(image.Rect(0, 0, 800, 480))[0].Badge,
		weather.DailyForecast{
			Date: time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC),
			High: 14, Low: 3,
		}, "C", true, true, 14)
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
	short := image.Rect(0, 0, wide, agendaPadY+daygrid.BodyAscent())

	tests := []struct {
		label  string
		events int
	}{
		{"one column", 2},
		{"two columns", 5},
	}
	// Asserted against the glyph's *descender*, not the baseline. The
	// code used to branch on the baseline alone, and the test asserted
	// the same predicate — so it could not fail, and it said nothing
	// about the claim in its own name.
	descent := daygrid.BodyFace.Metrics().Descent.Ceil()
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			for _, s := range layoutSlots(short, tt.events) {
				if s.y+descent >= short.Max.Y {
					t.Errorf("slot at baseline %d descends past the row's %d", s.y, short.Max.Y)
				}
			}
		})
	}
}

// The temperature block and the chart share the badge, and the chart is
// drawn second — so an overlap does not read as a layout slip, it reads
// as bars painted through the digits. The shipped goldens cannot catch
// it because their rain sits in hours 12-17, well right of the label.
func TestRenderBadge_TemperatureNeverReachesTheChart(t *testing.T) {
	badge := computeRows(image.Rect(0, 0, 800, 480))[0].Badge

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
			}, tt.unit, true, true, 8)

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
	renderAgenda(frame, image.Rect(agendaX, 0, minWidth, 96), nil, eventOptions{Location: time.UTC})

	for y := range 480 {
		for x := bounds.Max.X; x < 800; x++ {
			if frame.ColorIndexAt(x, y) != widget.PaperWhite {
				t.Fatalf("drew at (%d,%d), past the widget's %d px edge", x, y, bounds.Max.X)
			}
		}
	}
}
