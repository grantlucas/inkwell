package weekly

import (
	"image"
	"sort"
	"strings"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
)

const (
	eventPadX    = 4
	eventRuleW   = 2
	eventGap     = 2
	eventLineGap = 2
)

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
	y := bounds.Min.Y + lineHeight
	rendered := 0

	for _, p := range plan {
		// 2-px solid PaperBlack rule. The previous outer/inner gray pair
		// only read as "soft" because the Bayer dither broke up the
		// strokes; without dithering, PaperGray40 threshold-snaps away
		// entirely and the inner line vanishes. Solid black on both
		// strokes is the durable choice now.
		drawVLine(frame, ruleX, y-lineHeight+2, y+2, widget.PaperBlack)
		drawVLine(frame, ruleX+1, y-lineHeight+2, y+2, widget.PaperBlack)

		// All event text renders in solid PaperBlack. A gray source color
		// loses its AA fringe to the BW threshold and small body text
		// fragments. The event title's role as the primary read is
		// carried by being the second line in the cell (visually below
		// the time), not by being darker than the surrounding text.
		drawText(frame, textX, y, truncateText(p.timeLine, maxChars))
		y += lineHeight + eventLineGap
		rendered++

		if !p.drawTitle { // time-only event at the bottom of the column
			break
		}

		if p.titleBudget <= 1 {
			drawText(frame, textX, y, truncateText(p.summary, maxChars))
			y += lineHeight + eventLineGap
		} else {
			for _, line := range wrapText(p.summary, maxChars, p.titleBudget) {
				drawText(frame, textX, y, line)
				y += lineHeight + eventLineGap
			}
		}

		if p.drawLocation {
			drawText(frame, textX, y, truncateText(p.location, maxChars))
			y += lineHeight + eventLineGap
		}
	}

	if rendered == 0 {
		drawTextCentered(frame, bounds.Min.X, bounds.Max.X, bounds.Min.Y+lineHeight, "--")
	}

	return rendered
}

// maxTitleLines caps how many lines a single wrapped title may occupy so one
// very long title can't eat a whole sparse column.
const maxTitleLines = 3

// eventPlan describes how one event should be drawn within a day column.
type eventPlan struct {
	timeLine     string
	summary      string
	location     string
	drawTitle    bool
	drawLocation bool
	titleBudget  int // number of lines to spend on the title (>= 1)
}

// planEvents decides, in a single pass over the day's events, which events fit
// in the column and how many title lines each gets. It first lays events out at
// their baseline cost (time + title, plus a location line when shown), exactly
// mirroring the original draw loop's slot accounting, then hands any leftover
// slots to titles that overflow the column width so sparse days wrap instead of
// truncating.
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

	distributeWrap(plans, capacity-used, maxChars)
	return plans
}

// distributeWrap hands `leftover` line slots round-robin to planned events whose
// title overflows the column width, up to each title's natural wrap depth
// (capped at maxTitleLines). Non-overflowing titles are left at one line.
func distributeWrap(plans []eventPlan, leftover, maxChars int) {
	if leftover <= 0 {
		return
	}
	desired := make([]int, len(plans))
	any := false
	for i := range plans {
		if plans[i].drawTitle && len(plans[i].summary) > maxChars {
			desired[i] = len(wrapText(plans[i].summary, maxChars, maxTitleLines))
			if desired[i] > 1 {
				any = true
			}
		}
	}
	if !any {
		return
	}
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

// wrapText breaks text into at most maxLines lines no wider than maxChars,
// preferring word boundaries.
func wrapText(text string, maxChars, maxLines int) []string {
	if maxChars < 1 || maxLines < 1 {
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
		// hard-break it into column-width chunks.
		for len(w) > maxChars {
			flush()
			lines = append(lines, w[:maxChars])
			w = w[maxChars:]
		}
		switch {
		case cur == "":
			cur = w
		case len(cur)+1+len(w) <= maxChars:
			cur += " " + w
		default:
			flush()
			cur = w
		}
	}
	flush()
	if len(lines) > maxLines {
		lines = lines[:maxLines]
		// Mark the dropped remainder with an ellipsis when it still fits
		// within the column width.
		if last := lines[maxLines-1]; len(last)+3 <= maxChars {
			lines[maxLines-1] = last + "..."
		}
	}
	return lines
}

// filterEventsForDay returns events overlapping [dayStart, dayEnd), sorted by start.
func filterEventsForDay(events []calendar.Event, dayStart, dayEnd time.Time) []calendar.Event {
	var filtered []calendar.Event
	for _, e := range events {
		if e.Start.Before(dayEnd) && e.End.After(dayStart) {
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
