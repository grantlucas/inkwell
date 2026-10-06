package daytimeline

import (
	"image"
	"time"
	"unicode/utf8"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daygrid"
)

const (
	// labelPadX is the paper between a block's inside edge and its label.
	labelPadX = 4
	// capH is how many rows the body face's capitals and figures ink,
	// all of them above the baseline. The face's ascent carries blank
	// rows over them, so placing a label by its caps rather than its
	// ascent is what fits one into a half-hour block.
	capH = 10
	// labelMaxPadY is the most paper over a label's caps; a block with
	// fewer spare rows centres the caps in what it has.
	labelMaxPadY = 3
	// timeChars is the "15:04" a label starts with.
	timeChars = 5
	// ellipsis marks a cut title. U+2026 isn't in Tamzen; » is, and
	// it is what the event list cuts with.
	ellipsis = "»"
)

// markClear is the width at a block's right end a label leaves free, so
// a continuation mark always has its corner to itself.
const markClear = 2*markPad + markW

// drawLabel writes e's start time, in loc, then its title, on one line
// at the top of inner in ink, the block's contrast colour.
//
// The label is clipped to inner rather than left out of a block shorter
// than a text line. A half-hour event is about half a line tall on the
// default window; clipped, it loses its descenders and keeps its words,
// where leaving it out would draw a bar nobody can identify. A block too
// short even for the caps is left unlabelled, as is one too narrow for
// the time; a title that doesn't fit the width is cut with ».
func drawLabel(frame *image.Paletted, inner image.Rectangle, e calendar.Event, loc *time.Location, showLocation bool, ink uint8) {
	clip, _ := frame.SubImage(inner).(*image.Paletted)
	adv := daygrid.BodyAdvance()
	x := inner.Min.X + labelPadX
	chars := (inner.Max.X - markClear - x) / adv
	if chars < timeChars || inner.Dy() < capH {
		return
	}
	baseline := labelBaseline(inner)
	daygrid.DrawText(clip, x, baseline, e.Start.In(loc).Format("15:04"), daygrid.BodyBoldFace, ink)

	title := e.Summary
	if showLocation && e.Location != "" {
		title += " @ " + e.Location
	}
	if room := chars - timeChars - 1; room > 0 {
		daygrid.DrawText(clip, x+(timeChars+1)*adv, baseline, truncate(title, room), daygrid.BodyFace, ink)
	}

	// A block with room for a second line says when the event ends: the
	// block's length only shows it to the nearest few minutes, and an
	// event cut off at the window's end shows nothing of it at all. Only
	// whole capitals are drawn, so a block one line tall doesn't carry
	// the tops of a second.
	if until := baseline + daygrid.BodyLineH(); until < inner.Max.Y && e.End.After(e.Start) {
		daygrid.DrawText(clip, x, until, "UNTIL "+e.End.In(loc).Format("15:04"), daygrid.BodyFace, ink)
	}
}

// labelBaseline is the baseline of a label's first line in inner: its
// caps centred in a short block, and a little way under the top of a
// tall one.
func labelBaseline(inner image.Rectangle) int {
	return inner.Min.Y + min((inner.Dy()-capH)/2, labelMaxPadY) + capH
}

// truncate shortens s to at most n characters, marking a cut with ».
// It counts runes, so an accented title is neither cut early nor split
// mid-glyph.
func truncate(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n-1]) + ellipsis
}
