package calendar

import (
	"context"
	"sync"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar/ical"
)

// Fetcher fetches every event and series a set of feeds carries,
// unwindowed. HTTPSource is the production implementation.
type Fetcher interface {
	Fetch(ctx context.Context) ([]Event, error)
}

// CachedSource wraps a Fetcher with a time-based cache and turns what it
// fetched into the occurrences in a window. It re-fetches when the cache
// has expired (TTL elapsed). On fetch error after the cache has been
// populated, it returns stale cached data along with the error.
//
// The cache holds series, not occurrences, so every window — whichever
// one the fetch happened under — is expanded from the same events.
type CachedSource struct {
	inner Fetcher
	ttl   time.Duration
	now   func() time.Time

	mu      sync.Mutex
	events  []Event
	fetched time.Time
}

// NewCachedSource wraps inner with a cache that refreshes after ttl.
func NewCachedSource(inner Fetcher, ttl time.Duration, now func() time.Time) *CachedSource {
	return &CachedSource{
		inner: inner,
		ttl:   ttl,
		now:   now,
	}
}

// Events returns events in [start, end). If the cache is fresh, it returns
// cached events. Otherwise it fetches from the inner source. On error with
// a populated cache, returns stale events and the error. ctx bounds the
// underlying fetch; cache hits return immediately without touching ctx.
func (c *CachedSource) Events(ctx context.Context, start, end time.Time) ([]Event, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.events != nil && c.now().Sub(c.fetched) < c.ttl {
		return c.filterEvents(start, end), nil
	}

	events, err := c.inner.Fetch(ctx)
	if err != nil {
		if c.events != nil {
			// Return stale data with the error.
			return c.filterEvents(start, end), err
		}
		return nil, err
	}

	// Store a defensive copy so callers can't mutate the cached slice
	// (or rely on its identity for future hits) and we can't accidentally
	// return aliased storage on the next refresh.
	c.events = append(c.events[:0:0], events...)
	c.fetched = c.now()

	return c.filterEvents(start, end), nil
}

// filterEvents returns cached events overlapping [start, end). Both
// non-recurring overlap and recurring-event expansion happen inside
// ical.Occurrences; the returned slice is freshly allocated so callers
// can mutate it without affecting subsequent cache reads.
func (c *CachedSource) filterEvents(start, end time.Time) []Event {
	return ical.Occurrences(c.events, start, end)
}
