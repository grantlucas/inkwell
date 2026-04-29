package weekly

import (
	"context"
	"fmt"
	"image"
	"net/http"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar"
	"github.com/grantlucas/inkwell/internal/inkwell/weather"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/weatherview"
)

var _ widget.Widget = (*Widget)(nil)

const defaultWeatherH = 120

// Config holds parsed weekly-calendar configuration.
type Config struct {
	Feeds            []string
	Refresh          time.Duration
	WeekStart        time.Weekday
	MaxEvents        int
	ShowLocation     bool
	Latitude         float64
	Longitude        float64
	ShowWeather      bool
	ShowWeatherLabel bool
	TempUnit         string
	HighlightHour    int
}

// Widget renders a 7-day calendar+weather dashboard.
type Widget struct {
	bounds  image.Rectangle
	cal     calendar.Source
	weather weather.Source
	now     func() time.Time
	config  Config
}

// New creates a weekly Widget with pre-built data sources.
func New(bounds image.Rectangle, cal calendar.Source, ws weather.Source, now func() time.Time, cfg Config) *Widget {
	return &Widget{
		bounds:  bounds,
		cal:     cal,
		weather: ws,
		now:     now,
		config:  cfg,
	}
}

// Bounds returns the rectangle this widget occupies on the display.
func (w *Widget) Bounds() image.Rectangle { return w.bounds }

// Render draws the 7-day calendar+weather dashboard into frame.
func (w *Widget) Render(frame *image.Paletted) error {
	fillWhite(frame, w.bounds)

	now := w.now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	offset := (int(today.Weekday()) - int(w.config.WeekStart) + 7) % 7
	weekStart := today.AddDate(0, 0, -offset)
	weekEnd := weekStart.AddDate(0, 0, 7)

	events, _ := w.cal.Events(weekStart, weekEnd)

	var forecast *weather.Forecast
	weatherH := 0
	if w.config.ShowWeather && w.weather != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		loc := weather.Location{
			Latitude:  w.config.Latitude,
			Longitude: w.config.Longitude,
		}
		f, err := w.weather.Forecast(ctx, loc, 7)
		if err == nil && f != nil {
			forecast = f
			weatherH = defaultWeatherH
		}
	}

	cols := computeColumns(w.bounds, weatherH)

	var forecastDays []weather.DailyForecast
	if forecast != nil {
		forecastDays = forecast.Days
	}
	globalMin, globalMax := weatherview.GlobalTempRange(forecastDays)

	for i, col := range cols {
		day := weekStart.AddDate(0, 0, i)
		dayEnd := day.AddDate(0, 0, 1)
		isToday := day.Equal(today)

		renderDayHeader(frame, col.Header, day, isToday)

		if weatherH > 0 {
			dayForecast := findForecast(forecastDays, day)
			opts := weatherview.Options{
				TempUnit:      w.config.TempUnit,
				ShowLabel:     w.config.ShowWeatherLabel,
				GlobalTempMin: globalMin,
				GlobalTempMax: globalMax,
				HighlightHour: w.config.HighlightHour,
			}
			weatherview.RenderDayWeather(frame, col.Weather, dayForecast, opts)
		}

		dayEvents := filterEventsForDay(events, day, dayEnd)
		renderEvents(frame, col.Events, dayEvents, w.config.MaxEvents, w.config.ShowLocation)

		if !col.IsLast {
			drawVLine(frame, col.Bounds.Max.X-1, w.bounds.Min.Y, w.bounds.Max.Y)
		}
	}

	return nil
}

// findForecast returns the DailyForecast matching the given day, or a zero
// value if not found.
func findForecast(days []weather.DailyForecast, day time.Time) weather.DailyForecast {
	for _, d := range days {
		if d.Date.Year() == day.Year() && d.Date.YearDay() == day.YearDay() {
			return d
		}
	}
	return weather.DailyForecast{}
}

