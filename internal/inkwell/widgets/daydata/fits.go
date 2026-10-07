package daydata

import (
	"image"
	"log"
)

// Fits reports whether bounds hold need, the least room widget draws in.
// When they don't, it logs as much and the widget should draw nothing:
// the drawing helpers clip to the frame, not to the widget, so drawing
// anyway would ink its neighbours. Blank bounds are a misconfiguration an
// operator can see; ink on another widget looks like a fault somewhere
// else entirely. forWhat, when the size depends on the widget's settings,
// says which, as "4 days".
func Fits(widget string, bounds image.Rectangle, need image.Point, forWhat string) bool {
	if bounds.Dx() >= need.X && bounds.Dy() >= need.Y {
		return true
	}
	if forWhat != "" {
		forWhat = " for " + forWhat
	}
	log.Printf("%s: bounds are %dx%d, need at least %dx%d%s — drawing nothing",
		widget, bounds.Dx(), bounds.Dy(), need.X, need.Y, forWhat)
	return false
}
