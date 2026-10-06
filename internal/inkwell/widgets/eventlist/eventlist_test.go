package eventlist_test

import (
	"image"
	"testing"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daygrid"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/eventlist"
	"golang.org/x/image/font"
)

var toronto = mustZone("America/Toronto")

func mustZone(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return loc
}

func newFrame() *image.Paletted {
	frame := image.NewPaletted(image.Rect(0, 0, 400, 480), widget.PaperPalette)
	daygrid.FillWhite(frame, frame.Bounds())
	return frame
}

func timed(summary string, hour int) calendar.Event {
	start := time.Date(2026, 3, 16, hour, 0, 0, 0, time.UTC)
	return calendar.Event{Summary: summary, Start: start, End: start.Add(time.Hour)}
}

// line is one row of text a test expects drawn: its text, its face, the
// scale it is drawn at, how far right of the list's edge it starts and
// its baseline.
type line struct {
	text     string
	face     font.Face
	scale    int
	x        int
	baseline int
}

var (
	bold    = daygrid.BodyBoldFace
	regular = daygrid.BodyFace
	ascent  = daygrid.BodyAscent()
	lineH   = daygrid.BodyLineH()
	descent = lineH - ascent
)

// drawLines paints want into a fresh frame, starting each at x.
func drawLines(x int, want []line) *image.Paletted {
	ref := newFrame()
	for _, l := range want {
		daygrid.Scaled(l.face, max(l.scale, 1), widget.PaperBlack).Draw(ref, x+l.x, l.baseline, l.text)
	}
	return ref
}

// assertLines checks the frame carries exactly want and nothing else:
// the list's lines are the whole of what Draw may ink.
func assertLines(t *testing.T, frame *image.Paletted, x int, want []line) {
	t.Helper()
	assertFrame(t, frame, drawLines(x, want), want)
}

// assertFrame checks frame against ref pixel for pixel. want names the
// lines in the failure.
func assertFrame(t *testing.T, frame, ref *image.Paletted, want []line) {
	t.Helper()
	for y := range frame.Bounds().Dy() {
		for xx := range frame.Bounds().Dx() {
			if frame.ColorIndexAt(xx, y) != ref.ColorIndexAt(xx, y) {
				t.Fatalf("frame differs from the expected lines at (%d,%d); want %v", xx, y, texts(want))
			}
		}
	}
}

func texts(ls []line) []string {
	out := make([]string, len(ls))
	for i, l := range ls {
		out[i] = l.text
	}
	return out
}

// A stacked list puts each event's time above its title: the time in the
// bold cut in the display zone, the title in the regular cut a line
// below it.
func TestDraw_StackedEvent(t *testing.T) {
	frame := newFrame()
	r := image.Rect(10, 20, 210, 400)
	style := eventlist.Style{Location: toronto}

	hidden := style.Draw(frame, r, []calendar.Event{timed("Standup", 14)})

	if hidden != 0 {
		t.Errorf("hidden = %d, want 0", hidden)
	}
	assertLines(t, frame, r.Min.X, []line{
		{text: "10:00", face: bold, baseline: r.Min.Y + ascent},
		{text: "Standup", face: regular, baseline: r.Min.Y + ascent + lineH},
	})
}

