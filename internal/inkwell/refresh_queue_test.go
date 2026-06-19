package inkwell

import (
	"testing"
	"time"
)

// at builds a time on a fixed date at the given hour:minute (local), which is
// all refreshSchedule.anyDue inspects (minute-of-day).
func at(hour, minTd int) time.Time {
	return time.Date(2026, 6, 19, hour, minTd, 0, 0, time.Local)
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
		{"sub-minute never due", []time.Duration{30 * time.Second}, at(10, 5), false},
		{"sub-minute + 5m due on the mark", []time.Duration{30 * time.Second, 5 * time.Minute}, at(10, 10), true},
		{"sub-minute + 5m not due off mark", []time.Duration{30 * time.Second, 5 * time.Minute}, at(10, 11), false},
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

func TestScreen_AnyDue(t *testing.T) {
	s := NewScreen("s", nil)
	s.schedule = refreshSchedule{cadences: []time.Duration{5 * time.Minute}}
	if !s.AnyDue(at(10, 5)) {
		t.Error("AnyDue(10:05) = false, want true (5m due on the mark)")
	}
	if s.AnyDue(at(10, 7)) {
		t.Error("AnyDue(10:07) = true, want false (off the 5m mark)")
	}
}
