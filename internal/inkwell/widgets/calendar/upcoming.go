package calendar

import (
	"fmt"
	"image"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar"
)

// renderUpcoming draws events for today + next 2 days.
//
// Layout:
//
//	Today - Thu Apr 25
//	  9:00 Team Standup
//	 14:00 Sprint Planning
//
//	Tomorrow - Fri Apr 26
//	  (no events)
//
//	Sat Apr 27
//	 10:00 Farmers Market
func renderUpcoming(frame *image.Paletted, bounds image.Rectangle, events []calendar.Event, cfg Config) error {
	fillWhite(frame, bounds)

	x := bounds.Min.X + 4
	y := bounds.Min.Y + lineHeight
	maxX := bounds.Max.X - 4
	maxChars := (maxX - x) / charWidth
	now := cfg.now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	for dayOffset := range 3 {
		day := today.AddDate(0, 0, dayOffset)
		dayEnd := day.AddDate(0, 0, 1)

		if y+lineHeight > bounds.Max.Y {
			break
		}

		// Day header.
		var header string
		switch dayOffset {
		case 0:
			header = fmt.Sprintf("Today - %s", day.Format("Mon Jan 2"))
		case 1:
			header = fmt.Sprintf("Tomorrow - %s", day.Format("Mon Jan 2"))
		default:
			header = day.Format("Mon Jan 2")
		}
		if cfg.Title != "" && dayOffset == 0 {
			header = cfg.Title
		}
		drawText(frame, x, y, truncateText(header, maxChars))
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

		if rendered == 0 {
			drawText(frame, x, y, "  (no events)")
			y += lineHeight
		}

		// Blank line between days.
		y += lineHeight / 2
	}

	return nil
}

// filterEventsForDay returns events overlapping [dayStart, dayEnd).
func filterEventsForDay(events []calendar.Event, dayStart, dayEnd time.Time) []calendar.Event {
	var filtered []calendar.Event
	for _, e := range events {
		if e.Start.Before(dayEnd) && e.End.After(dayStart) {
			filtered = append(filtered, e)
		}
	}
	return filtered
}
