package daydata_test

import (
	"strings"
	"testing"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/weather"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daydata"
)

// listing is a calendar widget that lists events, three by default.
var listing = daydata.Spec{Widget: "test-widget", MaxEvents: 3}

// weatherOnly is a widget that draws the forecast and no events.
var weatherOnly = daydata.Spec{Widget: "test-widget", Reads: daydata.WeatherOnly}

// calendarOnly is a widget that lists events and draws no forecast.
var calendarOnly = daydata.Spec{Widget: "test-widget", MaxEvents: 3, Reads: daydata.CalendarOnly}

// oneDay is a widget that draws a single day, today or one after it.
var oneDay = daydata.Spec{Widget: "test-widget", Reads: daydata.WeatherOnly, OneDay: true}

// topLevel is the dashboard's top-level weather settings.
var topLevel = weather.NewProvider(nil, time.Hour, nil, weather.Settings{
	Location: weather.Location{Latitude: 43.25, Longitude: -79.87},
	TempUnit: "F",
	Model:    weather.ModelGEM,
})

// feedsAnd is the minimum valid config plus one key.
func feedsAnd(k string, v any) map[string]any {
	c := map[string]any{"feeds": []any{feedA}}
	if k != "" {
		c[k] = v
	}
	return c
}

// withFeed is a config whose one feed is feed.
func withFeed(feed any) map[string]any { return map[string]any{"feeds": []any{feed}} }

// withRules is a config whose one feed has rules.
func withRules(rules any) map[string]any {
	return withFeed(map[string]any{"url": feedA, "rules": rules})
}

// withRule is a config whose one feed has one rule.
func withRule(rule map[string]any) map[string]any { return withRules([]any{rule}) }

