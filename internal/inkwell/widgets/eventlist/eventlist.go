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

// Style chooses how a list is laid out and what it shows.
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

// Draw draws events into r and returns how many it hid.
func (s Style) Draw(frame *image.Paletted, r image.Rectangle, events []calendar.Event) int {
	blocks, hidden := s.layout(r, events)
	for i, b := range blocks {
		if s.Rules && i > 0 && !b.marker {
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

	var out []placed
	top := r.Min.Y
	for _, e := range s.listed(events) {
		b := s.event(e, maxChars)
		if top+b.height > r.Max.Y {
			break
		}
		out = append(out, placed{block: b, top: top})
		top += b.height + s.Gap
	}
	if len(out) == len(events) {
		return out, 0
	}

	// Something is hidden, so the last visible line has to say so. An
	// event that would leave no room for that line gives its place up,
	// and is counted with the rest: a day that silently drops events
	// reads as a quieter day than it is.
	for {
		top := r.Min.Y
		if n := len(out); n > 0 {
			top = out[n-1].top + out[n-1].height + s.Gap
		}
		more := marker(len(events)-len(out), maxChars)
		if top+more.height <= r.Max.Y {
			return append(out, placed{block: more, top: top}), len(events) - len(out)
		}
		if len(out) == 0 {
			return nil, len(events)
		}
		out = out[:len(out)-1]
	}
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
