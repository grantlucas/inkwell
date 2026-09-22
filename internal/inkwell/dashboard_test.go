package inkwell

import (
	"testing"
	"time"
)

func TestDashboard_SingleScreen(t *testing.T) {
	s := NewScreen("only", nil)
	d := NewDashboard([]*Screen{s}, 0, nil)

	got, _ := d.CurrentScreen()
	if got != s {
		t.Errorf("CurrentScreen = %v, want %v", got, s)
	}
	// Call again - should still be the same screen.
	if cur, _ := d.CurrentScreen(); cur != s {
		t.Error("CurrentScreen changed unexpectedly")
	}
}

func TestDashboard_NoScreens(t *testing.T) {
	d := NewDashboard(nil, 0, nil)
	if got, _ := d.CurrentScreen(); got != nil {
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
	if got, _ := d.CurrentScreen(); got != s1 {
		t.Errorf("initial = %q, want first", got.Name)
	}

	// Advance 4 minutes - should still be on first.
	now = now.Add(4 * time.Minute)
	if got, _ := d.CurrentScreen(); got != s1 {
		t.Errorf("after 4m = %q, want first", got.Name)
	}

	// Advance to 5 minutes - should rotate to second.
	now = now.Add(1 * time.Minute)
	if got, _ := d.CurrentScreen(); got != s2 {
		t.Errorf("after 5m = %q, want second", got.Name)
	}

	// Advance another 5 minutes - should rotate to third.
	now = now.Add(5 * time.Minute)
	if got, _ := d.CurrentScreen(); got != s3 {
		t.Errorf("after 10m = %q, want third", got.Name)
	}

	// Advance another 5 minutes - should wrap to first.
	now = now.Add(5 * time.Minute)
	if got, _ := d.CurrentScreen(); got != s1 {
		t.Errorf("after 15m = %q, want first (wrap)", got.Name)
	}
}

func TestDashboard_SkipsMultipleIntervals(t *testing.T) {
	s1 := NewScreen("first", nil)
	s2 := NewScreen("second", nil)
	s3 := NewScreen("third", nil)

	now := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }

	d := NewDashboard([]*Screen{s1, s2, s3}, 5*time.Minute, clock)

	// Advance 12 minutes in one jump — should skip 2 intervals (land on third).
	now = now.Add(12 * time.Minute)
	if got, _ := d.CurrentScreen(); got != s3 {
		t.Errorf("after 12m jump = %q, want third", got.Name)
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
	if got, _ := d.CurrentScreen(); got != s1 {
		t.Errorf("after 24h with 0 interval = %q, want first", got.Name)
	}
}

// A rotation is a deliberate, user-visible change of what the panel
// shows, so the caller has to be able to tell one happened — the
// refresh gate keys off it (see App.nextCycle). Reporting it from
// CurrentScreen keeps the signal with the call that actually mutates
// d.current, rather than making callers infer it by comparing screens.
func TestDashboard_ReportsRotation(t *testing.T) {
	s1 := NewScreen("first", nil)
	s2 := NewScreen("second", nil)

	now := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	d := NewDashboard([]*Screen{s1, s2}, 5*time.Minute, clock)

	tests := []struct {
		label       string
		advance     time.Duration
		wantScreen  *Screen
		wantRotated bool
	}{
		{"first call does not count as a rotation", 0, s1, false},
		{"before the interval elapses", 4 * time.Minute, s1, false},
		{"the interval elapses", 1 * time.Minute, s2, true},
		{"same screen on the next call", 0, s2, false},
		{"wraps back round", 5 * time.Minute, s1, true},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			now = now.Add(tt.advance)
			got, rotated := d.CurrentScreen()
			if got != tt.wantScreen {
				t.Errorf("screen = %q, want %q", got.Name, tt.wantScreen.Name)
			}
			if rotated != tt.wantRotated {
				t.Errorf("rotated = %v, want %v", rotated, tt.wantRotated)
			}
		})
	}
}
