package calendar

import (
	"fmt"
	"image"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar"
)

// renderWeek draws a 7-day week view with events listed per day.
//
// Layout:
//
//	Week of Apr 20, 2026
//	──────────────────────
//	Mon 20
//	  9:00 Team Standup
//	 14:00 Design Review
//	Tue 21
//	  All day: Holiday
func renderWeek(frame *image.Paletted, bounds image.Rectangle, events []calendar.Event, cfg Config) error {
	fillWhite(frame, bounds)

	x := bounds.Min.X + 4
	y := bounds.Min.Y + lineHeight
	maxX := bounds.Max.X - 4
	maxChars := (maxX - x) / charWidth
	now := cfg.now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	// Calculate week start.
	offset := (int(today.Weekday()) - int(cfg.WeekStart) + 7) % 7
	weekStart := today.AddDate(0, 0, -offset)

	// Header.
	header := fmt.Sprintf("Week of %s", weekStart.Format("Jan 2, 2006"))
	if cfg.Title != "" {
		header = cfg.Title
	}
	drawText(frame, x, y, truncateText(header, maxChars))
	y += 4
	drawHLine(frame, x, maxX, y)
	y += lineHeight

	// Render each day.
	for dayOffset := range 7 {
		day := weekStart.AddDate(0, 0, dayOffset)
		dayEnd := day.AddDate(0, 0, 1)

		if y+lineHeight > bounds.Max.Y {
			break
		}

		// Day header.
		dayHeader := day.Format("Mon 2")
		isToday := day.Equal(today)
		if isToday {
			dayHeader += " (today)"
		}
		drawText(frame, x, y, truncateText(dayHeader, maxChars))
		y += lineHeight

		// Events for this day.
		dayEvents := filterEventsForDay(events, day, dayEnd)
		rendered := 0
		for _, e := range dayEvents {
			if rendered >= cfg.MaxEvents {
				break
			}
			if y+lineHeight > bounds.Max.Y {
				break
			}

			var line string
			if e.AllDay {
				line = fmt.Sprintf("  All day: %s", e.Summary)
			} else {
				line = fmt.Sprintf("  %s %s", e.Start.Format("15:04"), e.Summary)
			}
			if cfg.ShowLocation && e.Location != "" {
				line += " @ " + e.Location
			}
			drawText(frame, x, y, truncateText(line, maxChars))
			y += lineHeight
			rendered++
		}

		if rendered == 0 && y+lineHeight <= bounds.Max.Y {
			drawText(frame, x, y, "  (no events)")
			y += lineHeight
		}
	}

	return nil
}
