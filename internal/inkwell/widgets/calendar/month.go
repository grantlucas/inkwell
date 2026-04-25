package calendar

import (
	"fmt"
	"image"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar"
)

// renderMonth draws a traditional month grid calendar.
//
// Layout:
//
//	    April 2026
//	Mo Tu We Th Fr Sa Su
//	       1  2  3  4  5
//	 6  7  8  9 10 11 12
//	...
func renderMonth(frame *image.Paletted, bounds image.Rectangle, events []calendar.Event, cfg Config) error {
	fillWhite(frame, bounds)

	now := cfg.now()
	year, month, _ := now.Date()
	loc := now.Location()
	today := time.Date(year, month, now.Day(), 0, 0, 0, 0, loc)
	firstOfMonth := time.Date(year, month, 1, 0, 0, 0, 0, loc)
	daysInMonth := time.Date(year, month+1, 0, 0, 0, 0, 0, loc).Day()

	x := bounds.Min.X + 4
	maxX := bounds.Max.X - 4
	colWidth := (maxX - x) / 7
	maxChars := (maxX - x) / charWidth

	y := bounds.Min.Y + lineHeight

	// Header: Month Year.
	header := firstOfMonth.Format("January 2006")
	if cfg.Title != "" {
		header = cfg.Title
	}
	drawTextCentered(frame, x, x+7*colWidth, y, header)
	y += lineHeight

	// Day-of-week abbreviations.
	dayNames := weekdayAbbreviations(cfg.WeekStart)
	for i, name := range dayNames {
		cx := x + i*colWidth + (colWidth-2*charWidth)/2
		drawText(frame, cx, y, name)
	}
	y += lineHeight + 2

	// Build set of days with events.
	eventDays := make(map[int]bool)
	for _, e := range events {
		if e.Start.Month() == month && e.Start.Year() == year {
			eventDays[e.Start.Day()] = true
		}
		// Multi-day events: mark each day.
		d := e.Start
		for d.Before(e.End) && d.Month() == month && d.Year() == year {
			eventDays[d.Day()] = true
			d = d.AddDate(0, 0, 1)
		}
	}

	// Calculate column offset for first day.
	firstWeekday := firstOfMonth.Weekday()
	colOffset := (int(firstWeekday) - int(cfg.WeekStart) + 7) % 7

	// Render grid.
	col := colOffset
	for day := 1; day <= daysInMonth; day++ {
		if y+lineHeight > bounds.Max.Y {
			break
		}

		cx := x + col*colWidth
		dayStr := fmt.Sprintf("%2d", day)
		dayDate := time.Date(year, month, day, 0, 0, 0, 0, loc)

		if dayDate.Equal(today) {
			// Inverted cell for today.
			cellRect := image.Rect(cx, y-lineHeight+2, cx+colWidth-1, y+3)
			drawInvertedRect(frame, cellRect)
			drawTextInverted(frame, cx+(colWidth-2*charWidth)/2, y, dayStr)
		} else {
			textX := cx + (colWidth-2*charWidth)/2
			drawText(frame, textX, y, dayStr)
		}

		// Event dot indicator.
		if eventDays[day] && !dayDate.Equal(today) {
			dotX := cx + colWidth/2
			dotY := y + 3
			if dotY < bounds.Max.Y {
				frame.SetColorIndex(dotX, dotY, 1)
				frame.SetColorIndex(dotX-1, dotY, 1)
				frame.SetColorIndex(dotX+1, dotY, 1)
			}
		}

		col++
		if col >= 7 {
			col = 0
			y += lineHeight + 4
		}
	}

	// Footer with event count if space permits.
	if col > 0 {
		y += lineHeight + 4
	}
	if y+lineHeight <= bounds.Max.Y {
		footer := fmt.Sprintf("%d events this month", len(events))
		if len(events) == 1 {
			footer = "1 event this month"
		}
		drawText(frame, x, y, truncateText(footer, maxChars))
	}

	return nil
}

// weekdayAbbreviations returns 2-letter day names starting from the given weekday.
func weekdayAbbreviations(start time.Weekday) [7]string {
	names := [7]string{"Su", "Mo", "Tu", "We", "Th", "Fr", "Sa"}
	var result [7]string
	for i := range 7 {
		result[i] = names[(int(start)+i)%7]
	}
	return result
}
