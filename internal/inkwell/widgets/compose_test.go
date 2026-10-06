package widgets_test

import (
	"fmt"
	"image"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar"
	"github.com/grantlucas/inkwell/internal/inkwell/testutil/fakehttp"
	"github.com/grantlucas/inkwell/internal/inkwell/weather"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets"
)

const (
	composeFeed     = "https://example.com/week.ics"
	composeForecast = "https://api.open-meteo.com/v1/gem"
)

// composeNow is a Monday mid-afternoon in Toronto, so today has finished
// and upcoming events and the now marker falls inside the chart window.
var composeNow = time.Date(2026, 3, 16, 14, 30, 0, 0, mustZone("America/Toronto"))

func mustZone(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return loc
}

// composeWeek is a calendar with a different shape on each day: a
// crowded Monday, a quiet Tuesday, an all-day Wednesday, an empty
// Thursday and a long title on Friday.
func composeWeek() string {
	var b strings.Builder
	b.WriteString("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Inkwell//Compose//EN\r\n")
	event := func(uid, summary string, day, hour int) {
		start := time.Date(2026, 3, day, hour, 0, 0, 0, composeNow.Location()).UTC()
		fmt.Fprintf(&b, "BEGIN:VEVENT\r\nUID:%s\r\nSUMMARY:%s\r\nDTSTART:%s\r\nDTEND:%s\r\nEND:VEVENT\r\n",
			uid, summary, start.Format("20060102T150405Z"), start.Add(time.Hour).Format("20060102T150405Z"))
	}
	event("a", "Standup", 16, 9)
	event("b", "Design review", 16, 11)
	event("c", "Lunch", 16, 12)
	event("d", "1:1", 16, 15)
	event("e", "Retro", 16, 16)
	event("f", "Dentist", 17, 10)
	b.WriteString("BEGIN:VEVENT\r\nUID:trip\r\nSUMMARY:Conference\r\nDTSTART;VALUE=DATE:20260318\r\nDTEND;VALUE=DATE:20260319\r\nEND:VEVENT\r\n")
	event("g", "Platform architecture review", 20, 14)
	b.WriteString("END:VCALENDAR\r\n")
	return b.String()
}

// composeSky is a week whose days differ in temperature, so a chart on
// the wrong range would draw its line at the wrong height, with rain
// midweek and a partly cloudy day, whose icon's rays overrun its box.
func composeSky(day, hour int) (temp, precip float64) {
	base := []float64{6, 1, 9, -3, 12, 8, 8, 8}[day]
	temp = base + 7*math.Sin(math.Pi*float64(hour-6)/18)
	if hour < 6 {
		temp = base
	}
	if day >= 1 && day <= 3 && hour >= 12 && hour <= 17 {
		precip = 40 + 10*float64(day)
	}
	return math.Round(temp*10) / 10, precip
}

// composeDeps is what the app hands every widget, fetching the week's
// calendar and forecast through one fake upstream.
func composeDeps() widget.Deps {
	client := fakehttp.New()
	client.Serve(composeFeed, composeWeek())
	client.Handle(composeForecast, fakehttp.OpenMeteo{
		Now: composeNow, Site: composeNow.Location(), Sky: composeSky,
		Codes: []int{80, 63, 2, 45, 3, 0, 0, 0},
	}.Reply)
	now := func() time.Time { return composeNow }
	return widget.Deps{
		Now:      now,
		Calendar: calendar.NewProvider(client, now),
		Weather: weather.NewProvider(client, time.Hour, now, weather.Settings{
			Location: weather.Location{Latitude: 43.25, Longitude: -79.87}, TempUnit: "C", Model: weather.ModelGEM,
		}),
	}
}

// placed is one widget entry of a screen: its type, bounds and config.
type placed struct {
	typeName string
	bounds   image.Rectangle
	config   map[string]any
}

// boldFiveFromWidgets is the bold-five screen composed from the placeable
// widgets, as a screen entry in a config would place them: per column, a
// day badge, its combined chart, a rule and its event list, with a rule
// between columns. Every chart asks for the same five days, so they
// share one temperature range though none knows about the others.
func boldFiveFromWidgets() []placed {
	feeds := []any{composeFeed}
	var out []placed
	for i := range 5 {
		x0, x1 := 160*i, 160*(i+1)
		out = append(out,
			placed{"day-badge", image.Rect(x0, 0, x1, 156), map[string]any{"style": "column", "day": i}},
			placed{"combined-chart", image.Rect(x0+8, 156, x1-8, 196), map[string]any{"day": i, "range_days": 5}},
			placed{"separator", image.Rect(x0, 196, x1, 197), map[string]any{"thickness": 1}},
			placed{"event-list", image.Rect(x0+6, 208, x1-6, 480), map[string]any{"feeds": feeds, "day": i}},
		)
		if i < 4 {
			out = append(out, placed{"separator", image.Rect(x1-1, 0, x1, 480), map[string]any{"thickness": 1, "orientation": "vertical"}})
		}
	}
	return out
}

// The point of lifting the day badge, the combined chart and the event
// list out of the full-screen widgets is that a screen can be composed
// from them. Composed as bold-five places them, built through the
// registry from config and fed by the same shared calendar and weather,
// they draw bold-five: the same pixels, but for the condition icons'
// rays, which bold-five lets overrun its column edges and a placed badge
// clips to its bounds.
func TestComposedScreen_DrawsBoldFive(t *testing.T) {
	r := widgets.NewDefaultRegistry()
	deps := composeDeps()
	panel := image.Rect(0, 0, 800, 480)

	whole, err := r.Create("bold-five", panel, map[string]any{"feeds": []any{composeFeed}}, deps)
	if err != nil {
		t.Fatalf("Create bold-five: %v", err)
	}
	want := image.NewPaletted(panel, widget.PaperPalette)
	if err := whole.Render(want); err != nil {
		t.Fatalf("Render bold-five: %v", err)
	}

	got := image.NewPaletted(panel, widget.PaperPalette)
	for _, p := range boldFiveFromWidgets() {
		w, err := r.Create(p.typeName, p.bounds, p.config, deps)
		if err != nil {
			t.Fatalf("Create %s at %v: %v", p.typeName, p.bounds, err)
		}
		if err := w.Render(got); err != nil {
			t.Fatalf("Render %s: %v", p.typeName, err)
		}
	}

	var differ []image.Point
	for y := range 480 {
		for x := range 800 {
			if got.ColorIndexAt(x, y) != want.ColorIndexAt(x, y) {
				differ = append(differ, image.Pt(x, y))
			}
		}
	}
	// The icons sit at the top-left of each column's weather summary,
	// 6 px in and 94 px down; the partly cloudy sun's rays reach past
	// the column's left edge into the last few pixels of the column
	// before.
	for _, p := range differ {
		nextColumn := (p.X/160 + 1) * 160
		if nextColumn-p.X <= 12 && p.Y >= 92 && p.Y < 140 {
			continue
		}
		t.Fatalf("composed screen differs from bold-five at %v (%d px differ in all)", p, len(differ))
	}
}
