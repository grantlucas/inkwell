package daytimeline

import (
	"image"
	"slices"
	"testing"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar/ical"
	"github.com/grantlucas/inkwell/internal/inkwell/testutil"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daygrid"
)

// testTime is a Monday early afternoon, so today has finished events
// and ones still to come.
var testTime = time.Date(2026, 3, 16, 13, 30, 0, 0, time.UTC)

// testBounds is the left of the panel under a header band, where the
// composed day-timeline screen puts the widget.
var testBounds = image.Rect(0, 48, 500, 480)

func fixedClock(t time.Time) func() time.Time { return func() time.Time { return t } }

// at is a time today, in the display zone.
func at(hour, minute int) time.Time {
	return time.Date(2026, 3, 16, hour, minute, 0, 0, time.UTC)
}

// span is a timed event today from one clock time to another.
func span(summary string, from, to time.Time) ical.Event {
	return ical.Event{UID: summary, Summary: summary, Start: from, End: to}
}

func defaultConfig() Config {
	return Config{Window: Window{StartHour: defaultStartHour, EndHour: defaultEndHour}}
}

func newWidget(bounds image.Rectangle, events []ical.Event, cfg Config) *Widget {
	return New(bounds, daygrid.InMemory(events, nil), fixedClock(testTime), cfg)
}

func newTestFrame() *image.Paletted {
	return image.NewPaletted(image.Rect(0, 0, 800, 480), widget.PaperPalette)
}

func renderToFrame(t *testing.T, w *Widget) *image.Paletted {
	t.Helper()
	frame := newTestFrame()
	if err := w.Render(frame); err != nil {
		t.Fatalf("Render: %v", err)
	}
	return frame
}

func countIndexIn(frame *image.Paletted, r image.Rectangle, idx uint8) int {
	n := 0
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			if frame.ColorIndexAt(x, y) == idx {
				n++
			}
		}
	}
	return n
}

// gridOf is where a render with no notes puts the grid and its clock.
func gridOf(bounds image.Rectangle, win Window) (layout, timeline) {
	l := computeLayout(bounds, sections{})
	return l, newTimeline(at(0, 0), win, l.Grid)
}

// An event's block runs from its start to its end, so its length on the
// grid is its length in the day. Whether it has finished changes how it
// is drawn, not where.
func TestWidget_EventsSitAtTheirTrueTimes(t *testing.T) {
	tests := []struct {
		label    string
		from, to time.Time
	}{
		{label: "finished two hours", from: at(9, 0), to: at(11, 0)},
		{label: "upcoming half hour", from: at(15, 0), to: at(15, 30)},
		{label: "upcoming off the hour", from: at(16, 40), to: at(18, 10)},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			frame := renderToFrame(t, newWidget(testBounds, []ical.Event{span("Block", tt.from, tt.to)}, defaultConfig()))
			l, tl := gridOf(testBounds, defaultConfig().Window)
			// The block's left edge, clear of any label.
			x := l.Events.Min.X
			top, bottom := tl.y(tt.from), tl.y(tt.to)

			if frame.ColorIndexAt(x, top) != widget.PaperBlack {
				t.Errorf("no ink at the start, y=%d", top)
			}
			if frame.ColorIndexAt(x, top-1) == widget.PaperBlack {
				t.Errorf("ink above the start, y=%d", top-1)
			}
			// One row of paper under every block keeps back-to-back
			// events apart.
			if frame.ColorIndexAt(x, bottom-2) != widget.PaperBlack {
				t.Errorf("no ink at the end, y=%d", bottom-2)
			}
			if frame.ColorIndexAt(x, bottom-1) == widget.PaperBlack {
				t.Errorf("ink below the end, y=%d", bottom-1)
			}
		})
	}
}

