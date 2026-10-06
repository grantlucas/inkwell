// Package eventlist turns a day's events into drawn lines. Every widget
// that lists events draws them through it, so they all write an event
// the same way, fit lines by the same rule and say how many events they
// hid the same way.
package eventlist

import (
	"fmt"
	"image"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar"
	"github.com/grantlucas/inkwell/internal/inkwell/fonts"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daygrid"
	"golang.org/x/image/font"
)

// minChars is the narrowest list, in characters, that draws anything.
const minChars = 3

// Style chooses how a list is laid out and what it shows. The zero value
// lists every event that fits, stacked: each event's bold time on one
// line and its title on the line below, at body size, edge to edge with
// no gap. Location must be set before a timed event is listed.
type Style struct {
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
	// ShowLocation writes " @ " and the location after a title.
	ShowLocation bool
	// Location is the zone clock times are written in.
	Location *time.Location
}

// text is one line of a block: what it says, how it is drawn and where
// its baseline sits below the block's top.
type text struct {
	s        string
	drawer   fonts.ScaledDrawer
	baseline int
}

// block is one event, or the "+N MORE" line, resolved to the exact lines
// it draws, so the draw pass never re-wraps and cannot disagree with the
// measurement that decided it fit.
type block struct {
	lines []text
	// height runs from the block's top to the bottom of its last line,
	// descent included.
	height int
	// marker is set on the "+N MORE" line.
	marker bool
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
// is "+N MORE", counting every event not shown. An empty list draws
// nothing: what an empty day says is the widget's.
func (s Style) Draw(frame *image.Paletted, r image.Rectangle, events []calendar.Event) int {
	blocks, hidden := s.layout(r, events)
	for i, b := range blocks {
		if s.Rules && i > 0 && !b.marker {
			// Across the middle of the gap above this event.
			daygrid.DrawHLine(frame, r.Min.X, r.Max.X, b.top-s.Gap+s.Gap/2, widget.PaperBlack)
		}
		for _, l := range b.lines {
			l.drawer.Draw(frame, r.Min.X, b.top+l.baseline, l.s)
		}
	}
	return hidden
}

// Lines reports how many lines events need to be drawn in full at width
// pixels: every listed event's time and title lines, and the "+N MORE"
// line when MaxEvents hides any. A line is one row of text, whatever
// size it is drawn at. A width too narrow to draw into needs none.
func (s Style) Lines(events []calendar.Event, width int) int {
	maxChars, ok := charsIn(width)
	if !ok {
		return 0
	}
	listed := s.listed(events)
	n := 0
	for _, e := range listed {
		n += len(s.event(e, maxChars).lines)
	}
	if len(listed) < len(events) {
		n++
	}
	return n
}

// charsIn is the character budget of a list width pixels wide, and
// whether it is wide enough to draw anything. Narrower than minChars a
// title is punctuation, and a column of » reads as a fault rather than
// as content.
func charsIn(width int) (int, bool) {
	maxChars := width / daygrid.BodyAdvance()
	return maxChars, maxChars >= minChars
}

// listed is the events the cap lets through.
func (s Style) listed(events []calendar.Event) []calendar.Event {
	if s.MaxEvents > 0 && s.MaxEvents < len(events) {
		return events[:s.MaxEvents]
	}
	return events
}

// layout places as many events as fit in r, then the "+N MORE" line
// when any are left over, and reports how many were left over.
func (s Style) layout(r image.Rectangle, events []calendar.Event) ([]placed, int) {
	maxChars, ok := charsIn(r.Dx())
	if !ok {
		return nil, len(events)
	}

	// The one fit rule: a block fits when its last line's descent ends
	// inside r. An event is placed whole or not at all, because a time
	// with its title clipped off reads as an event with no name.
	fits := func(b block, top int) bool { return top+b.height <= r.Max.Y }

	var out []placed
	for _, e := range s.listed(events) {
		b, top := s.event(e, maxChars), s.next(out, r)
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
	// that silently drops events reads as a quieter day than it is.
	for {
		more, top := marker(len(events)-len(out), maxChars), s.next(out, r)
		if fits(more, top) {
			return append(out, placed{block: more, top: top}), len(events) - len(out)
		}
		if len(out) == 0 {
			return nil, len(events)
		}
		out = out[:len(out)-1]
	}
}

// next is where the block after out starts: the top of r, or a gap
// below the last block placed.
func (s Style) next(out []placed, r image.Rectangle) int {
	if len(out) == 0 {
		return r.Min.Y
	}
	last := out[len(out)-1]
	return last.top + last.height + s.Gap
}

// event resolves one event to its time line and the title under it.
func (s Style) event(e calendar.Event, maxChars int) block {
	ascent, lineH := daygrid.BodyAscent(), daygrid.BodyLineH()
	descent := lineH - ascent
	scale := max(s.TimeScale, 1)

	baseline := scale * ascent
	b := block{
		lines: []text{{
			s:        Truncate(s.TimeLabel(e), maxChars/scale),
			drawer:   daygrid.Scaled(daygrid.BodyBoldFace, scale, widget.PaperBlack),
			baseline: baseline,
		}},
		height: baseline + scale*descent,
	}

	lead := s.TitleLead
	if lead <= 0 {
		lead = lineH
	}
	for i, l := range wrap(s.Title(e), maxChars, max(s.TitleLines, 1)) {
		if i == 0 {
			baseline += lead
		} else {
			baseline += lineH
		}
		b.lines = append(b.lines, text{s: l, drawer: body(daygrid.BodyFace), baseline: baseline})
		b.height = baseline + descent
	}
	return b
}

// marker is the "+N MORE" line. It is cut to the list's width like every
// other line: on a narrow list "+12 MORE" would otherwise overhang
// whatever is beside it.
func marker(hidden, maxChars int) block {
	return block{
		lines: []text{{
			s:        Truncate(fmt.Sprintf("+%d MORE", hidden), maxChars),
			drawer:   body(daygrid.BodyBoldFace),
			baseline: daygrid.BodyAscent(),
		}},
		height: daygrid.BodyLineH(),
		marker: true,
	}
}

// body draws in face at body size.
func body(face font.Face) fonts.ScaledDrawer {
	return daygrid.Scaled(face, 1, widget.PaperBlack)
}
