package inkwell

import "testing"

func TestRefreshPlanner_FirstCycleIsFull(t *testing.T) {
	p := newRefreshPlanner(BW, 60)
	if got, forceInit := p.next(true); got != refreshFull || !forceInit {
		t.Errorf("first BW cycle = (%v, forceInit=%v), want (refreshFull, forceInit=true)", got, forceInit)
	}
}

func TestRefreshPlanner_RoutineBWCycleIsFast(t *testing.T) {
	p := newRefreshPlanner(BW, 60)
	p.next(true) // cycle 1: full
	if got, forceInit := p.next(true); got != refreshFast || forceInit {
		t.Errorf("routine BW cycle = (%v, forceInit=%v), want (refreshFast, forceInit=false)", got, forceInit)
	}
}

func TestRefreshPlanner_UnchangedRoutineCycleSkips(t *testing.T) {
	p := newRefreshPlanner(BW, 60)
	p.next(true) // cycle 1: full
	if got, forceInit := p.next(false); got != refreshSkip || forceInit {
		t.Errorf("unchanged BW cycle = (%v, forceInit=%v), want (refreshSkip, forceInit=false)", got, forceInit)
	}
}

func TestRefreshPlanner_Gray4ChangedIsGray(t *testing.T) {
	// Gray4 has only one waveform and real hardware showed its electrical
	// state drifts even between consecutive pushes (not just over the long
	// burn-in window), so every Gray4 push — routine or periodic — forces a
	// genuine re-init.
	p := newRefreshPlanner(Gray4, 60)
	p.next(true) // cycle 1: gray (full-due)
	if got, forceInit := p.next(true); got != refreshGray || !forceInit {
		t.Errorf("Gray4 changed cycle = (%v, forceInit=%v), want (refreshGray, forceInit=true)", got, forceInit)
	}
}

func TestRefreshPlanner_Gray4UnchangedSkips(t *testing.T) {
	p := newRefreshPlanner(Gray4, 60)
	p.next(true) // cycle 1: gray
	if got, forceInit := p.next(false); got != refreshSkip || forceInit {
		t.Errorf("Gray4 unchanged cycle = (%v, forceInit=%v), want (refreshSkip, forceInit=false)", got, forceInit)
	}
}

func TestRefreshPlanner_FullCadenceRecurs(t *testing.T) {
	p := newRefreshPlanner(BW, 3)
	p.next(true) // cycle 1: full
	p.next(true) // cycle 2: fast
	if got, forceInit := p.next(true); got != refreshFull || !forceInit {
		t.Errorf("3rd BW cycle = (%v, forceInit=%v), want (refreshFull, forceInit=true)", got, forceInit)
	}
}

func TestRefreshPlanner_FullDueOnUnchangedFrame(t *testing.T) {
	// A full refresh is forced on the cadence even when content is static,
	// so the panel still refreshes periodically (burn-in protection).
	p := newRefreshPlanner(Gray4, 2)
	p.next(true) // cycle 1: gray
	if got, forceInit := p.next(false); got != refreshGray || !forceInit {
		t.Errorf("Gray4 full-due unchanged cycle = (%v, forceInit=%v), want (refreshGray, forceInit=true)", got, forceInit)
	}
}

func TestRefreshPlanner_BWRoutineCycleDoesNotForceInit(t *testing.T) {
	// The one case where forceInit stays false: a BW routine cycle between
	// cadence ticks. BW's label-change check alone already forces a re-init
	// at the Full/Fast boundary, so forcing it again here would just be a
	// needless reset on every single change. This is the deliberate
	// asymmetry with Gray4 (which forces on every push, forceInit==true
	// unconditionally whenever it doesn't skip) — BW has a cheaper steady
	// state worth preserving; Gray4, per real-hardware testing, does not.
	p := newRefreshPlanner(BW, 3)
	p.next(true) // cycle 1: full (forceInit)
	if got, forceInit := p.next(true); got != refreshFast || forceInit {
		t.Errorf("cycle 2 (routine BW) = (%v, forceInit=%v), want (refreshFast, forceInit=false)", got, forceInit)
	}
}
