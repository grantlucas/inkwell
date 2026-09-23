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

	// Tamzen carries encodings 32-255, so U+2026 HORIZONTAL ELLIPSIS
	// is not in it — the face reports no glyph, the drawers paint
	// nothing with zero advance, and a truncated title simply stopped
	// mid-word with no sign it had been cut. U+00BB is in the range
	// and actually draws.
	ellipsis = "»"
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

	// Two passes, because the agenda can run out of room before it
	// runs out of cap. Reserving the marker's line only when the cap
	// truncated the list left the room-limited case — any max_events
	// past what fits — silently stopping with nothing said about the
	// rest, which is the failure this reserve exists to prevent.
	plan := planEvents(events[:limit], y, bounds.Max.Y, maxChars, opts)
	if len(plan) < len(events) {
		plan = planEvents(events[:limit], y, bounds.Max.Y-lineH, maxChars, opts)
	}

	for i, p := range plan {
		if i > 0 {
			// Hairline between events, so a two-line title does not
			// run into the next event's time.
			daygrid.DrawHLine(frame, x, bounds.Max.X-heroPadX, p.top-agendaGap/2, widget.PaperBlack)
		}
		daygrid.Scaled(daygrid.BodyBoldFace, timeScale, widget.PaperBlack).Draw(
			frame, x, p.top+timeAscent, p.timeLine)
		ty := p.top + timeAscent + daygrid.BodyAscent()
		for _, line := range p.titleLines {
			daygrid.DrawText(frame, x, ty, line, daygrid.BodyFace, widget.PaperBlack)
			ty += lineH
		}
	}

	drawn := len(plan)
	if remaining := len(events) - drawn; remaining > 0 {
		my := bounds.Min.Y + agendaTopPad + daygrid.BodyAscent()
		if drawn > 0 {
			last := plan[drawn-1]
			my = last.top + timeAscent + daygrid.BodyAscent() + len(last.titleLines)*lineH + agendaGap
		}
		if my <= bounds.Max.Y {
			daygrid.DrawText(frame, x, my,
				truncate(fmt.Sprintf("+%d MORE TODAY", remaining), maxChars),
				daygrid.BodyBoldFace, widget.PaperBlack)
		}
	}
	return drawn
}

// eventPlan is one event resolved to the rows it will occupy, so the
// draw pass never re-wraps and cannot disagree with the measurement
// that decided the event fit.
type eventPlan struct {
	top        int
	timeLine   string
	titleLines []string
}

// planEvents lays events out from y down to bottom, keeping only those
// that fit whole. A time with its title clipped off below reads as an
// event with no name, so a partial event is not drawn at all.
func planEvents(events []calendar.Event, y, bottom, maxChars int, opts eventOptions) []eventPlan {
	lineH := daygrid.BodyLineH()
	timeAscent := daygrid.BodyAscent() * timeScale

	var out []eventPlan
	for _, e := range events {
		titleLines := wrapText(titleFor(e, opts), maxChars, maxTitleLines)
		needed := timeAscent + daygrid.BodyAscent() + len(titleLines)*lineH
		if y+needed > bottom {
			break
		}
		out = append(out, eventPlan{top: y, timeLine: timeLineFor(e, opts), titleLines: titleLines})
		y += needed + agendaGap
	}
	return out
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
		// Not After: a timed VEVENT with neither DTEND nor DURATION is
		// parsed with End == Start, so After would drop a reminder at
		// the very minute it fires — and if it were the last one, the
		// panel would say "DONE FOR TODAY" over an event happening now.
		// An all-day event applies to the whole day and never finishes
		// partway through it.
		if e.AllDay || !e.End.Before(now) {
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
		split[maxLines-1] = truncate(split[maxLines-1]+ellipsis, maxChars)
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
	return string(r[:maxChars-1]) + ellipsis
}
