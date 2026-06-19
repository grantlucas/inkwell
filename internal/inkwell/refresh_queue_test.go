package inkwell

import (
	"image"
	"testing"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/widget"
)

// cadenceWidget is a stub Widget that declares a refresh cadence.
type cadenceWidget struct{ every time.Duration }

func (c cadenceWidget) Bounds() image.Rectangle      { return image.Rectangle{} }
func (c cadenceWidget) Render(*image.Paletted) error { return nil }
func (c cadenceWidget) RefreshEvery() time.Duration  { return c.every }

// plainWidget is a stub Widget that does NOT implement RefreshCadence.
type plainWidget struct{}

func (plainWidget) Bounds() image.Rectangle      { return image.Rectangle{} }
func (plainWidget) Render(*image.Paletted) error { return nil }

// at builds a time on a fixed date at the given hour:minute (local), which is
// all refreshSchedule.anyDue inspects (minute-of-day).
func at(hour, minTd int) time.Time {
	return time.Date(2026, 6, 19, hour, minTd, 0, 0, time.Local)
}

func TestResolveRefreshCadence(t *testing.T) {
	cases := []struct {
		label    string
		w        widget.Widget
		override time.Duration
		want     time.Duration
	}{
		{"override wins over declared", cadenceWidget{every: time.Hour}, 5 * time.Minute, 5 * time.Minute},
		{"declared used when no override", cadenceWidget{every: 15 * time.Minute}, 0, 15 * time.Minute},
		{"default when not implemented", plainWidget{}, 0, time.Minute},
		{"static widget stays static", cadenceWidget{every: 0}, 0, 0},
		{"negative declared is static", cadenceWidget{every: -time.Second}, 0, 0},
		{"sub-minute declared clamps to floor", cadenceWidget{every: 30 * time.Second}, 0, time.Minute},
	}

	for _, c := range cases {
		t.Run(c.label, func(t *testing.T) {
			if got := resolveRefreshCadence(c.w, c.override); got != c.want {
				t.Errorf("resolveRefreshCadence(%v, %v) = %v, want %v", c.w, c.override, got, c.want)
			}
		})
	}
}

func TestBuildSchedule(t *testing.T) {
	t.Run("resolves declared cadences with nil overrides", func(t *testing.T) {
		s := buildSchedule([]widget.Widget{cadenceWidget{5 * time.Minute}, plainWidget{}}, nil)
		want := []time.Duration{5 * time.Minute, time.Minute}
		if len(s.cadences) != len(want) {
			t.Fatalf("len(cadences) = %d, want %d", len(s.cadences), len(want))
		}
		for i, c := range want {
			if s.cadences[i] != c {
				t.Errorf("cadences[%d] = %v, want %v", i, s.cadences[i], c)
			}
		}
	})

	t.Run("applies per-widget override", func(t *testing.T) {
		s := buildSchedule(
			[]widget.Widget{cadenceWidget{time.Hour}},
			[]time.Duration{5 * time.Minute},
		)
		if s.cadences[0] != 5*time.Minute {
			t.Errorf("cadences[0] = %v, want 5m (override)", s.cadences[0])
		}
	})
}

func TestScreen_AnyDue(t *testing.T) {
	s := NewScreen("s", []widget.Widget{cadenceWidget{5 * time.Minute}})
	if !s.AnyDue(at(10, 5)) {
		t.Error("AnyDue(10:05) = false, want true (5m due on the mark)")
	}
	if s.AnyDue(at(10, 7)) {
		t.Error("AnyDue(10:07) = true, want false (off the 5m mark)")
	}
}

func TestRefreshSchedule_AnyDue(t *testing.T) {
	cases := []struct {
		label    string
		cadences []time.Duration
		now      time.Time
		want     bool
	}{
		{"empty schedule never due", nil, at(10, 5), false},
		{"every-minute always due", []time.Duration{time.Minute}, at(10, 7), true},
		{"5m due on the mark", []time.Duration{5 * time.Minute}, at(10, 5), true},
		{"5m not due off the mark", []time.Duration{5 * time.Minute}, at(10, 7), false},
		{"daily due at midnight", []time.Duration{24 * time.Hour}, at(0, 0), true},
		{"daily not due midday", []time.Duration{24 * time.Hour}, at(12, 0), false},
		{"static alone never due", []time.Duration{0}, at(10, 5), false},
		{"static + 5m due on the mark", []time.Duration{0, 5 * time.Minute}, at(10, 10), true},
		{"static + 5m not due off mark", []time.Duration{0, 5 * time.Minute}, at(10, 11), false},
		{"two 5m coalesce on shared mark", []time.Duration{5 * time.Minute, 5 * time.Minute}, at(10, 15), true},
	}

	for _, c := range cases {
		t.Run(c.label, func(t *testing.T) {
			s := refreshSchedule{cadences: c.cadences}
			if got := s.anyDue(c.now); got != c.want {
				t.Errorf("anyDue(%v) = %v, want %v", c.now, got, c.want)
			}
		})
	}
}