// Finished events are outlines and upcoming ones solid, so the past and
// the rest of the day read apart without reading a time. An event still
// running is not finished.
func TestWidget_FinishedEventsAreOutlinedUpcomingAreSolid(t *testing.T) {
	tests := []struct {
		label     string
		from, to  time.Time
		wantSolid bool
	}{
		{label: "finished", from: at(9, 0), to: at(11, 0), wantSolid: false},
		{label: "ending this minute", from: at(11, 30), to: at(13, 30), wantSolid: true},
		{label: "running", from: at(13, 0), to: at(15, 0), wantSolid: true},
		{label: "upcoming", from: at(16, 0), to: at(18, 0), wantSolid: true},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			frame := renderToFrame(t, newWidget(testBounds, []ical.Event{span("Block", tt.from, tt.to)}, defaultConfig()))
			l, tl := gridOf(testBounds, defaultConfig().Window)
			// Inside the block, below its label and left of its right
			// edge.
			inside := image.Rect(l.Events.Max.X-20, tl.y(tt.to)-12, l.Events.Max.X-6, tl.y(tt.to)-6)
			black := countIndexIn(frame, inside, widget.PaperBlack)
			if tt.wantSolid && black != inside.Dx()*inside.Dy() {
				t.Errorf("upcoming block interior is %d/%d black, want solid", black, inside.Dx()*inside.Dy())
			}
			if !tt.wantSolid && black != 0 {
				t.Errorf("finished block interior has %d black px, want an outline", black)
			}
		})
	}
}

// An event crossing a window edge is cut off at the edge, with a mark on
// that edge saying it carries on. The mark is drawn in the block's
// contrasting colour at its right end, clear of the label. An event that
// fits the window carries no mark.
func TestWidget_MarksEventsClippedAtAnEdge(t *testing.T) {
	const top, bottom = "top", "bottom"
	tests := []struct {
		label    string
		from, to time.Time
		wantMark []string
	}{
		{label: "fits", from: at(15, 0), to: at(17, 0)},
		{label: "finished, fits", from: at(8, 0), to: at(10, 0)},
		{label: "crosses the start", from: at(6, 0), to: at(9, 0), wantMark: []string{top}},
		{label: "crosses the end", from: at(20, 0), to: at(23, 30), wantMark: []string{bottom}},
		{label: "crosses both", from: at(5, 0), to: at(23, 0), wantMark: []string{top, bottom}},
		{label: "runs past midnight", from: at(21, 0), to: at(26, 0), wantMark: []string{bottom}},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			e := span("Block", tt.from, tt.to)
			frame := renderToFrame(t, newWidget(testBounds, []ical.Event{e}, defaultConfig()))
			l, tl := gridOf(testBounds, defaultConfig().Window)
			block := blockRect(l.Events, tl, e)

			// The block's clipped edge sits on the grid's edge row.
			if slices.Contains(tt.wantMark, top) && block.Min.Y != l.Grid.Min.Y {
				t.Errorf("block starts at %d, want the grid's top %d", block.Min.Y, l.Grid.Min.Y)
			}
			if slices.Contains(tt.wantMark, bottom) && block.Max.Y != l.Grid.Max.Y-2 {
				t.Errorf("block ends at %d, want one row above the grid's last %d", block.Max.Y, l.Grid.Max.Y-2)
			}

			// The contrast colour is paper inside a solid block and ink
			// inside an outline.
			contrast, inner := widget.PaperWhite, block
			if finished(e, testTime) {
				contrast, inner = widget.PaperBlack, block.Inset(outlineW)
			}
			corners := map[string]image.Rectangle{
				top:    image.Rect(inner.Max.X-markClear, inner.Min.Y, inner.Max.X, inner.Min.Y+10),
				bottom: image.Rect(inner.Max.X-markClear, inner.Max.Y-10, inner.Max.X, inner.Max.Y),
			}
			for edge, r := range corners {
				got := countIndexIn(frame, r, contrast) > 0
				if want := slices.Contains(tt.wantMark, edge); got != want {
					t.Errorf("%s edge: mark drawn = %v, want %v", edge, got, want)
				}
			}
		})
	}
}

