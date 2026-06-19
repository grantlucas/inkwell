package widget

import (
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

// RefreshCadence is an optional interface a Widget may implement to declare how
// often its visible content can change. The render loop aligns refreshes to
// wall-clock boundaries of this period and coalesces widgets that fall due in
// the same minute.
//
// A returned value below one minute is clamped up to one minute (the hard
// refresh floor); a value <= 0 marks the widget as static — it never triggers a
// refresh on its own. Widgets that do not implement this interface are treated
// as refreshing every minute.
type RefreshCadence interface {
	RefreshEvery() time.Duration
}
