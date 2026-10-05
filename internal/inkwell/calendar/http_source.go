package calendar

import (
	"context"
	"fmt"
	"net/http"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar/ical"
)

// HTTPClient is the subset of *http.Client needed for fetching calendar feeds.
// Modeled on http.Client.Do so the request carries its context and
// Source.Events(ctx, …) can actually honor cancellation.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// newRequestWithContext is the indirection over http.NewRequestWithContext
// that tests override to exercise the otherwise-unreachable "build request"
// error branch. Production paths go straight through.
var newRequestWithContext = http.NewRequestWithContext

// HTTPSource fetches and parses iCal feeds from a list of Feeds.
// It merges events from all feeds and deduplicates by UID. Each feed's
// rules are applied to its own events as they are parsed, so everything
// downstream — dedup, caching, rendering — only ever sees cleaned-up
// events.
//
// It does not window. A recurring event comes back as its series, and
// only expanding the series says which of its occurrences fall in a
// window; filtering on the series' first instance threw away every
// series that began before the window. CachedSource expands and
// windows what this returns.
type HTTPSource struct {
	feeds  []Feed
	client HTTPClient
}

// NewHTTPSource creates an HTTPSource that fetches from the given feeds.
func NewHTTPSource(feeds []Feed, client HTTPClient) *HTTPSource {
	return &HTTPSource{feeds: feeds, client: client}
}

// Fetch fetches all feeds, merges and deduplicates them, and returns
// every event and series. ctx bounds each fetch.
func (s *HTTPSource) Fetch(ctx context.Context) ([]Event, error) {
	seen := make(map[string]bool)
	var all []Event

	for _, feed := range s.feeds {
		if err := s.fetchFeed(ctx, feed, seen, &all); err != nil {
			return nil, err
		}
	}

	return all, nil
}

// fetchFeed handles a single URL's request lifecycle so that resp.Body
// is closed at the end of each iteration rather than lingering until
// Fetch returns. Close errors are surfaced when no other error
// preceded them.
func (s *HTTPSource) fetchFeed(ctx context.Context, feed Feed, seen map[string]bool, all *[]Event) (retErr error) {
	url := feed.URL
	req, err := newRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("build request %q: %w", url, err) //nolint:goerr113 // only reachable via test override
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("fetch %q: %w", url, err)
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil && retErr == nil {
			retErr = fmt.Errorf("close %q: %w", url, cerr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fetch %q: status %d", url, resp.StatusCode)
	}

	events, err := ical.Parse(resp.Body)
	if err != nil {
		return fmt.Errorf("parse %q: %w", url, err)
	}

	for _, e := range events {
		if seen[e.UID] {
			continue
		}
		e, keep := applyRules(e, feed.Rules)
		if !keep {
			continue
		}
		seen[e.UID] = true
		*all = append(*all, e)
	}
	return nil
}
