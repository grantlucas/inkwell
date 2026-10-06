package daytimeline

import (
	"image"
	"testing"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar/ical"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daygrid"
)

// markerRows are the grid's rows inked solid from the lane's left edge to
// the events' right, the rule between them aside: the rows the now
// marker takes. The window's edge rules run that far too, so they are
// left out.
func markerRows(frame *image.Paletted, l layout) []int {
	var rows []int
	for y := l.Grid.Min.Y + 1; y < l.Grid.Max.Y-1; y++ {
		solid := true
		for x := l.Lane.Min.X; x < l.Events.Max.X && solid; x++ {
			solid = x == l.Lane.Max.X || frame.ColorIndexAt(x, y) == widget.PaperBlack
		}
		if solid {
			rows = append(rows, y)
		}
	}
	return rows
}

// The now marker crosses the lane and the grid at the current time, two
// rows thick, so what is past and what is still to come read apart at a
// glance. It sits a row clear of an hour rule rather than on it, a
// minute or two off at most. Outside the window there is no now on the
// grid, and no marker; the window's end is not in it.
func TestNowMarker_CrossesTheLaneAndGridInsideTheWindow(t *testing.T) {
	tests := []struct {
		label string
		now   time.Time
		want  bool
	}{
		{label: "at the window's start", now: at(7, 0), want: true},
		{label: "on the hour", now: at(13, 0), want: true},
		{label: "mid-hour", now: at(13, 30), want: true},
		{label: "just before the end", now: at(21, 59), want: true},
		{label: "before the window", now: at(6, 59)},
		{label: "at the window's end", now: at(22, 0)},
		{label: "after the window", now: at(23, 30)},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			frame := renderWeather(t, nil, defaultConfig(), tt.now)
			l, tl := gridOf(testBounds, defaultConfig().Window)
			rows := markerRows(frame, l)

			if !tt.want {
				if len(rows) != 0 {
					t.Errorf("marker drawn at rows %v outside the window", rows)
				}
				return
			}
			if len(rows) != 2 || rows[1] != rows[0]+1 {
				t.Fatalf("marker rows = %v, want two adjacent rows", rows)
			}
			if y := tl.y(tt.now); rows[0] < y-3 || rows[0] > y+3 {
				t.Errorf("marker at row %d, want within 3 rows of now's %d", rows[0], y)
			}
		})
	}
}

// The marker passes behind a block's label rather than through it: a
// line through words reads as struck through, as if the event were
// cancelled. The label reads exactly as it does with no marker on the
// grid, and the marker still crosses the rest of the block.
func TestNowMarker_PassesBehindALabel(t *testing.T) {
	e := span("Quarterly planning", at(15, 0), at(17, 0))
	l, tl := gridOf(testBounds, defaultConfig().Window)
	block := blockRect(l.Events, tl, e)
	render := func(now time.Time) *image.Paletted {
		return renderToFrame(t, New(testBounds, daygrid.InMemory([]ical.Event{e}, nil), fixedClock(now), defaultConfig()))
	}
	// The "UNTIL 17:00" line's caps sit a line under the first's.
	until := labelBaseline(block) + daygrid.BodyLineH() - capH/2
	var now time.Time
	for m := range 120 {
		if now = at(15, m); tl.y(now) == until {
			break
		}
	}
	crossing, unmarked := render(now), render(at(6, 0))

	text := image.Rect(block.Min.X+labelPadX, until-capH, block.Min.X+labelPadX+len("UNTIL 17:00")*daygrid.BodyAdvance(), until+capH)
	if !sameIn(crossing, unmarked, text) {
		t.Error("the marker changes the label it crosses")
	}
	right := image.Rect(block.Max.X-markClear, block.Min.Y, block.Max.X, block.Max.Y)
	if sameIn(crossing, unmarked, right) {
		t.Error("no marker across the block right of its label")
	}
}

// The marker follows the line-inversion rule: through a block still to
// come it is paper, so it reads across the solid block as well as the
// paper either side.
func TestNowMarker_IsPaperThroughASolidBlock(t *testing.T) {
	e := span("Workshop", at(13, 0), at(15, 0))
	w := New(testBounds, daygrid.InMemory([]ical.Event{e}, nil), fixedClock(at(14, 30)), defaultConfig())
	frame := renderToFrame(t, w)
	l, tl := gridOf(testBounds, defaultConfig().Window)
	block := blockRect(l.Events, tl, e)

	// Right of the label, the block's rows are solid but for the marker.
	var paper []int
	for y := block.Min.Y; y < block.Max.Y; y++ {
		if countIndexIn(frame, image.Rect(block.Max.X-markClear, y, block.Max.X, y+1), widget.PaperWhite) == markClear {
			paper = append(paper, y)
		}
	}
	if len(paper) != 2 || paper[1] != paper[0]+1 {
		t.Fatalf("paper rows through the block = %v, want the marker's two", paper)
	}
	if y := tl.y(at(14, 30)); paper[0] < y-3 || paper[0] > y+3 {
		t.Errorf("marker at row %d through the block, want within 3 rows of %d", paper[0], y)
	}
	// Between the rule and the block it is ink on paper.
	gap := image.Rect(l.Lane.Max.X+1, paper[0], block.Min.X, paper[0]+2)
	if n := countIndexIn(frame, gap, widget.PaperBlack); n != gap.Dx()*gap.Dy() {
		t.Errorf("marker beside the block is %d/%d px", n, gap.Dx()*gap.Dy())
	}
}
