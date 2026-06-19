package inkwell

import (
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/widget"
)

// Screen holds a named collection of widgets for one display layout.
type Screen struct {
	Name     string
	widgets  []widget.Widget
	schedule refreshSchedule
}

// NewScreen creates a Screen with the given name and widgets. The refresh
// schedule starts empty (no widget is due); buildDashboard populates it from
// each widget's required config cadence.
func NewScreen(name string, widgets []widget.Widget) *Screen {
	return &Screen{Name: name, widgets: widgets}
}

// Widgets returns the screen's widget list.
func (s *Screen) Widgets() []widget.Widget {
	return s.widgets
}

// AnyDue reports whether any widget on the screen is due to refresh at now.
func (s *Screen) AnyDue(now time.Time) bool {
	return s.schedule.anyDue(now)
}
