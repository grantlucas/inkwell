package eventlist

import "testing"

// truncate cuts on characters and marks the cut with » when there is
// room for one. Today's callers always give it at least minChars, so the
// smallest budgets are pinned here rather than through Draw: a budget
// of one or less must cut cleanly, never panic.
func TestTruncate(t *testing.T) {
	tests := []struct {
		label    string
		in       string
		maxChars int
		want     string
	}{
		{"fits", "Standup", 10, "Standup"},
		{"exactly fits", "Standup", 7, "Standup"},
		{"cut with »", "Standup", 5, "Stan»"},
		{"cut on characters", "Ñandúñandú", 6, "Ñandú»"},
		{"no room for »", "Standup", 1, "S"},
		{"no room at all", "Standup", 0, ""},
		{"a negative budget is no room", "Standup", -2, ""},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			if got := truncate(tt.in, tt.maxChars); got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.in, tt.maxChars, got, tt.want)
			}
		})
	}
}
