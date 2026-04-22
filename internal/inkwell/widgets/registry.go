package widgets

import (
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/claudeusage"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/clock"
)

// NewDefaultRegistry creates a Registry pre-loaded with all built-in widgets.
func NewDefaultRegistry() *widget.Registry {
	r := widget.NewRegistry()
	r.Register("clock", clock.Factory)
	r.Register("claude_usage", claudeusage.Factory)
	return r
}
