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

		// Soft accent rule: the inner pixel is mid-gray, the outer is darker
		// so the rule reads as a 2px stroke with a subtle anti-aliased edge.
		ruleX := bounds.Min.X + eventPadX
		drawVLine(frame, ruleX, y-lineHeight+2, y+2, widget.PaperGray60)
		drawVLine(frame, ruleX+1, y-lineHeight+2, y+2, widget.PaperGray40)

		textX := ruleX + eventRuleW + eventGap

		var timeLine string
		if e.AllDay {
			timeLine = "ALL DAY"
		} else {
			timeLine = e.Start.Format("15:04")
		}
		// Time line is secondary information — render in muted gray so the
		// event title becomes the primary read.
		drawTextGray(frame, textX, y, truncateText(timeLine, maxChars), widget.PaperGray70)
		y += lineHeight + eventLineGap

		if y > bounds.Max.Y-lineHeight {
			rendered++
			break
		}

		drawText(frame, textX, y, truncateText(e.Summary, maxChars))
		y += lineHeight + eventLineGap

		if showLocation && e.Location != "" && y <= bounds.Max.Y-lineHeight {
			drawTextGray(frame, textX, y, truncateText(e.Location, maxChars), widget.PaperGray70)
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
