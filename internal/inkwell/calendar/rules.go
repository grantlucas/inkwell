package calendar

import (
	"fmt"
	"regexp"
	"strings"
)

// Feed is one subscribed calendar URL together with the rules that clean
// up the events it returns. Feeds are configured per URL rather than
// globally because the boilerplate worth stripping is a property of the
// system that generated the feed: a league's team calendar prefixes
// every summary with the player and team, while a personal calendar
// needs no rewriting at all.
type Feed struct {
	URL string

	// Name is an optional human label used in configuration errors so a
	// bad regex points at the feed the operator recognises rather than
	// at an opaque URL.
	Name string

	Rules []Rule
}

// FeedsFromURLs adapts a plain list of URLs to rule-less Feeds, for
// callers (and tests) that have nothing to rewrite.
func FeedsFromURLs(urls []string) []Feed {
	feeds := make([]Feed, 0, len(urls))
	for _, u := range urls {
		feeds = append(feeds, Feed{URL: u})
	}
	return feeds
}

// Rule is one transformation applied to an event summary. A Rule either
// rewrites the summary (Replace) or drops the event outright (Exclude).
type Rule struct {
	match   *regexp.Regexp
	replace string
	exclude bool
}

// NewRule compiles match and pairs it with an action. An exclude rule
// drops any event whose summary matches; otherwise every match is
// replaced with replace, which may reference capture groups as $1.
// Passing an empty replace deletes the matched text, which is the common
// case — stripping a fixed prefix.
func NewRule(match, replace string, exclude bool) (Rule, error) {
	if match == "" {
		return Rule{}, fmt.Errorf("rule match must not be empty") //nolint:goerr113 // config validation message
	}
	if exclude && replace != "" {
		return Rule{}, fmt.Errorf("rule cannot set both replace and exclude") //nolint:goerr113 // config validation message
	}
	re, err := regexp.Compile(match)
	if err != nil {
		return Rule{}, fmt.Errorf("invalid match %q: %w", match, err)
	}
	return Rule{match: re, replace: replace, exclude: exclude}, nil
}

// applyRules runs rules over one event's summary in order, returning the
// rewritten event and whether it survived.
//
// Rules form a pipeline: each one sees the summary as the rules before
// it left it, so a replace that strips a prefix can be followed by an
// exclude that matches on what remains. A rewritten summary is trimmed,
// since stripping a leading line otherwise leaves the separator behind
// as blank space at the top of the event.
func applyRules(e Event, rules []Rule) (Event, bool) {
	for _, r := range rules {
		if r.exclude {
			if r.match.MatchString(e.Summary) {
				return e, false
			}
			continue
		}
		if rewritten := r.match.ReplaceAllString(e.Summary, r.replace); rewritten != e.Summary {
			e.Summary = strings.TrimSpace(rewritten)
		}
	}
	return e, true
}
