// Package daygrid holds the groundwork every calendar-plus-weather
// screen needs: parsing the feed list and its per-feed rewrite rules,
// resolving weather defaults against the shared provider, bucketing
// events into days, and finding the forecast for a day.
//
// It exists because that groundwork is roughly 300 lines per widget and
// the screens differ only in how they draw. The part that really must
// not be copied is the all-day bucketing in FilterEventsForDay: an iCal
// VALUE=DATE is anchored to UTC midnight by the parser while day
// columns are built in the viewer's local zone, so comparing them as
// instants leaks an all-day event into the previous local day in any
// negative-UTC zone. Independent copies of that would drift, and the
// failure is silent and off by one day.
//
// Every error message takes a widget name, so a dashboard that fails to
// load still says which widget rejected the config.
package daygrid

import (
	"fmt"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar"
)

// ParseFeeds reads the `feeds` list, where each entry is either a bare
// URL string or an object carrying that feed's rewrite rules. Both
// forms coexist so a dashboard that needs no rewriting keeps the
// one-line form it has always had.
func ParseFeeds(widgetName string, raw any) ([]calendar.Feed, error) {
	list, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("%s: feeds must be a list, got %T", widgetName, raw)
	}
	if len(list) == 0 {
		return nil, fmt.Errorf("%s: feeds must not be empty", widgetName) //nolint:goerr113 // config validation message
	}

	feeds := make([]calendar.Feed, 0, len(list))
	for i, item := range list {
		switch v := item.(type) {
		case string:
			feeds = append(feeds, calendar.Feed{URL: v})
		case map[string]any:
			feed, err := parseFeedObject(widgetName, v, i)
			if err != nil {
				return nil, err
			}
			feeds = append(feeds, feed)
		default:
			return nil, fmt.Errorf("%s: feeds[%d] must be a URL string or a feed object, got %T", widgetName, i, item)
		}
	}
	return feeds, nil
}

// parseFeedObject reads the object form of a feed entry. idx identifies
// the entry in errors until a name is known.
func parseFeedObject(widgetName string, m map[string]any, idx int) (calendar.Feed, error) {
	var feed calendar.Feed

	u, ok := m["url"]
	if !ok {
		return feed, fmt.Errorf("%s: feeds[%d]: url is required", widgetName, idx)
	}
	urlStr, ok := u.(string)
	if !ok {
		return feed, fmt.Errorf("%s: feeds[%d]: url must be a string, got %T", widgetName, idx, u)
	}
	feed.URL = urlStr

	if n, ok := m["name"]; ok {
		name, ok := n.(string)
		if !ok {
			return feed, fmt.Errorf("%s: feeds[%d]: name must be a string, got %T", widgetName, idx, n)
		}
		feed.Name = name
	}

	r, ok := m["rules"]
	if !ok {
		return feed, nil
	}
	rules, err := parseRules(widgetName, r, feedLabel(feed, idx))
	if err != nil {
		return feed, err
	}
	feed.Rules = rules
	return feed, nil
}

// feedLabel identifies a feed in error messages: its name when it has
// one, since an operator recognises "Hockey" far quicker than a
// positional index into a list of long subscription URLs.
func feedLabel(feed calendar.Feed, idx int) string {
	if feed.Name != "" {
		return fmt.Sprintf("feeds[%d] (%s)", idx, feed.Name)
	}
	return fmt.Sprintf("feeds[%d]", idx)
}

// parseRules reads one feed's rules list. A rule is {match, replace} or
// {match, exclude}; omitting replace deletes the matched text, which is
// the common case of stripping a fixed prefix.
func parseRules(widgetName string, raw any, label string) ([]calendar.Rule, error) {
	list, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("%s: %s: rules must be a list, got %T", widgetName, label, raw)
	}

	rules := make([]calendar.Rule, 0, len(list))
	for i, item := range list {
		m, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%s: %s: rules[%d] must be an object, got %T", widgetName, label, i, item)
		}
		rule, err := parseRule(widgetName, m, fmt.Sprintf("%s: rules[%d]", label, i))
		if err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

func parseRule(widgetName string, m map[string]any, label string) (calendar.Rule, error) {
	raw, ok := m["match"]
	if !ok {
		return calendar.Rule{}, fmt.Errorf("%s: %s: match is required", widgetName, label)
	}
	match, ok := raw.(string)
	if !ok {
		return calendar.Rule{}, fmt.Errorf("%s: %s: match must be a string, got %T", widgetName, label, raw)
	}

	var replace string
	if v, ok := m["replace"]; ok {
		replace, ok = v.(string)
		if !ok {
			return calendar.Rule{}, fmt.Errorf("%s: %s: replace must be a string, got %T", widgetName, label, v)
		}
	}

	var exclude bool
	if v, ok := m["exclude"]; ok {
		exclude, ok = v.(bool)
		if !ok {
			return calendar.Rule{}, fmt.Errorf("%s: %s: exclude must be a bool, got %T", widgetName, label, v)
		}
	}

	rule, err := calendar.NewRule(match, replace, exclude)
	if err != nil {
		return calendar.Rule{}, fmt.Errorf("%s: %s: %w", widgetName, label, err)
	}
	return rule, nil
}
