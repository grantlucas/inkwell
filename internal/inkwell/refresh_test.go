package inkwell

import "testing"

func TestRefreshPlanner_FirstCycleIsFull(t *testing.T) {
	p := newRefreshPlanner(BW, 60, 10)
	if got := p.next(true); got != refreshFull {
		t.Errorf("first BW cycle = %v, want refreshFull", got)
	}
}

func TestRefreshPlanner_RoutineBWCycleIsPartial(t *testing.T) {
	p := newRefreshPlanner(BW, 60, 10)
	p.next(true) // cycle 1: full
	if got := p.next(true); got != refreshPartial {
		t.Errorf("routine BW cycle = %v, want refreshPartial", got)
	}
}

func TestRefreshPlanner_UnchangedRoutineCycleSkips(t *testing.T) {
	p := newRefreshPlanner(BW, 60, 10)
	p.next(true) // cycle 1: full
	if got := p.next(false); got != refreshSkip {
		t.Errorf("unchanged BW cycle = %v, want refreshSkip", got)
	}
}

func TestRefreshPlanner_FastCadence(t *testing.T) {
	p := newRefreshPlanner(BW, 60, 5)
	for range 4 { // cycles 1-4 (1 full, 2-4 partial)
		p.next(true)
	}
	if got := p.next(true); got != refreshFast { // cycle 5
		t.Errorf("5th BW cycle = %v, want refreshFast", got)
	}
}

func TestRefreshPlanner_Gray4ChangedIsGray(t *testing.T) {
	p := newRefreshPlanner(Gray4, 60, 10)
	p.next(true) // cycle 1: gray (full-due)
	if got := p.next(true); got != refreshGray {
		t.Errorf("Gray4 changed cycle = %v, want refreshGray", got)
	}
}

func TestRefreshPlanner_Gray4UnchangedSkips(t *testing.T) {
	p := newRefreshPlanner(Gray4, 60, 10)
	p.next(true) // cycle 1: gray
	if got := p.next(false); got != refreshSkip {
		t.Errorf("Gray4 unchanged cycle = %v, want refreshSkip", got)
	}
}

func TestRefreshPlanner_FullCadenceRecurs(t *testing.T) {
	p := newRefreshPlanner(BW, 3, 0)
	p.next(true)                                 // cycle 1: full
	p.next(true)                                 // cycle 2: partial
	if got := p.next(true); got != refreshFull { // cycle 3: full again
		t.Errorf("3rd BW cycle = %v, want refreshFull", got)
	}
}

func TestRefreshPlanner_FullDueOnUnchangedFrame(t *testing.T) {
	// A full refresh is forced on the cadence even when content is static,
	// so the panel still refreshes periodically (burn-in protection).
	p := newRefreshPlanner(Gray4, 2, 0)
	p.next(true)                                  // cycle 1: gray
	if got := p.next(false); got != refreshGray { // cycle 2: full-due despite no change
		t.Errorf("Gray4 full-due unchanged cycle = %v, want refreshGray", got)
	}
}