// How one event becomes text. The list is 12 characters wide, so the
// cases can wrap and cut at known places. Every budget is counted in
// characters, never bytes, so a multi-byte title wraps and cuts on the
// same columns an ASCII one does.
func TestDraw_EventText(t *testing.T) {
	r := image.Rect(0, 0, 12*daygrid.BodyAdvance(), 400)
	allDay := calendar.Event{Summary: "Holiday", AllDay: true, Start: time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC)}
	withPlace := timed("Lunch", 16)
	withPlace.Location = "Cafe"
	long := timed("Platform architecture review", 9)

	tests := []struct {
		label  string
		event  calendar.Event
		style  eventlist.Style
		titles []string
	}{
		{"an all-day event says so", allDay, eventlist.Style{}, []string{"Holiday"}},
		{"the location follows an @ when shown", withPlace, eventlist.Style{ShowLocation: true}, []string{"Lunch @ Cafe"}},
		{"the location is left off when not", withPlace, eventlist.Style{}, []string{"Lunch"}},
		{"a long title wraps on words", long, eventlist.Style{TitleLines: 3}, []string{"Platform", "architecture", "review"}},
		{"a title past its lines is cut with »", long, eventlist.Style{TitleLines: 2}, []string{"Platform", "architectur»"}},
		{"a title gets one line by default", long, eventlist.Style{}, []string{"Platform»"}},
		{"a word longer than the line is broken", timed("Supercalifragilistic", 9), eventlist.Style{TitleLines: 2},
			[]string{"Supercalifra", "gilistic"}},
		{"a multi-byte title wraps on characters", timed("Réunion équipe café", 9), eventlist.Style{TitleLines: 2},
			[]string{"Réunion", "équipe café"}},
		{"a multi-byte title is cut on characters", timed("Ñandúñandúñandú", 9), eventlist.Style{},
			[]string{"Ñandúñandúñ»"}},
		{"a blank title draws only the time", timed("   ", 9), eventlist.Style{}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			style := tt.style
			style.Location = time.UTC
			frame := newFrame()
			if hidden := style.Draw(frame, r, []calendar.Event{tt.event}); hidden != 0 {
				t.Errorf("hidden = %d, want 0", hidden)
			}

			timeText := tt.event.Start.Format("15:04")
			if tt.event.AllDay {
				timeText = "ALL DAY"
			}
			want := []line{{text: timeText, face: bold, baseline: ascent}}
			for i, title := range tt.titles {
				want = append(want, line{text: title, face: regular, baseline: ascent + (i+1)*lineH})
			}
			assertLines(t, frame, 0, want)
		})
	}
}

// events builds n one-line events an hour apart from 09:00.
func events(n int) []calendar.Event {
	out := make([]calendar.Event, n)
	for i := range out {
		out[i] = timed(string(rune('A'+i)), 9+i)
	}
	return out
}

// stacked is what a list of events from events() draws when the first
// drawn of them fit and more is the "+N MORE" line under them ("" for
// none).
func stacked(drawn int, gap int, more string) []line {
	pitch := 2*lineH + gap
	var out []line
	for i := range drawn {
		top := i * pitch
		out = append(out,
			line{text: timed("", 9+i).Start.Format("15:04"), face: bold, baseline: top + ascent},
			line{text: string(rune('A' + i)), face: regular, baseline: top + ascent + lineH},
		)
	}
	if more != "" {
		out = append(out, line{text: more, face: bold, baseline: drawn*pitch + ascent})
	}
	return out
}

// inline is what an inline list of events from events() draws when the
// first drawn of them fit and more is the "+N MORE" line under them (""
// for none): one line an event, the time in the regular cut and the
// title past the time column.
func inline(drawn int, more string) []line {
	var out []line
	for i := range drawn {
		baseline := i*lineH + ascent
		out = append(out,
			line{text: timed("", 9+i).Start.Format("15:04"), face: regular, baseline: baseline},
			line{text: string(rune('A' + i)), face: regular, x: timeColumn, baseline: baseline},
		)
	}
	if more != "" {
		out = append(out, line{text: more, face: bold, baseline: drawn*lineH + ascent})
	}
	return out
}

