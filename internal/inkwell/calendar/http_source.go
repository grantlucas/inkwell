package calendar

import (
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar/ical"
)

// HTTPClient is the subset of *http.Client needed for fetching calendar feeds.
type HTTPClient interface {
	Get(url string) (*http.Response, error)
}

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
// and returns events sorted by start time.
func (s *HTTPSource) Events(start, end time.Time) ([]Event, error) {
	seen := make(map[string]bool)
	var all []Event

	for _, url := range s.urls {
		resp, err := s.client.Get(url)
		if err != nil {
			return nil, fmt.Errorf("fetch %q: %w", url, err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("fetch %q: status %d", url, resp.StatusCode)
		}

		events, err := ical.Parse(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("parse %q: %w", url, err)
		}

		for _, e := range events {
			if seen[e.UID] {
				continue
			}
			// Filter: event overlaps [start, end) if event.Start < end && event.End > start.
			if e.Start.Before(end) && e.End.After(start) {
				seen[e.UID] = true
				all = append(all, e)
			}
		}
	}

	sort.Slice(all, func(i, j int) bool {
		return all[i].Start.Before(all[j].Start)
	})

	return all, nil
}
