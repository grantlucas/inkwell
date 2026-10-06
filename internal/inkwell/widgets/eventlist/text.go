package eventlist

import (
	"strings"
	"unicode/utf8"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar"
)

// Tamzen carries encodings 32-255, so U+2026 HORIZONTAL ELLIPSIS is not
// in it: the face reports no glyph, the drawers paint nothing with zero
// advance, and a cut title simply stopped mid-word with no sign it had
// been cut. U+00BB is in the range and actually draws.
const ellipsis = "»"

// The text rules are exported only for today-hero's day rows, which
// write events inline until #146 gives the module an inline layout.
// Once nothing outside the package calls them they go back to being
// unexported.

// TimeLabel is the event's clock label: "ALL DAY", or a 24-hour time in
// the style's Location. Times stay precise (16:15, not "quarter past
// four"): they are data, not a clock, and fuzzing them would lose real
// information for no refresh benefit.
//
// A parsed Event.Start is a correct instant but carries whatever zone
// its feed serialized it with, so it is converted to the display zone
// before it is written.
func (s Style) TimeLabel(e calendar.Event) string {
	if e.AllDay {
		return "ALL DAY"
	}
	return e.Start.In(s.Location).Format("15:04")
}

// Title is the event's summary, followed by " @ " and where it is when
// the style shows locations and the event has one.
func (s Style) Title(e calendar.Event) string {
	if s.ShowLocation && e.Location != "" {
		return e.Summary + " @ " + e.Location
	}
	return e.Summary
}

// wrap breaks text into at most maxLines lines of at most maxChars,
// preferring word boundaries and cutting whatever will not fit.
//
// The budget is in characters, so every measurement and every cut is in
// runes. Byte arithmetic would wrap an accented title a character or
// two early and, worse, slice a multi-byte glyph in half, leaving the
// panel to paint replacement glyphs.
func wrap(text string, maxChars, maxLines int) []string {
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return nil
	}

	var lines []string
	cur := fields[0]
	for _, w := range fields[1:] {
		if runeLen(cur)+1+runeLen(w) <= maxChars {
			cur += " " + w
			continue
		}
		lines = append(lines, cur)
		cur = w
	}
	lines = append(lines, cur)

	// A single word longer than the line is broken rather than left to
	// overhang whatever is beside the list.
	var split []string
	for _, l := range lines {
		for runeLen(l) > maxChars {
			r := []rune(l)
			split = append(split, string(r[:maxChars]))
			l = string(r[maxChars:])
		}
		split = append(split, l)
	}

	if len(split) > maxLines {
		split = split[:maxLines]
		split[maxLines-1] = Truncate(split[maxLines-1]+ellipsis, maxChars)
	}
	return split
}

// runeLen counts characters, the unit every text budget here is in.
func runeLen(s string) int { return utf8.RuneCountInString(s) }

// Truncate shortens s to maxChars characters, marking the cut with »
// when there is room for one. It counts runes, never bytes.
func Truncate(s string, maxChars int) string {
	r := []rune(s)
	if len(r) <= maxChars {
		return s
	}
	if maxChars <= 1 {
		return string(r[:maxChars])
	}
	return string(r[:maxChars-1]) + ellipsis
}
