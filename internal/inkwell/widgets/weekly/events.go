package weekly

import (
	"image"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
)

const (
	eventPadX    = 4
	eventRuleW   = 2
	eventGap     = 2
	eventLineGap = 2
)

// maxTitleLines caps how many lines a single wrapped title may occupy so one
// very long title can't eat a whole sparse column.
const maxTitleLines = 3

// eventPlan describes how one event should be drawn within a day column. It is
// the single source of truth produced by planEvents: titleLines holds the exact
// text rows to draw for the title (already wrapped/truncated), so the draw pass
// stays dumb and never re-wraps.
type eventPlan struct {
	timeLine     string
	summary      string
	location     string
	titleLines   []string
	drawTitle    bool
	drawLocation bool
	titleBudget  int // number of lines budgeted for the title (>= 1)
}

// renderEvents draws calendar events for a single day within the given bounds.
// Each event gets a 2px left rule, time line, and title. On a sparse day the
// leftover vertical space is spent wrapping overflowing titles across extra
// lines (see planEvents); a full column draws exactly as it did before.
// Returns the number of events actually drawn.
func renderEvents(frame *image.Paletted, bounds image.Rectangle, events []calendar.Event, maxEvents int, showLocation bool) int {
	maxChars := (bounds.Dx() - eventPadX - eventRuleW - eventGap) / charWidth
	if maxChars < 3 {
		return 0
	}

	plan := planEvents(events, maxEvents, lineCapacity(bounds), maxChars, showLocation)

	ruleX := bounds.Min.X + eventPadX
	textX := ruleX + eventRuleW + eventGap
	step := lineHeight + eventLineGap
	y := bounds.Min.Y + lineHeight
	rendered := 0

	for _, p := range plan {
		// 2-px solid PaperBlack rule. The previous outer/inner gray pair
		// only read as "soft" because the Bayer dither broke up the
		// strokes; without dithering, PaperGray40 threshold-snaps away
		// entirely and the inner line vanishes. Solid black on both
		// strokes is the durable choice now. For a wrapped (multi-line)
		// title the rule extends to span the whole event so the stacked
		// lines read as one block; single-line events keep the original
		// one-line tick, leaving full columns visually unchanged.
		ruleTop := y - lineHeight + 2
		ruleBottom := y + 2
		if len(p.titleLines) > 1 {
			tail := len(p.titleLines)
			if p.drawLocation {
				tail++
			}
			ruleBottom = y + tail*step + 2
		}
		drawVLine(frame, ruleX, ruleTop, ruleBottom, widget.PaperBlack)
		drawVLine(frame, ruleX+1, ruleTop, ruleBottom, widget.PaperBlack)

		// All event text renders in solid PaperBlack. A gray source color
		// loses its AA fringe to the BW threshold and small body text
		// fragments. The event title's role as the primary read is
		// carried by being the second line in the cell (visually below
		// the time), not by being darker than the surrounding text.
		drawText(frame, textX, y, truncateText(p.timeLine, maxChars))
		y += step
		rendered++

		if !p.drawTitle { // time-only event at the bottom of the column
			break
		}

		for _, line := range p.titleLines {
			drawText(frame, textX, y, line)
			y += step
		}

		if p.drawLocation {
			drawText(frame, textX, y, truncateText(p.location, maxChars))
			y += step
		}
	}

	if rendered == 0 {
		drawTextCentered(frame, bounds.Min.X, bounds.Max.X, bounds.Min.Y+lineHeight, "--")
	}

	return rendered
}

// planEvents decides, in a single pass over the day's events, which events fit
// in the column and how many title lines each gets. It first lays events out at
// their baseline cost (time + title, plus a location line when shown), exactly
// mirroring the original draw loop's slot accounting, then assignTitleLines
// hands any leftover slots to titles that overflow the column width so sparse
// days wrap instead of truncating.
func planEvents(events []calendar.Event, maxEvents, capacity, maxChars int, showLocation bool) []eventPlan {
	var plans []eventPlan
	used := 0
	for _, e := range events {
		if len(plans) >= maxEvents {
			break
		}
		if used >= capacity { // no slot for this event's time line
			break
		}

		p := eventPlan{summary: e.Summary, titleBudget: 1}
		if e.AllDay {
			p.timeLine = "ALL DAY"
		} else {
			p.timeLine = e.Start.Format("15:04")
		}
		used++ // time line

		if used >= capacity { // time fit but title doesn't — render time only
			plans = append(plans, p)
			break
		}
		p.drawTitle = true
		used++ // title line

		if showLocation && e.Location != "" && used < capacity {
			p.location = e.Location
			p.drawLocation = true
			used++ // location line
		}

		plans = append(plans, p)
	}

	assignTitleLines(plans, capacity-used, maxChars)
	return plans
}

