package widgets

import (
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	calendarwidget "github.com/grantlucas/inkwell/internal/inkwell/widgets/calendar"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/clock"
)

// NewDefaultRegistry creates a Registry pre-loaded with all built-in widgets.
func NewDefaultRegistry() *widget.Registry {
	r := widget.NewRegistry()
	r.Register("clock", clock.Factory)
	r.Register("calendar", calendarwidget.Factory)
	return r
}