// Factory creates a weekly-calendar Widget from config and dependencies.
func Factory(bounds image.Rectangle, config map[string]any, deps widget.Deps) (widget.Widget, error) {
	cfg, err := parseConfig(config)
	if err != nil {
		return nil, err
	}

	now := deps.Now
	if now == nil {
		now = time.Now
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

	calSource := calendar.NewHTTPSource(cfg.Feeds, httpClient)
	cachedCal := calendar.NewCachedSource(calSource, cfg.Refresh, now)

	var ws weather.Source
	if cfg.ShowWeather {
		if src, ok := deps.DataSources["weather_source"].(weather.Source); ok {
			ws = src
		}
	}

	return New(bounds, cachedCal, ws, now, cfg), nil
}

// parseConfig validates and extracts config values.
func parseConfig(config map[string]any) (Config, error) {
	cfg := Config{
		Refresh:          15 * time.Minute,
		WeekStart:        time.Monday,
		MaxEvents:        5,
		ShowWeather:      true,
		ShowWeatherLabel: true,
		TempUnit:         "C",
		HighlightHour:    15,
	}

	f, ok := config["feeds"]
	if !ok {
		return cfg, fmt.Errorf("weekly-calendar: feeds is required")
	}
	feedList, ok := f.([]any)
	if !ok {
		return cfg, fmt.Errorf("weekly-calendar: feeds must be a list, got %T", f)
	}
	if len(feedList) == 0 {
		return cfg, fmt.Errorf("weekly-calendar: feeds must not be empty")
	}
	for i, item := range feedList {
		s, ok := item.(string)
		if !ok {
			return cfg, fmt.Errorf("weekly-calendar: feeds[%d] must be a string, got %T", i, item)
		}
		cfg.Feeds = append(cfg.Feeds, s)
	}

	if v, ok := config["refresh"]; ok {
		s, ok := v.(string)
		if !ok {
			return cfg, fmt.Errorf("weekly-calendar: refresh must be a string, got %T", v)
		}
		d, err := time.ParseDuration(s)
		if err != nil {
			return cfg, fmt.Errorf("weekly-calendar: invalid refresh %q: %w", s, err)
		}
		if d < time.Minute {
			return cfg, fmt.Errorf("weekly-calendar: refresh must be >= 1m, got %v", d)
		}
		cfg.Refresh = d
	}

	if v, ok := config["week_start"]; ok {
		s, ok := v.(string)
		if !ok {
			return cfg, fmt.Errorf("weekly-calendar: week_start must be a string, got %T", v)
		}
		switch s {
		case "monday":
			cfg.WeekStart = time.Monday
		case "sunday":
			cfg.WeekStart = time.Sunday
		default:
			return cfg, fmt.Errorf("weekly-calendar: invalid week_start %q (must be monday or sunday)", s)
		}
	}

	if v, ok := config["max_events"]; ok {
		n, ok := v.(int)
		if !ok {
			return cfg, fmt.Errorf("weekly-calendar: max_events must be an integer, got %T", v)
		}
		if n <= 0 {
			return cfg, fmt.Errorf("weekly-calendar: max_events must be positive, got %d", n)
		}
		cfg.MaxEvents = n
	}

	if v, ok := config["show_location"]; ok {
		b, ok := v.(bool)
		if !ok {
			return cfg, fmt.Errorf("weekly-calendar: show_location must be a bool, got %T", v)
		}
		cfg.ShowLocation = b
	}

	if v, ok := config["latitude"]; ok {
		f, ok := v.(float64)
		if !ok {
			return cfg, fmt.Errorf("weekly-calendar: latitude must be a number, got %T", v)
		}
		cfg.Latitude = f
	}

	if v, ok := config["longitude"]; ok {
		f, ok := v.(float64)
		if !ok {
			return cfg, fmt.Errorf("weekly-calendar: longitude must be a number, got %T", v)
		}
		cfg.Longitude = f
	}

	if v, ok := config["show_weather"]; ok {
		b, ok := v.(bool)
		if !ok {
			return cfg, fmt.Errorf("weekly-calendar: show_weather must be a bool, got %T", v)
		}
		cfg.ShowWeather = b
	}

	if v, ok := config["show_weather_label"]; ok {
		b, ok := v.(bool)
		if !ok {
			return cfg, fmt.Errorf("weekly-calendar: show_weather_label must be a bool, got %T", v)
		}
		cfg.ShowWeatherLabel = b
	}

	if v, ok := config["temp_unit"]; ok {
		s, ok := v.(string)
		if !ok {
			return cfg, fmt.Errorf("weekly-calendar: temp_unit must be a string, got %T", v)
		}
		switch s {
		case "C", "F":
			cfg.TempUnit = s
		default:
			return cfg, fmt.Errorf("weekly-calendar: invalid temp_unit %q (must be C or F)", s)
		}
	}

	if v, ok := config["highlight_hour"]; ok {
		n, ok := v.(int)
		if !ok {
			return cfg, fmt.Errorf("weekly-calendar: highlight_hour must be an integer, got %T", v)
		}
		cfg.HighlightHour = n
	}

	return cfg, nil
}
