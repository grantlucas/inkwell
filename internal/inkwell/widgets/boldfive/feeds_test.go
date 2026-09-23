package boldfive

import (
	"strings"
	"testing"
)

// These mirror weekly's feed-parsing tests, because the parser is the
// same code with a different error prefix. Both copies go away with the
// shared package in issue #92; until then the coverage has to live in
// both places or the duplicate is untested.
func TestParseConfig_FeedObjectForm(t *testing.T) {
	cfg, err := parseConfig(map[string]any{
		"feeds": []any{
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
		},
	})
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}

	if len(cfg.Feeds) != 2 {
		t.Fatalf("got %d feeds, want 2", len(cfg.Feeds))
	}
	if got, want := cfg.Feeds[0].URL, "https://example.com/personal.ics"; got != want {
		t.Errorf("feeds[0].URL = %q, want %q", got, want)
	}
	if len(cfg.Feeds[0].Rules) != 0 {
		t.Errorf("feeds[0].Rules = %v, want none for a bare string feed", cfg.Feeds[0].Rules)
	}
	if got, want := cfg.Feeds[1].URL, "https://team.example/cal.ics"; got != want {
		t.Errorf("feeds[1].URL = %q, want %q", got, want)
	}
	if got, want := cfg.Feeds[1].Name, "Team calendar"; got != want {
		t.Errorf("feeds[1].Name = %q, want %q", got, want)
	}
	if len(cfg.Feeds[1].Rules) != 3 {
		t.Fatalf("got %d rules, want 3", len(cfg.Feeds[1].Rules))
	}
}

func TestParseConfig_FeedErrors(t *testing.T) {
	cases := []struct {
		label   string
		feeds   any
		wantErr string
	}{
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
			_, err := parseConfig(map[string]any{"feeds": tc.feeds})
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
func TestParseConfig_RuleErrorNamesTheFeed(t *testing.T) {
	_, err := parseConfig(map[string]any{
		"feeds": []any{map[string]any{
			"url":   "https://team.example/cal.ics",
			"name":  "Team calendar",
			"rules": []any{map[string]any{"match": "(unclosed"}},
		}},
	})
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "Team calendar") {
		t.Errorf("error = %q, want it to name the feed", err)
	}
}

// The object form is usable purely to label a feed, with no rules at all.
func TestParseConfig_FeedObjectWithoutRules(t *testing.T) {
	cfg, err := parseConfig(map[string]any{
		"feeds": []any{map[string]any{
			"url":  "https://team.example/cal.ics",
			"name": "Team calendar",
		}},
	})
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	if len(cfg.Feeds) != 1 {
		t.Fatalf("got %d feeds, want 1", len(cfg.Feeds))
	}
	if cfg.Feeds[0].Name != "Team calendar" || cfg.Feeds[0].URL != "https://team.example/cal.ics" {
		t.Errorf("feed = %+v", cfg.Feeds[0])
	}
	if len(cfg.Feeds[0].Rules) != 0 {
		t.Errorf("Rules = %v, want none", cfg.Feeds[0].Rules)
	}
}
