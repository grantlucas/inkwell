package rowagenda

import (
	"image"
	"testing"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/weather"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daygrid"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/weatherview"
)

// busyRow is the first row of a week whose first day carries n events
// and whose other days are empty, as the widget would lay it out.
func busyRow(n int) rowLayout {
	return planRows(panel, []int{n, 0, 0, 0, 0})[0]
}

// A day the forecast never covered draws no badge: drawing a zero
// would state a 0°/0° reading nobody forecast.
func TestRenderBadge_MissingForecast(t *testing.T) {
	frame := newTestFrame(800, 480)
	renderBadge(frame, busyRow(0).Badge,
		nil, "C", true, true, 14, weatherview.TempRange{Min: 0, Max: 25})
	if got := countIndexIn(frame, frame.Bounds(), widget.PaperBlack); got != 0 {
		t.Errorf("drew %d px for a day with no forecast", got)
	}
}

// Every row carries the combined chart, so a dry day still draws its
// temperature line rather than leaving the chart blank, and two days on
// one shared range sit at different heights when one is colder.
func TestRenderBadge_CombinedChart(t *testing.T) {
	badge := busyRow(0).Badge
	chart := image.Rect(badge.Min.X+chartDX, badge.Min.Y, badge.Min.X+chartDX+chartW, badge.Max.Y)
	rng := weatherview.TempRange{Min: -10, Max: 30}

	dryDay := func(temp float64) *weather.DailyForecast {
		var hourly []weather.HourlyPoint
		for h := range 24 {
			hourly = append(hourly, weather.HourlyPoint{Hour: h, Temperature: temp})
		}
		return &weather.DailyForecast{
			Date: time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC),
			High: temp, Low: temp, Hourly: hourly,
		}
	}

	// topInk is the first row of the chart carrying ink above its
	// baseline, which on a dry day is the temperature line.
	topInk := func(frame *image.Paletted) int {
		for y := chart.Min.Y; y < chart.Max.Y; y++ {
			if countIndexIn(frame, image.Rect(chart.Min.X, y, chart.Max.X, y+1), widget.PaperBlack) > 0 {
				return y
			}
		}
		return -1
	}

	cold, warm := newTestFrame(800, 480), newTestFrame(800, 480)
	renderBadge(cold, badge, dryDay(-5), "C", false, false, 14, rng)
	renderBadge(warm, badge, dryDay(25), "C", false, false, 14, rng)

	coldY, warmY := topInk(cold), topInk(warm)
	if coldY < 0 || warmY < 0 {
		t.Fatal("a dry day drew a blank chart")
	}
	if warmY >= coldY {
		t.Errorf("the warm day's line (y=%d) is not above the cold day's (y=%d)", warmY, coldY)
	}
}

// The temperature block and the chart share the badge, and the chart is
// drawn second — so an overlap does not read as a layout slip, it reads
// as bars painted through the digits. The shipped goldens cannot catch
// it because their rain sits in hours 12-17, well right of the label.
func TestRenderBadge_TemperatureNeverReachesTheChart(t *testing.T) {
	badge := busyRow(0).Badge

	// Morning rain, so the leftmost bars land where the label would
	// overrun if it could.
	var hourly []weather.HourlyPoint
	for h := range 24 {
		prob := 0.0
		if h >= 6 && h <= 9 {
			prob = 0.9
		}
		hourly = append(hourly, weather.HourlyPoint{Hour: h, PrecipitationProb: prob})
	}

	// The widest readings the block has to hold.
	tests := []struct {
		label  string
		hi, lo float64
		unit   string
	}{
		{"negative celsius", -15, -22, "C"},
		{"three-digit fahrenheit", 37.8, 20, "F"}, // 100°F
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			frame := newTestFrame(800, 480)
			renderBadge(frame, badge, &weather.DailyForecast{
				Date: time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC),
				High: tt.hi, Low: tt.lo, Hourly: hourly,
			}, tt.unit, true, true, 8, weatherview.TempRange{Min: -30, Max: 40})

			// The widest reading the block can hold must end before
			// the chart's left edge.
			if end := hiDX + tempMaxChars*daygrid.BodyAdvance(); end > chartDX {
				t.Errorf("temperature block can reach %d, chart starts at %d — they overlap", end, chartDX)
			}
			// And this day's actual reading must not have drawn into
			// the chart's first bar slot either.
			if daygrid.TextWidth(daygrid.BodyBoldFace, "-100°F") > tempMaxChars*daygrid.BodyAdvance() {
				t.Error("tempMaxChars no longer covers the widest reading")
			}
		})
	}
}
