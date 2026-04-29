package weekly

import (
	"image"
	"testing"
)

func TestComputeColumns_SevenColumns(t *testing.T) {
	bounds := image.Rect(0, 52, 800, 480)
	cols := computeColumns(bounds, 120)

	if len(cols) != 7 {
		t.Fatalf("got %d columns, want 7", len(cols))
	}

	for i, col := range cols {
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
	}

	if cols[0].IsLast {
		t.Error("col[0] should not be IsLast")
	}
	if !cols[6].IsLast {
		t.Error("col[6] should be IsLast")
	}
}

func TestComputeColumns_LastColumnAbsorbsRemainder(t *testing.T) {
	bounds := image.Rect(0, 0, 100, 200)
	cols := computeColumns(bounds, 50)

	totalW := 0
	for _, col := range cols {
		totalW += col.Bounds.Dx()
	}
	if totalW != 100 {
		t.Errorf("total width = %d, want 100", totalW)
	}

	if cols[6].Bounds.Max.X != 100 {
		t.Errorf("last col Max.X = %d, want 100", cols[6].Bounds.Max.X)
	}
}

func TestComputeColumns_WeatherClampedToMaxY(t *testing.T) {
	bounds := image.Rect(0, 0, 700, 50)
	cols := computeColumns(bounds, 200)

	for i, col := range cols {
		if col.Weather.Max.Y > 50 {
			t.Errorf("col[%d] weather Max.Y = %d, exceeds bounds", i, col.Weather.Max.Y)
		}
	}
}
