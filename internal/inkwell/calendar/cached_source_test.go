package calendar

import (
	"context"
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

func (s *countingSource) Events(_ context.Context, _, _ time.Time) ([]Event, error) {
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
	got, err := src.Events(context.Background(), start, end)
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
	got, err = src.Events(context.Background(), start, end)
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
	_, _ = src.Events(context.Background(), start, end)
	if inner.callCount() != 1 {
		t.Fatalf("inner calls = %d, want 1", inner.callCount())
	}

	// Advance time past TTL.
	currentTime = now.Add(16 * time.Minute)

	_, _ = src.Events(context.Background(), start, end)
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
	_, _ = src.Events(context.Background(), start, end)

	// Make inner return error and advance past TTL.
	inner.mu.Lock()
	inner.err = fmt.Errorf("network down")
	inner.mu.Unlock()
	currentTime = now.Add(16 * time.Minute)

	got, err := src.Events(context.Background(), start, end)
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
	got, err := src.Events(context.Background(), start, end)
	if err == nil {
		t.Fatal("expected error")
	}
	if got != nil {
		t.Fatalf("got %d events on first error, want nil", len(got))
	}
}

// TestCachedSource_ExpandsRecurringEvents confirms the cache layer
// runs RRULE expansion at filter time. A real feed contains a master
// recurring event; the dashboard wants concrete occurrences per its
// requested window. Without this wiring, only the master DTSTART
// would surface and a weekly standup would appear exactly once.
func TestCachedSource_ExpandsRecurringEvents(t *testing.T) {
	now := time.Date(2026, 4, 27, 0, 0, 0, 0, time.UTC) // Monday
	master := Event{
		UID:     "weekly",
		Summary: "Standup",
		Start:   time.Date(2026, 4, 27, 9, 0, 0, 0, time.UTC),
		End:     time.Date(2026, 4, 27, 9, 30, 0, 0, time.UTC),
		Recurrence: &Recurrence{
			Freq:  FreqDaily,
			Count: 5,
		},
	}
	inner := &countingSource{events: []Event{master}}
	src := NewCachedSource(inner, 15*time.Minute, func() time.Time { return now })

	got, err := src.Events(context.Background(), now, now.Add(7*24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	// COUNT=5 fits inside a 7-day window → 5 expanded occurrences.
	if len(got) != 5 {
		t.Fatalf("got %d, want 5 (%v)", len(got), got)
	}
	// Assert actual instants, not just count — a count-only check
	// would pass even if expansion emitted five wrong/duplicate days.
	baseStart := time.Date(2026, 4, 27, 9, 0, 0, 0, time.UTC)
	baseEnd := time.Date(2026, 4, 27, 9, 30, 0, 0, time.UTC)
	for i, e := range got {
		wantStart := baseStart.AddDate(0, 0, i)
		wantEnd := baseEnd.AddDate(0, 0, i)
		if !e.Start.Equal(wantStart) || !e.End.Equal(wantEnd) {
			t.Errorf("occ[%d] = [%s,%s), want [%s,%s)", i, e.Start, e.End, wantStart, wantEnd)
		}
	}
	// Each occurrence must be flattened (no Recurrence pointer leaking
	// downstream — widgets would otherwise re-expand recursively).
	for i, e := range got {
		if e.Recurrence != nil {
			t.Errorf("occ[%d] still has Recurrence", i)
		}
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
		wg.Go(func() {
			_, _ = src.Events(context.Background(), start, end)
		})
	}
	wg.Wait()

	// Should have populated cache; no panics from concurrent access.
	if inner.callCount() == 0 {
		t.Fatal("inner was never called")
	}
}