// Each block is labelled with its start time and title in its contrast
// colour, paper on solid and ink in an outline, so the label reads on
// both. A long title is cut short of the mark corner, so a clipped event
// keeps its mark readable.
func TestWidget_LabelsBlocks(t *testing.T) {
	long := "Quarterly planning session with the whole platform group and guests"
	tests := []struct {
		label    string
		title    string
		from, to time.Time
	}{
		{label: "upcoming", title: "Standup", from: at(15, 0), to: at(16, 0)},
		{label: "finished", title: "Standup", from: at(9, 0), to: at(10, 0)},
		{label: "long upcoming title", title: long, from: at(15, 0), to: at(16, 0)},
		{label: "long finished title", title: long, from: at(9, 0), to: at(10, 0)},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			e := span(tt.title, tt.from, tt.to)
			frame := renderToFrame(t, newWidget(testBounds, []ical.Event{e}, defaultConfig()))
			l, tl := gridOf(testBounds, defaultConfig().Window)
			block := blockRect(l.Events, tl, e)

			contrast, inner := widget.PaperWhite, block
			if finished(e, testTime) {
				contrast, inner = widget.PaperBlack, block.Inset(outlineW)
			}
			// The label starts at the block's left: its first few
			// characters, the start time, are inked.
			timeArea := image.Rect(inner.Min.X, inner.Min.Y, inner.Min.X+5*daygrid.BodyAdvance(), inner.Max.Y)
			if countIndexIn(frame, timeArea, contrast) == 0 {
				t.Error("no label at the block's left")
			}
			corner := image.Rect(inner.Max.X-markClear, inner.Min.Y, inner.Max.X, inner.Max.Y)
			if n := countIndexIn(frame, corner, contrast); n != 0 {
				t.Errorf("label runs %d px into the mark corner", n)
			}
		})
	}
}

// A block with room for a second line says when its event ends, which
// the block's length only shows approximately. One a line tall doesn't,
// and neither does a reminder with no end.
func TestWidget_TallBlocksSayWhenTheyEnd(t *testing.T) {
	tests := []struct {
		label     string
		from, to  time.Time
		wantUntil string
	}{
		{label: "upcoming two hours", from: at(15, 0), to: at(17, 0), wantUntil: "UNTIL 17:00"},
		{label: "finished two hours", from: at(9, 0), to: at(11, 0), wantUntil: "UNTIL 11:00"},
		{label: "runs past the window", from: at(20, 0), to: at(25, 0), wantUntil: "UNTIL 01:00"},
		{label: "one hour", from: at(15, 0), to: at(16, 0)},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			e := span("Block", tt.from, tt.to)
			frame := renderToFrame(t, newWidget(testBounds, []ical.Event{e}, defaultConfig()))
			l, tl := gridOf(testBounds, defaultConfig().Window)
			block := blockRect(l.Events, tl, e)

			contrast, inner := widget.PaperWhite, block
			if finished(e, testTime) {
				contrast, inner = widget.PaperBlack, block.Inset(outlineW)
			}
			// Everything under the first line.
			below := image.Rect(inner.Min.X, inner.Min.Y+daygrid.BodyLineH(), inner.Max.X-markClear, inner.Max.Y)
			if tt.wantUntil == "" {
				if n := countIndexIn(frame, below, contrast); n != 0 {
					t.Errorf("%d px under the first line of a block with no room for a second", n)
				}
				return
			}
			// The second line reads exactly the end time, written in
			// the regular cut at the label's left.
			ref := newTestFrame()
			daygrid.FillRect(ref, block, widget.PaperBlack)
			if finished(e, testTime) {
				daygrid.FillWhite(ref, inner)
			}
			daygrid.DrawText(ref, inner.Min.X+labelPadX, labelBaseline(inner)+daygrid.BodyLineH(),
				tt.wantUntil, daygrid.BodyFace, contrast)
			if !sameIn(frame, ref, below) {
				t.Errorf("second line doesn't read %q", tt.wantUntil)
			}
		})
	}
}

