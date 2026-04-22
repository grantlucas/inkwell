package widgets

import (
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	claude_usage "github.com/grantlucas/inkwell/internal/inkwell/widgets/claude_usage"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/clock"
)

// NewDefaultRegistry creates a Registry pre-loaded with all built-in widgets.
func NewDefaultRegistry() *widget.Registry {
	r := widget.NewRegistry()
	r.Register("clock", clock.Factory)
	r.Register("claude_usage", claude_usage.Factory)
	return r
}
