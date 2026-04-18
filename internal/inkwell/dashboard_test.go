package inkwell

import (
	"testing"
	"time"
)

func TestDashboard_SingleScreen(t *testing.T) {
	s := NewScreen("only", nil)
	d := NewDashboard([]*Screen{s}, 0, nil)

	got := d.CurrentScreen()
	if got != s {
		t.Errorf("CurrentScreen = %v, want %v", got, s)
	}
	// Call again - should still be the same screen.
	if d.CurrentScreen() != s {
		t.Error("CurrentScreen changed unexpectedly")
	}
}

func TestDashboard_NoScreens(t *testing.T) {
	d := NewDashboard(nil, 0, nil)
	if got := d.CurrentScreen(); got != nil {
		t.Errorf("CurrentScreen = %v, want nil", got)
	}
}

func TestDashboard_RotatesAfterInterval(t *testing.T) {
	s1 := NewScreen("first", nil)
	s2 := NewScreen("second", nil)
	s3 := NewScreen("third", nil)

	now := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }

	d := NewDashboard([]*Screen{s1, s2, s3}, 5*time.Minute, clock)

	// Initially on first screen.
	if got := d.CurrentScreen(); got != s1 {
		t.Errorf("initial = %q, want first", got.Name)
	}

	// Advance 4 minutes - should still be on first.
	now = now.Add(4 * time.Minute)
	if got := d.CurrentScreen(); got != s1 {
		t.Errorf("after 4m = %q, want first", got.Name)
	}

	// Advance to 5 minutes - should rotate to second.
	now = now.Add(1 * time.Minute)
	if got := d.CurrentScreen(); got != s2 {
		t.Errorf("after 5m = %q, want second", got.Name)
	}

	// Advance another 5 minutes - should rotate to third.
	now = now.Add(5 * time.Minute)
	if got := d.CurrentScreen(); got != s3 {
		t.Errorf("after 10m = %q, want third", got.Name)
	}

	// Advance another 5 minutes - should wrap to first.
	now = now.Add(5 * time.Minute)
	if got := d.CurrentScreen(); got != s1 {
		t.Errorf("after 15m = %q, want first (wrap)", got.Name)
	}
}

func TestDashboard_ZeroIntervalNeverRotates(t *testing.T) {
	s1 := NewScreen("first", nil)
	s2 := NewScreen("second", nil)

	now := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }

	d := NewDashboard([]*Screen{s1, s2}, 0, clock)

	// Advance a long time - should never rotate.
	now = now.Add(24 * time.Hour)
	if got := d.CurrentScreen(); got != s1 {
		t.Errorf("after 24h with 0 interval = %q, want first", got.Name)
	}
}
