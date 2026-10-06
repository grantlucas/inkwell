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

// fetchFeed fetches and parses one feed as it is, before any rules. It
// keeps every event, including several sharing a UID: a series and the
// overrides that edit single instances of it all carry the series' UID.
//
// It does not window. A recurring event comes back as its series, and
// only expanding the series says which of its occurrences fall in a
// window; filtering on the series' first instance threw away every
// series that began before the window.
//
// Close errors are surfaced when no other error preceded them.
func fetchFeed(ctx context.Context, client HTTPClient, url string) (_ []Event, retErr error) {
	req, err := newRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request %q: %w", url, err) //nolint:goerr113 // only reachable via test override
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch %q: %w", url, err)
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil && retErr == nil {
			retErr = fmt.Errorf("close %q: %w", url, cerr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch %q: status %d", url, resp.StatusCode)
	}

	events, err := ical.Parse(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parse %q: %w", url, err)
	}
	return events, nil
}
