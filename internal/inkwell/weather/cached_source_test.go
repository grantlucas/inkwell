package weather

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type countingSource struct {
	mu       sync.Mutex
	calls    int
	forecast *Forecast
	err      error
}

func (s *countingSource) Forecast(_ context.Context, _ Location, _ int) (*Forecast, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	return s.forecast, nil
}

func TestCachedSource_CachesWithinTTL(t *testing.T) {
	now := time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC)
	inner := &countingSource{forecast: &Forecast{Days: []DailyForecast{{High: 20}}}}
	cached := NewCachedSource(inner, time.Hour, func() time.Time { return now })

	loc := Location{Latitude: 45.4, Longitude: -75.7}
	fc1, err := cached.Forecast(context.Background(), loc, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	fc2, err := cached.Forecast(context.Background(), loc, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if inner.calls != 1 {
		t.Errorf("inner called %d times, want 1 (cached)", inner.calls)
	}
	if fc1.Days[0].High != fc2.Days[0].High {
		t.Error("cached result differs")
	}
}

func TestCachedSource_RefreshesAfterTTL(t *testing.T) {
	now := time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC)
	inner := &countingSource{forecast: &Forecast{Days: []DailyForecast{{High: 20}}}}
	cached := NewCachedSource(inner, time.Hour, func() time.Time { return now })

	loc := Location{Latitude: 45.4, Longitude: -75.7}
	_, _ = cached.Forecast(context.Background(), loc, 1)

	now = now.Add(2 * time.Hour)
	_, _ = cached.Forecast(context.Background(), loc, 1)

	if inner.calls != 2 {
		t.Errorf("inner called %d times, want 2 (TTL expired)", inner.calls)
	}
}

func TestCachedSource_StaleOnError(t *testing.T) {
	now := time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC)
	inner := &countingSource{forecast: &Forecast{Days: []DailyForecast{{High: 20}}}}
	cached := NewCachedSource(inner, time.Hour, func() time.Time { return now })

	loc := Location{Latitude: 45.4, Longitude: -75.7}
	_, _ = cached.Forecast(context.Background(), loc, 1)

	now = now.Add(2 * time.Hour)
	inner.err = errors.New("network error")
	inner.forecast = nil

	fc, err := cached.Forecast(context.Background(), loc, 1)
	if err == nil {
		t.Fatal("expected error")
	}
	if fc == nil {
		t.Fatal("expected stale forecast")
	}
	if fc.Days[0].High != 20 {
		t.Errorf("stale High = %v, want 20", fc.Days[0].High)
	}
}

func TestCachedSource_ErrorNoCache(t *testing.T) {
	inner := &countingSource{err: errors.New("fail")}
	cached := NewCachedSource(inner, time.Hour, time.Now)

	_, err := cached.Forecast(context.Background(), Location{}, 1)
	if err == nil {
		t.Fatal("expected error with no cache")
	}
}

func TestCachedSource_DifferentLocation(t *testing.T) {
	now := time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC)
	inner := &countingSource{forecast: &Forecast{Days: []DailyForecast{{High: 20}}}}
	cached := NewCachedSource(inner, time.Hour, func() time.Time { return now })

	loc1 := Location{Latitude: 45.4, Longitude: -75.7}
	loc2 := Location{Latitude: 43.7, Longitude: -79.4}
	_, _ = cached.Forecast(context.Background(), loc1, 1)
	_, _ = cached.Forecast(context.Background(), loc2, 1)

	if inner.calls != 2 {
		t.Errorf("inner called %d times, want 2 (different location)", inner.calls)
	}
}

func TestCachedSource_SameRoundedLocation(t *testing.T) {
	now := time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC)
	inner := &countingSource{forecast: &Forecast{Days: []DailyForecast{{High: 20}}}}
	cached := NewCachedSource(inner, time.Hour, func() time.Time { return now })

	loc1 := Location{Latitude: 45.42, Longitude: -75.69}
	loc2 := Location{Latitude: 45.44, Longitude: -75.71}
	_, _ = cached.Forecast(context.Background(), loc1, 1)
	_, _ = cached.Forecast(context.Background(), loc2, 1)

	if inner.calls != 1 {
		t.Errorf("inner called %d times, want 1 (same rounded location)", inner.calls)
	}
}

func TestCachedSource_ConcurrentAccess(t *testing.T) {
	now := time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC)
	inner := &countingSource{forecast: &Forecast{Days: []DailyForecast{{High: 20}}}}
	cached := NewCachedSource(inner, time.Hour, func() time.Time { return now })

	loc := Location{Latitude: 45.4, Longitude: -75.7}
	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			_, _ = cached.Forecast(context.Background(), loc, 1)
		})
	}
	wg.Wait()

	inner.mu.Lock()
	calls := inner.calls
	inner.mu.Unlock()

	if calls < 1 || calls > 10 {
		t.Errorf("inner called %d times, expected 1-10", calls)
	}
}

func TestCachedSource_StaleOnErrorDifferentLocation(t *testing.T) {
	now := time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC)
	inner := &countingSource{forecast: &Forecast{Days: []DailyForecast{{High: 20}}}}
	cached := NewCachedSource(inner, time.Hour, func() time.Time { return now })

	loc1 := Location{Latitude: 45.4, Longitude: -75.7}
	_, _ = cached.Forecast(context.Background(), loc1, 1)

	now = now.Add(2 * time.Hour)
	inner.err = errors.New("fail")
	inner.forecast = nil

	loc2 := Location{Latitude: 40.0, Longitude: -74.0}
	fc, err := cached.Forecast(context.Background(), loc2, 1)
	if err == nil {
		t.Fatal("expected error")
	}
	if fc != nil {
		t.Error("expected nil forecast for different location with error")
	}
}
