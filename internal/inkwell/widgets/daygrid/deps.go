package daygrid

import (
	"fmt"

	"github.com/grantlucas/inkwell/internal/inkwell/widget"
)

// requireDeps checks that a calendar widget was handed everything it fetches
// through and the clock that decides its Today. The app always supplies all
// three, so a missing one is a wiring fault to report, not a gap to fill with
// a default: a widget quietly reading the wall clock would disagree with the
// rest of the panel about the display zone.
func requireDeps(widgetName string, deps widget.Deps) error {
	if deps.Calendar == nil {
		return fmt.Errorf("%s: no calendar module", widgetName)
	}
	if deps.Weather == nil {
		return fmt.Errorf("%s: no weather provider", widgetName)
	}
	if deps.Now == nil {
		return fmt.Errorf("%s: no clock", widgetName)
	}
	return nil
}
