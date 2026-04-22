package datasource

import (
	"sync"
	"time"
)

// Cached wraps a fetch function with TTL-based expiration and stale-on-error.
// No background goroutines — refresh happens lazily when Get is called after
// the TTL has expired. Safe for concurrent use.
type Cached[T any] struct {
	fetch   func() (T, error)
	ttl     time.Duration
	now     func() time.Time

	mu      sync.Mutex
	value   T
	fetched bool
	expires time.Time
}

// NewCached creates a Cached[T] that calls fetch to obtain a value, caches it
// for ttl, and uses now for the current time.
func NewCached[T any](fetch func() (T, error), ttl time.Duration, now func() time.Time) *Cached[T] {
	return &Cached[T]{
		fetch: fetch,
		ttl:   ttl,
		now:   now,
	}
}

// Get returns the cached value if fresh, or calls fetch if stale/missing.
// On fetch error with a previous good value, returns the stale value and nil
// (stale-on-error). If no previous good value exists, returns zero T and the
// error.
func (c *Cached[T]) Get() (T, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.fetched && c.now().Before(c.expires) {
		return c.value, nil
	}

	val, err := c.fetch()
	if err != nil {
		if c.fetched {
			return c.value, nil
		}
		var zero T
		return zero, err
	}

	c.value = val
	c.fetched = true
	c.expires = c.now().Add(c.ttl)
	return c.value, nil
}
