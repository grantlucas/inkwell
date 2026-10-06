package daytimeline

import (
	"image"
	"slices"
	"testing"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar/ical"
	"github.com/grantlucas/inkwell/internal/inkwell/testutil"
	"github.com/grantlucas/inkwell/internal/inkwell/weather"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daydata"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/drawkit"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/eventlist"
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
	return New(bounds, daydata.InMemory(events, nil), fixedClock(testTime), cfg)
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
		{label: "upcoming hour", from: at(15, 0), to: at(16, 0)},
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
			timeArea := image.Rect(inner.Min.X, inner.Min.Y, inner.Min.X+5*drawkit.BodyAdvance(), inner.Max.Y)
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

// A block with room for more than one line gives them to the title: it
// wraps at word boundaries onto the lines under the start time, and only
// the last line that fits is cut with ». The block's height and its
// continuation marks say where the event ends. A block one line tall
// keeps the time and the title on that line, and draws nothing under it.
func TestWidget_TallBlocksWrapTheTitle(t *testing.T) {
	const title = "Quarterly planning session with the extended platform group"
	tests := []struct {
		label    string
		title    string
		from, to time.Time
		// want is each line after the start time, the first beside it.
		want []string
	}{
		{
			label: "upcoming, wrapped at a word",
			title: title, from: at(15, 0), to: at(16, 30),
			want: []string{"Quarterly planning session", "with the extended platform group"},
		},
		{
			label: "finished, wrapped at a word",
			title: title, from: at(9, 0), to: at(10, 30),
			want: []string{"Quarterly planning session", "with the extended platform group"},
		},
		{
			label: "the last line that fits is cut",
			title: "Quarterly planning session with the extended platform engineering group",
			from:  at(15, 0), to: at(16, 30),
			want: []string{"Quarterly planning session", "with the extended platform enginee»"},
		},
		{
			label: "one line tall",
			title: title, from: at(15, 0), to: at(16, 0),
			want: []string{"Quarterly planning session w»"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			e := span(tt.title, tt.from, tt.to)
			frame := renderToFrame(t, newWidget(testBounds, []ical.Event{e}, defaultConfig()))
			l, tl := gridOf(testBounds, defaultConfig().Window)
			block := blockRect(l.Events, tl, e)

			ref := newTestFrame()
			drawkit.FillRect(ref, block, widget.PaperBlack)
			contrast, inner := widget.PaperWhite, block
			if finished(e, testTime) {
				contrast, inner = widget.PaperBlack, block.Inset(outlineW)
				drawkit.FillWhite(ref, inner)
			}
			x, baseline := inner.Min.X+labelPadX, labelBaseline(inner)
			clip, _ := ref.SubImage(inner).(*image.Paletted)
			drawkit.DrawText(clip, x, baseline, tt.from.Format("15:04"), drawkit.BodyBoldFace, contrast)
			drawkit.DrawText(clip, x+6*drawkit.BodyAdvance(), baseline, tt.want[0], drawkit.BodyFace, contrast)
			for i, line := range tt.want[1:] {
				drawkit.DrawText(clip, x, baseline+(i+1)*drawkit.BodyLineH(), line, drawkit.BodyFace, contrast)
			}
			if !sameIn(frame, ref, block) {
				t.Errorf("block doesn't read %q", tt.want)
			}
		})
	}
}

// An event too short for a legible label still gets one: its block
// starts at its true time but is drawn a whole text line tall, so the
// label is written in full rather than clipped or left off. The block
// is exactly that line, with no second line of title under it.
func TestWidget_ShortBlocksGetAWholeLine(t *testing.T) {
	tests := []struct {
		label    string
		from, to time.Time
	}{
		{label: "upcoming ten minutes", from: at(15, 0), to: at(15, 10)},
		{label: "finished ten minutes", from: at(9, 0), to: at(9, 10)},
		{label: "upcoming half hour", from: at(15, 0), to: at(15, 30)},
		{label: "a reminder with no end", from: at(16, 0), to: at(16, 0)},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			e := span("Standup", tt.from, tt.to)
			frame := renderToFrame(t, newWidget(testBounds, []ical.Event{e}, defaultConfig()))
			l, tl := gridOf(testBounds, defaultConfig().Window)
			x := l.Events.Min.X
			top := tl.y(tt.from)

			if frame.ColorIndexAt(x, top-1) == widget.PaperBlack {
				t.Errorf("ink above the start, y=%d", top-1)
			}
			for y := top; y < top+drawkit.BodyLineH(); y++ {
				if frame.ColorIndexAt(x, y) != widget.PaperBlack {
					t.Fatalf("block edge has paper at y=%d, %d rows under its start; want a whole line", y, y-top)
				}
			}
			if frame.ColorIndexAt(x, top+drawkit.BodyLineH()) == widget.PaperBlack {
				t.Errorf("block runs past one line, y=%d", top+drawkit.BodyLineH())
			}

			// The label reads exactly as one drawn into a line-tall
			// block: nothing of it is cut off.
			block := image.Rect(x, top, l.Events.Max.X, top+drawkit.BodyLineH())
			contrast, inner := widget.PaperWhite, block
			ref := newTestFrame()
			drawkit.FillRect(ref, block, widget.PaperBlack)
			if finished(e, testTime) {
				contrast, inner = widget.PaperBlack, block.Inset(outlineW)
				drawkit.FillWhite(ref, inner)
			}
			drawkit.DrawText(ref, inner.Min.X+labelPadX, labelBaseline(inner), tt.from.Format("15:04"), drawkit.BodyBoldFace, contrast)
			drawkit.DrawText(ref, inner.Min.X+labelPadX+6*drawkit.BodyAdvance(), labelBaseline(inner), "Standup", drawkit.BodyFace, contrast)
			if !sameIn(frame, ref, block) {
				t.Errorf("block doesn't read %q in full", tt.from.Format("15:04")+" Standup")
			}
		})
	}
}

