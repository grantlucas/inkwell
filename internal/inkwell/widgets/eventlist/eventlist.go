// Package eventlist turns a day's events into drawn lines: how an event
// is written, one rule for whether a line fits, and one "+N MORE" line
// for the events that do not. Widgets that list events through it write
// them the same way and say how many they hid the same way.
//
// The shapes the day screens list in are its styles: bold-five's column
// (stacked), today-hero's agenda (large) and the rows of row-agenda and
// today-hero (inline). A Style filled in with a widget's choices is a
// List, which draws. The event-list widget places one day's list on a
// screen on its own in any of them.
package eventlist

import (
	"fmt"
	"image"
	"math"
	"strings"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar"
	"github.com/grantlucas/inkwell/internal/inkwell/fonts"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daydata"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/drawkit"
	"golang.org/x/image/font"
)

// minChars is a width in body characters with two uses. A stacked list
// narrower than it lists no events, only the "+N MORE" line counting
// them, and an inline title with less room than it past the time column
// is left off. An empty list's Note gives out at the same width, so the
// list and its empty state give out together.
const minChars = 3

// noteLines is how many lines the unavailable-calendar note may wrap to:
// one a word, so a bold-five column says "CALENDAR" over "UNAVAILABLE"
// rather than cut it short.
const noteLines = 2

// Layout is how an event's time and title sit relative to each other.
type Layout int

const (
	// Stacked puts the time on one line and the title on the lines
	// below it. It is the zero value.
	Stacked Layout = iota
	// Inline puts the time and the title on one line, the title past a
	// column as wide as the widest time label.
	Inline
)

// List chooses how a list is laid out and what it shows. The zero value
// lists every event that fits, stacked: each event's bold time on one
// line and its title on the line below, at body size, edge to edge with
// no gap. Location must be set before a timed event is listed.
type List struct {
	// Layout is stacked (the zero value) or inline.
	Layout Layout
	// MaxEvents is the most events listed. Below 1 there is no cap, and
	// the room alone decides.
	MaxEvents int
	// TitleLines is the most lines one title may wrap to. Below 1 it
	// is 1.
	TitleLines int
	// TimeScale draws the time line at this integer multiple of body
	// size, dilated for weight. Below 1 it is body size.
	TimeScale int
	// TitleLead is how far the title's first baseline sits below the
	// time's. 0 is one body line. Clock labels have no descenders, so a
	// large time can take the title closer than a full line.
	TitleLead int
	// Gap is the paper between one event and the next, and between the
	// last event and the "+N MORE" line.
	Gap int
	// Rules draws a hairline across the middle of each gap between two
	// events, so a wrapped title does not run into the next time.
	Rules bool
	// Empty is what a list with no events says, on its first line. The
	// zero Note says nothing.
	Empty Note
	// Unavailable is set when a calendar the events come from could not
	// be read, so the list may be missing some. The list then says so
	// (daydata.NoCalendar) on its first lines, in the Empty note's style
	// or a shorter form when that does not fit, and lists what did arrive
	// under it. It never says Empty: a day it could not read is not a
	// free one.
	Unavailable bool
	// ShowLocation writes " @ " and the location after a title.
	ShowLocation bool
	// Location is the zone clock times are written in.
	Location *time.Location
}

// Note is a line a list draws in place of events: what an empty list
// says. It is held to the list's fit rule and cut to its width like any
// other line, so it gives out exactly when the list does.
type Note struct {
	// Text is what the note says. "" says nothing.
	Text string
	// Scale draws the note at this integer multiple of body size, in the
	// bold cut and dilated for weight, the way a scaled time is. Below 2
	// it is body size in the regular cut.
	Scale int
	// Centred centres the note across the list rather than starting it
	// at the list's left edge.
	Centred bool
}

// text is one line of a block: what it says, how it is drawn and where
// its baseline sits below the block's top.
type text struct {
	s        string
	drawer   fonts.ScaledDrawer
	dx       int
	baseline int
}

// block is one event, the "+N MORE" line or the Empty line, resolved to
// the exact lines it draws, so the draw pass never re-wraps and cannot
// disagree with the measurement that decided it fit.
type block struct {
	lines []text
	// rows is how many lines of text the block takes down the list. An
	// inline event draws two texts on one row.
	rows int
	// height runs from the block's top to the bottom of its last line,
	// descent included.
	height int
	// more is set on the "+N MORE" line.
	more bool
	// note is set on the unavailable-calendar note.
	note bool
}

