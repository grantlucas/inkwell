package inkwell

import "testing"

// TestRefreshPlanner covers the planner's decision table. The planner only
// picks a waveform; App.refresh runs the full hardware lifecycle (reset, init,
// display, sleep) around every non-skip decision regardless of mode, so there
// is no separate "force re-init" signal to plan.
func TestRefreshPlanner(t *testing.T) {
	tests := []struct {
		label     string
		color     ColorDepth
		fullEvery int
		changed   []bool // one entry per cycle; the last entry is the one asserted
		want      refreshKind
	}{
		{"BW first cycle is full", BW, 60, []bool{true}, refreshFull},
		{"BW routine changed cycle is fast", BW, 60, []bool{true, true}, refreshFast},
		{"BW unchanged routine cycle skips", BW, 60, []bool{true, false}, refreshSkip},
		{"BW full cadence recurs", BW, 3, []bool{true, true, true}, refreshFull},
		{"BW full cadence fires on an unchanged frame", BW, 2, []bool{true, false}, refreshFull},
		{"Gray4 first cycle is gray", Gray4, 60, []bool{true}, refreshGray},
		{"Gray4 changed cycle is gray", Gray4, 60, []bool{true, true}, refreshGray},
		{"Gray4 unchanged cycle skips", Gray4, 60, []bool{true, false}, refreshSkip},
		{"Gray4 cadence fires on an unchanged frame", Gray4, 2, []bool{true, false}, refreshGray},
		{"cadence of zero never recurs", BW, 0, []bool{true, false, false}, refreshSkip},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			p := newRefreshPlanner(tt.color, tt.fullEvery)
			var got refreshKind
			for _, changed := range tt.changed {
				got = p.next(changed)
			}
			if got != tt.want {
				t.Errorf("planner = %v, want %v", got, tt.want)
			}
		})
	}
}