// One fit rule and one overflow rule, whichever the layout. A line fits
// when its whole height, ascent and descent, is inside the rectangle;
// an event is drawn whole or not at all. When not every event is drawn,
// the last visible line is "+N MORE", counting every event not shown:
// an event that would leave no room for it gives its place up and is
// counted. An inline list of n body lines therefore holds n of them,
// which is how a caller hands the list a line budget: a rectangle
// exactly that many lines tall.
func TestDraw_FitAndOverflow(t *testing.T) {
	const gap, width = 8, 300
	pitch := 2*lineH + gap
	threeExactly := 2*pitch + 2*lineH
	st := func(maxEvents int) eventlist.Style { return eventlist.Style{Gap: gap, MaxEvents: maxEvents} }
	in := func(maxEvents int) eventlist.Style {
		return eventlist.Style{Layout: eventlist.Inline, MaxEvents: maxEvents}
	}

	tests := []struct {
		label      string
		style      eventlist.Style
		width      int
		height     int
		events     int
		wantHidden int
		want       []line
	}{
		{"stacked: every event fits to the last pixel", st(0), width, threeExactly, 3, 0, stacked(3, gap, "")},
		{"stacked: a pixel short, the last event gives way to the line", st(0), width, threeExactly - 1, 3, 1, stacked(2, gap, "+1 MORE")},
		{"stacked: the cap hides events and the line counts them", st(3), width, 400, 5, 2, stacked(3, gap, "+2 MORE")},
		{"stacked: a cap of zero is no cap", st(0), width, 400, 5, 0, stacked(5, gap, "")},
		{"stacked: a negative cap is no cap", st(-1), width, 400, 5, 0, stacked(5, gap, "")},
		{"stacked: the cap and the room together", st(4), width, threeExactly - 1, 5, 3, stacked(2, gap, "+3 MORE")},
		{"stacked: the line lands exactly on the bottom edge", st(0), width, 2*pitch + lineH, 3, 1, stacked(2, gap, "+1 MORE")},
		{"stacked: a pixel short of that, one more event gives way", st(0), width, 2*pitch + lineH - 1, 3, 2, stacked(1, gap, "+2 MORE")},
		{"stacked: room for the line alone", st(0), width, lineH, 2, 2, stacked(0, gap, "+2 MORE")},
		{"stacked: no room for any line", st(0), width, lineH - 1, 2, 2, nil},
		{"inline: every event fits to the last pixel", in(0), width, 3 * lineH, 3, 0, inline(3, "")},
		{"inline: a line short, the last event gives way to the line", in(0), width, 3 * lineH, 4, 2, inline(2, "+2 MORE")},
		{"inline: a pixel short, the last line does not fit", in(0), width, 3*lineH - 1, 3, 2, inline(1, "+2 MORE")},
		{"inline: the cap hides events and the line counts them", in(3), width, 400, 5, 2, inline(3, "+2 MORE")},
		{"inline: room for the line alone", in(0), width, lineH, 4, 4, inline(0, "+4 MORE")},
		{"inline: narrower than the time column, the line still counts them", in(0), timeColumn - 1, 400, 2, 2, inline(0, "+2 MORE")},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			style := tt.style
			style.Location = time.UTC
			frame := newFrame()
			hidden := style.Draw(frame, image.Rect(0, 0, tt.width, tt.height), events(tt.events))
			if hidden != tt.wantHidden {
				t.Errorf("hidden = %d, want %d", hidden, tt.wantHidden)
			}
			assertLines(t, frame, 0, tt.want)
		})
	}
}

// today-hero's agenda: the time at twice body size with the title tucked
// a body ascent under it, a wider gap, and a hairline across the middle
// of each gap between events. The "+N MORE" line takes the next event's
// place under the same gap, without a hairline: it is not an event.
func TestDraw_ScaledTimeWithRules(t *testing.T) {
	const gap = 20
	r := image.Rect(12, 0, 212, 400)
	style := eventlist.Style{
		TimeScale: 2, TitleLead: ascent, Gap: gap, Rules: true,
		MaxEvents: 2, Location: time.UTC,
	}
	frame := newFrame()

	if hidden := style.Draw(frame, r, events(3)); hidden != 1 {
		t.Errorf("hidden = %d, want 1", hidden)
	}

	timeAscent := 2 * ascent
	height := timeAscent + ascent + descent
	pitch := height + gap
	want := []line{
		{text: "09:00", face: bold, scale: 2, baseline: timeAscent},
		{text: "A", face: regular, baseline: timeAscent + ascent},
		{text: "10:00", face: bold, scale: 2, baseline: pitch + timeAscent},
		{text: "B", face: regular, baseline: pitch + timeAscent + ascent},
		{text: "+1 MORE", face: bold, baseline: 2*pitch + ascent},
	}
	ref := drawLines(r.Min.X, want)
	daygrid.DrawHLine(ref, r.Min.X, r.Max.X, height+gap/2, widget.PaperBlack)
	assertFrame(t, frame, ref, want)
}

