package daytimeline

import (
	"cmp"
	"fmt"
	"image"
	"slices"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar"
)

const (
	// lanes is how many events share the column side by side. A third
	// simultaneous event would cut every block to a sliver too narrow
	// for its title, so it is counted in a tag instead.
	lanes = 2
	// laneGap is the paper between two blocks side by side, and between
	// the right-hand block and a tag.
	laneGap = 4
)

// slot is one event's block, where it is drawn.
type slot struct {
	Event calendar.Event
	Rect  image.Rectangle
}

// arrangement is today's placed events laid out on the grid: the blocks
// drawn, and tags counting the events there was no lane for.
type arrangement struct {
	Blocks []slot
	Tags   []tag
}

// arrange lays events out in col on the grid tl maps. An event whose
// block overlaps no other takes the column's width. Events whose blocks
// overlap, directly or through a chain of others, form a group sharing
// two lanes side by side, each block still at its own start and end. An
// event that finds both lanes taken at its start isn't drawn; the group
// gives up room at its right for a tag counting those, level with the
// first of them.
//
// Overlap is judged on the blocks as drawn, not the events' times: a
// short event's line-tall block runs past its end, and two blocks with
// no row of paper between them read as one. Judging on the drawn extent
// is what guarantees nothing overprints.
func arrange(events []calendar.Event, col image.Rectangle, tl timeline) arrangement {
	full := make([]slot, len(events))
	for i, e := range events {
		full[i] = slot{Event: e, Rect: blockRect(col, tl, e)}
	}
	// Earliest first, and of two starting together the longer first, so
	// the long block takes the left lane and keeps it.
	slices.SortStableFunc(full, func(a, b slot) int {
		return cmp.Or(cmp.Compare(a.Rect.Min.Y, b.Rect.Min.Y), cmp.Compare(b.Rect.Max.Y, a.Rect.Max.Y))
	})

	var out arrangement
	for len(full) > 0 {
		n := groupLen(full)
		out.add(full[:n], col)
		full = full[n:]
	}
	return out
}

// groupLen is how many of sorted, from the first, overlap as a group:
// each block starts no lower than the lowest bottom before it. A block
// starting on the row a previous one ends on has no paper between them,
// so it overlaps too.
func groupLen(sorted []slot) int {
	bottom := sorted[0].Rect.Max.Y
	n := 1
	for ; n < len(sorted) && sorted[n].Rect.Min.Y <= bottom; n++ {
		bottom = max(bottom, sorted[n].Rect.Max.Y)
	}
	return n
}

// add lays out one overlapping group in col.
func (a *arrangement) add(group []slot, col image.Rectangle) {
	if len(group) == 1 {
		a.Blocks = append(a.Blocks, group[0])
		return
	}

	// Each event takes the first lane free at its start, one whose last
	// block ended above it with a row of paper to spare.
	laneOf := make([]int, len(group))
	var laneEnd [lanes]int
	for l := range laneEnd {
		laneEnd[l] = col.Min.Y - 1
	}
	var hidden []slot
	for i, s := range group {
		laneOf[i] = -1
		for l := range lanes {
			if laneEnd[l] < s.Rect.Min.Y {
				laneOf[i], laneEnd[l] = l, s.Rect.Max.Y
				break
			}
		}
		if laneOf[i] < 0 {
			hidden = append(hidden, s)
		}
	}

	right := col.Max.X
	if len(hidden) > 0 {
		t := newTag(fmt.Sprintf("+%d", len(hidden)), col.Max.X, hidden[0].Rect.Min.Y)
		a.Tags = append(a.Tags, t)
		right = t.Rect.Min.X - laneGap
	}
	w := (right - col.Min.X - laneGap) / lanes
	xs := [lanes]int{col.Min.X, right - w}
	for i, s := range group {
		if laneOf[i] < 0 {
			continue
		}
		s.Rect.Min.X, s.Rect.Max.X = xs[laneOf[i]], xs[laneOf[i]]+w
		a.Blocks = append(a.Blocks, s)
	}
}
