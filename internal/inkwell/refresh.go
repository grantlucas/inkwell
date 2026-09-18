package inkwell

// refreshKind is the refresh action the planner selects for a render cycle.
type refreshKind int

const (
	// refreshSkip pushes no frame to the panel (content unchanged).
	refreshSkip refreshKind = iota
	// refreshFast is a single-flicker fast full refresh (BW only). It is the
	// per-change BW waveform: a windowed partial refresh cannot be combined with
	// the force-drive needed to redraw changed pixels cleanly (the controller
	// reverts to the partial waveform on partial-in and settles the box
	// inverted), so each due change does one full-screen fast flash instead.
	refreshFast
	// refreshFull is the multi-flash full refresh that clears ghosting.
	refreshFull
	// refreshGray is the 4-level grayscale refresh (the only Gray4 waveform).
	refreshGray
)

// refreshPlanner decides which refresh waveform to use on each render cycle.
// The strategy is mode-aware: BW does a fast full-screen refresh on each changed
// tick (single flash) while ghosting is cleared with a multi-flash full refresh
// on a cadence; Gray4 has no fast waveform, so it refreshes only when content
// changes (plus a periodic forced refresh to guard against burn-in).
type refreshPlanner struct {
	color     ColorDepth
	fullEvery int // cycles between full / forced grayscale refreshes
	tick      int
}

// Burn-in / ghosting cadence is fixed internally rather than user-configurable:
// it's a property of the panel hardware (how often it needs a full clearing
// flash), not a per-widget concern. A full / forced-grayscale refresh runs
// roughly hourly (every 60 cycles at the default interval). This feeds the
// planner; the per-widget cadence the refresh queue gates on is separate.
const defaultFullEvery = 60

// newRefreshPlanner builds a planner for the given color depth and cadence.
func newRefreshPlanner(color ColorDepth, fullEvery int) *refreshPlanner {
	return &refreshPlanner{color: color, fullEvery: fullEvery}
}

// next advances the cycle counter and returns the refresh action to take,
// plus whether this decision falls on the burn-in cadence. changed reports
// whether the packed frame differs from what's on the panel.
//
// periodic distinguishes the cadence tick from a routine one even when they
// select the same refreshKind. In BW mode the periodic tick's refreshFull
// differs from a routine refreshFast, so the caller naturally re-inits on
// the label change alone. Gray4 has only one waveform, so its periodic tick
// and a routine tick both return refreshGray — the label never changes — and
// periodic is the only signal the caller has that this tick must still force
// a genuine hardware re-init (reset + power-on/booster) rather than reusing
// whatever electrical state the panel has drifted into. Without it, the
// "periodic burn-in refresh" degrades into an ordinary Display() push and
// never actually re-clears the panel.
func (p *refreshPlanner) next(changed bool) (kind refreshKind, periodic bool) {
	p.tick++

	// A full refresh on the first cycle and on the full cadence clears
	// ghosting and satisfies the panel's "refresh at least once per day"
	// rule even when content is static.
	if p.tick == 1 || (p.fullEvery > 0 && p.tick%p.fullEvery == 0) {
		if p.color == Gray4 {
			return refreshGray, true
		}
		return refreshFull, true
	}

	// Nothing changed since the last frame on the panel — don't reflash.
	if !changed {
		return refreshSkip, false
	}

	if p.color == Gray4 {
		return refreshGray, false
	}

	// BW: a single-flicker fast full refresh redraws the changed content cleanly.
	return refreshFast, false
}