// The edges of the width: every line is cut to the list, and a list too
// narrow to carry a title lists no events rather than a column of »,
// which reads as a fault rather than as content. The "+N MORE" line
// still says they are there while it has a character of room: hidden
// events are never unannounced.
func TestDraw_Width(t *testing.T) {
	adv := daygrid.BodyAdvance()
	tests := []struct {
		label      string
		chars      int
		style      eventlist.Style
		events     []calendar.Event
		wantHidden int
		want       []line
	}{
		{"no events draws nothing", 20, eventlist.Style{}, nil, 0, nil},
		{"too narrow for a title, the line still counts it", 2, eventlist.Style{}, events(1), 1, []line{
			{text: "+»", face: bold, baseline: ascent},
		}},
		{"no room for a character draws nothing", 0, eventlist.Style{}, events(1), 1, nil},
		{"the line is cut to a narrow list", 5, eventlist.Style{MaxEvents: 1}, events(13), 12, []line{
			{text: "09:00", face: bold, baseline: ascent},
			{text: "A", face: regular, baseline: ascent + lineH},
			{text: "+12 »", face: bold, baseline: 2*lineH + ascent},
		}},
		// A clock time cut short reads as a different time ("0" for
		// 09:00), so a time without room to be drawn whole is left off and
		// its title keeps its place.
		{"a large time with no room is left off", 3, eventlist.Style{TimeScale: 2}, events(1), 0, []line{
			{text: "A", face: regular, baseline: 2*ascent + lineH},
		}},
		{"a body-size time with no room is left off", 4, eventlist.Style{}, events(1), 0, []line{
			{text: "A", face: regular, baseline: ascent + lineH},
		}},
		{"an all-day label with no room is left off", 6, eventlist.Style{}, []calendar.Event{
			{Summary: "Off", AllDay: true, Start: time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC)},
		}, 0, []line{
			{text: "Off", face: regular, baseline: ascent + lineH},
		}},
		// Dilation spills a pixel past a 2x glyph's cell, so a 2x time
		// needs that pixel beside its advance as well.
		{"a large time needs room for its dilation", 10, eventlist.Style{TimeScale: 2}, events(1), 0, []line{
			{text: "A", face: regular, baseline: 2*ascent + lineH},
		}},
		{"a large time with room for its dilation is drawn", 11, eventlist.Style{TimeScale: 2}, events(1), 0, []line{
			{text: "09:00", face: bold, scale: 2, baseline: 2 * ascent},
			{text: "A", face: regular, baseline: 2*ascent + lineH},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			style := tt.style
			style.Location = time.UTC
			frame := newFrame()
			if hidden := style.Draw(frame, image.Rect(0, 0, tt.chars*adv, 400), tt.events); hidden != tt.wantHidden {
				t.Errorf("hidden = %d, want %d", hidden, tt.wantHidden)
			}
			assertLines(t, frame, 0, tt.want)
		})
	}
}