// lane is one side of the events when two events share them.
type lane int

const (
	left lane = iota
	right
)

// Two events whose blocks would overlap share the events' width side by
// side, each at its own true start and end, so a partial overlap shows
// as two blocks of different heights. Overlap is judged on the blocks as
// drawn: a short event's line-tall block running into the next event
// puts the two side by side too, so nothing overprints. A gap down the
// middle keeps the two apart.
func TestWidget_OverlappingEventsSitSideBySide(t *testing.T) {
	type want struct {
		side     lane
		from, to time.Time
	}
	tests := []struct {
		label  string
		events []ical.Event
		want   []want
	}{
		{
			label:  "partial overlap",
			events: []ical.Event{span("Review", at(15, 0), at(17, 0)), span("Call", at(16, 0), at(18, 0))},
			want:   []want{{left, at(15, 0), at(17, 0)}, {right, at(16, 0), at(18, 0)}},
		},
		{
			label:  "full overlap",
			events: []ical.Event{span("Review", at(15, 0), at(17, 0)), span("Call", at(15, 0), at(17, 0))},
			want:   []want{{left, at(15, 0), at(17, 0)}, {right, at(15, 0), at(17, 0)}},
		},
		{
			label:  "one inside the other, finished",
			events: []ical.Event{span("Offsite", at(8, 0), at(12, 0)), span("Call", at(9, 0), at(10, 0))},
			want:   []want{{left, at(8, 0), at(12, 0)}, {right, at(9, 0), at(10, 0)}},
		},
		{
			label: "a short event running into the next",
			events: []ical.Event{
				span("Standup", at(15, 0), at(15, 30)),
				span("Design review", at(15, 30), at(17, 0)),
			},
			want: []want{{left, at(15, 0), at(15, 0)}, {right, at(15, 30), at(17, 0)}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			frame := renderToFrame(t, newWidget(testBounds, tt.events, defaultConfig()))
			l, tl := gridOf(testBounds, defaultConfig().Window)
			xs := map[lane]int{left: l.Events.Min.X, right: l.Events.Max.X - 1}

			for _, w := range tt.want {
				x, top := xs[w.side], tl.y(w.from)
				// A block ends a row above its end, and is never shorter
				// than a line.
				bottom := max(tl.y(w.to)-1, top+drawkit.BodyLineH())
				if frame.ColorIndexAt(x, top-1) == widget.PaperBlack {
					t.Errorf("%v side: ink above %s, y=%d", w.side, w.from.Format("15:04"), top-1)
				}
				for y := top; y < bottom; y++ {
					if frame.ColorIndexAt(x, y) != widget.PaperBlack {
						t.Fatalf("%v side: paper at y=%d inside %s-%s", w.side, y, w.from.Format("15:04"), w.to.Format("15:04"))
					}
				}
				if frame.ColorIndexAt(x, bottom) == widget.PaperBlack {
					t.Errorf("%v side: ink below the block, y=%d", w.side, bottom)
				}
			}

			// Down the middle of the column, only the dotted hour rules.
			mid := l.Events.Min.X + l.Events.Dx()/2
			run := 0
			for y := l.Grid.Min.Y + 1; y < l.Grid.Max.Y-1; y++ {
				if frame.ColorIndexAt(mid, y) != widget.PaperBlack || onNowMarker(frame, l, y) {
					run = 0
					continue
				}
				if run++; run > 1 {
					t.Fatalf("blocks meet in the middle of the column at y=%d", y)
				}
			}
		})
	}
}

