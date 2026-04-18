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

// CurrentScreen returns the screen that should be displayed. It advances to
// the next screen if the rotation interval has elapsed.
func (d *Dashboard) CurrentScreen() *Screen {
	if len(d.screens) == 0 {
		return nil
	}
	if d.rotateInterval > 0 {
		now := d.now()
		if elapsed := now.Sub(d.lastRotation); elapsed >= d.rotateInterval {
			steps := int(elapsed / d.rotateInterval)
			d.current = (d.current + steps) % len(d.screens)
			d.lastRotation = d.lastRotation.Add(time.Duration(steps) * d.rotateInterval)
		}
	}
	return d.screens[d.current]
}