// Lines is how many lines a list needs to draw in full at a width: each
// listed event's lines up to the cap, and the "+N MORE" line when the
// cap hides any. A stacked event takes its time and title lines; an
// inline one takes one line whatever its title. It is the count Draw
// lays out, so a caller can size a list before drawing it.
func TestLines(t *testing.T) {
	adv := daygrid.BodyAdvance()
	inlineWidth := timeColumn + 20*adv
	long := timed("Platform architecture review", 9)
	longer := timed("Platform architecture review with the whole infra team", 9)
	tests := []struct {
		label  string
		layout eventlist.Layout
		width  int
		style  eventlist.Style
		events []calendar.Event
		want   int
	}{
		{"stacked: no events need no lines", eventlist.Stacked, 20 * adv, eventlist.Style{}, nil, 0},
		{"stacked: a time and a title each", eventlist.Stacked, 20 * adv, eventlist.Style{}, events(3), 6},
		{"stacked: a wrapped title takes its lines", eventlist.Stacked, 12 * adv, eventlist.Style{TitleLines: 2}, []calendar.Event{long, long}, 6},
		{"stacked: up to its title lines", eventlist.Stacked, 12 * adv, eventlist.Style{TitleLines: 3}, []calendar.Event{long}, 4},
		{"stacked: a blank title takes none", eventlist.Stacked, 20 * adv, eventlist.Style{}, []calendar.Event{timed(" ", 9)}, 1},
		{"stacked: the cap adds the +N MORE line", eventlist.Stacked, 20 * adv, eventlist.Style{MaxEvents: 2}, events(5), 5},
		{"stacked: a cap the list is under adds nothing", eventlist.Stacked, 20 * adv, eventlist.Style{MaxEvents: 5}, events(2), 4},
		{"stacked: too narrow to list events needs the +N MORE line", eventlist.Stacked, 2 * adv, eventlist.Style{}, events(2), 1},
		{"stacked: too narrow with no events needs none", eventlist.Stacked, 2 * adv, eventlist.Style{}, nil, 0},
		{"stacked: no room for a character needs none", eventlist.Stacked, adv - 1, eventlist.Style{}, events(2), 0},
		{"inline: one line an event", eventlist.Inline, inlineWidth, eventlist.Style{}, events(3), 3},
		{"inline: a long title still takes one", eventlist.Inline, inlineWidth, eventlist.Style{TitleLines: 3}, []calendar.Event{longer, longer}, 2},
		{"inline: the cap adds the +N MORE line", eventlist.Inline, inlineWidth, eventlist.Style{MaxEvents: 3}, events(5), 4},
		{"inline: narrower than the time column needs the +N MORE line", eventlist.Inline, timeColumn - 1, eventlist.Style{}, events(2), 1},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			style := tt.style
			style.Layout, style.Location = tt.layout, time.UTC
			if got := style.Lines(tt.events, tt.width); got != tt.want {
				t.Errorf("Lines = %d, want %d", got, tt.want)
			}
		})
	}
}

// A dilated time never inks outside the list, at any label and any
// scale. Below and to the right that is the measured grow; above and to
// the left it is the empty rows and column Tamzen's label glyphs carry.
func TestDraw_LargeTimeStaysInsideTheList(t *testing.T) {
	labels := []calendar.Event{{AllDay: true, Start: time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC)}}
	for h := range 24 {
		labels = append(labels, calendar.Event{Start: time.Date(2026, 3, 16, h, h+35, 0, 0, time.UTC)})
	}
	for _, scale := range []int{2, 3, 4} {
		for _, e := range labels {
			style := eventlist.Style{TimeScale: scale, Location: time.UTC}
			// As tight as the list allows: exactly the time's height, and
			// just wide enough for "ALL DAY" and its dilation.
			grow := growAt(scale)
			r := image.Rect(30, 30, 30+7*scale*daygrid.BodyAdvance()+grow, 30+scale*lineH+grow)
			frame := newFrame()
			if hidden := style.Draw(frame, r, []calendar.Event{e}); hidden != 0 {
				t.Fatalf("scale %d %v: hidden = %d, want the time drawn", scale, e.Start, hidden)
			}
			inside := 0
			for y := range frame.Bounds().Dy() {
				for x := range frame.Bounds().Dx() {
					if frame.ColorIndexAt(x, y) != widget.PaperBlack {
						continue
					}
					if !image.Pt(x, y).In(r) {
						t.Fatalf("scale %d %v: ink at (%d,%d), outside %v", scale, e.Start, x, y, r)
					}
					inside++
				}
			}
			if inside == 0 {
				t.Fatalf("scale %d %v: the time was left off", scale, e.Start)
			}
		}
	}
}