// Every calendar widget's shared settings are parsed by one parser, so a
// setting is accepted the same way on every widget, and widget-level
// weather settings inherit from the top-level ones.
func TestParseConfig_Accepts(t *testing.T) {
	tests := []struct {
		label   string
		spec    daydata.Spec
		raw     map[string]any
		inherit *weather.Provider
		check   func(*testing.T, daydata.Config)
	}{
		{
			label: "defaults", spec: listing, raw: feedsAnd("", nil), inherit: topLevel,
			check: func(t *testing.T, c daydata.Config) {
				if len(c.Feeds) != 1 || c.Feeds[0].URL != feedA {
					t.Errorf("Feeds = %+v", c.Feeds)
				}
				if c.Refresh != 15*time.Minute || c.MaxEvents != 3 || c.ShowLocation {
					t.Errorf("Refresh, MaxEvents, ShowLocation = %v, %d, %v", c.Refresh, c.MaxEvents, c.ShowLocation)
				}
			},
		},
		{
			label: "the top-level weather settings are inherited", spec: listing, raw: feedsAnd("", nil), inherit: topLevel,
			check: func(t *testing.T, c daydata.Config) {
				w := c.Weather
				if w.Latitude != 43.25 || w.Longitude != -79.87 || w.TempUnit != "F" || w.Model != weather.ModelGEM {
					t.Errorf("Weather = %+v, want the top-level settings", w)
				}
			},
		},
		{
			label: "widget weather settings override the top level", spec: listing, inherit: topLevel,
			raw: map[string]any{
				"feeds": []any{feedA}, "latitude": 0.0, "longitude": 2.5, "temp_unit": "C", "weather_model": "ecmwf",
			},
			check: func(t *testing.T, c daydata.Config) {
				w := c.Weather
				// Zero is a real latitude, so setting it overrides too.
				if w.Latitude != 0 || w.Longitude != 2.5 || w.TempUnit != "C" || w.Model != weather.ModelECMWF {
					t.Errorf("Weather = %+v, want the widget's own", w)
				}
			},
		},
		{
			// A blank unit would draw a bare number, which says less
			// than the wrong scale would.
			label: "Celsius with no top-level settings", spec: listing, raw: feedsAnd("", nil), inherit: nil,
			check: func(t *testing.T, c daydata.Config) {
				if c.Weather.TempUnit != "C" {
					t.Errorf("TempUnit = %q, want C", c.Weather.TempUnit)
				}
			},
		},
		{
			label: "refresh", spec: listing, raw: feedsAnd("refresh", "30m"), inherit: topLevel,
			check: func(t *testing.T, c daydata.Config) {
				if c.Refresh != 30*time.Minute {
					t.Errorf("Refresh = %v", c.Refresh)
				}
			},
		},
		{
			label: "max_events", spec: listing, raw: feedsAnd("max_events", 2), inherit: topLevel,
			check: func(t *testing.T, c daydata.Config) {
				if c.MaxEvents != 2 {
					t.Errorf("MaxEvents = %d", c.MaxEvents)
				}
			},
		},
		{
			label: "show_location", spec: listing, raw: feedsAnd("show_location", true), inherit: topLevel,
			check: func(t *testing.T, c daydata.Config) {
				if !c.ShowLocation {
					t.Error("ShowLocation = false")
				}
			},
		},
		{
			// A bare URL needs no rewriting; the object form carries a
			// name and rules, or just a name to label the feed.
			label: "both forms of feed", spec: listing, inherit: topLevel,
			raw: map[string]any{"feeds": []any{
				feedB,
				map[string]any{"url": feedA, "name": "Team", "rules": []any{
					map[string]any{"match": `^Jane Doe\n`},
					map[string]any{"match": "vs ", "replace": "v "},
					map[string]any{"match": "Tournament", "exclude": true},
				}},
				map[string]any{"url": feedA, "name": "Labelled"},
			}},
			check: func(t *testing.T, c daydata.Config) {
				f := c.Feeds
				if len(f) != 3 {
					t.Fatalf("Feeds = %+v, want 3", f)
				}
				if f[0].URL != feedB || f[0].Name != "" || len(f[0].Rules) != 0 {
					t.Errorf("bare feed = %+v", f[0])
				}
				if f[1].URL != feedA || f[1].Name != "Team" || len(f[1].Rules) != 3 {
					t.Errorf("feed with rules = %+v", f[1])
				}
				if f[2].URL != feedA || f[2].Name != "Labelled" || len(f[2].Rules) != 0 {
					t.Errorf("labelled feed = %+v", f[2])
				}
			},
		},
		{
			label: "a key the widget declares as its own", inherit: topLevel,
			spec: daydata.Spec{Widget: "test-widget", MaxEvents: 3, Extra: []string{"days"}},
			raw:  feedsAnd("days", 7),
			check: func(*testing.T, daydata.Config) {
			},
		},
		{
			// A weather widget reads no calendar, so it has nothing to
			// put in feeds; the weather settings work as they do anywhere.
			label: "a weather-only widget needs no feeds", spec: weatherOnly, inherit: topLevel,
			raw: map[string]any{"latitude": 51.5, "temp_unit": "C"},
			check: func(t *testing.T, c daydata.Config) {
				if len(c.Feeds) != 0 {
					t.Errorf("Feeds = %+v, want none", c.Feeds)
				}
				w := c.Weather
				if w.Latitude != 51.5 || w.Longitude != -79.87 || w.TempUnit != "C" || w.Model != weather.ModelGEM {
					t.Errorf("Weather = %+v, want its own latitude and unit over the top level", w)
				}
			},
		},
		{
			label: "a weather-only widget with no settings at all", spec: weatherOnly, raw: nil, inherit: topLevel,
			check: func(t *testing.T, c daydata.Config) {
				if c.Weather.TempUnit != "F" {
					t.Errorf("TempUnit = %q, want the top level's F", c.Weather.TempUnit)
				}
			},
		},
		{
			label: "a weather-only widget's own key", inherit: topLevel,
			spec: daydata.Spec{Widget: "test-widget", Reads: daydata.WeatherOnly, Extra: []string{"days"}},
			raw:  map[string]any{"days": 4},
			check: func(*testing.T, daydata.Config) {
			},
		},
		{
			// A widget placed for one day draws today unless told which.
			label: "a one-day widget draws today by default", spec: oneDay, raw: nil, inherit: topLevel,
			check: func(t *testing.T, c daydata.Config) {
				if c.Day != 0 {
					t.Errorf("Day = %d, want 0", c.Day)
				}
			},
		},
		{
			label: "a one-day widget's day", spec: oneDay, raw: map[string]any{"day": 6}, inherit: topLevel,
			check: func(t *testing.T, c daydata.Config) {
				if c.Day != 6 {
					t.Errorf("Day = %d, want 6", c.Day)
				}
			},
		},
		{
			// An event list reads no forecast, so it has no weather to set.
			label: "a calendar-only widget", spec: calendarOnly, raw: feedsAnd("max_events", 2), inherit: topLevel,
			check: func(t *testing.T, c daydata.Config) {
				if len(c.Feeds) != 1 || c.MaxEvents != 2 {
					t.Errorf("Feeds, MaxEvents = %+v, %d", c.Feeds, c.MaxEvents)
				}
			},
		},
		{
			// The example config's bold-five screen.
			label: "an existing bold-five config", spec: daydata.Spec{Widget: "bold-five", MaxEvents: 4}, inherit: topLevel,
			raw: map[string]any{
				"feeds": []any{"https://example.com/my-calendar.ics"}, "max_events": 3,
				"show_location": false, "refresh": "15m", "temp_unit": "F", "weather_model": "ecmwf",
			},
			check: func(t *testing.T, c daydata.Config) {
				if c.MaxEvents != 3 || c.Weather.TempUnit != "F" || c.Weather.Model != weather.ModelECMWF {
					t.Errorf("Config = %+v", c)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			cfg, err := daydata.ParseConfig(tt.spec, tt.raw, tt.inherit)
			if err != nil {
				t.Fatalf("ParseConfig: %v", err)
			}
			tt.check(t, cfg)
		})
	}
}

// Every rejection names the widget and says what was wrong, the same way
// on every calendar widget. A key the parser doesn't know, including a
// misspelling, is rejected rather than ignored.
func TestParseConfig_Rejects(t *testing.T) {
	const accepted = "(accepted: feeds, latitude, longitude, max_events, refresh, show_location, temp_unit, weather_model)"
	noMaxEvents := daydata.Spec{Widget: "test-widget"}
	explains := daydata.Spec{Widget: "test-widget", MaxEvents: 3, Rejected: map[string]string{
		"days": "always five columns",
	}}

	tests := []struct {
		label string
		spec  daydata.Spec
		raw   map[string]any
		want  string
	}{
		{"feeds missing", listing, map[string]any{}, "feeds is required"},
		{"feeds not a list", listing, map[string]any{"feeds": feedA}, "feeds must be a list"},
		{"feeds empty", listing, map[string]any{"feeds": []any{}}, "feeds must not be empty"},
		{"a feed neither a URL nor an object", listing, withFeed(123), "feeds[0] must be a URL string or a feed object"},
		{"a feed object without a url", listing, withFeed(map[string]any{"name": "nameless"}), "feeds[0]: url is required"},
		{"a feed url not a string", listing, withFeed(map[string]any{"url": 42}), "feeds[0]: url must be a string"},
		{"a feed name not a string", listing, withFeed(map[string]any{"url": feedA, "name": 42}), "feeds[0]: name must be a string"},
		{"rules not a list", listing, withRules("nope"), "feeds[0]: rules must be a list"},
		{"a rule not an object", listing, withRules([]any{"nope"}), "feeds[0]: rules[0] must be an object"},
		{"a rule without a match", listing, withRule(map[string]any{"replace": "x"}), "rules[0]: match is required"},
		{"a match not a string", listing, withRule(map[string]any{"match": 42}), "rules[0]: match must be a string"},
		{"a replace not a string", listing, withRule(map[string]any{"match": "x", "replace": 42}), "rules[0]: replace must be a string"},
		{"an exclude not a bool", listing, withRule(map[string]any{"match": "x", "exclude": "yes"}), "rules[0]: exclude must be a bool"},
		{"an invalid match", listing, withRule(map[string]any{"match": "(unclosed"}), "rules[0]: invalid match"},
		{"replace and exclude together", listing, withRule(map[string]any{"match": "x", "replace": "y", "exclude": true}), "both replace and exclude"},
		// A bare index into a list of long URLs tells an operator
		// nothing, so a named feed is called by its name.
		{"a rule error names its feed", listing, withFeed(map[string]any{
			"url": feedA, "name": "Team calendar", "rules": []any{map[string]any{"match": "(unclosed"}},
		}), "feeds[0] (Team calendar): rules[0]: invalid match"},
		{"refresh not a string", listing, feedsAnd("refresh", 5), "refresh must be a string"},
		{"refresh unparseable", listing, feedsAnd("refresh", "soon"), "invalid refresh"},
		{"refresh under a minute", listing, feedsAnd("refresh", "30s"), "refresh must be >= 1m"},
		{"max_events not an int", listing, feedsAnd("max_events", "four"), "max_events must be an integer"},
		{"max_events zero", listing, feedsAnd("max_events", 0), "max_events must be positive"},
		{"max_events negative", listing, feedsAnd("max_events", -1), "max_events must be positive"},
		{"show_location not a bool", listing, feedsAnd("show_location", "yes"), "show_location must be a bool"},
		{"latitude not a number", listing, feedsAnd("latitude", "45"), "latitude must be a number"},
		{"latitude too high", listing, feedsAnd("latitude", 91.0), "latitude must be in [-90, 90]"},
		{"latitude too low", listing, feedsAnd("latitude", -91.0), "latitude must be in [-90, 90]"},
		{"longitude not a number", listing, feedsAnd("longitude", "-75"), "longitude must be a number"},
		{"longitude too high", listing, feedsAnd("longitude", 181.0), "longitude must be in [-180, 180]"},
		{"longitude too low", listing, feedsAnd("longitude", -181.0), "longitude must be in [-180, 180]"},
		{"temp_unit not a string", listing, feedsAnd("temp_unit", 1), "temp_unit must be a string"},
		{"temp_unit unknown", listing, feedsAnd("temp_unit", "K"), "invalid temp_unit"},
		{"weather_model not a string", listing, feedsAnd("weather_model", 1), "weather_model must be a string"},
		{"weather_model unknown", listing, feedsAnd("weather_model", "nope"), "invalid weather_model"},
		{"a misspelt key", listing, feedsAnd("max_event", 3), `unsupported setting "max_event" ` + accepted},
		{"another widget's key", listing, feedsAnd("days", 7), `unsupported setting "days" ` + accepted},
		// A key the widget knows another calendar widget takes is
		// rejected with the widget's reason; anything else still gets
		// the accepted list.
		{"another widget's key, explained", explains, feedsAnd("days", 7), "days is not supported: always five columns"},
		{"a typo beside an explained key", explains, feedsAnd("max_event", 3), `unsupported setting "max_event" ` + accepted},
		// A weather-only widget reads no calendar, so a calendar key
		// pasted from a calendar widget's config says why it is wrong,
		// and a typo lists only the weather settings.
		{"feeds on a weather-only widget", weatherOnly, feedsAnd("", nil), "feeds is not supported: test-widget shows only the weather, so it reads no calendar"},
		{"refresh on a weather-only widget", weatherOnly, map[string]any{"refresh": "15m"}, "refresh is not supported: test-widget shows only the weather"},
		{"show_location on a weather-only widget", weatherOnly, map[string]any{"show_location": true}, "show_location is not supported: test-widget shows only the weather"},
		{"max_events on a weather-only widget", weatherOnly, map[string]any{"max_events": 3}, "max_events is not supported: test-widget shows only the weather"},
		{
			"a typo on a weather-only widget", weatherOnly, map[string]any{"latitde": 43.0},
			`unsupported setting "latitde" (accepted: latitude, longitude, temp_unit, weather_model)`,
		},
		{"a weather setting on a weather-only widget is still checked", weatherOnly, map[string]any{"temp_unit": "K"}, "invalid temp_unit"},
		// A widget placed for one day takes a day from today to a week
		// out; a widget drawing several days has no single day to set.
		{"day not an int", oneDay, map[string]any{"day": "monday"}, "day must be an integer, got string"},
		{"day before today", oneDay, map[string]any{"day": -1}, "day must be in [0, 6], got -1"},
		{"day past a week out", oneDay, map[string]any{"day": 7}, "day must be in [0, 6], got 7"},
		{
			"day on a widget that draws several days", weatherOnly, map[string]any{"day": 1},
			`unsupported setting "day" (accepted: latitude, longitude, temp_unit, weather_model)`,
		},
		// A calendar-only widget reads no forecast, so a weather key
		// pasted from another widget's config says why it is wrong.
		{"latitude on a calendar-only widget", calendarOnly, feedsAnd("latitude", 43.0), "latitude is not supported: test-widget shows only events, so it reads no forecast"},
		{"weather_model on a calendar-only widget", calendarOnly, feedsAnd("weather_model", "gem"), "weather_model is not supported: test-widget shows only events"},
		{
			"a typo on a calendar-only widget", calendarOnly, feedsAnd("feed", feedA),
			`unsupported setting "feed" (accepted: feeds, max_events, refresh, show_location)`,
		},
		{
			"max_events on a widget that doesn't list a fixed number", noMaxEvents, feedsAnd("max_events", 3),
			`unsupported setting "max_events" (accepted: feeds, latitude, longitude, refresh, show_location, temp_unit, weather_model)`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			_, err := daydata.ParseConfig(tt.spec, tt.raw, topLevel)
			if err == nil {
				t.Fatal("expected an error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %q, want it to mention %q", err, tt.want)
			}
			if !strings.HasPrefix(err.Error(), "test-widget: ") {
				t.Errorf("error = %q, want it to name the widget", err)
			}
		})
	}
}
