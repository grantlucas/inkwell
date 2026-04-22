package widget

import (
	"context"
	"image"
	"time"
)

// Widget renders content into a sub-region of the display frame.
type Widget interface {
	// Bounds returns the rectangle this widget occupies on the display.
	Bounds() image.Rectangle
	// Render draws the widget's content into the given frame.
	Render(frame *image.Paletted) error
}

// CalendarEvent represents a single calendar event.
type CalendarEvent struct {
	Title  string
	Start  time.Time
	End    time.Time
	AllDay bool
}

// CalendarSource provides upcoming calendar events.
type CalendarSource interface {
	Events(ctx context.Context, start, end time.Time) ([]CalendarEvent, error)
}

// UsageSnapshot captures Claude API rate-limit utilization at a point in time.
type UsageSnapshot struct {
	FiveHourUtilization float64
	FiveHourResetsAt    time.Time
	SevenDayUtilization float64
	SevenDayResetsAt    time.Time
}

// UsageSource provides Claude API usage data.
type UsageSource interface {
	Usage(ctx context.Context) (UsageSnapshot, error)
}