// assignTitleLines wraps every drawable title once, hands `leftover` line slots
// round-robin to titles that overflow the column width (capped at
// maxTitleLines), and records the final text rows on each plan. Wrapping each
// title a single time keeps the computation out of the draw pass and out of a
// second pass here.
func assignTitleLines(plans []eventPlan, leftover, maxChars int) {
	full := make([][]string, len(plans))
	desired := make([]int, len(plans))
	wrappable := false
	for i := range plans {
		if !plans[i].drawTitle {
			continue
		}
		full[i] = wrapLines(plans[i].summary, maxChars)
		desired[i] = min(len(full[i]), maxTitleLines)
		if desired[i] > 1 {
			wrappable = true
		}
	}

	if leftover > 0 && wrappable {
		for leftover > 0 {
			progressed := false
			for i := range plans {
				if leftover <= 0 {
					break
				}
				if plans[i].titleBudget < desired[i] {
					plans[i].titleBudget++
					leftover--
					progressed = true
				}
			}
			if !progressed {
				break
			}
		}
	}

	for i := range plans {
		if !plans[i].drawTitle {
			continue
		}
		switch {
		case plans[i].titleBudget > 1:
			plans[i].titleLines = capLines(full[i], plans[i].titleBudget, maxChars)
		case len(full[i]) <= 1:
			// Fits on one (whitespace-collapsed) line — show it in full.
			plans[i].titleLines = full[i]
		default:
			// Overflows but earned no extra slot (e.g. a full column):
			// keep the original character-level truncation so packed
			// columns stay byte-for-byte unchanged.
			plans[i].titleLines = []string{truncateText(plans[i].summary, maxChars)}
		}
	}
}

// lineCapacity reports how many text-line slots fit vertically in bounds.
// It mirrors the draw loop's "y > Max.Y-lineHeight" stop condition: the first
// baseline sits at Min.Y+lineHeight and each subsequent line advances by
// lineHeight+eventLineGap.
func lineCapacity(bounds image.Rectangle) int {
	n := 0
	for y := bounds.Min.Y + lineHeight; y <= bounds.Max.Y-lineHeight; y += lineHeight + eventLineGap {
		n++
	}
	return n
}

// runeLen counts runes — the display width on the monospace face — so wrapping
// math is correct for multi-byte UTF-8 titles instead of slicing mid-rune.
func runeLen(s string) int { return utf8.RuneCountInString(s) }

// wrapLines word-wraps text into lines no wider than maxChars runes, breaking a
// word that is itself wider than the column on a rune boundary. It does not cap
// the number of lines; see capLines.
func wrapLines(text string, maxChars int) []string {
	if maxChars < 1 {
		return nil
	}
	var lines []string
	cur := ""
	flush := func() {
		if cur != "" {
			lines = append(lines, cur)
			cur = ""
		}
	}
	for w := range strings.FieldsSeq(text) {
		// A single word wider than the column can't fit on any line, so
		// hard-break it into column-width chunks on rune boundaries.
		for runeLen(w) > maxChars {
			flush()
			rs := []rune(w)
			lines = append(lines, string(rs[:maxChars]))
			w = string(rs[maxChars:])
		}
		switch {
		case cur == "":
			cur = w
		case runeLen(cur)+1+runeLen(w) <= maxChars:
			cur += " " + w
		default:
			flush()
			cur = w
		}
	}
	flush()
	return lines
}

// capLines truncates lines to at most maxLines, marking the dropped remainder
// with an ellipsis on the last kept line when it still fits the column width.
func capLines(lines []string, maxLines, maxChars int) []string {
	if maxLines < 1 {
		return nil
	}
	if len(lines) <= maxLines {
		return lines
	}
	lines = lines[:maxLines]
	if last := lines[maxLines-1]; runeLen(last)+3 <= maxChars {
		lines[maxLines-1] = last + "..."
	}
	return lines
}

// wrapText breaks text into at most maxLines lines no wider than maxChars,
// preferring word boundaries.
func wrapText(text string, maxChars, maxLines int) []string {
	return capLines(wrapLines(text, maxChars), maxLines, maxChars)
}

// dateOnly strips the clock and zone from t, returning a comparable
// midnight-UTC anchor of t's calendar date. It lets all-day events
// (date labels) be bucketed by date regardless of the zone each side
// carries.
func dateOnly(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// filterEventsForDay returns events overlapping [dayStart, dayEnd), sorted by start.
//
// All-day events are calendar date labels, not instants: an iCal VALUE=DATE
// is anchored to UTC midnight by the parser, but the day columns are built in
// the viewer's local zone (now.Location()). Comparing the two as instants
// leaks an all-day event into the previous local day in any negative-UTC zone
// (e.g. a Thursday trip showing up on Wednesday in America/Toronto). So we
// bucket all-day events by their date components alone — zone-independently —
// and reserve instant overlap for timed events.
func filterEventsForDay(events []calendar.Event, dayStart, dayEnd time.Time) []calendar.Event {
	col := dateOnly(dayStart)
	var filtered []calendar.Event
	for _, e := range events {
		var overlaps bool
		if e.AllDay {
			// DTEND is exclusive, so the column date must fall in
			// [startDate, endDate): col >= start and col < end.
			overlaps = !col.Before(dateOnly(e.Start)) && col.Before(dateOnly(e.End))
		} else {
			overlaps = e.Start.Before(dayEnd) && e.End.After(dayStart)
		}
		if overlaps {
			filtered = append(filtered, e)
		}
	}
	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].AllDay != filtered[j].AllDay {
			return filtered[i].AllDay
		}
		return filtered[i].Start.Before(filtered[j].Start)
	})
	return filtered
}