// placed is a block at its top edge.
type placed struct {
	block
	top int
}

// Draw draws events into r, from its top-left corner, and returns how
// many it hid. Every line is cut to r's width. A line fits when its whole
// height, ascent and descent, is inside r, and an event is drawn whole
// or not at all. When not every event is drawn, the last visible line
// is "+N MORE", counting every event not shown. A time without room to
// be drawn whole is left off, since a clock time cut short reads as a
// different time. A list too narrow to list events still draws the
// "+N MORE" line, cut to its width, so hidden events are announced
// whenever any line fits. An empty list draws its Empty line, or
// nothing when there is none. An Unavailable list draws its note first,
// in whatever form fits, and its events under it.
func (s List) Draw(frame *image.Paletted, r image.Rectangle, events []calendar.Event) int {
	blocks, hidden := s.layout(r, events)
	for i, b := range blocks {
		if s.Rules && i > 0 && !b.more && !blocks[i-1].note {
			// Across the middle of the gap between this event and the
			// one above it.
			drawkit.DrawHLine(frame, r.Min.X, r.Max.X, b.top-s.Gap+s.Gap/2, widget.PaperBlack)
		}
		for _, l := range b.lines {
			l.drawer.Draw(frame, r.Min.X+l.dx, b.top+l.baseline, l.s)
		}
	}
	return hidden
}

// Lines reports how many lines events need to be drawn in full at width
// pixels: every listed event's lines (a stacked event's time and title
// lines, an inline event's one), and the "+N MORE" line when MaxEvents
// hides any. An empty list needs its Empty line, if it has one. An
// Unavailable list needs its note's lines instead, in full, ahead of its
// events'. A line is one row of text, whatever size it is drawn at. A
// width too narrow to list events needs only the "+N MORE" line, and one
// without a character of room needs none.
func (s List) Lines(events []calendar.Event, width int) int {
	if maxChars, ok := s.charsIn(width); !ok {
		if len(events) > 0 && maxChars >= 1 {
			return 1
		}
		return 0
	}
	if len(events) == 0 && s.empty().Text != "" {
		return 1
	}
	n := 0
	if note, ok := s.unavailable(width, math.MaxInt); ok {
		n = note.rows
	}
	listed := s.listed(events)
	for _, e := range listed {
		n += s.event(e, width).rows
	}
	if len(listed) < len(events) {
		n++
	}
	return n
}

// charsIn is the character budget of a list width pixels wide, and
// whether it is wide enough to list events. Narrower than minChars a
// stacked title is punctuation, and a column of » reads as a fault
// rather than as content. An inline list needs its time column: below
// that the times themselves would be cut.
func (s List) charsIn(width int) (int, bool) {
	maxChars := width / drawkit.BodyAdvance()
	if s.Layout == Inline {
		return maxChars, width >= timeColumn()
	}
	return maxChars, maxChars >= minChars
}

// listed is the events the cap lets through.
func (s List) listed(events []calendar.Event) []calendar.Event {
	if s.MaxEvents > 0 && s.MaxEvents < len(events) {
		return events[:s.MaxEvents]
	}
	return events
}

// layout places the unavailable-calendar note when there is one, then as
// many events as fit in r under it, then the "+N MORE" line when any are
// left over, and reports how many were left over.
func (s List) layout(r image.Rectangle, events []calendar.Event) ([]placed, int) {
	var head []placed
	if note, ok := s.unavailable(r.Dx(), r.Dy()); ok {
		head = append(head, placed{block: note, top: r.Min.Y})
		r.Min.Y += note.height + s.Gap
	}
	out, hidden := s.list(r, events)
	return append(head, out...), hidden
}

// empty is what the list says when it has no events: its Empty note, or
// nothing when its calendar is unavailable, since a day the list could
// not read is not a free one.
func (s List) empty() Note {
	if s.Unavailable {
		return Note{}
	}
	return s.Empty
}

