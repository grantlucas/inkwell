package calendar

import (
	"fmt"
	"image"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar"
)

// renderToday draws today's events list.
//
// Layout:
//
//	Thursday, Apr 25
//	──────────────────
//	 All day: Holiday
//	 9:00 – 10:00  Standup
//	14:00 – 15:30  Planning
//	──────────────────
//	3 events today
func renderToday(frame *image.Paletted, bounds image.Rectangle, events []calendar.Event, cfg Config) error {
	fillWhite(frame, bounds)

	x := bounds.Min.X + 4
	y := bounds.Min.Y + lineHeight
	maxX := bounds.Max.X - 4
	maxChars := (maxX - x) / charWidth

	// Header: day name + date.
	now := cfg.now()
	header := now.Format("Monday, Jan 2")
	if cfg.Title != "" {
		header = cfg.Title
	}
	drawText(frame, x, y, truncateText(header, maxChars))
	y += 4

	// Separator.
	drawHLine(frame, x, maxX, y)
	y += lineHeight

	// Events.
	rendered := 0
	for _, e := range events {
		if rendered >= cfg.MaxEvents {
			break
		}
		if y+lineHeight > bounds.Max.Y-lineHeight {
			break // leave room for footer
		}

		var line string
		if e.AllDay {
			line = fmt.Sprintf("All day: %s", e.Summary)
		} else {
			line = fmt.Sprintf("%s - %s  %s",
				e.Start.Format("15:04"),
				e.End.Format("15:04"),
				e.Summary,
			)
		}
		if cfg.ShowLocation && e.Location != "" {
			line += " @ " + e.Location
		}
		drawText(frame, x, y, truncateText(line, maxChars))
		y += lineHeight
		rendered++
	}

	if rendered == 0 {
		drawText(frame, x, y, "No events today")
		y += lineHeight
	}

	// Footer separator + count.
	drawHLine(frame, x, maxX, y)
	y += lineHeight
	footer := fmt.Sprintf("%d events today", len(events))
	if len(events) == 1 {
		footer = "1 event today"
	}
	drawText(frame, x, y, footer)

	return nil
}