func growAt(scale int) int { return daygrid.Scaled(bold, scale, widget.PaperBlack).Grow }

// An event is as tall as whichever of its lines reaches lowest: the
// title's descent, or the time's when there is no title or the title
// is tucked close under a large time. A large time's descent is scaled
// and carries the pixel its dilation can spill past the glyph's cell.
func TestDraw_EventHeight(t *testing.T) {
	more := []line{{text: "+1 MORE", face: bold, baseline: ascent}}
	large := 2*lineH + 1 // a 2x time's ascent and descent, plus a pixel of dilation
	tests := []struct {
		label      string
		title      string
		scale      int
		lead       int
		height     int
		wantHidden int
		want       []line
	}{
		{"an untitled large time fits on its own height", "", 2, 0, large, 0,
			[]line{{text: "09:00", face: bold, scale: 2, baseline: 2 * ascent}}},
		{"a pixel short, it gives way to the line", "", 2, 0, large - 1, 1, more},
		{"an untitled body-size time is one line", "", 1, 0, lineH, 0,
			[]line{{text: "09:00", face: bold, baseline: ascent}}},
		{"a large time below a tucked title sets the height", "A", 2, 1, large, 0, []line{
			{text: "09:00", face: bold, scale: 2, baseline: 2 * ascent},
			{text: "A", face: regular, baseline: 2*ascent + 1},
		}},
		{"a pixel short of the time's descent, it gives way", "A", 2, 1, large - 1, 1, more},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			style := eventlist.Style{TimeScale: tt.scale, TitleLead: tt.lead, Location: time.UTC}
			frame := newFrame()
			events := []calendar.Event{timed(tt.title, 9)}
			if hidden := style.Draw(frame, image.Rect(0, 0, 200, tt.height), events); hidden != tt.wantHidden {
				t.Errorf("hidden = %d, want %d", hidden, tt.wantHidden)
			}
			assertLines(t, frame, 0, tt.want)
		})
	}
}

// timeColumn is where an inline title starts: past the widest clock
// label, "ALL DAY", and a space.
var timeColumn = daygrid.TextWidth(regular, "ALL DAY ")

// An inline list puts each event's time and title on one line: the
// time in the regular cut, the title after the time column so an
// all-day label never runs into it.
func TestDraw_InlineEvent(t *testing.T) {
	frame := newFrame()
	r := image.Rect(10, 20, 310, 400)
	style := eventlist.Style{Layout: eventlist.Inline, Location: toronto}

	hidden := style.Draw(frame, r, []calendar.Event{timed("Standup", 14)})

	if hidden != 0 {
		t.Errorf("hidden = %d, want 0", hidden)
	}
	ref := newFrame()
	daygrid.DrawText(ref, r.Min.X, r.Min.Y+ascent, "10:00", regular, widget.PaperBlack)
	daygrid.DrawText(ref, r.Min.X+timeColumn, r.Min.Y+ascent, "Standup", regular, widget.PaperBlack)
	assertFrame(t, frame, ref, nil)
}