// A block too short to hold a line of capitals carries no label: the
// tops of clipped glyphs would read as noise, not words.
func TestWidget_ShortBlocksAreUnlabelled(t *testing.T) {
	tests := []struct {
		label    string
		from, to time.Time
	}{
		{label: "upcoming ten minutes", from: at(15, 0), to: at(15, 10)},
		{label: "finished ten minutes", from: at(9, 0), to: at(9, 10)},
		{label: "a reminder with no end", from: at(16, 0), to: at(16, 0)},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			e := span("Standup", tt.from, tt.to)
			frame := renderToFrame(t, newWidget(testBounds, []ical.Event{e}, defaultConfig()))
			l, tl := gridOf(testBounds, defaultConfig().Window)
			block := blockRect(l.Events, tl, e)

			contrast, inner := widget.PaperWhite, block
			if finished(e, testTime) {
				contrast, inner = widget.PaperBlack, block.Inset(outlineW)
			}
			if n := countIndexIn(frame, inner, contrast); n != 0 {
				t.Errorf("%d px of label in a %d-row block", n, block.Dy())
			}
		})
	}
}

// The label writes the event's start in the display zone, whatever zone
// its feed serialised it in.
func TestWidget_LabelTimeIsInTheDisplayZone(t *testing.T) {
	toronto, err := time.LoadLocation("America/Toronto")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 3, 16, 8, 0, 0, 0, toronto)
	// 19:00 UTC is 15:00 in Toronto on 16 March (EDT).
	e := span("Call", time.Date(2026, 3, 16, 19, 0, 0, 0, time.UTC), time.Date(2026, 3, 16, 20, 0, 0, 0, time.UTC))

	frame := newTestFrame()
	w := New(testBounds, daygrid.InMemory([]ical.Event{e}, nil), fixedClock(now), defaultConfig())
	if err := w.Render(frame); err != nil {
		t.Fatal(err)
	}

	l := computeLayout(testBounds, sections{})
	tl := newTimeline(time.Date(2026, 3, 16, 0, 0, 0, 0, toronto), defaultConfig().Window, l.Grid)
	block := blockRect(l.Events, tl, e)
	if block.Min.Y != tl.y(time.Date(2026, 3, 16, 15, 0, 0, 0, toronto)) {
		t.Fatalf("block starts at row %d, not 15:00 Toronto", block.Min.Y)
	}

	ref := newTestFrame()
	daygrid.FillRect(ref, block, widget.PaperBlack)
	drawLabel(ref, block, e, toronto, false, widget.PaperWhite)
	if !sameIn(frame, ref, block) {
		t.Error("label differs from one written at 15:00")
	}
	wrong := newTestFrame()
	daygrid.FillRect(wrong, block, widget.PaperBlack)
	drawLabel(wrong, block, e, time.UTC, false, widget.PaperWhite)
	if sameIn(frame, wrong, block) {
		t.Error("label matches one written in UTC")
	}
}

// sameIn reports whether a and b agree on every pixel of r.
func sameIn(a, b *image.Paletted, r image.Rectangle) bool {
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			if a.ColorIndexAt(x, y) != b.ColorIndexAt(x, y) {
				return false
			}
		}
	}
	return true
}

