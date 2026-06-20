package inkwell

import "testing"

func TestRefreshPlanner_FirstCycleIsFull(t *testing.T) {
	p := newRefreshPlanner(BW, 60)
	if got := p.next(true); got != refreshFull {
		t.Errorf("first BW cycle = %v, want refreshFull", got)
	}
}

func TestRefreshPlanner_RoutineBWCycleIsFast(t *testing.T) {
	p := newRefreshPlanner(BW, 60)
	p.next(true) // cycle 1: full
	if got := p.next(true); got != refreshFast {
		t.Errorf("routine BW cycle = %v, want refreshFast", got)
	}
}

func TestRefreshPlanner_UnchangedRoutineCycleSkips(t *testing.T) {
	p := newRefreshPlanner(BW, 60)
	p.next(true) // cycle 1: full
	if got := p.next(false); got != refreshSkip {
		t.Errorf("unchanged BW cycle = %v, want refreshSkip", got)
	}
}

func TestRefreshPlanner_Gray4ChangedIsGray(t *testing.T) {
	p := newRefreshPlanner(Gray4, 60)
	p.next(true) // cycle 1: gray (full-due)
	if got := p.next(true); got != refreshGray {
		t.Errorf("Gray4 changed cycle = %v, want refreshGray", got)
	}
}

func TestRefreshPlanner_Gray4UnchangedSkips(t *testing.T) {
	p := newRefreshPlanner(Gray4, 60)
	p.next(true) // cycle 1: gray
	if got := p.next(false); got != refreshSkip {
		t.Errorf("Gray4 unchanged cycle = %v, want refreshSkip", got)
	}
}

func TestRefreshPlanner_FullCadenceRecurs(t *testing.T) {
	p := newRefreshPlanner(BW, 3)
	p.next(true)                                 // cycle 1: full
	p.next(true)                                 // cycle 2: fast
	if got := p.next(true); got != refreshFull { // cycle 3: full again
		t.Errorf("3rd BW cycle = %v, want refreshFull", got)
	}
}

func TestRefreshPlanner_FullDueOnUnchangedFrame(t *testing.T) {
	// A full refresh is forced on the cadence even when content is static,
	// so the panel still refreshes periodically (burn-in protection).
	p := newRefreshPlanner(Gray4, 2)
	p.next(true)                                  // cycle 1: gray
	if got := p.next(false); got != refreshGray { // cycle 2: full-due despite no change
		t.Errorf("Gray4 full-due unchanged cycle = %v, want refreshGray", got)
	}
}