// A third event at the same time as two others isn't drawn: there is no
// lane left for it, and a third lane would cut every block to a sliver.
// A tag at the column's right, level with the first event left out,
// counts every one left out of the group, and the two lanes narrow to
// leave it room.
func TestWidget_CountsAThirdSimultaneousEventInATag(t *testing.T) {
	tests := []struct {
		label   string
		events  []ical.Event
		first   time.Time
		wantTag string
	}{
		{
			label: "three at once",
			events: []ical.Event{
				span("Offsite", at(15, 0), at(17, 0)),
				span("Call", at(15, 0), at(16, 0)),
				span("Dentist", at(15, 30), at(16, 30)),
			},
			first:   at(15, 30),
			wantTag: "+1",
		},
		{
			label: "four at once",
			events: []ical.Event{
				span("Offsite", at(15, 0), at(17, 0)),
				span("Call", at(15, 0), at(16, 0)),
				span("Dentist", at(15, 30), at(16, 30)),
				span("Pickup", at(15, 45), at(16, 15)),
			},
			first:   at(15, 30),
			wantTag: "+2",
		},
		{
			label: "finished, three at once",
			events: []ical.Event{
				span("Offsite", at(8, 0), at(12, 0)),
				span("Call", at(9, 0), at(10, 0)),
				span("Dentist", at(9, 0), at(11, 0)),
			},
			first:   at(9, 0),
			wantTag: "+1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			frame := renderToFrame(t, newWidget(testBounds, tt.events, defaultConfig()))
			l, tl := gridOf(testBounds, defaultConfig().Window)
			top := tl.y(tt.first)

			tg := newTag(tt.wantTag, l.Events.Max.X, top)
			if tg.Rect.Max.X != l.Events.Max.X || tg.Rect.Min.Y != top {
				t.Fatalf("tag at %v, want it at the column's right level with %s", tg.Rect, tt.first.Format("15:04"))
			}
			ref := newTestFrame()
			drawTag(ref, tg)
			if !sameIn(frame, ref, tg.Rect) {
				t.Errorf("no %q tag at %v", tt.wantTag, tg.Rect)
			}
			// The tag reads as ink on paper.
			if countIndexIn(ref, tg.Rect.Inset(2), widget.PaperBlack) == 0 {
				t.Errorf("tag %q has no text", tt.wantTag)
			}
			// The lanes stop short of the tag: down its column, above
			// and below it, there is only paper and the hour rules.
			x := tg.Rect.Min.X - 1
			for y := l.Grid.Min.Y + 1; y < l.Grid.Max.Y-1; y++ {
				if onNowMarker(frame, l, y) || onNowMarker(frame, l, y+1) {
					continue
				}
				if frame.ColorIndexAt(x, y) == widget.PaperBlack && frame.ColorIndexAt(x, y+1) == widget.PaperBlack {
					t.Fatalf("a block runs into the tag's column at y=%d", y)
				}
			}
		})
	}
}

// allDay is an all-day event today, anchored at UTC midnight as the
// parser anchors a VALUE=DATE.
func allDay(summary string) ical.Event {
	return ical.Event{UID: summary, Summary: summary, AllDay: true,
		Start: time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC), End: time.Date(2026, 3, 17, 0, 0, 0, 0, time.UTC)}
}

