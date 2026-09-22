package daygrid

import (
	"strings"
	"testing"
)

// testWidget stands in for a widget name; every daygrid error carries
// one so a dashboard that fails to load says which widget rejected it.
const testWidget = "test-widget"

func TestParseFeeds_ObjectForm(t *testing.T) {
	feeds, err := ParseFeeds(testWidget, []any{
		"https://example.com/personal.ics",
		map[string]any{
			"url":  "https://team.example/cal.ics",
			"name": "Team calendar",
			"rules": []any{
				map[string]any{"match": `^Jane Doe\n(Ravens\n)?`},
				map[string]any{"match": `vs `, "replace": "v "},
				map[string]any{"match": `Tournament`, "exclude": true},
			},
		},
	})
	if err != nil {
		t.Fatalf("ParseFeeds: %v", err)
	}

	if len(feeds) != 2 {
		t.Fatalf("got %d feeds, want 2", len(feeds))
	}
	if got, want := feeds[0].URL, "https://example.com/personal.ics"; got != want {
		t.Errorf("feeds[0].URL = %q, want %q", got, want)
	}
	if len(feeds[0].Rules) != 0 {
		t.Errorf("feeds[0].Rules = %v, want none for a bare string feed", feeds[0].Rules)
	}
	if got, want := feeds[1].URL, "https://team.example/cal.ics"; got != want {
		t.Errorf("feeds[1].URL = %q, want %q", got, want)
	}
	if got, want := feeds[1].Name, "Team calendar"; got != want {
		t.Errorf("feeds[1].Name = %q, want %q", got, want)
	}
	if len(feeds[1].Rules) != 3 {
		t.Fatalf("got %d rules, want 3", len(feeds[1].Rules))
	}
}

func TestParseFeeds_Errors(t *testing.T) {
	cases := []struct {
		label   string
		feeds   any
		wantErr string
	}{
		{
			label:   "feeds is not a list",
			feeds:   "https://example.com/a.ics",
			wantErr: "feeds must be a list",
		},
		{
			label:   "feeds is empty",
			feeds:   []any{},
			wantErr: "feeds must not be empty",
		},
		{
			label:   "entry is neither string nor object",
			feeds:   []any{123},
			wantErr: "must be a URL string or a feed object",
		},
		{
			label:   "object missing url",
			feeds:   []any{map[string]any{"name": "nameless"}},
			wantErr: "url is required",
		},
		{
			label:   "url is not a string",
			feeds:   []any{map[string]any{"url": 42}},
			wantErr: "url must be a string",
		},
		{
			label:   "name is not a string",
			feeds:   []any{map[string]any{"url": "https://x.example/c.ics", "name": 42}},
			wantErr: "name must be a string",
		},
		{
			label:   "rules is not a list",
			feeds:   []any{map[string]any{"url": "https://x.example/c.ics", "rules": "nope"}},
			wantErr: "rules must be a list",
		},
		{
			label:   "rule is not an object",
			feeds:   []any{map[string]any{"url": "https://x.example/c.ics", "rules": []any{"nope"}}},
			wantErr: "rules[0] must be an object",
		},
		{
			label:   "rule missing match",
			feeds:   []any{map[string]any{"url": "https://x.example/c.ics", "rules": []any{map[string]any{"replace": "x"}}}},
			wantErr: "match is required",
		},
		{
			label:   "match is not a string",
			feeds:   []any{map[string]any{"url": "https://x.example/c.ics", "rules": []any{map[string]any{"match": 42}}}},
			wantErr: "match must be a string",
		},
		{
			label:   "replace is not a string",
			feeds:   []any{map[string]any{"url": "https://x.example/c.ics", "rules": []any{map[string]any{"match": "x", "replace": 42}}}},
			wantErr: "replace must be a string",
		},
		{
			label:   "exclude is not a bool",
			feeds:   []any{map[string]any{"url": "https://x.example/c.ics", "rules": []any{map[string]any{"match": "x", "exclude": "yes"}}}},
			wantErr: "exclude must be a bool",
		},
		{
			label:   "invalid regex",
			feeds:   []any{map[string]any{"url": "https://x.example/c.ics", "rules": []any{map[string]any{"match": "(unclosed"}}}},
			wantErr: "invalid match",
		},
		{
			label:   "replace and exclude together",
			feeds:   []any{map[string]any{"url": "https://x.example/c.ics", "rules": []any{map[string]any{"match": "x", "replace": "y", "exclude": true}}}},
			wantErr: "both replace and exclude",
		},
	}

	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			_, err := ParseFeeds(testWidget, tc.feeds)
			if err == nil {
				t.Fatal("expected an error")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error = %q, want it to contain %q", err, tc.wantErr)
			}
		})
	}
}

// A feed's name should identify the feed in rule errors, since a bare
// index into a list of long URLs tells an operator nothing.
func TestParseFeeds_RuleErrorNamesTheFeed(t *testing.T) {
	_, err := ParseFeeds(testWidget, []any{map[string]any{
		"url":   "https://team.example/cal.ics",
		"name":  "Team calendar",
		"rules": []any{map[string]any{"match": "(unclosed"}},
	}})
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "Team calendar") {
		t.Errorf("error = %q, want it to name the feed", err)
	}
}

// The object form is usable purely to label a feed, with no rules at all.
func TestParseFeeds_ObjectWithoutRules(t *testing.T) {
	feeds, err := ParseFeeds(testWidget, []any{map[string]any{
		"url":  "https://team.example/cal.ics",
		"name": "Team calendar",
	}})
	if err != nil {
		t.Fatalf("ParseFeeds: %v", err)
	}
	if len(feeds) != 1 {
		t.Fatalf("got %d feeds, want 1", len(feeds))
	}
	if feeds[0].Name != "Team calendar" || feeds[0].URL != "https://team.example/cal.ics" {
		t.Errorf("feed = %+v", feeds[0])
	}
	if len(feeds[0].Rules) != 0 {
		t.Errorf("Rules = %v, want none", feeds[0].Rules)
	}
}
