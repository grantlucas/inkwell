package daytimeline

import (
	"image"
	"math"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/drawkit"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/eventlist"
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

// drawLabel writes e's start time, in loc, then its title, from the top
// of inner in ink, the block's contrast colour. The title follows the time
// on the first line and wraps at word boundaries onto as many lines as the
// block has room for; only the last of them is cut with », when the title
// runs on past it. The block's height and its continuation marks say
// where the event ends, so no line is spent on it.
//
// The label is clipped to inner rather than left out of a block shorter
// than a text line. A half-hour event is about half a line tall on the
// default window; clipped, it loses its descenders and keeps its words,
// where leaving it out would draw a bar nobody can identify. A block too
// short even for the caps is left unlabelled, as is one too narrow for
// the time.
//
// It returns the box each line of text takes, clipped to inner, so the
// now marker can pass behind the words rather than strike them through.
func drawLabel(frame *image.Paletted, inner image.Rectangle, e calendar.Event, loc *time.Location, showLocation bool, ink uint8) []image.Rectangle {
	clip, _ := frame.SubImage(inner).(*image.Paletted)
	adv := drawkit.BodyAdvance()
	x := inner.Min.X + labelPadX
	chars := (inner.Max.X - markClear - x) / adv
	if chars < timeChars || inner.Dy() < capH {
		return nil
	}
	title := e.Summary
	if showLocation && e.Location != "" {
		title += " @ " + e.Location
	}
	start := e.Start.In(loc).Format("15:04")

	// A line after the first is drawn only when its capitals are whole,
	// so a block one line tall doesn't carry the tops of a second.
	baseline := labelBaseline(inner)
	lines := 1
	for baseline+lines*drawkit.BodyLineH() < inner.Max.Y {
		lines++
	}

	var boxes []image.Rectangle
	for i, line := range labelLines(start, title, chars, lines) {
		y := baseline + i*drawkit.BodyLineH()
		if i == 0 {
			drawkit.DrawText(clip, x, y, start, drawkit.BodyBoldFace, ink)
			drawkit.DrawText(clip, x+timeChars*adv, y, line[len(start):], drawkit.BodyFace, ink)
		} else {
			drawkit.DrawText(clip, x, y, line, drawkit.BodyFace, ink)
		}
		boxes = append(boxes, textBox(inner, x, y, utf8.RuneCountInString(line)))
	}
	return boxes
}

// labelLines breaks a label into at most n lines of chars: the start time
// and the title as one run of words, wrapped at word boundaries, so the
// first line always opens with the time. When the title runs on past the
// last line, that line is filled to the width and cut with ». The time
// itself is never cut.
func labelLines(start, title string, chars, n int) []string {
	lines := eventlist.Wrap(start+" "+title, chars, math.MaxInt)
	if len(lines) <= n {
		return lines
	}
	rest := strings.Join(lines[n-1:], " ")
	if n > 1 {
		return append(lines[:n-1], cutTitle(rest, chars))
	}
	line := start
	if room := chars - timeChars - 1; room > 0 {
		line += " " + cutTitle(strings.TrimPrefix(rest, start+" "), room)
	}
	return []string{line}
}

// textBox is the box a line of chars characters takes when written from
// x on baseline, from the top of its caps to the foot of its descenders,
// clipped to inner.
func textBox(inner image.Rectangle, x, baseline, chars int) image.Rectangle {
	descent := drawkit.BodyLineH() - drawkit.BodyAscent()
	return image.Rect(x, baseline-capH, x+chars*drawkit.BodyAdvance(), baseline+descent).Intersect(inner)
}

// labelBaseline is the baseline of a label's first line in inner: its
// caps centred in a short block, and a little way under the top of a
// tall one.
func labelBaseline(inner image.Rectangle) int {
	return inner.Min.Y + min((inner.Dy()-capH)/2, labelMaxPadY) + capH
}

// cutTitle shortens s, which runs past n characters, to n, the last of them
// ». It counts runes, so an accented title is neither cut early nor split
// mid-glyph.
func cutTitle(s string, n int) string {
	return string([]rune(s)[:n-1]) + ellipsis
}
