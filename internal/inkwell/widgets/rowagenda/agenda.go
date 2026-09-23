package rowagenda

import (
	"fmt"
	"image"
	"strings"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daygrid"
)

const (
	agendaPadX = 10
	agendaPadY = 10

	// oneColumnMax is the most events a row will lay out across its
	// full width. Past it the row splits in two, trading title length
	// for slot count.
	oneColumnMax = 3

	// twoColumnRows is how many lines each column gets once the row
	// splits: two columns of two is four slots.
	twoColumnRows = 2

	colGap   = 12
	emptyMsg = "Nothing scheduled"
)

// eventOptions carries the per-render knobs from widget config.
//
// Location is the zone event clock labels are rendered in: a parsed
// Event.Start is a correct instant but carries whatever zone its feed
// serialized it with, so formatting it directly leaks that zone onto
// the panel. It must never be nil; the widget defaults it.
type eventOptions struct {
	ShowLocation bool
	Location     *time.Location
}

// slot is one drawn line of the agenda.
type slot struct {
	x, y int
	// width is the pixels the slot may use, title included.
	width int
}

// renderAgenda draws a day's events across the row.
//
// The column count adapts to the day's load, which is what keeps titles
// long on the days that can afford it: three events or fewer take the
// full row width, and four or more split into two columns. That is the
// whole point of the screen — width is what titles were starving for,
// so it is spent on them whenever the day allows.
//
// A day with more events than slots gives its last slot to the overflow
// marker rather than overprinting an event with it.
func renderAgenda(frame *image.Paletted, bounds image.Rectangle, events []calendar.Event, opts eventOptions) int {
	slots := layoutSlots(bounds, len(events))
	if len(slots) == 0 {
		return 0
	}

	if len(events) == 0 {
		daygrid.DrawText(frame, slots[0].x, slots[0].y, emptyMsg, daygrid.BodyFace, widget.PaperBlack)
		return 0
	}

	// When there is more than fits, the last slot becomes the marker,
	// so the number of events drawn is one fewer than the slot count.
	capacity := len(slots)
	overflow := len(events) > capacity
	if overflow {
		capacity--
	}
	// The slot count is fixed by the layout, not by the day: a row
	// with one event still gets three one-column slots. Bounding the
	// walk by the events as well is what keeps it from reading past
	// them.
	capacity = min(capacity, len(events))

	drawn := 0
	for i, s := range slots {
		if i >= capacity {
			break
		}
		drawEventInSlot(frame, s, events[i], opts)
		drawn++
	}

	if overflow {
		s := slots[capacity]
		daygrid.DrawText(frame, s.x, s.y,
			fitTo(fmt.Sprintf("+%d MORE", len(events)-drawn), s.width),
			daygrid.BodyBoldFace, widget.PaperBlack)
	}
	return drawn
}

// layoutSlots places the drawable lines for a row carrying n events.
func layoutSlots(bounds image.Rectangle, n int) []slot {
	lineH := daygrid.BodyLineH()
	x := bounds.Min.X + agendaPadX
	y := bounds.Min.Y + agendaPadY + daygrid.BodyAscent()
	full := bounds.Dx() - 2*agendaPadX
	if full < daygrid.BodyAdvance()*8 {
		// Too narrow for a time and any title worth reading.
		return nil
	}

	if n <= oneColumnMax {
		var out []slot
		for i := range oneColumnMax {
			sy := y + i*lineH
			if sy > bounds.Max.Y {
				break
			}
			out = append(out, slot{x: x, y: sy, width: full})
		}
		return out
	}

	colW := (full - colGap) / 2
	var out []slot
	for col := range 2 {
		for row := range twoColumnRows {
			sy := y + row*lineH
			if sy > bounds.Max.Y {
				break
			}
			out = append(out, slot{x: x + col*(colW+colGap), y: sy, width: colW})
		}
	}
	// Read down the left column then down the right, which is the
	// order the times run in.
	return out
}

// drawEventInSlot draws one event as a time and a title on one line.
func drawEventInSlot(frame *image.Paletted, s slot, e calendar.Event, opts eventOptions) {
	timeText := timeLineFor(e, opts)
	daygrid.DrawText(frame, s.x, s.y, timeText, daygrid.BodyFace, widget.PaperBlack)

	// The time column is sized for the widest label, not for a clock
	// time: "ALL DAY" is seven characters against 00:00's five, and
	// measuring the clock alone runs the all-day label into the title.
	timeW := daygrid.TextWidth(daygrid.BodyFace, "ALL DAY ")
	titleX := s.x + timeW
	titleW := s.width - timeW
	if titleW < daygrid.BodyAdvance()*3 {
		return
	}
	daygrid.DrawText(frame, titleX, s.y, fitTo(titleFor(e, opts), titleW),
		daygrid.BodyFace, widget.PaperBlack)
}

// timeLineFor is the event's clock label.
func timeLineFor(e calendar.Event, opts eventOptions) string {
	if e.AllDay {
		return "ALL DAY"
	}
	return e.Start.In(opts.Location).Format("15:04")
}

func titleFor(e calendar.Event, opts eventOptions) string {
	if opts.ShowLocation && e.Location != "" {
		return e.Summary + " @ " + e.Location
	}
	return e.Summary
}

// fitTo shortens s to the characters that fit in width pixels, marking
// the cut with an ellipsis. The budget is in characters, so the cut is
// in runes — byte slicing would cut a multi-byte glyph in half and
// leave the panel painting a replacement.
func fitTo(s string, width int) string {
	maxChars := width / daygrid.BodyAdvance()
	r := []rune(strings.TrimSpace(s))
	if len(r) <= maxChars {
		return string(r)
	}
	if maxChars <= 1 {
		if maxChars < 1 {
			return ""
		}
		return string(r[:1])
	}
	return string(r[:maxChars-1]) + "…"
}