// unavailable resolves the note an Unavailable list says ahead of its
// events, in a list width by height pixels, and reports whether it has
// one. The note takes its Empty note's style, wrapped on words, and
// falls back to shorter forms when that does not fit: at body size, then
// on one body line cut to the width, so a list with a line of room never
// leaves a calendar it could not read looking like a free day. Like the
// Empty note it gives out with the list: a width too narrow to list
// events says nothing.
func (s List) unavailable(width, height int) (block, bool) {
	if _, ok := s.charsIn(width); !s.Unavailable || !ok {
		return block{}, false
	}
	said := Note{Text: daydata.NoCalendar, Scale: s.Empty.Scale, Centred: s.Empty.Centred}
	body := Note{Text: said.Text, Centred: said.Centred}
	forms := []struct {
		note  Note
		lines int
	}{{said, noteLines}, {body, noteLines}, {body, 1}}
	for _, f := range forms {
		if b := f.note.block(width, f.lines); b.rows > 0 && b.height <= height {
			b.note = true
			return b, true
		}
	}
	return block{}, false
}

// list places as many events as fit in r, then the "+N MORE" line when
// any are left over, and reports how many were left over.
func (s List) list(r image.Rectangle, events []calendar.Event) ([]placed, int) {
	maxChars, ok := s.charsIn(r.Dx())

	// The one fit rule: a block fits when its last line's descent ends
	// inside r. An event is placed whole or not at all, because a time
	// with its title clipped off reads as an event with no name.
	fits := func(b block, top int) bool { return top+b.height <= r.Max.Y }

	if len(events) == 0 {
		empty := s.empty()
		if b := empty.block(r.Dx(), 1); ok && empty.Text != "" && fits(b, r.Min.Y) {
			return []placed{{block: b, top: r.Min.Y}}, 0
		}
		return nil, 0
	}

	// Too narrow to list events, every one is hidden, and the overflow
	// rule below still announces them.
	listed := s.listed(events)
	if !ok {
		listed = nil
	}
	var out []placed
	for _, e := range listed {
		b, top := s.event(e, r.Dx()), s.next(out, r)
		if !fits(b, top) {
			break
		}
		out = append(out, placed{block: b, top: top})
	}
	if len(out) == len(events) {
		return out, 0
	}

	// The one overflow rule: something is hidden, so the last visible
	// line says how much. An event that would leave no room for that
	// line gives its place up and is counted with the rest, since a day
	// that silently drops events reads as a quieter day than it is. Only
	// a list without a character of room, or a line of height, hides
	// them unannounced.
	if maxChars < 1 {
		return nil, len(events)
	}
	for {
		line, top := moreLine(len(events)-len(out), maxChars), s.next(out, r)
		if fits(line, top) {
			return append(out, placed{block: line, top: top}), len(events) - len(out)
		}
		if len(out) == 0 {
			return nil, len(events)
		}
		out = out[:len(out)-1]
	}
}

// next is where the block after out starts: the top of r, or a gap
// below the last block placed.
func (s List) next(out []placed, r image.Rectangle) int {
	if len(out) == 0 {
		return r.Min.Y
	}
	last := out[len(out)-1]
	return last.top + last.height + s.Gap
}

// event resolves one event to the lines the list draws it in.
func (s List) event(e calendar.Event, width int) block {
	if s.Layout == Inline {
		return s.inline(e, width)
	}
	return s.stacked(e, width)
}

// inline resolves one event to a single line: the time, then the title
// past the time column, cut on characters to the room left. A title
// with less room than minChars is left off and the time stands alone,
// since a stub of » reads as a fault rather than as a name.
func (s List) inline(e calendar.Event, width int) block {
	ascent := drawkit.BodyAscent()
	col := timeColumn()
	title := ""
	if chars := (width - col) / drawkit.BodyAdvance(); chars >= minChars {
		title = truncate(strings.TrimSpace(s.title(e)), chars)
	}
	return block{
		lines: []text{
			{s: s.timeLabel(e), drawer: body(drawkit.BodyFace), baseline: ascent},
			{s: title, drawer: body(drawkit.BodyFace), dx: col, baseline: ascent},
		},
		rows:   1,
		height: drawkit.BodyLineH(),
	}
}

