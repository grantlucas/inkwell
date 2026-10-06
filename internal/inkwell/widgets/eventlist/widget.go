package eventlist

import (
	"image"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/weather"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daydata"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/drawkit"
)

// widgetName prefixes every config error so a dashboard that fails to
// load says which widget rejected it.
const widgetName = "event-list"

// defaultMaxEvents is how many events each preset lists when config
// doesn't say: what the full-screen widget it comes from shows.
var defaultMaxEvents = map[Preset]int{
	PresetStacked: 4, // bold-five's column
	PresetLarge:   3, // today-hero's agenda
	PresetInline:  3, // today-hero's day rows
}

var _ widget.Widget = (*Widget)(nil)

// Config is the event-list widget's parsed configuration: the calendar
// settings and day every one-day widget shares, and its own.
type Config struct {
	daydata.Config
	// Preset is the shape the events are listed in.
	Preset Preset
	// HideFinished drops events that finished before now, for a list of
	// what is left of today.
	HideFinished bool
	// Empty, when set, replaces what the preset says on a day with no
	// events. "" says nothing.
	Empty *string
}

// Widget is the event list placed on a screen on its own: one day's
// events, listed into its bounds in one of the presets the full-screen
// widgets list in.
type Widget struct {
	daydata.Base
	// Config is the widget's parsed settings: the shared ones Base holds
	// as well, and its own.
	Config Config
}

// New creates an event-list Widget listing cfg.Day's events from days.
func New(bounds image.Rectangle, days daydata.Source, now func() time.Time, cfg Config) *Widget {
	return &Widget{Base: daydata.NewBase(bounds, days, now, cfg.Config), Config: cfg}
}

// Render lists the day's events into the widget's bounds. The list keeps
// to the rectangle it is given, fitting whole events and announcing the
// rest, so there is no size too small to draw into: a short list hides
// more and says so. A failed fetch is logged by the day data module and
// never returned, so it lists what arrived.
func (w *Widget) Render(frame *image.Paletted) error {
	drawkit.FillWhite(frame, w.Bounds())

	now := w.Now()
	cfg := w.Config
	events := w.Days.Days(now, cfg.Day+1).Days[cfg.Day].Events
	if cfg.HideFinished {
		events = Remaining(events, now)
	}
	style := cfg.Preset.Style(cfg.MaxEvents, cfg.ShowLocation, now.Location())
	if cfg.Empty != nil {
		style.Empty.Text = *cfg.Empty
	}
	style.Draw(frame, w.Bounds(), events)
	return nil
}

// ownKeys are the widget's own settings, beside the shared ones.
var ownKeys = []string{"empty", "hide_finished", "style"}

// parseConfig reads the widget's preset first, since the default number
// of events depends on it, then the shared settings through the shared
// parser, then the rest of its own. It draws no forecast, so it has no
// weather settings to inherit.
func parseConfig(raw map[string]any, _ *weather.Provider) (Config, error) {
	cfg := Config{Preset: PresetStacked}
	keys := daydata.ReadKeys(widgetName, raw)
	daydata.Choice(keys, "style", PresetNames, &cfg.Preset)
	if err := keys.Err(); err != nil {
		return cfg, err
	}

	shared, err := daydata.ParseConfig(daydata.Spec{
		Widget:       widgetName,
		MaxEvents:    defaultMaxEvents[cfg.Preset],
		Extra:        ownKeys,
		CalendarOnly: true,
		OneDay:       true,
	}, raw, nil)
	if err != nil {
		return cfg, err
	}
	cfg.Config = shared

	keys.Bool("hide_finished", &cfg.HideFinished)
	var empty string
	if keys.String("empty", &empty) {
		cfg.Empty = &empty
	}
	return cfg, keys.Err()
}

// Factory creates an event-list Widget from config and dependencies. Its
// day data reads the calendar only: it draws no forecast, so it fetches
// none.
var Factory = daydata.Factory(widgetName, parseConfig, New, daydata.WithoutWeather())
