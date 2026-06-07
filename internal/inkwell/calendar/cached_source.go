package calendar

import (
	"context"
	"sync"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar/ical"
)

// CachedSource wraps a Source with a time-based cache. It re-fetches from
// the inner source when the cache has expired (TTL elapsed). On fetch error
// after the cache has been populated, it returns stale cached data along
// with the error.
type CachedSource struct {
	inner Source
	ttl   time.Duration
	now   func() time.Time

	mu      sync.Mutex
	events  []Event
	fetched time.Time
}

// NewCachedSource wraps inner with a cache that refreshes after ttl.
func NewCachedSource(inner Source, ttl time.Duration, now func() time.Time) *CachedSource {
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

	events, err := c.inner.Events(ctx, start, end)
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

	// Run RRULE expansion + window filter through the same path as
	// cache hits — otherwise the first call returns master recurrences
	// while subsequent cache-hit calls return expanded occurrences,
	// surprising callers who'd see different shapes for the same input.
	return c.filterEvents(start, end), nil
}

// filterEvents returns cached events overlapping [start, end). Both
// non-recurring overlap and recurring-event expansion happen inside
// ical.Occurrences; the returned slice is freshly allocated so callers
// can mutate it without affecting subsequent cache reads.
func (c *CachedSource) filterEvents(start, end time.Time) []Event {
	return ical.Occurrences(c.events, start, end)
}
