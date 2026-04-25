package calendar

import (
	"sync"
	"time"
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
// a populated cache, returns stale events and the error.
func (c *CachedSource) Events(start, end time.Time) ([]Event, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.events != nil && c.now().Sub(c.fetched) < c.ttl {
		return c.filterEvents(start, end), nil
	}

	events, err := c.inner.Events(start, end)
	if err != nil {
		if c.events != nil {
			// Return stale data with the error.
			return c.filterEvents(start, end), err
		}
		return nil, err
	}

	c.events = events
	c.fetched = c.now()
	return events, nil
}

// filterEvents returns cached events overlapping [start, end).
func (c *CachedSource) filterEvents(start, end time.Time) []Event {
	var filtered []Event
	for _, e := range c.events {
		if e.Start.Before(end) && e.End.After(start) {
			filtered = append(filtered, e)
		}
	}
	return filtered
}
