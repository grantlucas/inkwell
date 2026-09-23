package boldfive

import (
	"fmt"
	"image"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daygrid"
)

const (
	// eventsPadX keeps text off the column divider on both sides.
	eventsPadX = 6
	// eventsTopPad puts the first baseline 26 px below the rule.
	eventsTopPad = 12
	// eventsGap separates one event from the next. Events are variable
	// height — a one-line title costs two rows, a wrapped one costs
	// three — so a fixed grid would either waste the short ones or
	// clip the long ones.
	eventsGap = 8
	// maxTitleLines caps a single title so one long summary cannot eat
	// the whole column.
	maxTitleLines = 2

	// Tamzen carries encodings 32-255, so U+2026 HORIZONTAL ELLIPSIS
	// is not in it — the face reports no glyph, the drawers paint
	// nothing with zero advance, and a truncated title simply stopped
	// mid-word with no sign it had been cut. U+00BB is in the range
	// and actually draws.
	ellipsis = "»"
)

// eventOptions carries the per-render knobs from widget config.
//
// Location is the zone event clock labels are rendered in. A parsed
// Event.Start is a correct instant but carries whatever zone its feed
// serialized it with, so formatting it directly would leak that zone
// onto the panel. It must never be nil; parseConfig defaults it.
type eventOptions struct {
	MaxEvents    int
	ShowLocation bool
	Location     *time.Location
}

// eventPlan is one event resolved to the exact rows that will be drawn,
// so the draw pass never re-wraps and cannot disagree with the
// measurement that decided the event fit.
type eventPlan struct {
	timeLine   string
	titleLines []string
}

// rows is how many text rows the plan occupies.
func (p eventPlan) rows() int { return 1 + len(p.titleLines) }

// renderEvents draws a day's agenda into bounds and returns how many
// events were drawn. A rule is drawn along the top of the cell to
// separate the agenda from the weather band above it.
func renderEvents(frame *image.Paletted, bounds image.Rectangle, events []calendar.Event, opts eventOptions) int {
	daygrid.DrawHLine(frame, bounds.Min.X, bounds.Max.X, bounds.Min.Y, widget.PaperBlack)

	maxChars := (bounds.Dx() - 2*eventsPadX) / daygrid.BodyAdvance()
	if maxChars < 3 {
		// Narrower than this and a title is punctuation; drawing a
		// column of ellipses reads as a fault rather than as content.
		return 0
	}

	lineH := daygrid.BodyLineH()
	x := bounds.Min.X + eventsPadX
	y := bounds.Min.Y + eventsTopPad + daygrid.BodyAscent()

	if len(events) == 0 {
		daygrid.DrawTextCentered(frame, bounds.Min.X, bounds.Max.X, y, "--", daygrid.BodyFace, widget.PaperBlack)
		return 0
	}

	// Clamped rather than trusted: parseConfig rejects a non-positive
	// max_events, but New takes a hand-built Config verbatim, and a
	// zero value there would blank the agenda while a negative one
	// would panic on the slice bound.
	limit := min(max(opts.MaxEvents, 0), len(events))
	drawn := 0
	for _, e := range events[:limit] {
		p := planEvent(e, maxChars, opts)
		// The whole event has to fit, title included: a time line with
		// its title clipped off below is worse than not showing the
		// event, because it reads as an event with no name.
		if y+(p.rows()-1)*lineH > bounds.Max.Y {
			break
		}
		daygrid.DrawText(frame, x, y, p.timeLine, daygrid.BodyBoldFace, widget.PaperBlack)
		y += lineH
		for _, line := range p.titleLines {
			daygrid.DrawText(frame, x, y, line, daygrid.BodyFace, widget.PaperBlack)
			y += lineH
		}
		y += eventsGap
		drawn++
	}

	if remaining := len(events) - drawn; remaining > 0 && y <= bounds.Max.Y {
		// Fitted to the column like every other row: on a narrow
		// column "+12 MORE" is wider than the cell and would overhang
		// the divider into the next day's agenda.
		daygrid.DrawText(frame, x, y, truncate(fmt.Sprintf("+%d MORE", remaining), maxChars), daygrid.BodyBoldFace, widget.PaperBlack)
	}
	return drawn
}

// planEvent resolves one event to its drawn rows.
func planEvent(e calendar.Event, maxChars int, opts eventOptions) eventPlan {
	timeLine := "ALL DAY"
	if !e.AllDay {
		timeLine = e.Start.In(opts.Location).Format("15:04")
	}
	title := e.Summary
	if opts.ShowLocation && e.Location != "" {
		title += " @ " + e.Location
	}
	return eventPlan{
		timeLine:   truncate(timeLine, maxChars),
		titleLines: wrapText(title, maxChars, maxTitleLines),
	}
}

// wrapText breaks text into at most maxLines lines of at most maxChars,
// preferring word boundaries and ellipsing whatever will not fit.
//
// The budget is in characters, so every measurement and every cut is in
// runes. Byte arithmetic would wrap an accented title a character or two
// early and, worse, slice a multi-byte glyph in half — the panel then
// paints replacement glyphs for a Japanese or emoji title.
func wrapText(text string, maxChars, maxLines int) []string {
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
	// overhang the column divider.
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
		split[maxLines-1] = truncate(split[maxLines-1]+ellipsis, maxChars)
	}
	return split
}

// runeLen counts characters, which is the unit every text budget here
// is expressed in.
func runeLen(s string) int { return utf8.RuneCountInString(s) }

// truncate shortens s to maxChars, marking the cut with an ellipsis when
// there is room for one.
func truncate(s string, maxChars int) string {
	r := []rune(s)
	if len(r) <= maxChars {
		return s
	}
	if maxChars <= 1 {
		return string(r[:maxChars])
	}
	return string(r[:maxChars-1]) + ellipsis
}
