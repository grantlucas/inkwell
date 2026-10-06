package calendar

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar/ical"
)

// Provider is the calendar module: given a widget's feeds, a window and
// how fresh that widget wants its data, it returns the occurrences that
// overlap the window. Fetching, parsing, rules, recurrence expansion,
// windowing, collapsing duplicates and caching all happen behind it.
//
// It holds one cache per feed URL, shared by every widget in the process
// across every screen, so a feed is fetched once however many widgets
// show it. Construct one per process (see app wiring) and inject it, the
// way the weather Provider is; widgets never build their own.
//
// The cache holds what a feed carries as parsed, before any widget's
// rules: two widgets may give one URL different rules, so rules are
// applied per request, after the cache.
type Provider struct {
	client HTTPClient
	now    func() time.Time

	mu    sync.Mutex
	feeds map[string]*feedCache
}

// NewProvider creates a Provider that fetches feeds with client and ages
// its cached copies by now.
func NewProvider(client HTTPClient, now func() time.Time) *Provider {
	return &Provider{client: client, now: now, feeds: map[string]*feedCache{}}
}

// Occurrences returns the occurrences of feeds overlapping [start, end),
// sorted by start, with duplicates collapsed.
//
// A feed is served from the cache when its cached copy is younger than
// refresh, the requesting widget's own setting, so one widget's setting
// never makes another's data staler than it asked for. A feed that fails
// to fetch serves its last good copy, if it has one, and its error is
// returned alongside whatever the feeds produced.
func (p *Provider) Occurrences(ctx context.Context, feeds []Feed, start, end time.Time, refresh time.Duration) ([]Event, error) {
	var (
		out  []Event
		errs []error
	)
	for _, f := range feeds {
		events, err := p.cacheFor(f.URL).events(ctx, p, f.URL, refresh)
		if err != nil {
			errs = append(errs, err)
		}
		out = append(out, ical.Occurrences(withRules(events, f.Rules), start, end)...)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Start.Before(out[j].Start) })
	return collapse(out), errors.Join(errs...)
}

// Source binds a widget's feeds and refresh setting to the Provider, so
// the widget can depend on the small Source interface while sharing the
// Provider's cache.
func (p *Provider) Source(feeds []Feed, refresh time.Duration) Source {
	return feedSource{provider: p, feeds: feeds, refresh: refresh}
}

type feedSource struct {
	provider *Provider
	feeds    []Feed
	refresh  time.Duration
}

func (s feedSource) Events(ctx context.Context, start, end time.Time) ([]Event, error) {
	return s.provider.Occurrences(ctx, s.feeds, start, end, s.refresh)
}

// cacheFor returns url's cache, creating it on first use.
func (p *Provider) cacheFor(url string) *feedCache {
	p.mu.Lock()
	defer p.mu.Unlock()
	c, ok := p.feeds[url]
	if !ok {
		c = &feedCache{}
		p.feeds[url] = c
	}
	return c
}

// feedCache is one feed's last good copy. Its lock is held across a
// fetch, so concurrent requests for one feed wait for that fetch rather
// than each making their own.
type feedCache struct {
	mu      sync.Mutex
	have    bool
	parsed  []Event
	fetched time.Time
}

// events returns the feed's events, fetching when there is no copy or
// the copy is not younger than refresh. On a failed fetch the last good
// copy, if any, comes back with the error.
//
// The fetch runs under the first caller's ctx while the lock is held, so
// a waiter can't give up on its own ctx; it waits for that fetch to
// finish. That wait is bounded because every caller goes through
// the day data module (daydata), whose fetch always sets a deadline. A caller
// without one would make the others wait as long as the upstream takes.
func (c *feedCache) events(ctx context.Context, p *Provider, url string, refresh time.Duration) ([]Event, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.have && p.now().Sub(c.fetched) < refresh {
		return c.parsed, nil
	}
	parsed, err := fetchFeed(ctx, p.client, url)
	if err != nil {
		return c.parsed, err
	}
	c.have, c.parsed, c.fetched = true, parsed, p.now()
	return c.parsed, nil
}

// withRules returns events with rules applied, in a new slice so the
// shared cached copy is never edited. An excluded override is kept as a
// cancellation: it still stands in for its occurrence, and dropping it
// would bring that occurrence back at its usual time.
func withRules(events []Event, rules []Rule) []Event {
	out := make([]Event, 0, len(events))
	for _, e := range events {
		e, keep := applyRules(e, rules)
		if !keep {
			if !e.IsOverride() {
				continue
			}
			e.Cancelled = true
		}
		out = append(out, e)
	}
	return out
}
