package inkwell

import "testing"

func TestRefreshPlanner_FirstCycleIsFull(t *testing.T) {
	p := newRefreshPlanner(BW, 60)
	if got, periodic := p.next(true); got != refreshFull || !periodic {
		t.Errorf("first BW cycle = (%v, periodic=%v), want (refreshFull, periodic=true)", got, periodic)
	}
}

func TestRefreshPlanner_RoutineBWCycleIsFast(t *testing.T) {
	p := newRefreshPlanner(BW, 60)
	p.next(true) // cycle 1: full
	if got, periodic := p.next(true); got != refreshFast || periodic {
		t.Errorf("routine BW cycle = (%v, periodic=%v), want (refreshFast, periodic=false)", got, periodic)
	}
}

func TestRefreshPlanner_UnchangedRoutineCycleSkips(t *testing.T) {
	p := newRefreshPlanner(BW, 60)
	p.next(true) // cycle 1: full
	if got, periodic := p.next(false); got != refreshSkip || periodic {
		t.Errorf("unchanged BW cycle = (%v, periodic=%v), want (refreshSkip, periodic=false)", got, periodic)
	}
}

func TestRefreshPlanner_Gray4ChangedIsGray(t *testing.T) {
	p := newRefreshPlanner(Gray4, 60)
	p.next(true) // cycle 1: gray (full-due)
	if got, periodic := p.next(true); got != refreshGray || periodic {
		t.Errorf("Gray4 changed cycle = (%v, periodic=%v), want (refreshGray, periodic=false)", got, periodic)
	}
}

func TestRefreshPlanner_Gray4UnchangedSkips(t *testing.T) {
	p := newRefreshPlanner(Gray4, 60)
	p.next(true) // cycle 1: gray
	if got, periodic := p.next(false); got != refreshSkip || periodic {
		t.Errorf("Gray4 unchanged cycle = (%v, periodic=%v), want (refreshSkip, periodic=false)", got, periodic)
	}
}

func TestRefreshPlanner_FullCadenceRecurs(t *testing.T) {
	p := newRefreshPlanner(BW, 3)
	p.next(true) // cycle 1: full
	p.next(true) // cycle 2: fast
	if got, periodic := p.next(true); got != refreshFull || !periodic {
		t.Errorf("3rd BW cycle = (%v, periodic=%v), want (refreshFull, periodic=true)", got, periodic)
	}
}

func TestRefreshPlanner_FullDueOnUnchangedFrame(t *testing.T) {
	// A full refresh is forced on the cadence even when content is static,
	// so the panel still refreshes periodically (burn-in protection).
	p := newRefreshPlanner(Gray4, 2)
	p.next(true) // cycle 1: gray
	if got, periodic := p.next(false); got != refreshGray || !periodic {
		t.Errorf("Gray4 full-due unchanged cycle = (%v, periodic=%v), want (refreshGray, periodic=true)", got, periodic)
	}
}

func TestRefreshPlanner_Gray4PeriodicCadenceRecursAsPeriodic(t *testing.T) {
	// Every Gray4 refresh (periodic or routine) maps to the same refreshGray
	// kind, since Gray4 has only one waveform. The planner must still flag
	// the cadence tick as periodic so the caller can force a genuine
	// hardware re-init (the label alone can't signal it — it never changes).
	p := newRefreshPlanner(Gray4, 3)
	p.next(true) // cycle 1: periodic (tick == 1)
	p.next(true) // cycle 2: routine
	if got, periodic := p.next(true); got != refreshGray || !periodic {
		t.Errorf("3rd Gray4 cycle = (%v, periodic=%v), want (refreshGray, periodic=true)", got, periodic)
	}
}
