package weather

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"
)

// CachedSource wraps a Source with a time-based, location-rounded cache.
// On fetch error after the cache has been populated, it returns stale
// cached data along with the error.
type CachedSource struct {
	inner Source
	ttl   time.Duration
	now   func() time.Time

	mu       sync.Mutex
	forecast *Forecast
	cacheKey string
	fetched  time.Time
}

// NewCachedSource wraps inner with a cache that refreshes after ttl.
func NewCachedSource(inner Source, ttl time.Duration, now func() time.Time) *CachedSource {
	return &CachedSource{
		inner: inner,
		ttl:   ttl,
		now:   now,
	}
}

// Forecast returns a cached forecast if fresh and for the same rounded
// location. Otherwise it fetches from the inner source.
func (c *CachedSource) Forecast(ctx context.Context, loc Location, days int) (*Forecast, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := locationKey(loc)
	if c.forecast != nil && c.cacheKey == key && c.now().Sub(c.fetched) < c.ttl {
		return c.forecast, nil
	}

	fc, err := c.inner.Forecast(ctx, loc, days)
	if err != nil {
		if c.forecast != nil && c.cacheKey == key {
			return c.forecast, err
		}
		return nil, err
	}

	c.forecast = fc
	c.cacheKey = key
	c.fetched = c.now()
	return fc, nil
}

func locationKey(loc Location) string {
	lat := math.Round(loc.Latitude*10) / 10
	lon := math.Round(loc.Longitude*10) / 10
	return fmt.Sprintf("%.1f,%.1f", lat, lon)
}
