// Package calendar implements a calendar widget that renders iCal
// subscription feeds in various view modes for e-ink displays.
package calendar

import (
	"fmt"
	"image"
	"net/http"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
)

// ViewMode defines how the calendar is rendered.
type ViewMode string

const (
	ViewMonth    ViewMode = "month"
	ViewWeek     ViewMode = "week"
	ViewToday    ViewMode = "today"
	ViewUpcoming ViewMode = "upcoming"
)

// Compile-time interface check.
var _ widget.Widget = (*Widget)(nil)

// Config holds parsed and validated calendar widget configuration.
type Config struct {
	View         ViewMode
	Feeds        []string
	Refresh      time.Duration
	WeekStart    time.Weekday
	MaxEvents    int
	ShowLocation bool
	Title        string
}

// Widget renders calendar events from iCal subscriptions.
type Widget struct {
	bounds image.Rectangle
	source calendar.Source
	now    func() time.Time
	config Config
}

// Bounds returns the rectangle this widget occupies on the display.
func (w *Widget) Bounds() image.Rectangle { return w.bounds }

// Render draws calendar events into the frame. The specific layout depends
// on the configured view mode.
func (w *Widget) Render(frame *image.Paletted) error {
	// TODO: delegate to view-specific renderers in Phase 3.
	return nil
}

// Factory creates a calendar Widget from config and dependencies.
func Factory(bounds image.Rectangle, config map[string]any, deps widget.Deps) (widget.Widget, error) {
	cfg, err := parseConfig(config)
	if err != nil {
		return nil, err
	}

	var httpClient calendar.HTTPClient
	if deps.DataSources != nil {
		if c, ok := deps.DataSources["http_client"].(calendar.HTTPClient); ok {
			httpClient = c
		}
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	now := deps.Now
	if now == nil {
		now = time.Now
	}

	httpSource := calendar.NewHTTPSource(cfg.Feeds, httpClient)
	cachedSource := calendar.NewCachedSource(httpSource, cfg.Refresh, now)

	return &Widget{
		bounds: bounds,
		source: cachedSource,
		now:    now,
		config: cfg,
	}, nil
}

// parseConfig validates and extracts configuration from the raw config map.
func parseConfig(config map[string]any) (Config, error) {
	cfg := Config{
		Refresh:   15 * time.Minute,
		WeekStart: time.Monday,
		MaxEvents: 10,
	}

	// view (required)
	v, ok := config["view"]
	if !ok {
		return cfg, fmt.Errorf("calendar: view is required")
	}
	viewStr, ok := v.(string)
	if !ok {
		return cfg, fmt.Errorf("calendar: view must be a string, got %T", v)
	}
	switch ViewMode(viewStr) {
	case ViewMonth, ViewWeek, ViewToday, ViewUpcoming:
		cfg.View = ViewMode(viewStr)
	default:
		return cfg, fmt.Errorf("calendar: invalid view %q (must be month, week, today, or upcoming)", viewStr)
	}

	// feeds (required)
	f, ok := config["feeds"]
	if !ok {
		return cfg, fmt.Errorf("calendar: feeds is required")
	}
	feedList, ok := f.([]any)
	if !ok {
		return cfg, fmt.Errorf("calendar: feeds must be a list, got %T", f)
	}
	if len(feedList) == 0 {
		return cfg, fmt.Errorf("calendar: feeds must not be empty")
	}
	for i, item := range feedList {
		s, ok := item.(string)
		if !ok {
			return cfg, fmt.Errorf("calendar: feeds[%d] must be a string, got %T", i, item)
		}
		cfg.Feeds = append(cfg.Feeds, s)
	}

	// refresh (optional)
	if v, ok := config["refresh"]; ok {
		s, ok := v.(string)
		if !ok {
			return cfg, fmt.Errorf("calendar: refresh must be a string, got %T", v)
		}
		d, err := time.ParseDuration(s)
		if err != nil {
			return cfg, fmt.Errorf("calendar: invalid refresh %q: %w", s, err)
		}
		if d < time.Minute {
			return cfg, fmt.Errorf("calendar: refresh must be >= 1m, got %v", d)
		}
		cfg.Refresh = d
	}

	// week_start (optional)
	if v, ok := config["week_start"]; ok {
		s, ok := v.(string)
		if !ok {
			return cfg, fmt.Errorf("calendar: week_start must be a string, got %T", v)
		}
		switch s {
		case "monday":
			cfg.WeekStart = time.Monday
		case "sunday":
			cfg.WeekStart = time.Sunday
		default:
			return cfg, fmt.Errorf("calendar: invalid week_start %q (must be monday or sunday)", s)
		}
	}

	// max_events (optional)
	if v, ok := config["max_events"]; ok {
		n, ok := v.(int)
		if !ok {
			return cfg, fmt.Errorf("calendar: max_events must be an integer, got %T", v)
		}
		if n <= 0 {
			return cfg, fmt.Errorf("calendar: max_events must be positive, got %d", n)
		}
		cfg.MaxEvents = n
	}

	// title (optional)
	if v, ok := config["title"]; ok {
		s, ok := v.(string)
		if !ok {
			return cfg, fmt.Errorf("calendar: title must be a string, got %T", v)
		}
		cfg.Title = s
	}

	// show_location (optional)
	if v, ok := config["show_location"]; ok {
		b, ok := v.(bool)
		if !ok {
			return cfg, fmt.Errorf("calendar: show_location must be a bool, got %T", v)
		}
		cfg.ShowLocation = b
	}

	return cfg, nil
}