// All-day events, and timed events running through the whole of today,
// have no time on the grid to sit at, so they're listed in a strip above
// it through the event list: up to two lines, the second becoming
// "+N MORE" when there are more. A multi-day timed event reads ALL DAY
// for today, like the all-day events beside it. The strip takes height
// only when it has something to list.
func TestWidget_ListsAllDayEventsInAStripAboveTheGrid(t *testing.T) {
	conference := span("Conference", at(-15, 0), at(41, 0))
	tests := []struct {
		label     string
		events    []ical.Event
		wantLines int
		wantList  []ical.Event
	}{
		{label: "no all-day events", events: []ical.Event{span("Lunch", at(12, 0), at(13, 0))}},
		{
			label:     "one all-day event",
			events:    []ical.Event{allDay("Car in for service"), span("Lunch", at(12, 0), at(13, 0))},
			wantLines: 1,
			wantList:  []ical.Event{allDay("Car in for service")},
		},
		{
			label:     "two all-day events",
			events:    []ical.Event{allDay("Car in for service"), allDay("Recycling day")},
			wantLines: 2,
			wantList:  []ical.Event{allDay("Car in for service"), allDay("Recycling day")},
		},
		{
			label:     "more than two",
			events:    []ical.Event{allDay("Car in for service"), allDay("Recycling day"), allDay("Grandma's birthday")},
			wantLines: 2,
			wantList:  []ical.Event{allDay("Car in for service"), allDay("Recycling day"), allDay("Grandma's birthday")},
		},
		{
			label:     "a timed event through the whole day",
			events:    []ical.Event{conference},
			wantLines: 1,
			wantList:  []ical.Event{{UID: "Conference", Summary: "Conference", AllDay: true, Start: conference.Start, End: conference.End}},
		},
		{
			label:  "a timed event from yesterday ending today",
			events: []ical.Event{span("Night shift", at(-2, 0), at(8, 0))},
		},
		{
			label:  "a timed event from today ending tomorrow",
			events: []ical.Event{span("Night shift", at(20, 0), at(30, 0))},
		},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			frame := renderToFrame(t, newWidget(testBounds, tt.events, defaultConfig()))
			l := computeLayout(testBounds, sections{AllDay: tt.wantLines})

			if tt.wantLines == 0 {
				if !l.AllDay.Empty() {
					t.Fatalf("strip %v with nothing to list", l.AllDay)
				}
				if l.Grid.Min.Y != testBounds.Min.Y {
					t.Errorf("grid starts at %d with no strip, want the widget's top, %d", l.Grid.Min.Y, testBounds.Min.Y)
				}
				return
			}
			if l.AllDay.Min.Y != testBounds.Min.Y || l.Grid.Min.Y <= l.AllDay.Max.Y-1 {
				t.Fatalf("strip %v isn't at the top, above the grid at %d", l.AllDay, l.Grid.Min.Y)
			}
			if got := l.AllDay.Dy(); got < tt.wantLines*drawkit.BodyLineH() || got >= (tt.wantLines+1)*drawkit.BodyLineH() {
				t.Errorf("strip is %d px tall, want room for %d lines", got, tt.wantLines)
			}

			// The strip reads as the event list draws these events.
			ref := newTestFrame()
			eventlist.Style{Layout: eventlist.Inline, Location: time.UTC}.Draw(ref, stripText(l), tt.wantList)
			if !sameIn(frame, ref, l.AllDay) {
				t.Error("strip differs from the event list of its events")
			}
		})
	}
}

