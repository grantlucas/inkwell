package daygrid

import (
	"fmt"

	"github.com/grantlucas/inkwell/internal/inkwell/widget"
)

// RequireDeps checks that a calendar widget was handed everything it fetches
// through. The app always supplies both, so a missing one is a wiring fault
// to report, not a gap to fill with a default.
func RequireDeps(widgetName string, deps widget.Deps) error {
	if deps.Calendar == nil {
		return fmt.Errorf("%s: no calendar module", widgetName)
	}
	if deps.Weather == nil {
		return fmt.Errorf("%s: no weather provider", widgetName)
	}
	return nil
}