// Every hour in the window gets a label in the gutter and a rule across
// the event column, and the window's last edge gets a rule too.
func TestWidget_DrawsAnHourGrid(t *testing.T) {
	tests := []struct {
		label string
		win   Window
	}{
		{label: "default", win: Window{StartHour: 7, EndHour: 22}},
		{label: "working day", win: Window{StartHour: 9, EndHour: 17}},
		{label: "whole day", win: Window{StartHour: 0, EndHour: 24}},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			cfg := Config{Window: tt.win}
			frame := renderToFrame(t, newWidget(testBounds, nil, cfg))
			l, tl := gridOf(testBounds, tt.win)
			start, _ := tt.win.on(at(0, 0))

			// A solid rule runs the grid's height between the hour
			// labels (and the weather lane after them) and the events.
			ruleX := l.Lane.Max.X
			if ruleX < l.Gutter.Max.X || ruleX >= l.Events.Min.X {
				t.Fatalf("rule at x=%d is not between the labels (to %d) and the events (from %d)",
					ruleX, l.Gutter.Max.X, l.Events.Min.X)
			}
			if n := countIndexIn(frame, image.Rect(ruleX, l.Grid.Min.Y, ruleX+1, l.Grid.Max.Y), widget.PaperBlack); n != l.Grid.Dy() {
				t.Errorf("rule between labels and events is %d/%d px", n, l.Grid.Dy())
			}

			for h := 0; h <= tt.win.EndHour-tt.win.StartHour; h++ {
				y := tl.y(start.Add(time.Duration(h) * time.Hour))
				if countIndexIn(frame, image.Rect(l.Events.Min.X, y, l.Events.Max.X, y+1), widget.PaperBlack) == 0 {
					t.Errorf("no rule across the events at hour %d (y=%d)", tt.win.StartHour+h, y)
				}
			}
			// Labels: the gutter is inked in every labelled hour's row,
			// and every other hour is labelled when the rows are too
			// short for a label each.
			step := 1
			if tl.y(start.Add(time.Hour))-tl.y(start) < daygrid.BodyLineH() {
				step = 2
			}
			for h := 0; h < tt.win.EndHour-tt.win.StartHour; h += step {
				y0 := tl.y(start.Add(time.Duration(h) * time.Hour))
				row := image.Rect(l.Gutter.Min.X, y0+1, l.Gutter.Max.X-1, y0+daygrid.BodyLineH())
				if countIndexIn(frame, row, widget.PaperBlack) == 0 {
					t.Errorf("no label for hour %d", tt.win.StartHour+h)
				}
			}
		})
	}
}

// typicalDay is a weekday with the morning behind it and the afternoon
// ahead: back-to-back blocks, a short one, a long one, one running now
// and a title too long for the column.
func typicalDay() []ical.Event {
	return []ical.Event{
		span("Standup", at(9, 0), at(9, 30)),
		span("Design review", at(9, 30), at(11, 0)),
		span("Lunch", at(12, 0), at(13, 0)),
		span("1:1 with Sam", at(13, 0), at(14, 0)),
		span("Quarterly planning session with the platform group", at(15, 0), at(17, 0)),
		span("Swim lessons", at(18, 30), at(19, 15)),
	}
}

// Golden renders of the widget. These are the regression net for the
// geometry: every constant in this package shows up in them.
func TestWidget_Golden(t *testing.T) {
	tests := []struct {
		label  string
		events []ical.Event
		cfg    func(*Config)
		now    time.Time
	}{
		{label: "typical day", events: typicalDay()},
		{
			label: "clipped at each edge",
			events: []ical.Event{
				span("Red-eye flight home", at(5, 0), at(8, 30)),
				span("Lunch", at(12, 0), at(13, 0)),
				span("Late release window", at(20, 30), at(25, 0)),
			},
		},
		{
			label: "outside the window on both sides",
			events: []ical.Event{
				span("Gym", at(5, 30), at(6, 30)),
				span("Commute", at(6, 30), at(7, 0)),
				span("Lunch", at(12, 0), at(13, 0)),
				span("Late call", at(22, 0), at(23, 0)),
				span("Take pills", at(23, 0), at(23, 0)),
			},
		},
		{label: "empty day"},
		{
			label:  "working hours window",
			events: typicalDay(),
			cfg:    func(c *Config) { c.Window = Window{StartHour: 9, EndHour: 17} },
		},
		{
			label:  "whole day window",
			events: typicalDay(),
			cfg:    func(c *Config) { c.Window = Window{StartHour: 0, EndHour: 24} },
		},
		{
			label: "locations shown in the evening",
			events: []ical.Event{
				{UID: "l", Summary: "Lunch", Location: "Cafe", Start: at(12, 0), End: at(13, 0)},
				{UID: "d", Summary: "Dinner", Location: "Nonna's", Start: at(19, 0), End: at(21, 0)},
			},
			cfg: func(c *Config) { c.ShowLocation = true },
			now: at(20, 0),
		},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			cfg := defaultConfig()
			if tt.cfg != nil {
				tt.cfg(&cfg)
			}
			now := tt.now
			if now.IsZero() {
				now = testTime
			}
			w := New(testBounds, daygrid.InMemory(tt.events, nil), fixedClock(now), cfg)
			testutil.AssertGoldenPNG(t, renderToFrame(t, w))
		})
	}
}

