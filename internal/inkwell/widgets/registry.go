package widgets

import (
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/boldfive"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/clock"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/combinedchart"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/date"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daybadge"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daytimeline"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/eventlist"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/fuzzyclock"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/rowagenda"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/separator"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/todayhero"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/todayweather"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/weatherahead"
)

// NewDefaultRegistry creates a Registry pre-loaded with all built-in widgets.
func NewDefaultRegistry() *widget.Registry {
	r := widget.NewRegistry()
	r.Register("bold-five", boldfive.Factory)
	r.Register("clock", clock.Factory)
	r.Register("combined-chart", combinedchart.Factory)
	r.Register("date", date.Factory)
	r.Register("day-badge", daybadge.Factory)
	r.Register("day-timeline", daytimeline.Factory)
	r.Register("event-list", eventlist.Factory)
	r.Register("fuzzy_clock", fuzzyclock.Factory)
	r.Register("row-agenda", rowagenda.Factory)
	r.Register("separator", separator.Factory)
	r.Register("today-hero", todayhero.Factory)
	r.Register("today-weather", todayweather.Factory)
	r.Register("weather-ahead", weatherahead.Factory)
	return r
}
