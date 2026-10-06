package daytimeline

import (
	"image"

	"github.com/grantlucas/inkwell/internal/inkwell/weather"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/drawkit"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/weatherview"
)

const (
	// lanePadX is the paper between the lane's plot and the hour labels
	// on its left and the rule on its right, so neither a bar nor the
	// temperature line runs into them.
	lanePadX = 3
	// barPadY is the paper above and below an hour's bar inside its row,
	// clear of the hour rules, so neighbouring hours' bars read as
	// separate bars rather than one shape.
	barPadY = 2
)

// drawLane draws the weather lane: the combined chart turned on its side
// to share the grid's rows. Each hour of the window has a row, and the
// chance of precipitation that hour is a bar growing from the lane's left
// edge. The temperature line runs down through the middle of the rows,
// colder to the left and warmer to the right across rng, and is drawn
// under the combined chart's line-inversion rule. hourly is today's
// forecast and rng today's own range, since the lane shows only today.
func drawLane(frame *image.Paletted, lane image.Rectangle, tl timeline, win Window, hourly []weather.HourlyPoint, rng weatherview.TempRange) {
	plot := image.Rect(lane.Min.X+lanePadX, lane.Min.Y, lane.Max.X-lanePadX, lane.Max.Y)
	hours := inWindow(hourly, win)
	// A dry window draws the line alone, as the combined chart does: a
	// row of stubs reads as a broken lane where the line says it's dry.
	dry := weatherview.Dry(hours)

	line := make([]image.Point, 0, len(hours))
	for _, hp := range hours {
		top, bottom := rowOf(tl, hp.Hour-win.StartHour)
		line = append(line, image.Pt(rng.X(hp.Temperature, plot.Min.X, plot.Dx()), (top+bottom)/2))
		if !dry {
			drawBar(frame, plot, top, bottom, hp.PrecipitationProb)
		}
	}
	weatherview.DrawContrastLine(frame, line, weatherview.RunsDown)
}

// inWindow is the forecast's hours the window shows.
func inWindow(hourly []weather.HourlyPoint, win Window) []weather.HourlyPoint {
	var in []weather.HourlyPoint
	for _, hp := range hourly {
		if hp.Hour >= win.StartHour && hp.Hour < win.EndHour {
			in = append(in, hp)
		}
	}
	return in
}

// rowOf is the band of grid rows hour h of the window takes, from its
// hour rule at top to the next one at bottom.
func rowOf(tl timeline, h int) (top, bottom int) {
	return tl.y(hourAt(tl.start, h)), tl.y(hourAt(tl.start, h+1))
}

// drawBar draws an hour's chance of precipitation as a bar growing right
// from plot's left edge, in the row between the hour rules at top and
// bottom: a PaperGray70 fill, which lands dark gray on Gray4 and solid
// black under the BW threshold, with a PaperBlack cap at its far end to
// carry the value at a distance, as the combined chart's bars do.
func drawBar(frame *image.Paletted, plot image.Rectangle, top, bottom int, prob float64) {
	n := weatherview.BarLength(prob, plot.Dx())
	if n <= 0 {
		return
	}
	r := image.Rect(plot.Min.X, top+1+barPadY, plot.Min.X+n, bottom-barPadY)
	drawkit.FillRect(frame, r, widget.PaperGray70)
	drawkit.DrawVLine(frame, r.Max.X-1, r.Min.Y, r.Max.Y, widget.PaperBlack)
}