// paintNeighbours inks every pixel of the frame outside bounds, standing
// in for the widgets around this one.
func paintNeighbours(frame *image.Paletted, bounds image.Rectangle) {
	for y := range frame.Bounds().Dy() {
		for x := range frame.Bounds().Dx() {
			if !image.Pt(x, y).In(bounds) {
				frame.SetColorIndex(x, y, widget.PaperBlack)
			}
		}
	}
}

// The draw helpers clip to the frame, not the widget, so everything the
// widget draws has to land inside its bounds by construction: on a busy
// day, with notes on both sides, and at the smallest size it draws at.
func TestWidget_StaysInsideItsBounds(t *testing.T) {
	busy := append(typicalDay(),
		span("Gym", at(5, 0), at(6, 0)),
		span("Late call", at(22, 0), at(23, 0)),
		span("Overnight", at(20, 0), at(30, 0)),
		span("Early", at(4, 0), at(8, 0)),
	)
	tests := []struct {
		label  string
		bounds image.Rectangle
	}{
		{label: "left of the panel", bounds: testBounds},
		{label: "in the middle", bounds: image.Rect(150, 100, 650, 400)},
		{label: "smallest", bounds: image.Rect(300, 200, 300+minWidth, 200+minHeight)},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			frame := newTestFrame()
			paintNeighbours(frame, tt.bounds)
			w := newWidget(tt.bounds, busy, defaultConfig())
			if err := w.Render(frame); err != nil {
				t.Fatalf("Render: %v", err)
			}
			for y := range frame.Bounds().Dy() {
				for x := range frame.Bounds().Dx() {
					if !image.Pt(x, y).In(tt.bounds) && frame.ColorIndexAt(x, y) != widget.PaperBlack {
						t.Fatalf("painted outside its bounds at (%d,%d)", x, y)
					}
				}
			}
			if countIndexIn(frame, tt.bounds, widget.PaperBlack) == 0 {
				t.Error("drew nothing inside its bounds")
			}
		})
	}
}

// Given less room than the grid needs, the widget leaves its bounds
// blank rather than drawing a grid too cramped to read or spilling onto a
// neighbour. A blank region is a misconfiguration you can see.
func TestWidget_TooSmallDrawsNothing(t *testing.T) {
	tests := []struct {
		label  string
		bounds image.Rectangle
	}{
		{label: "too short", bounds: image.Rect(0, 0, 500, minHeight-1)},
		{label: "too narrow", bounds: image.Rect(0, 0, minWidth-1, 480)},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			frame := newTestFrame()
			paintNeighbours(frame, tt.bounds)
			daygrid.FillRect(frame, tt.bounds, widget.PaperBlack)
			if err := newWidget(tt.bounds, typicalDay(), defaultConfig()).Render(frame); err != nil {
				t.Fatalf("Render: %v", err)
			}
			if n := countIndexIn(frame, tt.bounds, widget.PaperBlack); n != 0 {
				t.Errorf("drew %d px into a widget too small to draw into", n)
			}
		})
	}
}

