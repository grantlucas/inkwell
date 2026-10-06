package weatherview

import (
	"image"
	"testing"

	"github.com/grantlucas/inkwell/internal/inkwell/widget"
)

// The line-inversion rule, exposed for any drawing code that lays a line
// over its own fills (the day-timeline's weather lane): each pixel is
// black over bare paper and white over anything already drawn, and the
// line is two pixels thick. A line running across a plot, as the
// combined chart's does, takes its second pixel below; one running down,
// as the weather lane's does, takes it to the right, since a mostly
// vertical line thickened downward would be one pixel wide.
func TestDrawContrastLine(t *testing.T) {
	across := func(x, y int) bool { return x >= 2 && x <= 12 && (y == 4 || y == 5) }
	down := func(x, y int) bool { return y >= 2 && y <= 12 && (x == 4 || x == 5) }
	cases := []struct {
		label  string
		under  uint8
		run    LineRun
		pts    []image.Point
		onLine func(x, y int) bool
		want   uint8
	}{
		{"across, over paper", widget.PaperWhite, RunsAcross, []image.Point{{2, 4}, {12, 4}}, across, widget.PaperBlack},
		{"across, over a bar's fill", widget.PaperGray70, RunsAcross, []image.Point{{2, 4}, {12, 4}}, across, widget.PaperWhite},
		{"across, over a bar's cap", widget.PaperBlack, RunsAcross, []image.Point{{2, 4}, {12, 4}}, across, widget.PaperWhite},
		{"down, over paper", widget.PaperWhite, RunsDown, []image.Point{{4, 2}, {4, 12}}, down, widget.PaperBlack},
		{"down, over a bar's fill", widget.PaperGray70, RunsDown, []image.Point{{4, 2}, {4, 12}}, down, widget.PaperWhite},
	}

	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			frame := newTestFrame(20, 20)
			fillRect(frame, frame.Bounds(), tc.under)

			DrawContrastLine(frame, tc.pts, tc.run)

			for x := range 20 {
				for y := range 20 {
					onLine := tc.onLine(x, y)
					want := tc.under
					if onLine {
						want = tc.want
					}
					if got := frame.ColorIndexAt(x, y); got != want {
						t.Fatalf("(%d,%d) = index %d, want %d", x, y, got, want)
					}
				}
			}
		})
	}
}

// The colour is read before any of the line is written, so where the
// line doubles back over itself on paper it stays black rather than
// flipping to white over its own ink.
func TestDrawContrastLine_DoesNotInvertItself(t *testing.T) {
	frame := newTestFrame(20, 10)
	DrawContrastLine(frame, []image.Point{{2, 4}, {12, 4}, {2, 4}}, RunsAcross)

	for x := 2; x <= 12; x++ {
		if got := frame.ColorIndexAt(x, 4); got != widget.PaperBlack {
			t.Fatalf("(%d,4) = index %d, want PaperBlack", x, got)
		}
	}
}

// A line half over a bar and half over paper changes colour where the bar
// ends, decided pixel by pixel.
func TestDrawContrastLine_SplitsAtABarEdge(t *testing.T) {
	frame := newTestFrame(20, 10)
	fillRect(frame, image.Rect(8, 0, 20, 10), widget.PaperGray70)

	DrawContrastLine(frame, []image.Point{{2, 4}, {12, 4}}, RunsAcross)

	if got := frame.ColorIndexAt(7, 4); got != widget.PaperBlack {
		t.Errorf("over paper = index %d, want PaperBlack", got)
	}
	if got := frame.ColorIndexAt(8, 4); got != widget.PaperWhite {
		t.Errorf("over the bar = index %d, want PaperWhite", got)
	}
}
