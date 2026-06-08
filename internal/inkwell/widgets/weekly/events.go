package weekly

import (
	"image"
	"sort"
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
// Each event gets a 2px left rule, time line, and title. Returns the number of
// events actually drawn.
func renderEvents(frame *image.Paletted, bounds image.Rectangle, events []calendar.Event, maxEvents int, showLocation bool) int {
	maxChars := (bounds.Dx() - eventPadX - eventRuleW - eventGap) / charWidth
	if maxChars < 3 {
		return 0
	}

	y := bounds.Min.Y + lineHeight
	rendered := 0

	for _, e := range events {
		if rendered >= maxEvents {
			break
		}
		if y > bounds.Max.Y-lineHeight {
			break
		}

		// 2-px solid PaperBlack rule. The previous outer/inner gray pair
		// only read as "soft" because the Bayer dither broke up the
		// strokes; without dithering, PaperGray40 threshold-snaps away
		// entirely and the inner line vanishes. Solid black on both
		// strokes is the durable choice now.
		ruleX := bounds.Min.X + eventPadX
		drawVLine(frame, ruleX, y-lineHeight+2, y+2, widget.PaperBlack)
		drawVLine(frame, ruleX+1, y-lineHeight+2, y+2, widget.PaperBlack)

		textX := ruleX + eventRuleW + eventGap

		var timeLine string
		if e.AllDay {
			timeLine = "ALL DAY"
		} else {
			timeLine = e.Start.Format("15:04")
		}
		// All event text renders in solid PaperBlack. A gray source color
		// loses its AA fringe to the BW threshold and small body text
		// fragments. The event title's role as the primary read is
		// carried by being the second line in the cell (visually below
		// the time), not by being darker than the surrounding text.
		drawText(frame, textX, y, truncateText(timeLine, maxChars))
		y += lineHeight + eventLineGap

		if y > bounds.Max.Y-lineHeight {
			rendered++
			break
		}

		drawText(frame, textX, y, truncateText(e.Summary, maxChars))
		y += lineHeight + eventLineGap

		if showLocation && e.Location != "" && y <= bounds.Max.Y-lineHeight {
			drawText(frame, textX, y, truncateText(e.Location, maxChars))
			y += lineHeight + eventLineGap
		}

		rendered++
	}

	if rendered == 0 {
		drawTextCentered(frame, bounds.Min.X, bounds.Max.X, bounds.Min.Y+lineHeight, "--")
	}

	return rendered
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