// noteIn reports whether band carries exactly the note text, drawn where
// the widget draws its notes, and nothing else.
func noteIn(frame *image.Paletted, band image.Rectangle, x int, want string) bool {
	ref := newTestFrame()
	drawNote(ref, band, x, want)
	for y := band.Min.Y; y < band.Max.Y; y++ {
		for xx := band.Min.X; xx < band.Max.X; xx++ {
			if frame.ColorIndexAt(xx, y) != ref.ColorIndexAt(xx, y) {
				return false
			}
		}
	}
	return countIndexIn(ref, band, widget.PaperBlack) > 0
}

// Events wholly outside the window aren't drawn; a note above or below
// the grid counts them, so nothing on the calendar goes missing without
// a trace. An event touching the window's edge from outside is outside.
func TestWidget_CountsEventsOutsideTheWindow(t *testing.T) {
	tests := []struct {
		label       string
		events      []ical.Event
		wantEarlier string
		wantLater   string
	}{
		{
			label:  "nothing outside",
			events: []ical.Event{span("Lunch", at(12, 0), at(13, 0))},
		},
		{
			label: "both sides",
			events: []ical.Event{
				span("Gym", at(5, 0), at(6, 0)),
				span("Commute", at(6, 0), at(7, 0)),
				span("Lunch", at(12, 0), at(13, 0)),
				span("Late call", at(22, 0), at(23, 0)),
			},
			wantEarlier: "+2 EARLIER",
			wantLater:   "+1 LATER",
		},
		{
			label: "only later, with a reminder that has no end",
			events: []ical.Event{
				span("Late call", at(22, 30), at(23, 0)),
				span("Take pills", at(23, 0), at(23, 0)),
			},
			wantLater: "+2 LATER",
		},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			frame := renderToFrame(t, newWidget(testBounds, tt.events, defaultConfig()))
			l := computeLayout(testBounds, sections{Earlier: tt.wantEarlier != "", Later: tt.wantLater != ""})

			if tt.wantEarlier != "" && !noteIn(frame, l.Earlier, l.Events.Min.X, tt.wantEarlier) {
				t.Errorf("no %q note above the grid", tt.wantEarlier)
			}
			if tt.wantLater != "" && !noteIn(frame, l.Later, l.Events.Min.X, tt.wantLater) {
				t.Errorf("no %q note below the grid", tt.wantLater)
			}
			// Without a note the grid takes the height, and its edge
			// rules sit at the bounds' padding.
			if tt.wantEarlier == "" && l.Grid.Min.Y != testBounds.Min.Y+gridPadY {
				t.Errorf("grid starts at %d with nothing earlier", l.Grid.Min.Y)
			}
			if tt.wantLater == "" && l.Grid.Max.Y != testBounds.Max.Y-gridPadY {
				t.Errorf("grid ends at %d with nothing later", l.Grid.Max.Y)
			}
		})
	}
}

// Nothing outside the window reaches the grid: down the event column's
// edges there is no ink but the one-pixel hour rules.
func TestWidget_DrawsNoBlockForEventsOutsideTheWindow(t *testing.T) {
	events := []ical.Event{span("Gym", at(5, 0), at(7, 0)), span("Late call", at(22, 0), at(23, 0))}
	frame := renderToFrame(t, newWidget(testBounds, events, defaultConfig()))
	l := computeLayout(testBounds, sections{Earlier: true, Later: true})

	for _, x := range []int{l.Events.Min.X, l.Events.Max.X - 1} {
		run := 0
		for y := l.Grid.Min.Y; y < l.Grid.Max.Y; y++ {
			if frame.ColorIndexAt(x, y) != widget.PaperBlack {
				run = 0
				continue
			}
			if run++; run > 1 {
				t.Fatalf("column x=%d is inked for more than a rule at y=%d; nothing should be placed", x, y)
			}
		}
	}
}
