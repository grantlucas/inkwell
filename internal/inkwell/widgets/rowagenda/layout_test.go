package rowagenda

import (
	"image"
	"testing"

	"github.com/grantlucas/inkwell/internal/inkwell/widgets/drawkit"
)

var panel = image.Rect(0, 0, 800, 480)

// There are always five rows, and together they tile the bounds with no
// gap and no overlap, whatever the week holds.
func TestPlanRows_AlwaysFiveRowsTilingTheBounds(t *testing.T) {
	tests := []struct {
		label  string
		counts []int
	}{
		{"an empty week", []int{0, 0, 0, 0, 0}},
		{"a quiet week", []int{1, 0, 2, 0, 1}},
		{"a busy week", []int{5, 4, 3, 2, 4}},
		{"a week that overflows", []int{9, 8, 2, 7, 1}},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			got := planRows(panel, tt.counts)
			if len(got) != rows {
				t.Fatalf("got %d rows, want %d", len(got), rows)
			}
			y := panel.Min.Y
			for i, r := range got {
				if r.Bounds.Min.Y != y {
					t.Errorf("row %d starts at %d, want %d", i, r.Bounds.Min.Y, y)
				}
				if r.Bounds.Min.X != panel.Min.X || r.Bounds.Max.X != panel.Max.X {
					t.Errorf("row %d spans x %d-%d, want the full width", i, r.Bounds.Min.X, r.Bounds.Max.X)
				}
				if r.Bounds.Dy() < minRowH {
					t.Errorf("row %d is %d px, below the %d px minimum", i, r.Bounds.Dy(), minRowH)
				}
				if r.IsLast != (i == rows-1) {
					t.Errorf("row %d IsLast = %v", i, r.IsLast)
				}
				y = r.Bounds.Max.Y
			}
			if y != panel.Max.Y {
				t.Errorf("rows end at %d, want %d", y, panel.Max.Y)
			}
		})
	}
}

// Across a row, the day badge, the chart and the rule before the agenda
// sit side by side with no overlap: the chart starts where the badge's
// readings end, so bars never paint through the digits, and the rule
// sits in the gap between the chart and the agenda. The badge and chart
// share the row's centred block, so they sit level on every row.
func TestPlanRows_BadgeThenChartThenAgenda(t *testing.T) {
	for i, r := range planRows(panel, []int{6, 0, 2, 0, 1}) {
		if r.Badge.Max.X != r.Chart.Min.X {
			t.Errorf("row %d: badge ends at %d, chart starts at %d", i, r.Badge.Max.X, r.Chart.Min.X)
		}
		if r.Chart.Max.X+ruleInset != r.Agenda.Min.X {
			t.Errorf("row %d: chart ends at %d, agenda starts at %d; want the %d px rule gap between",
				i, r.Chart.Max.X, r.Agenda.Min.X, ruleInset)
		}
		if r.Badge.Dy() != minRowH || r.Chart.Min.Y != r.Badge.Min.Y+chartPadY || r.Chart.Max.Y != r.Badge.Max.Y-chartPadY {
			t.Errorf("row %d: badge %v and chart %v are not one centred block", i, r.Badge, r.Chart)
		}
		if mid := (r.Badge.Min.Y + r.Badge.Max.Y) - (r.Bounds.Min.Y + r.Bounds.Max.Y); mid < -1 || mid > 1 {
			t.Errorf("row %d: badge %v is not centred in the row %v", i, r.Badge, r.Bounds)
		}
	}
}

