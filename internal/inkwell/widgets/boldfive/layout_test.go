package boldfive

import (
	"image"
	"testing"
)

// The columns must be equal width and the bands must line up across all
// five, because a date numeral that sits a pixel lower in one column
// than the next is the one thing a five-column grid cannot get away
// with at this size.
func TestComputeColumns(t *testing.T) {
	bounds := image.Rect(0, 0, 800, 480)
	cols := computeColumns(bounds)

	if len(cols) != columns {
		t.Fatalf("got %d columns, want %d", len(cols), columns)
	}

	for i, c := range cols {
		if got := c.Header.Dy(); got != headerH {
			t.Errorf("column %d: header height = %d, want %d", i, got, headerH)
		}
		if got := c.Weather.Dy(); got != weatherH {
			t.Errorf("column %d: weather height = %d, want %d", i, got, weatherH)
		}
		if c.Header.Max.Y != c.Weather.Min.Y {
			t.Errorf("column %d: gap between header and weather", i)
		}
		if c.Weather.Max.Y != c.Events.Min.Y {
			t.Errorf("column %d: gap between weather and events", i)
		}
		if c.Events.Max.Y != bounds.Max.Y {
			t.Errorf("column %d: events do not reach the bottom", i)
		}
		if got, want := c.IsLast, i == columns-1; got != want {
			t.Errorf("column %d: IsLast = %v, want %v", i, got, want)
		}
	}

	// Columns tile the width with no gaps and no overlap.
	if cols[0].Bounds.Min.X != bounds.Min.X {
		t.Errorf("first column starts at %d, want %d", cols[0].Bounds.Min.X, bounds.Min.X)
	}
	if cols[columns-1].Bounds.Max.X != bounds.Max.X {
		t.Errorf("last column ends at %d, want %d", cols[columns-1].Bounds.Max.X, bounds.Max.X)
	}
	for i := 1; i < columns; i++ {
		if cols[i].Bounds.Min.X != cols[i-1].Bounds.Max.X {
			t.Errorf("column %d starts at %d, previous ended at %d", i, cols[i].Bounds.Min.X, cols[i-1].Bounds.Max.X)
		}
	}
}

// A width that does not divide by five must not produce four narrow
// columns and one that silently swallows the remainder unevenly — the
// leftover goes to the last column and nowhere else.
func TestComputeColumns_RemainderGoesToTheLastColumn(t *testing.T) {
	cols := computeColumns(image.Rect(0, 0, 803, 480))

	for i, c := range cols[:columns-1] {
		if got := c.Bounds.Dx(); got != 160 {
			t.Errorf("column %d width = %d, want 160", i, got)
		}
	}
	if got := cols[columns-1].Bounds.Dx(); got != 163 {
		t.Errorf("last column width = %d, want 163 (160 + the 3 px remainder)", got)
	}
}

// A widget shorter than its bands must clamp rather than produce
// rectangles that run past the frame.
func TestComputeColumns_ShortBounds(t *testing.T) {
	bounds := image.Rect(0, 0, 800, 60)
	for i, c := range computeColumns(bounds) {
		if c.Header.Max.Y > bounds.Max.Y {
			t.Errorf("column %d: header runs past the bounds", i)
		}
		if c.Weather.Max.Y > bounds.Max.Y {
			t.Errorf("column %d: weather runs past the bounds", i)
		}
		if c.Events.Min.Y > bounds.Max.Y {
			t.Errorf("column %d: events start past the bounds", i)
		}
	}
}

// The widget is positioned by the dashboard, so every band has to be
// offset by the widget's own origin rather than assuming (0,0).
func TestComputeColumns_RespectsOrigin(t *testing.T) {
	bounds := image.Rect(20, 30, 820, 510)
	cols := computeColumns(bounds)

	if cols[0].Bounds.Min.X != 20 {
		t.Errorf("first column x = %d, want 20", cols[0].Bounds.Min.X)
	}
	if cols[0].Header.Min.Y != 30 {
		t.Errorf("header top = %d, want 30", cols[0].Header.Min.Y)
	}
	if cols[0].Header.Max.Y != 30+headerH {
		t.Errorf("header bottom = %d, want %d", cols[0].Header.Max.Y, 30+headerH)
	}
}