// A short event at the very end of the window has no line's height left
// below its start, so its block is lifted to end where the grid does
// rather than run over the bottom rule.
func TestWidget_ShortBlockAtTheWindowEndStaysOnTheGrid(t *testing.T) {
	tests := []struct {
		label    string
		from, to time.Time
	}{
		{label: "last ten minutes", from: at(21, 50), to: at(22, 0)},
		{label: "a reminder in the last minute", from: at(21, 59), to: at(21, 59)},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			e := span("Lock up", tt.from, tt.to)
			frame := renderToFrame(t, newWidget(testBounds, []ical.Event{e}, defaultConfig()))
			l, _ := gridOf(testBounds, defaultConfig().Window)
			x := l.Events.Min.X
			// The bottom rule is the grid's last row; one row of paper
			// sits above it, and the block a line tall above that.
			last := l.Grid.Max.Y - 1
			if frame.ColorIndexAt(x, last-1) == widget.PaperBlack {
				t.Errorf("block runs onto the row above the bottom rule, y=%d", last-1)
			}
			for y := last - 1 - drawkit.BodyLineH(); y < last-1; y++ {
				if frame.ColorIndexAt(x, y) != widget.PaperBlack {
					t.Fatalf("paper at y=%d; want a whole line ending above the bottom rule", y)
				}
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
	w := New(testBounds, daydata.InMemory([]ical.Event{e}, nil), fixedClock(now), defaultConfig())
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
	drawkit.FillRect(ref, block, widget.PaperBlack)
	drawLabel(ref, block, e, toronto, false, widget.PaperWhite)
	if !sameIn(frame, ref, block) {
		t.Error("label differs from one written at 15:00")
	}
	wrong := newTestFrame()
	drawkit.FillRect(wrong, block, widget.PaperBlack)
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
// the events, and the window's last edge gets a rule too. The
// window's opening edge is the widget's top, ruled only when a band sits
// above it (see TestWidget_TopEdge).
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

			for h := 1; h <= tt.win.EndHour-tt.win.StartHour; h++ {
				y := tl.y(start.Add(time.Duration(h) * time.Hour))
				if countIndexIn(frame, image.Rect(l.Events.Min.X, y, l.Events.Max.X, y+1), widget.PaperBlack) == 0 {
					t.Errorf("no rule across the events at hour %d (y=%d)", tt.win.StartHour+h, y)
				}
			}
			// Labels: the gutter is inked in every labelled hour's row,
			// and every other hour is labelled when the rows are too
			// short for a label each.
			step := 1
			if tl.y(start.Add(time.Hour))-tl.y(start) < drawkit.BodyLineH() {
				step = 2
			}
			for h := 0; h < tt.win.EndHour-tt.win.StartHour; h += step {
				y0 := tl.y(start.Add(time.Duration(h) * time.Hour))
				row := image.Rect(l.Gutter.Min.X, y0+1, l.Gutter.Max.X-1, y0+drawkit.BodyLineH())
				if countIndexIn(frame, row, widget.PaperBlack) == 0 {
					t.Errorf("no label for hour %d", tt.win.StartHour+h)
				}
			}
		})
	}
}

