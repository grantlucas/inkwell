package inkwell

import "github.com/grantlucas/inkwell/internal/inkwell/widget"

// Screen holds a named collection of widgets for one display layout.
type Screen struct {
	Name    string
	widgets []widget.Widget
}

// NewScreen creates a Screen with the given name and widgets.
func NewScreen(name string, widgets []widget.Widget) *Screen {
	return &Screen{Name: name, widgets: widgets}
}

// Widgets returns the screen's widget list.
func (s *Screen) Widgets() []widget.Widget {
	return s.widgets
}