// How one event becomes an inline line. The title has 12 characters of
// room past the time column. It keeps to one line whatever TitleLines
// says and is cut on characters, not wrapped on words: on one line a
// word wrap would throw away the rest of a line that had room for it.
func TestDraw_InlineEventText(t *testing.T) {
	adv := daygrid.BodyAdvance()
	allDay := calendar.Event{Summary: "Holiday", AllDay: true, Start: time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC)}
	withPlace := timed("Lunch", 16)
	withPlace.Location = "Cafe"
	long := timed("Platform architecture review", 9)

	tests := []struct {
		label      string
		event      calendar.Event
		style      eventlist.Style
		titleChars int
		time       string
		title      string
	}{
		{"an all-day event says so", allDay, eventlist.Style{}, 12, "ALL DAY", "Holiday"},
		{"the location follows an @ when shown", withPlace, eventlist.Style{ShowLocation: true}, 12, "16:00", "Lunch @ Cafe"},
		{"a long title is cut on characters with »", long, eventlist.Style{}, 12, "09:00", "Platform ar»"},
		{"title lines do not wrap an inline title", long, eventlist.Style{TitleLines: 3}, 12, "09:00", "Platform ar»"},
		{"a multi-byte title is cut on characters", timed("Ñandúñandúñandú", 9), eventlist.Style{}, 12, "09:00", "Ñandúñandúñ»"},
		{"space around a title is dropped", timed("  Standup  ", 9), eventlist.Style{}, 12, "09:00", "Standup"},
		{"the time is drawn alone with no room for a title", long, eventlist.Style{}, eventlist.MinChars - 1, "09:00", ""},
		{"a title gets the narrowest room it can use", long, eventlist.Style{}, eventlist.MinChars, "09:00", "Pl»"},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			style := tt.style
			style.Layout, style.Location = eventlist.Inline, time.UTC
			frame := newFrame()
			r := image.Rect(0, 0, timeColumn+tt.titleChars*adv, 400)
			if hidden := style.Draw(frame, r, []calendar.Event{tt.event}); hidden != 0 {
				t.Errorf("hidden = %d, want 0", hidden)
			}
			ref := newFrame()
			daygrid.DrawText(ref, 0, ascent, tt.time, regular, widget.PaperBlack)
			daygrid.DrawText(ref, timeColumn, ascent, tt.title, regular, widget.PaperBlack)
			assertFrame(t, frame, ref, []line{{text: tt.time}, {text: tt.title}})
		})
	}
}

// A list with no events says what Empty says, on its first line in the
// regular cut, cut to the width like every other line. It is held to
// the same fit rule, and it is a line a caller sizing the list needs.
func TestDraw_Empty(t *testing.T) {
	adv := daygrid.BodyAdvance()
	const msg = "NOTHING SCHEDULED"
	tests := []struct {
		label  string
		style  eventlist.Style
		width  int
		height int
		want   string
		lines  int
	}{
		{"an inline list says it", eventlist.Style{Layout: eventlist.Inline, Empty: msg}, 30 * adv, 400, msg, 1},
		{"a stacked list says it", eventlist.Style{Empty: msg}, 30 * adv, 400, msg, 1},
		{"it is cut to the width", eventlist.Style{Layout: eventlist.Inline, Empty: msg}, 10 * adv, 400, "NOTHING S»", 1},
		{"it needs a whole line", eventlist.Style{Layout: eventlist.Inline, Empty: msg}, 30 * adv, lineH - 1, "", 1},
		{"too narrow for the list, too narrow for it", eventlist.Style{Layout: eventlist.Inline, Empty: msg}, timeColumn - 1, 400, "", 0},
		{"without one an empty list draws nothing", eventlist.Style{Layout: eventlist.Inline}, 30 * adv, 400, "", 0},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			style := tt.style
			style.Location = time.UTC
			frame := newFrame()
			if hidden := style.Draw(frame, image.Rect(0, 0, tt.width, tt.height), nil); hidden != 0 {
				t.Errorf("hidden = %d, want 0", hidden)
			}
			assertLines(t, frame, 0, []line{{text: tt.want, face: regular, baseline: ascent}})
			if got := style.Lines(nil, tt.width); got != tt.lines {
				t.Errorf("Lines = %d, want %d", got, tt.lines)
			}
		})
	}
}
