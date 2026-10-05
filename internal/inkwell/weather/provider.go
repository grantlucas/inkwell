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
// Settings for widgets. A nil client falls through to http.DefaultClient and a
// nil now to time.Now.
//
// Forecasts are asked for in now's zone. The dashboard hands widgets a clock
// already in its display zone, and that clock decides Today and the now
// marker, so taking the forecast's zone from it keeps a forecast's dates and
// hours on the same days and hours as everything else on the panel — even when
// the forecast location sits in another zone.
func NewProvider(client HTTPClient, ttl time.Duration, now func() time.Time, defaults Settings) *Provider {
	if now == nil {
		now = time.Now
	}
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

// ForecastHorizon is how many days the Provider fetches for every location,
// whatever span a widget asks for. It must be at least the longest span any
// widget can be configured for (weekly-calendar's seven columns), since a
// longer request is answered from the same forecast and would come back short.
const ForecastHorizon = 7

// Forecast returns the first days of a forecast for loc from the given model.
// Each (model, location) is fetched once at ForecastHorizon and cached, so
// widgets asking for different spans — and concurrent callers — share a single
// upstream fetch.
func (p *Provider) Forecast(ctx context.Context, loc Location, model Model, days int) (*Forecast, error) {
	fc, err := p.cacheFor(model, loc).Forecast(ctx, loc, ForecastHorizon)
	return span(fc, p.now(), days), err
}

// span returns the n days of fc starting at now's date. The cache outlives
// midnight, so the forecast may begin on a day that is already over; those
// days are skipped rather than counted. The cached forecast is shared, so the
// span is a new Forecast rather than a reslice written back into it.
func span(fc *Forecast, now time.Time, n int) *Forecast {
	if fc == nil {
		return nil
	}
	// Forecast dates are calendar dates parsed at UTC midnight, so Today is
	// compared as one too.
	y, m, d := now.Date()
	today := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	days := fc.Days
	for len(days) > 0 && days[0].Date.Before(today) {
		days = days[1:]
	}
	out := *fc
	out.Days = days[:min(max(n, 0), len(days))]
	return &out
}

// cacheFor returns the CachedSource for a (model, location) key, lazily
// creating it on first use. Each key gets its own cache so different locations
// or models never evict one another.
func (p *Provider) cacheFor(model Model, loc Location) *CachedSource {
	// cacheKey rounds the location, so each wrapped CachedSource only ever sees
	// this one rounded location — its own location-change detection is
	// intentionally redundant here; the map key is what separates locations.
	key := string(model) + "|" + cacheKey(loc, ForecastHorizon)
	p.mu.Lock()
	defer p.mu.Unlock()
	cs, ok := p.caches[key]
	if !ok {
		cs = NewCachedSource(NewOpenMeteoSource(model, p.client, p.now().Location()), p.ttl, p.now)
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