// timeColumn is the width an inline time takes before its title: the
// widest label, "ALL DAY", and a space. Measuring a clock time alone
// would run the all-day label straight into the title.
func timeColumn() int { return drawkit.TextWidth(drawkit.BodyFace, "ALL DAY ") }

// stacked resolves one event to its time line and the title under it.
func (s List) stacked(e calendar.Event, width int) block {
	maxChars := width / drawkit.BodyAdvance()
	ascent, lineH := drawkit.BodyAscent(), drawkit.BodyLineH()
	descent := lineH - ascent
	scale := max(s.TimeScale, 1)
	timeDrawer := drawkit.Scaled(drawkit.BodyBoldFace, scale, widget.PaperBlack)
	// Dilation can spill this far past a glyph's cell, so the time is
	// measured with it below and to the right. Above and to the left the
	// time keeps to the list's edges, so it lines up with its title:
	// Tamzen's digits, colon and ALL DAY leave empty rows above them and
	// an empty first column, more than any grow reaches.
	grow := timeDrawer.Grow

	baseline := scale * ascent
	b := block{
		lines: []text{{
			s:        s.timeText(e, (width-grow)/(scale*drawkit.BodyAdvance())),
			drawer:   timeDrawer,
			baseline: baseline,
		}},
		height: baseline + scale*descent + grow,
	}

	lead := s.TitleLead
	if lead <= 0 {
		lead = lineH
	}
	for i, l := range Wrap(s.title(e), maxChars, max(s.TitleLines, 1)) {
		if i == 0 {
			baseline += lead
		} else {
			baseline += lineH
		}
		b.lines = append(b.lines, text{s: l, drawer: body(drawkit.BodyFace), baseline: baseline})
		// Whichever line reaches lowest: a title tucked close under a
		// large time can end above the time's own descent.
		b.height = max(b.height, baseline+descent)
	}
	b.rows = len(b.lines)
	return b
}

// timeText is the event's clock label when it fits whole in maxChars,
// and nothing when it does not: a clock time cut short reads as a
// different time ("0" for 09:00), which is worse than no time at all.
func (s List) timeText(e calendar.Event, maxChars int) string {
	label := s.timeLabel(e)
	if runeLen(label) > maxChars {
		return ""
	}
	return label
}

// moreLine is the "+N MORE" line. It is cut to the list's width like every
// other line: on a narrow list "+12 MORE" would otherwise overhang
// whatever is beside it.
func moreLine(hidden, maxChars int) block {
	b := note(fmt.Sprintf("+%d MORE", hidden), drawkit.BodyBoldFace, maxChars)
	b.more = true
	return b
}

// note is one line of body text in face that is not an event, cut to
// maxChars.
func note(s string, face font.Face, maxChars int) block {
	return block{
		lines: []text{{
			s:        truncate(s, maxChars),
			drawer:   body(face),
			baseline: drawkit.BodyAscent(),
		}},
		rows:   1,
		height: drawkit.BodyLineH(),
	}
}

// block resolves the note to the lines it draws in a list width pixels
// wide: one line cut to the width, or up to maxLines wrapped on words.
// A scaled note is cut on its own characters, clear of the pixels its
// dilation spills past its advance, so it stays inside the list.
func (n Note) block(width, maxLines int) block {
	face, scale := drawkit.BodyFace, 1
	if n.Scale > 1 {
		face, scale = drawkit.BodyBoldFace, n.Scale
	}
	drawer := drawkit.Scaled(face, scale, widget.PaperBlack)
	chars := (width - drawer.Grow) / (scale * drawkit.BodyAdvance())
	lines := []string{truncate(n.Text, chars)}
	if maxLines > 1 {
		lines = Wrap(n.Text, chars, maxLines)
	}

	step := scale * drawkit.BodyLineH()
	b := block{rows: len(lines), height: len(lines)*step + drawer.Grow}
	for i, s := range lines {
		dx := 0
		if n.Centred {
			dx = (width - drawer.Measure(s)) / 2
		}
		b.lines = append(b.lines, text{s: s, drawer: drawer, dx: dx, baseline: scale*drawkit.BodyAscent() + i*step})
	}
	return b
}

// body draws in face at body size.
func body(face font.Face) fonts.ScaledDrawer {
	return drawkit.Scaled(face, 1, widget.PaperBlack)
}
