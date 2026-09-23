package todayhero

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
	// The hero agenda. The time is scaled because it is what you scan
	// for; the title is body size because it is what you read once you
	// are close enough to care.
	timeScale     = 2
	agendaTopPad  = 2
	agendaGap     = 6
	maxTitleLines = 2
	doneText      = "DONE FOR TODAY"
	doneScale     = 2
)

// eventOptions carries the per-render knobs from widget config.
//
// Location is the zone event clock labels are rendered in: a parsed
// Event.Start is a correct instant but carries whatever zone its feed
// serialized it with, so formatting it directly leaks that zone onto
// the panel. It must never be nil; the widget defaults it.
type eventOptions struct {
	MaxEvents    int
	ShowLocation bool
	Location     *time.Location
}

// renderHeroAgenda draws today's remaining events under a rule, and
// returns how many were drawn.
//
// "Remaining" is the point of this block: an event that finished two
// hours ago is history, and on the one screen that spends real estate
// on today it would be spending it on the past. When nothing is left it
// says so, at a size you can read from the same distance as the date.
func renderHeroAgenda(frame *image.Paletted, bounds image.Rectangle, events []calendar.Event, opts eventOptions) int {
	daygrid.DrawHLine(frame, bounds.Min.X+heroPadX, bounds.Max.X-heroPadX, bounds.Min.Y, widget.PaperBlack)

	maxChars := (bounds.Dx() - 2*heroPadX) / daygrid.BodyAdvance()
	if maxChars < 3 {
		return 0
	}

	lineH := daygrid.BodyLineH()
	timeAscent := daygrid.BodyAscent() * timeScale
	x := bounds.Min.X + heroPadX
	y := bounds.Min.Y + agendaTopPad

	if len(events) == 0 {
		daygrid.Scaled(daygrid.BodyBoldFace, doneScale, widget.PaperBlack).Draw(
			frame, x, y+daygrid.BodyAscent()*doneScale, doneText)
		return 0
	}

	limit := min(max(opts.MaxEvents, 0), len(events))

	// When there is more than will be drawn, the marker's line is
	// reserved up front rather than fitted in afterwards. The agenda is
	// almost exactly three events tall, so a marker squeezed in at the
	// end never fits — and an agenda that silently stops is worse than
	// one that shows a event fewer and says how many it dropped.
	bottom := bounds.Max.Y
	if len(events) > limit {
		bottom -= lineH
	}

	drawn := 0
	for i, e := range events[:limit] {
		titleLines := wrapText(titleFor(e, opts), maxChars, maxTitleLines)
		// A whole event or none of it: a time with its title clipped
		// off below reads as an event with no name.
		needed := timeAscent + daygrid.BodyAscent() + len(titleLines)*lineH
		if y+needed > bottom {
			break
		}

		if i > 0 {
			// Hairline between events, so a two-line title does not
			// run into the next event's time.
			daygrid.DrawHLine(frame, x, bounds.Max.X-heroPadX, y-agendaGap/2, widget.PaperBlack)
		}

		daygrid.Scaled(daygrid.BodyBoldFace, timeScale, widget.PaperBlack).Draw(
			frame, x, y+timeAscent, timeLineFor(e, opts))
		y += timeAscent + daygrid.BodyAscent()

		for _, line := range titleLines {
			daygrid.DrawText(frame, x, y, line, daygrid.BodyFace, widget.PaperBlack)
			y += lineH
		}
		y += agendaGap
		drawn++
	}

	if remaining := len(events) - drawn; remaining > 0 && y+daygrid.BodyAscent() <= bounds.Max.Y {
		daygrid.DrawText(frame, x, y+daygrid.BodyAscent(),
			truncate(fmt.Sprintf("+%d MORE TODAY", remaining), maxChars),
			daygrid.BodyBoldFace, widget.PaperBlack)
	}
	return drawn
}

// timeLineFor is the event's clock label. Times stay precise — 16:15,
// not "quarter past four". They are data, not a clock: they do not
// change on a tick, so fuzzing them would lose real information for no
// refresh benefit. Only the identity block's clock is fuzzy.
func timeLineFor(e calendar.Event, opts eventOptions) string {
	if e.AllDay {
		return "ALL DAY"
	}
	return e.Start.In(opts.Location).Format("15:04")
}

func titleFor(e calendar.Event, opts eventOptions) string {
	if opts.ShowLocation && e.Location != "" {
		return e.Summary + " @ " + e.Location
	}
	return e.Summary
}

// remainingToday drops events that have already finished. An event
// still running counts as remaining — it is the one you most want to
// see.
func remainingToday(events []calendar.Event, now time.Time) []calendar.Event {
	var out []calendar.Event
	for _, e := range events {
		// An all-day event applies to the whole day, so it never
		// "finishes" partway through it.
		if e.AllDay || e.End.After(now) {
			out = append(out, e)
		}
	}
	return out
}

// wrapText breaks text into at most maxLines lines of at most maxChars,
// preferring word boundaries and ellipsing whatever will not fit.
//
// The budget is in characters, so every measurement and cut is in runes
// — byte arithmetic would wrap an accented title early and slice a
// multi-byte glyph in half, leaving the panel to paint replacements.
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
		split[maxLines-1] = truncate(split[maxLines-1]+"…", maxChars)
	}
	return split
}

func runeLen(s string) int { return utf8.RuneCountInString(s) }

// truncate shortens s to maxChars, marking the cut with an ellipsis
// when there is room for one.
func truncate(s string, maxChars int) string {
	r := []rune(s)
	if len(r) <= maxChars {
		return s
	}
	if maxChars <= 1 {
		return string(r[:maxChars])
	}
	return string(r[:maxChars-1]) + "…"
}
