package calendar

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// countingSource wraps a Source and counts calls.
type countingSource struct {
	mu     sync.Mutex
	calls  int
	events []Event
	err    error
}

func (s *countingSource) Events(start, end time.Time) ([]Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	return s.events, s.err
}

func (s *countingSource) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

func TestCachedSource_CachesWithinTTL(t *testing.T) {
	now := time.Date(2026, 4, 25, 9, 0, 0, 0, time.UTC)
	events := []Event{
		{UID: "1", Summary: "Test", Start: now, End: now.Add(time.Hour)},
	}
	inner := &countingSource{events: events}
	src := NewCachedSource(inner, 15*time.Minute, func() time.Time { return now })

	start := now
	end := now.Add(24 * time.Hour)

	// First call should fetch.
	got, err := src.Events(start, end)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d events, want 1", len(got))
	}
	if inner.callCount() != 1 {
		t.Fatalf("inner calls = %d, want 1", inner.callCount())
	}

	// Second call within TTL should use cache.
	got, err = src.Events(start, end)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d events, want 1", len(got))
	}
	if inner.callCount() != 1 {
		t.Fatalf("inner calls = %d, want 1 (should have used cache)", inner.callCount())
	}
}

func TestCachedSource_RefetchesAfterTTL(t *testing.T) {
	now := time.Date(2026, 4, 25, 9, 0, 0, 0, time.UTC)
	events := []Event{
		{UID: "1", Summary: "Test", Start: now, End: now.Add(time.Hour)},
	}
	inner := &countingSource{events: events}

	currentTime := now
	src := NewCachedSource(inner, 15*time.Minute, func() time.Time { return currentTime })

	start := now
	end := now.Add(24 * time.Hour)

	// First call.
	_, _ = src.Events(start, end)
	if inner.callCount() != 1 {
		t.Fatalf("inner calls = %d, want 1", inner.callCount())
	}

	// Advance time past TTL.
	currentTime = now.Add(16 * time.Minute)

	_, _ = src.Events(start, end)
	if inner.callCount() != 2 {
		t.Fatalf("inner calls = %d, want 2 (should have re-fetched)", inner.callCount())
	}
}

func TestCachedSource_ReturnsStaleCacheOnError(t *testing.T) {
	now := time.Date(2026, 4, 25, 9, 0, 0, 0, time.UTC)
	events := []Event{
		{UID: "1", Summary: "Stale", Start: now, End: now.Add(time.Hour)},
	}
	inner := &countingSource{events: events}

	currentTime := now
	src := NewCachedSource(inner, 15*time.Minute, func() time.Time { return currentTime })

	start := now
	end := now.Add(24 * time.Hour)

	// First call succeeds.
	_, _ = src.Events(start, end)

	// Make inner return error and advance past TTL.
	inner.mu.Lock()
	inner.err = fmt.Errorf("network down")
	inner.mu.Unlock()
	currentTime = now.Add(16 * time.Minute)

	got, err := src.Events(start, end)
	if err == nil {
		t.Fatal("expected error")
	}
	// Should still return stale data.
	if len(got) != 1 {
		t.Fatalf("got %d stale events, want 1", len(got))
	}
	if got[0].Summary != "Stale" {
		t.Errorf("Summary = %q, want %q", got[0].Summary, "Stale")
	}
}

func TestCachedSource_FirstCallError(t *testing.T) {
	inner := &countingSource{err: fmt.Errorf("network error")}
	now := time.Date(2026, 4, 25, 9, 0, 0, 0, time.UTC)
	src := NewCachedSource(inner, 15*time.Minute, func() time.Time { return now })

	start := now
	end := now.Add(24 * time.Hour)
	got, err := src.Events(start, end)
	if err == nil {
		t.Fatal("expected error")
	}
	if got != nil {
		t.Fatalf("got %d events on first error, want nil", len(got))
	}
}

func TestCachedSource_ConcurrentAccess(t *testing.T) {
	now := time.Date(2026, 4, 25, 9, 0, 0, 0, time.UTC)
	events := []Event{
		{UID: "1", Summary: "Test", Start: now, End: now.Add(time.Hour)},
	}
	inner := &countingSource{events: events}
	src := NewCachedSource(inner, 15*time.Minute, func() time.Time { return now })

	start := now
	end := now.Add(24 * time.Hour)

	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = src.Events(start, end)
		}()
	}
	wg.Wait()

	// Should have populated cache; no panics from concurrent access.
	if inner.callCount() == 0 {
		t.Fatal("inner was never called")
	}
}
