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
	if got, _ := d.Advance(); got != s1 {
		t.Errorf("initial = %q, want first", got.Name)
	}

	// Advance 4 minutes - should still be on first.
	now = now.Add(4 * time.Minute)
	if got, _ := d.Advance(); got != s1 {
		t.Errorf("after 4m = %q, want first", got.Name)
	}

	// Advance to 5 minutes - should rotate to second.
	now = now.Add(1 * time.Minute)
	if got, _ := d.Advance(); got != s2 {
		t.Errorf("after 5m = %q, want second", got.Name)
	}

	// Advance another 5 minutes - should rotate to third.
	now = now.Add(5 * time.Minute)
	if got, _ := d.Advance(); got != s3 {
		t.Errorf("after 10m = %q, want third", got.Name)
	}

	// Advance another 5 minutes - should wrap to first.
	now = now.Add(5 * time.Minute)
	if got, _ := d.Advance(); got != s1 {
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
	if got, _ := d.Advance(); got != s3 {
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
	if got, _ := d.Advance(); got != s1 {
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

	// These are sequential steps against one Dashboard, not independent
	// cases: the rotation flag is consume-once, so a step only means what
	// it means after the ones before it. They deliberately do not run as
	// subtests — narrowing to a single step with -run would replay it
	// against the wrong history and fail spuriously.
	steps := []struct {
		label       string
		at          time.Time
		wantScreen  *Screen
		wantRotated bool
	}{
		{"first call does not count as a rotation", now, s1, false},
		{"before the interval elapses", now.Add(4 * time.Minute), s1, false},
		{"the interval elapses", now.Add(5 * time.Minute), s2, true},
		{"same screen on the next call", now.Add(5 * time.Minute), s2, false},
		{"wraps back round", now.Add(10 * time.Minute), s1, true},
	}
	for _, step := range steps {
		now = step.at
		got, rotated := d.Advance()
		if got != step.wantScreen {
			t.Errorf("%s: screen = %q, want %q", step.label, got.Name, step.wantScreen.Name)
		}
		if rotated != step.wantRotated {
			t.Errorf("%s: rotated = %v, want %v", step.label, rotated, step.wantRotated)
		}
	}
}

// CurrentScreen is the read-only view: it must report what Advance last
// selected without itself moving the rotation on, or a second caller (a
// preview handler reporting the screen name, say) would consume the
// rotation flag the render loop needs and put #96 back intermittently.
func TestDashboard_CurrentScreenDoesNotAdvance(t *testing.T) {
	s1 := NewScreen("first", nil)
	s2 := NewScreen("second", nil)

	now := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	d := NewDashboard([]*Screen{s1, s2}, 5*time.Minute, clock)

	// Well past the rotation interval, but nothing has advanced yet.
	// 25m is five intervals, an odd number, so it genuinely lands on the
	// other screen — an even multiple would wrap back to the first and
	// report no rotation for the wrong reason.
	now = now.Add(25 * time.Minute)
	for range 3 {
		if got := d.CurrentScreen(); got != s1 {
			t.Fatalf("CurrentScreen = %q, want first — it must not rotate", got.Name)
		}
	}

	// The render loop's Advance still sees the rotation waiting for it.
	got, rotated := d.Advance()
	if !rotated {
		t.Error("rotated = false, want true — CurrentScreen swallowed the rotation")
	}
	if got != s2 {
		t.Errorf("screen = %q, want second", got.Name)
	}
}

func TestDashboard_CurrentScreenNoScreens(t *testing.T) {
	if got := NewDashboard(nil, 0, nil).CurrentScreen(); got != nil {
		t.Errorf("CurrentScreen = %v, want nil", got)
	}
}
