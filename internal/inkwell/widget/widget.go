package widget

import "image"

// Widget renders content into a sub-region of the display frame.
type Widget interface {
	// Bounds returns the rectangle this widget occupies on the display.
	Bounds() image.Rectangle
	// Render draws the widget's content into the given frame.
	Render(frame *image.Paletted) error
}