// Space follows the events: a busy day's row is tall enough for every
// one of them, and a quiet day's row is shorter than a busy one's.
func TestPlanRows_HeightFollowsContent(t *testing.T) {
	counts := []int{1, 6, 0, 4, 2}
	got := planRows(panel, counts)

	for i, r := range got {
		if r.Lines != max(counts[i], 1) {
			t.Errorf("row %d has %d lines for %d events, want room for all of them", i, r.Lines, counts[i])
		}
		if need := 2*agendaPadY + r.Lines*drawkit.BodyLineH(); r.Bounds.Dy() < need {
			t.Errorf("row %d is %d px, too short for its %d lines (%d px)", i, r.Bounds.Dy(), r.Lines, need)
		}
	}
	if got[1].Bounds.Dy() <= got[0].Bounds.Dy() {
		t.Errorf("the six-event row (%d px) is no taller than the one-event row (%d px)",
			got[1].Bounds.Dy(), got[0].Bounds.Dy())
	}
	if got[3].Bounds.Dy() <= got[2].Bounds.Dy() {
		t.Errorf("the four-event row (%d px) is no taller than the empty row (%d px)",
			got[3].Bounds.Dy(), got[2].Bounds.Dy())
	}
	// Quiet days that both fit the minimum get the same height, so the
	// spare room is shared rather than handed to whichever came first.
	if got[0].Bounds.Dy() != got[2].Bounds.Dy() {
		t.Errorf("two quiet rows differ: %d px and %d px", got[0].Bounds.Dy(), got[2].Bounds.Dy())
	}
}

// When the rows would not fit, the busiest rows give up lines first,
// one at a time, so quiet days are never squeezed to make room.
func TestPlanRows_OverflowTakesFromTheBusiestRows(t *testing.T) {
	tests := []struct {
		label     string
		counts    []int
		wantLines []int
	}{
		{
			// 9+8+2+7+1 lines need far more than 480 px. The three busy
			// rows are trimmed toward each other until the week fits;
			// the two quiet rows keep every event.
			label:     "three busy days",
			counts:    []int{9, 8, 2, 7, 1},
			wantLines: []int{5, 5, 2, 4, 1},
		},
		{
			// One packed day against four quiet ones: the packed day
			// takes everything the quiet days leave.
			label:     "one packed day",
			counts:    []int{20, 1, 0, 2, 1},
			wantLines: []int{8, 1, 1, 2, 1},
		},
		{
			// A tie is broken toward the later day, so today and
			// tomorrow keep their detail longest.
			label:     "a tie",
			counts:    []int{5, 5, 5, 5, 1},
			wantLines: []int{5, 4, 4, 4, 1},
		},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			got := planRows(panel, tt.counts)
			for i, r := range got {
				if r.Lines != tt.wantLines[i] {
					t.Errorf("row %d: %d lines, want %d", i, r.Lines, tt.wantLines[i])
				}
			}
			if end := got[rows-1].Bounds.Max.Y; end != panel.Max.Y {
				t.Errorf("rows end at %d, want %d", end, panel.Max.Y)
			}
			for i, r := range got {
				if need := 2*agendaPadY + r.Lines*drawkit.BodyLineH(); r.Bounds.Dy() < need {
					t.Errorf("row %d is %d px, too short for its %d lines", i, r.Bounds.Dy(), r.Lines)
				}
			}
		})
	}
}

// Bounds shorter than five minimum rows cannot be fitted by trimming —
// a row is never shorter than minRowH — so the plan stops trimming
// rather than looping forever. Render refuses these bounds before it
// gets here; the guard is for any other caller.
func TestPlanRows_StopsTrimmingAtTheMinimum(t *testing.T) {
	got := planRows(image.Rect(0, 0, 800, 200), []int{9, 9, 9, 9, 9})
	for i, r := range got {
		if r.Lines < 3 {
			t.Errorf("row %d trimmed to %d lines, below what a minimum row holds", i, r.Lines)
		}
	}
}

// The minimum row is what keeps the chart readable, and it carries
// three lines of agenda before a row needs to grow.
func TestMinRowH_HoldsThreeLines(t *testing.T) {
	if need := 2*agendaPadY + 3*drawkit.BodyLineH(); minRowH < need {
		t.Errorf("minRowH = %d, too short for three agenda lines (%d)", minRowH, need)
	}
}
