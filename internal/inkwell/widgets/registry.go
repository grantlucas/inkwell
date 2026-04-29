package widgets

import (
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/clock"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/date"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/weekly"
)

// NewDefaultRegistry creates a Registry pre-loaded with all built-in widgets.
func NewDefaultRegistry() *widget.Registry {
	r := widget.NewRegistry()
	r.Register("clock", clock.Factory)
	r.Register("date", date.Factory)
	r.Register("weekly-calendar", weekly.Factory)
	return r
}
