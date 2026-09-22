package inkwell

import "time"

// Dashboard manages a collection of screens and optional rotation.
type Dashboard struct {
	screens        []*Screen
	rotateInterval time.Duration
	current        int
	lastRotation   time.Time
	now            func() time.Time
}

// NewDashboard creates a Dashboard. If rotateInterval is 0, it stays on the
// first screen. The now function is used for rotation timing; if nil,
// time.Now is used.
func NewDashboard(screens []*Screen, rotateInterval time.Duration, now func() time.Time) *Dashboard {
	if now == nil {
		now = time.Now
	}
	return &Dashboard{
		screens:        screens,
		rotateInterval: rotateInterval,
		now:            now,
		lastRotation:   now(),
	}
}

// CurrentScreen returns the screen that should be displayed, and reports
// whether this call advanced the rotation. It advances to the next screen if
// the rotation interval has elapsed.
//
// The rotated flag is returned rather than left for the caller to infer
// because the refresh gate depends on it: a rotation has to reach the panel
// on the cycle it happens, not whenever a widget next falls due. Only the
// call that mutates d.current can report it, and only once — a second call
// on the same rotation reports false, so the flag must be consumed where it
// is read.
func (d *Dashboard) CurrentScreen() (screen *Screen, rotated bool) {
	if len(d.screens) == 0 {
		return nil, false
	}
	if d.rotateInterval > 0 {
		now := d.now()
		if elapsed := now.Sub(d.lastRotation); elapsed >= d.rotateInterval {
			steps := int(elapsed / d.rotateInterval)
			next := (d.current + steps) % len(d.screens)
			// A wrap back onto the same screen (a single configured
			// screen, or steps a multiple of the screen count) changes
			// nothing on the panel, so it is not a rotation worth
			// flashing for.
			rotated = next != d.current
			d.current = next
			d.lastRotation = d.lastRotation.Add(time.Duration(steps) * d.rotateInterval)
		}
	}
	return d.screens[d.current], rotated
}
