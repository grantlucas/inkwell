package weather

import (
	"context"
	"sync"
	"time"
)

// Settings is the resolved weather configuration a widget renders with: where
// to fetch, which model to use, and the temperature unit to display. It is the
// shared default carried by a Provider; widgets may override individual fields.
type Settings struct {
	Location Location
	Model    Model
	TempUnit string
}

// Provider is the shared, reusable weather entry point widgets build on. It
// fetches forecasts for any (model, location) on demand and caches each one
// independently, so multiple widgets — even at different locations or using
// different models — deduplicate fetches and survive a transient API failure
// by serving the last good forecast. Construct one per process (see app wiring)
// and inject it; do not build per-widget sources.
type Provider struct {
	client HTTPClient
	ttl    time.Duration
	now    func() time.Time

	defaults Settings

	mu     sync.Mutex
	caches map[string]*CachedSource
}

// NewProvider creates a Provider that fetches with client, caches each
// (model, location) forecast for ttl, and exposes defaults as the baseline
// Settings for widgets. A nil client / now fall through to the wrapped
// constructors' defaults (http.DefaultClient / time.Now).
func NewProvider(client HTTPClient, ttl time.Duration, now func() time.Time, defaults Settings) *Provider {
	return &Provider{
		client:   client,
		ttl:      ttl,
		now:      now,
		defaults: defaults,
		caches:   make(map[string]*CachedSource),
	}
}

// Defaults returns the baseline Settings widgets resolve their overrides
// against.
func (p *Provider) Defaults() Settings { return p.defaults }

// Forecast returns a forecast for loc from the given model, cached per
// (model, location, days). Concurrent callers requesting the same key share a
// single cache entry and therefore a single upstream fetch.
func (p *Provider) Forecast(ctx context.Context, loc Location, model Model, days int) (*Forecast, error) {
	return p.cacheFor(model, loc, days).Forecast(ctx, loc, days)
}

// cacheFor returns the CachedSource for a (model, location, days) key, lazily
// creating it on first use. Each key gets its own cache so different locations
// or models never evict one another.
func (p *Provider) cacheFor(model Model, loc Location, days int) *CachedSource {
	// cacheKey rounds the location, so each wrapped CachedSource only ever sees
	// this one rounded location — its own location-change detection is
	// intentionally redundant here; the map key is what separates locations.
	key := string(model) + "|" + cacheKey(loc, days)
	p.mu.Lock()
	defer p.mu.Unlock()
	cs, ok := p.caches[key]
	if !ok {
		cs = NewCachedSource(NewOpenMeteoSource(model, p.client), p.ttl, p.now)
		p.caches[key] = cs
	}
	return cs
}

// SourceForModel returns a Source bound to model that delegates to this
// Provider, so a widget can depend on the small Source interface while still
// sharing the Provider's cache.
func (p *Provider) SourceForModel(model Model) Source {
	return modelSource{provider: p, model: model}
}

type modelSource struct {
	provider *Provider
	model    Model
}

func (m modelSource) Forecast(ctx context.Context, loc Location, days int) (*Forecast, error) {
	return m.provider.Forecast(ctx, loc, m.model, days)
}
