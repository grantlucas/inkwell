package weather

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

// countingClient records how many requests it served and the last URL, so a
// test can assert the Provider deduplicates fetches and targets the right
// model endpoint. It is safe for concurrent use so the dedup guarantee can be
// exercised under -race. A fresh body is created per call since bodies are
// consumed.
type countingClient struct {
	mu      sync.Mutex
	calls   int
	lastURL string
}

func (c *countingClient) Do(req *http.Request) (*http.Response, error) {
	c.mu.Lock()
	c.calls++
	c.lastURL = req.URL.String()
	c.mu.Unlock()
	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader(sampleResponse)),
	}, nil
}

// count returns the number of requests served, safe to call after concurrent
// Do calls have been synchronized (e.g. via a WaitGroup).
func (c *countingClient) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.calls
}

func providerClock(t time.Time) func() time.Time {
	return func() time.Time { return t }
}

func TestProvider_ForecastFetchesAndCaches(t *testing.T) {
	now := time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
	client := &countingClient{}
	p := NewProvider(client, time.Hour, providerClock(now), Settings{})

	loc := Location{Latitude: 43.244, Longitude: -79.837}
	fc1, err := p.Forecast(context.Background(), loc, ModelGEM, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fc1 == nil || len(fc1.Days) != 1 {
		t.Fatalf("got %v, want a 1-day forecast", fc1)
	}
	if !strings.Contains(client.lastURL, "/v1/gem") {
		t.Errorf("URL = %q, want gem endpoint", client.lastURL)
	}

	// Second identical call is served from cache: no extra HTTP fetch.
	if _, err := p.Forecast(context.Background(), loc, ModelGEM, 1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client.calls != 1 {
		t.Errorf("client called %d times, want 1 (cached)", client.calls)
	}
}

func TestProvider_DistinctKeysCacheIndependently(t *testing.T) {
	now := time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
	client := &countingClient{}
	p := NewProvider(client, time.Hour, providerClock(now), Settings{})

	loc1 := Location{Latitude: 43.244, Longitude: -79.837}
	loc2 := Location{Latitude: 49.283, Longitude: -123.121}
	ctx := context.Background()

	// Three distinct keys: loc1/gem, loc2/gem, loc1/ecmwf → three fetches.
	_, _ = p.Forecast(ctx, loc1, ModelGEM, 1)
	_, _ = p.Forecast(ctx, loc2, ModelGEM, 1)
	_, _ = p.Forecast(ctx, loc1, ModelECMWF, 1)
	// Repeats of each key are cached — no further fetches, no thrash.
	_, _ = p.Forecast(ctx, loc1, ModelGEM, 1)
	_, _ = p.Forecast(ctx, loc2, ModelGEM, 1)

	if client.calls != 3 {
		t.Errorf("client called %d times, want 3 (one per distinct key)", client.calls)
	}
}

func TestProvider_SourceForModelSharesCache(t *testing.T) {
	now := time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
	client := &countingClient{}
	p := NewProvider(client, time.Hour, providerClock(now), Settings{})
	loc := Location{Latitude: 43.244, Longitude: -79.837}

	src := p.SourceForModel(ModelECMWF)
	if _, err := src.Forecast(context.Background(), loc, 1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(client.lastURL, "/v1/ecmwf") {
		t.Errorf("URL = %q, want ecmwf endpoint", client.lastURL)
	}
	// A direct Provider call for the same key reuses the bound source's cache.
	if _, err := p.Forecast(context.Background(), loc, ModelECMWF, 1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client.calls != 1 {
		t.Errorf("client called %d times, want 1 (shared cache)", client.calls)
	}
}

func TestProvider_Defaults(t *testing.T) {
	want := Settings{
		Location: Location{Latitude: 43.244, Longitude: -79.837},
		Model:    ModelGEM,
		TempUnit: "C",
	}
	p := NewProvider(&countingClient{}, time.Hour, nil, want)
	if got := p.Defaults(); got != want {
		t.Errorf("Defaults() = %+v, want %+v", got, want)
	}
}


func TestProvider_ConcurrentForecastDedupes(t *testing.T) {
	now := time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
	client := &countingClient{}
	p := NewProvider(client, time.Hour, providerClock(now), Settings{})
	loc := Location{Latitude: 43.244, Longitude: -79.837}

	const goroutines = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			_, _ = p.Forecast(context.Background(), loc, ModelGEM, 1)
		}()
	}
	wg.Wait()

	// All concurrent callers for the same key share one cache entry, so the
	// upstream is hit exactly once.
	if got := client.count(); got != 1 {
		t.Errorf("client served %d requests, want 1 (concurrent callers share one fetch)", got)
	}
}
