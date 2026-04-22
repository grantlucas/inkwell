package datasource

import (
	"errors"
	"testing"
	"time"
)

func TestCached_FirstFetchSuccess(t *testing.T) {
	calls := 0
	c := NewCached(func() (string, error) {
		calls++
		return "hello", nil
	}, time.Minute, time.Now)

	got, err := c.Get()
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "hello" {
		t.Errorf("Get() = %q, want %q", got, "hello")
	}
	if calls != 1 {
		t.Errorf("fetch called %d times, want 1", calls)
	}
}

func TestCached_CachedWithinTTL(t *testing.T) {
	now := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }

	calls := 0
	c := NewCached(func() (int, error) {
		calls++
		return 42, nil
	}, time.Minute, clock)

	if _, err := c.Get(); err != nil {
		t.Fatalf("first Get: %v", err)
	}

	// Advance time but stay within TTL.
	now = now.Add(30 * time.Second)
	got, err := c.Get()
	if err != nil {
		t.Fatalf("second Get: %v", err)
	}
	if got != 42 {
		t.Errorf("Get() = %d, want 42", got)
	}
	if calls != 1 {
		t.Errorf("fetch called %d times, want 1", calls)
	}
}

func TestCached_RefreshAfterTTL(t *testing.T) {
	now := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }

	calls := 0
	c := NewCached(func() (string, error) {
		calls++
		if calls == 1 {
			return "first", nil
		}
		return "second", nil
	}, time.Minute, clock)

	if _, err := c.Get(); err != nil {
		t.Fatalf("first Get: %v", err)
	}

	// Advance past TTL.
	now = now.Add(2 * time.Minute)
	got, err := c.Get()
	if err != nil {
		t.Fatalf("second Get: %v", err)
	}
	if got != "second" {
		t.Errorf("Get() = %q, want %q", got, "second")
	}
	if calls != 2 {
		t.Errorf("fetch called %d times, want 2", calls)
	}
}

func TestCached_StaleOnError(t *testing.T) {
	now := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }

	calls := 0
	c := NewCached(func() (string, error) {
		calls++
		if calls == 1 {
			return "good", nil
		}
		return "", errors.New("fetch failed")
	}, time.Minute, clock)

	if _, err := c.Get(); err != nil {
		t.Fatalf("first Get: %v", err)
	}

	// Advance past TTL — fetch will fail.
	now = now.Add(2 * time.Minute)
	got, err := c.Get()
	if err != nil {
		t.Fatalf("stale-on-error should return nil error, got: %v", err)
	}
	if got != "good" {
		t.Errorf("Get() = %q, want stale value %q", got, "good")
	}
}

func TestCached_FirstFetchError(t *testing.T) {
	c := NewCached(func() (string, error) {
		return "", errors.New("unavailable")
	}, time.Minute, time.Now)

	got, err := c.Get()
	if err == nil {
		t.Fatal("expected error on first fetch failure")
	}
	if got != "" {
		t.Errorf("Get() = %q, want zero value", got)
	}
}

func TestCached_ErrorThenRecovery(t *testing.T) {
	now := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }

	calls := 0
	c := NewCached(func() (string, error) {
		calls++
		switch calls {
		case 1:
			return "good", nil
		case 2:
			return "", errors.New("transient error")
		default:
			return "recovered", nil
		}
	}, time.Minute, clock)

	// First fetch succeeds.
	if _, err := c.Get(); err != nil {
		t.Fatalf("first Get: %v", err)
	}

	// Second fetch fails — stale served.
	now = now.Add(2 * time.Minute)
	got, err := c.Get()
	if err != nil {
		t.Fatalf("stale-on-error: %v", err)
	}
	if got != "good" {
		t.Errorf("Get() = %q, want stale %q", got, "good")
	}

	// Third fetch recovers.
	now = now.Add(2 * time.Minute)
	got, err = c.Get()
	if err != nil {
		t.Fatalf("recovery Get: %v", err)
	}
	if got != "recovered" {
		t.Errorf("Get() = %q, want %q", got, "recovered")
	}
}
