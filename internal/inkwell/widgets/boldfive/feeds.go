package boldfive

import (
	"fmt"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar"
)

// parseFeeds reads the `feeds` list, where each entry is either a bare
// URL string or an object carrying that feed's rewrite rules. Both forms
// coexist so a dashboard that needs no rewriting keeps the one-line form
// it has always had.
//
// This duplicates weekly's parser, differing only in the error prefix.
// That is deliberate and temporary: issue #92 extracts the shared
// calendar + weather scaffolding once a second screen exists to show
// where the seams really are, and this widget is the first.
func parseFeeds(raw any) ([]calendar.Feed, error) {
	list, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("bold-five: feeds must be a list, got %T", raw)
	}
	if len(list) == 0 {
		return nil, fmt.Errorf("bold-five: feeds must not be empty") //nolint:goerr113 // config validation message
	}

	feeds := make([]calendar.Feed, 0, len(list))
	for i, item := range list {
		switch v := item.(type) {
		case string:
			feeds = append(feeds, calendar.Feed{URL: v})
		case map[string]any:
			feed, err := parseFeedObject(v, i)
			if err != nil {
				return nil, err
			}
			feeds = append(feeds, feed)
		default:
			return nil, fmt.Errorf("bold-five: feeds[%d] must be a URL string or a feed object, got %T", i, item)
		}
	}
	return feeds, nil
}

// parseFeedObject reads the object form of a feed entry. idx identifies
// the entry in errors until a name is known.
func parseFeedObject(m map[string]any, idx int) (calendar.Feed, error) {
	var feed calendar.Feed

	u, ok := m["url"]
	if !ok {
		return feed, fmt.Errorf("bold-five: feeds[%d]: url is required", idx)
	}
	urlStr, ok := u.(string)
	if !ok {
		return feed, fmt.Errorf("bold-five: feeds[%d]: url must be a string, got %T", idx, u)
	}
	feed.URL = urlStr

	if n, ok := m["name"]; ok {
		name, ok := n.(string)
		if !ok {
			return feed, fmt.Errorf("bold-five: feeds[%d]: name must be a string, got %T", idx, n)
		}
		feed.Name = name
	}

	r, ok := m["rules"]
	if !ok {
		return feed, nil
	}
	rules, err := parseRules(r, feedLabel(feed, idx))
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
func parseRules(raw any, label string) ([]calendar.Rule, error) {
	list, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("bold-five: %s: rules must be a list, got %T", label, raw)
	}

	rules := make([]calendar.Rule, 0, len(list))
	for i, item := range list {
		m, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("bold-five: %s: rules[%d] must be an object, got %T", label, i, item)
		}
		rule, err := parseRule(m, fmt.Sprintf("%s: rules[%d]", label, i))
		if err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

func parseRule(m map[string]any, label string) (calendar.Rule, error) {
	raw, ok := m["match"]
	if !ok {
		return calendar.Rule{}, fmt.Errorf("bold-five: %s: match is required", label)
	}
	match, ok := raw.(string)
	if !ok {
		return calendar.Rule{}, fmt.Errorf("bold-five: %s: match must be a string, got %T", label, raw)
	}

	var replace string
	if v, ok := m["replace"]; ok {
		replace, ok = v.(string)
		if !ok {
			return calendar.Rule{}, fmt.Errorf("bold-five: %s: replace must be a string, got %T", label, v)
		}
	}

	var exclude bool
	if v, ok := m["exclude"]; ok {
		exclude, ok = v.(bool)
		if !ok {
			return calendar.Rule{}, fmt.Errorf("bold-five: %s: exclude must be a bool, got %T", label, v)
		}
	}

	rule, err := calendar.NewRule(match, replace, exclude)
	if err != nil {
		return calendar.Rule{}, fmt.Errorf("bold-five: %s: %w", label, err)
	}
	return rule, nil
}