// The window's opening edge gets a solid rule only when the widget has
// its own band above the grid to close off. With nothing above, the grid
// starts at the widget's top edge and draws no rule there: whatever sits
// above the widget, a screen's separator or the panel's edge, already
// closes it, and a second rule just under a separator reads as a double
// line. The rule between the labels and the events then runs up to the
// widget's edge, so it meets that separator.
func TestWidget_TopEdge(t *testing.T) {
	ruleX := testBounds.Min.X + gutterW + laneW
	events := image.Rect(ruleX+ruleW+eventsPadX, 0, testBounds.Max.X-eventsPadX, 0)
	tests := []struct {
		label  string
		events []ical.Event
		// ruleY is the row the solid top edge rule is on, or -1 for none.
		ruleY int
	}{
		{label: "nothing above the grid", ruleY: -1},
		{
			label:  "an earlier note above",
			events: []ical.Event{span("Gym", at(5, 0), at(6, 0))},
			ruleY:  testBounds.Min.Y + noteH(),
		},
		{
			label:  "an all-day strip above",
			events: []ical.Event{allDay("Car in for service")},
			ruleY:  testBounds.Min.Y + stripH(1) + gridPadY,
		},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			// Late in the evening, so no now marker crosses the top.
			w := New(testBounds, daydata.InMemory(tt.events, nil), fixedClock(at(23, 30)), defaultConfig())
			frame := renderToFrame(t, w)

			if tt.ruleY < 0 {
				top := image.Rect(events.Min.X, testBounds.Min.Y, events.Max.X, testBounds.Min.Y+2)
				if n := countIndexIn(frame, top, widget.PaperBlack); n != 0 {
					t.Errorf("%d px inked across the events' top rows, want no rule", n)
				}
				if frame.ColorIndexAt(ruleX, testBounds.Min.Y) != widget.PaperBlack {
					t.Errorf("rule between labels and events doesn't reach the widget's top edge")
				}
				return
			}
			row := image.Rect(testBounds.Min.X, tt.ruleY, events.Max.X, tt.ruleY+1)
			if n := countIndexIn(frame, row, widget.PaperBlack); n != row.Dx() {
				t.Errorf("top edge rule at y=%d is %d/%d px", tt.ruleY, n, row.Dx())
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
		label    string
		events   []ical.Event
		forecast []weather.DailyForecast
		cfg      func(*Config)
		now      time.Time
	}{
		{label: "typical day", events: typicalDay()},
		{label: "rainy day", events: typicalDay(), forecast: rainyToday()},
		{label: "dry day", events: typicalDay(), forecast: dryToday()},
		{
			label:    "rainy day in a working hours window",
			events:   typicalDay(),
			forecast: rainyToday(),
			cfg:      func(c *Config) { c.Window = Window{StartHour: 9, EndHour: 17} },
		},
		{label: "now at the window start", events: typicalDay(), forecast: rainyToday(), now: at(7, 0)},
		{label: "now in the middle of a block", events: typicalDay(), forecast: rainyToday(), now: at(15, 45)},
		{label: "now just before the window end", events: typicalDay(), forecast: rainyToday(), now: at(21, 45)},
		{
			label: "now through side-by-side blocks and a tag",
			events: []ical.Event{
				span("Offsite", at(13, 0), at(15, 0)),
				span("Vendor call", at(13, 30), at(15, 0)),
				span("Dentist", at(13, 45), at(14, 30)),
			},
			forecast: rainyToday(),
			now:      at(14, 10),
		},
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
		{
			label: "partial overlap",
			events: []ical.Event{
				span("Design review", at(10, 0), at(12, 0)),
				span("Vendor call", at(11, 0), at(12, 30)),
				span("Planning", at(15, 0), at(16, 30)),
				span("Interview", at(16, 0), at(17, 0)),
			},
		},
		{
			label: "full overlap",
			events: []ical.Event{
				span("Board meeting", at(14, 0), at(16, 0)),
				span("School pickup", at(14, 0), at(16, 0)),
			},
		},
		{
			label: "three simultaneous events",
			events: []ical.Event{
				span("Offsite", at(9, 0), at(12, 0)),
				span("Standup", at(9, 0), at(9, 15)),
				span("Expense report", at(9, 0), at(10, 0)),
				span("Workshop", at(14, 0), at(17, 0)),
				span("Dentist", at(14, 30), at(15, 30)),
				span("Pickup", at(15, 0), at(16, 0)),
				span("Call Mom", at(15, 15), at(15, 45)),
			},
		},
		{
			label:  "one all-day event",
			events: append([]ical.Event{allDay("Car in for service")}, typicalDay()...),
		},
		{
			label: "more than two all-day events",
			events: append([]ical.Event{
				allDay("Car in for service"),
				allDay("Recycling day"),
				allDay("Grandma's birthday"),
				span("Conference", at(-15, 0), at(41, 0)),
				span("Gym", at(5, 30), at(6, 30)),
			}, typicalDay()...),
		},
		{
			label: "a multi-day event through today",
			events: []ical.Event{
				span("Cottage weekend", at(-30, 0), at(40, 0)),
				span("Lunch", at(12, 0), at(13, 0)),
			},
		},
		{
			label: "neither all-day nor multi-day",
			events: []ical.Event{
				span("Night shift", at(-2, 0), at(8, 0)),
				span("Lunch", at(12, 0), at(13, 0)),
				span("Red-eye flight", at(21, 0), at(30, 0)),
			},
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
			w := New(testBounds, daydata.InMemory(tt.events, tt.forecast), fixedClock(now), cfg)
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
		allDay("Car in for service"),
		allDay("Recycling day"),
		allDay("Grandma's birthday"),
		span("Clash", at(13, 0), at(13, 30)),
		span("Another clash", at(13, 15), at(14, 0)),
		span("Lock up", at(21, 55), at(22, 0)),
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
// neighbour. Blank bounds are a misconfiguration you can see.
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
			drawkit.FillRect(frame, tt.bounds, widget.PaperBlack)
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
			// Without a note the grid takes the height: it starts at
			// the bounds' top, unruled, and ends at the bottom rule's
			// padding.
			if tt.wantEarlier == "" && l.Grid.Min.Y != testBounds.Min.Y {
				t.Errorf("grid starts at %d with nothing earlier", l.Grid.Min.Y)
			}
			if tt.wantLater == "" && l.Grid.Max.Y != testBounds.Max.Y-gridPadY {
				t.Errorf("grid ends at %d with nothing later", l.Grid.Max.Y)
			}
		})
	}
}

// Nothing outside the window reaches the grid: down the events' edges
// there is no ink but the one-pixel hour rules.
func TestWidget_DrawsNoBlockForEventsOutsideTheWindow(t *testing.T) {
	events := []ical.Event{span("Gym", at(5, 0), at(7, 0)), span("Late call", at(22, 0), at(23, 0))}
	// Late in the evening, so no now marker crosses the column either.
	w := New(testBounds, daydata.InMemory(events, nil), fixedClock(at(23, 30)), defaultConfig())
	frame := renderToFrame(t, w)
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
