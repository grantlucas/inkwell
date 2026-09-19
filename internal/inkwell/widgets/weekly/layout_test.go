package weekly

import (
	"image"
	"testing"
)

func TestComputeColumns(t *testing.T) {
	cases := []struct {
		label string
		days  int
	}{
		{"single day", 1},
		{"work week", 5},
		{"full week", 7},
	}
	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			bounds := image.Rect(0, 52, 800, 480)
			cols := computeColumns(bounds, 120, tc.days)

			if len(cols) != tc.days {
				t.Fatalf("got %d columns, want %d", len(cols), tc.days)
			}

			totalW := 0
			for i, col := range cols {
				totalW += col.Bounds.Dx()

				if col.Bounds.Min.Y != 52 || col.Bounds.Max.Y != 480 {
					t.Errorf("col[%d] vertical bounds = [%d,%d], want [52,480]",
						i, col.Bounds.Min.Y, col.Bounds.Max.Y)
				}

				if col.Header.Dy() != dayHeaderH {
					t.Errorf("col[%d] header height = %d, want %d", i, col.Header.Dy(), dayHeaderH)
				}

				if col.Weather.Dy() != 120 {
					t.Errorf("col[%d] weather height = %d, want 120", i, col.Weather.Dy())
				}

				if col.Events.Min.Y != col.Weather.Max.Y {
					t.Errorf("col[%d] events starts at %d, weather ends at %d",
						i, col.Events.Min.Y, col.Weather.Max.Y)
				}

				// Only the rightmost column is IsLast, so exactly that one
				// column skips the divider stroke.
				if want := i == tc.days-1; col.IsLast != want {
					t.Errorf("col[%d].IsLast = %v, want %v", i, col.IsLast, want)
				}
			}

			// The last column absorbs the integer-division remainder, so the
			// columns always tile the full width with no gap on the right.
			if totalW != bounds.Dx() {
				t.Errorf("total width = %d, want %d", totalW, bounds.Dx())
			}
			if last := cols[len(cols)-1]; last.Bounds.Max.X != bounds.Max.X {
				t.Errorf("last col Max.X = %d, want %d", last.Bounds.Max.X, bounds.Max.X)
			}
		})
	}
}

func TestComputeColumns_LastColumnAbsorbsRemainder(t *testing.T) {
	// 100 / 7 = 14 remainder 2, so the last column must be 2 px wider.
	cols := computeColumns(image.Rect(0, 0, 100, 200), 50, 7)

	totalW := 0
	for _, col := range cols {
		totalW += col.Bounds.Dx()
	}
	if totalW != 100 {
		t.Errorf("total width = %d, want 100", totalW)
	}
	if cols[6].Bounds.Dx() != 16 {
		t.Errorf("last col width = %d, want 16", cols[6].Bounds.Dx())
	}
}

func TestComputeColumns_WeatherClampedToMaxY(t *testing.T) {
	cols := computeColumns(image.Rect(0, 0, 700, 60), 200, 7)

	for i, col := range cols {
		if col.Weather.Max.Y > 60 {
			t.Errorf("col[%d] weather Max.Y = %d, exceeds bounds", i, col.Weather.Max.Y)
		}
	}
}
