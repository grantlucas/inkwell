package calendar

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"time"

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

// HTTPSource fetches and parses iCal feeds from a list of URLs.
// It merges events from all feeds, deduplicates by UID, and filters
// to the requested time range.
type HTTPSource struct {
	urls   []string
	client HTTPClient
}

// NewHTTPSource creates an HTTPSource that fetches from the given URLs.
func NewHTTPSource(urls []string, client HTTPClient) *HTTPSource {
	return &HTTPSource{urls: urls, client: client}
}

// Events fetches all feeds, merges, deduplicates, filters to [start, end),
// and returns events sorted by start time. ctx bounds each fetch.
func (s *HTTPSource) Events(ctx context.Context, start, end time.Time) ([]Event, error) {
	seen := make(map[string]bool)
	var all []Event

	for _, url := range s.urls {
		if err := s.fetchFeed(ctx, url, start, end, seen, &all); err != nil {
			return nil, err
		}
	}

	sort.Slice(all, func(i, j int) bool {
		return all[i].Start.Before(all[j].Start)
	})

	return all, nil
}

// fetchFeed handles a single URL's request lifecycle so that resp.Body
// is closed at the end of each iteration rather than lingering until
// Events returns. Close errors are surfaced when no other error
// preceded them.
func (s *HTTPSource) fetchFeed(ctx context.Context, url string, start, end time.Time, seen map[string]bool, all *[]Event) (retErr error) {
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
		// Filter: event overlaps [start, end) if event.Start < end && event.End > start.
		if e.Start.Before(end) && e.End.After(start) {
			seen[e.UID] = true
			*all = append(*all, e)
		}
	}
	return nil
}
