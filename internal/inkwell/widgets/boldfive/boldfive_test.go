package boldfive

import (
	"image"
	"math"
	"testing"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar"
	"github.com/grantlucas/inkwell/internal/inkwell/calendar/ical"
	"github.com/grantlucas/inkwell/internal/inkwell/testutil"
	"github.com/grantlucas/inkwell/internal/inkwell/testutil/fakehttp"
	"github.com/grantlucas/inkwell/internal/inkwell/weather"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daydata"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/drawkit"
)

// testTime is a Monday mid-afternoon, so today's column has both
// finished and upcoming events and the now-marker falls inside the
// chart's 06:00-21:00 window.
var testTime = time.Date(2026, 3, 16, 14, 30, 0, 0, time.UTC)

func fixedClock(t time.Time) func() time.Time { return func() time.Time { return t } }

// sampleForecast covers all five columns, with rain on the middle days
// and a dry last day so the dry path is exercised by the full render.
func sampleForecast() []weather.DailyForecast {
	var days []weather.DailyForecast
	for i := range columns {
		// Each day warms from its low before dawn to its high mid
		// afternoon, and the days differ, so the combined chart's line
		// has a shape and the shared scale has something to compare.
		high := []float64{14, 9, 17, 4, 12}[i]
		low := high - []float64{8, 5, 9, 6, 4}[i]
		var hourly []weather.HourlyPoint
		for h := range 24 {
			prob := 0.0
			if i > 0 && i < columns-1 && h >= 12 && h <= 16 {
				prob = 0.3 + 0.15*float64(i)
			}
			warmth := math.Exp(-math.Pow(float64(h-15), 2) / 40)
			hourly = append(hourly, weather.HourlyPoint{
				Hour:              h,
				Temperature:       low + (high-low)*warmth,
				PrecipitationProb: prob,
			})
		}
		days = append(days, weather.DailyForecast{
			Date:      time.Date(2026, 3, 16+i, 0, 0, 0, 0, time.UTC),
			High:      high,
			Low:       low,
			Condition: weather.Condition(i % 4),
			Hourly:    hourly,
		})
	}
	return days
}

func ev(summary string, day, hour int) ical.Event {
	start := time.Date(2026, 3, day, hour, 0, 0, 0, time.UTC)
	return ical.Event{UID: summary, Summary: summary, Start: start, End: start.Add(time.Hour)}
}

// sampleEvents gives each column a different shape: Monday packed past
// the cap, Tuesday sparse, Wednesday all-day only, Thursday empty and
// Friday a single long title that has to wrap.
func sampleEvents() []ical.Event {
	return []ical.Event{
		ev("Standup", 16, 9),
		ev("Design review", 16, 11),
		ev("Lunch", 16, 12),
		ev("1:1", 16, 15),
		ev("Retro", 16, 16),
		ev("Dentist", 17, 10),
		{
			UID: "trip", Summary: "Conference", AllDay: true,
			Start: time.Date(2026, 3, 18, 0, 0, 0, 0, time.UTC),
			End:   time.Date(2026, 3, 19, 0, 0, 0, 0, time.UTC),
		},
		ev("Platform architecture review", 20, 14),
	}
}

// drawConfig is a config with the knobs bold-five draws with.
func drawConfig(maxEvents int, unit string) daydata.Config {
	return daydata.Config{MaxEvents: maxEvents, Weather: daydata.WeatherConfig{TempUnit: unit}}
}

// newWidget draws the whole panel from the given events and forecast.
func newWidget(t *testing.T, events []ical.Event, forecast []weather.DailyForecast) *Widget {
	t.Helper()
	return New(image.Rect(0, 0, 800, 480), daydata.InMemory(events, forecast), fixedClock(testTime), drawConfig(defaultMaxEvents, "C"))
}

func renderToFrame(t *testing.T, w *Widget) *image.Paletted {
	t.Helper()
	frame := image.NewPaletted(image.Rect(0, 0, 800, 480), widget.PaperPalette)
	if err := w.Render(frame); err != nil {
		t.Fatalf("Render: %v", err)
	}
	return frame
}

func TestWidget_Bounds(t *testing.T) {
	w := newWidget(t, nil, nil)
	if got := w.Bounds(); got != image.Rect(0, 0, 800, 480) {
		t.Errorf("Bounds = %v", got)
	}
}

// Four dividers for five columns, and none after the last one.
func TestWidget_DrawsColumnDividers(t *testing.T) {
	frame := renderToFrame(t, newWidget(t, nil, sampleForecast()))

	for i, col := range computeColumns(image.Rect(0, 0, 800, 480)) {
		x := col.Bounds.Max.X - 1
		inked := 0
		for y := range 480 {
			if frame.ColorIndexAt(x, y) == widget.PaperBlack {
				inked++
			}
		}
		if col.IsLast {
			if inked == 480 {
				t.Errorf("column %d: a full-height rule was drawn after the last column", i)
			}
			continue
		}
		if inked != 480 {
			t.Errorf("column %d: divider is %d/480 px tall", i, inked)
		}
	}
}

// With a feed down and nothing cached, every column says the calendar is
// unavailable where an empty day says "--": the event list's note, in
// the column's list. A "--" would tell the viewer the day is free.
func TestWidget_EveryColumnSaysTheCalendarIsUnavailable(t *testing.T) {
	bounds := image.Rect(0, 0, 800, 480)
	w := New(bounds, daydata.InMemory(nil, sampleForecast(), daydata.CalendarDown()), fixedClock(testTime), drawConfig(defaultMaxEvents, "C"))
	frame := renderToFrame(t, w)

	for i, col := range computeColumns(bounds) {
		list := image.Rect(col.Events.Min.X+eventsPadX, col.Events.Min.Y+eventsTopPad, col.Events.Max.X-eventsPadX, col.Events.Max.Y)
		ref := image.NewPaletted(bounds, widget.PaperPalette)
		drawkit.FillWhite(ref, bounds)
		style := agendaStyle(defaultMaxEvents, false, time.UTC)
		style.Unavailable = true
		style.Draw(ref, list, nil)
		if !testutil.Inked(ref, list) {
			t.Fatalf("column %d: the note did not fit the column's list", i)
		}
		if !testutil.SameIn(frame, ref, list) {
			t.Errorf("column %d: the list does not say the calendar is unavailable", i)
		}
	}
}

// Today is the leftmost column and gets no highlight, so the five
// header bands must be structurally alike — none of them inverted.
func TestWidget_NoColumnIsHighlighted(t *testing.T) {
	frame := renderToFrame(t, newWidget(t, sampleEvents(), sampleForecast()))

	for i, col := range computeColumns(image.Rect(0, 0, 800, 480)) {
		black := countIndexIn(frame, col.Badge, widget.PaperBlack)
		if area := col.Badge.Dx() * col.Badge.Dy(); black > area/2 {
			t.Errorf("column %d header is %d/%d black — it looks inverted", i, black, area)
		}
	}
}

// dryForecast is sampleForecast with the rain taken out, so every column
// is a dry day.
func dryForecast() []weather.DailyForecast {
	f := sampleForecast()
	for i := range f {
		for h := range f[i].Hourly {
			f[i].Hourly[h].PrecipitationProb = 0
		}
	}
	return f
}

// rainyForecast is sampleForecast with rain on every day, building
// through the afternoon and easing off in the evening.
func rainyForecast() []weather.DailyForecast {
	f := sampleForecast()
	for i := range f {
		for h := range f[i].Hourly {
			if h >= 10 && h <= 19 {
				f[i].Hourly[h].PrecipitationProb = 0.3 + 0.07*float64(min(h-10, 19-h)) + 0.05*float64(i)
			}
		}
	}
	return f
}

// columnChart is the chart cell of one column, in frame coordinates.
func columnChart(col columnLayout) image.Rectangle {
	return col.Chart
}

// Every column carries the combined chart, so a dry day still draws the
// temperature line and no column's chart band is left empty. Five
// columns that read unevenly look like a rendering fault.
func TestWidget_DryDayStillDrawsAChart(t *testing.T) {
	frame := renderToFrame(t, newWidget(t, nil, dryForecast()))

	for i, col := range computeColumns(image.Rect(0, 0, 800, 480)) {
		chart := columnChart(col)
		// Above the baseline, so the axis alone does not count: only the
		// temperature line can ink the plot of a dry day.
		plot := image.Rect(chart.Min.X, chart.Min.Y, chart.Max.X, chartBaseline(t, frame, chart))
		if countIndexIn(frame, plot, widget.PaperBlack) == 0 {
			t.Errorf("column %d: a dry day drew nothing in its chart plot", i)
		}
	}
}

// chartBaseline is the first row of chart that is solid PaperBlack from
// edge to edge: the rule the bars stand on.
func chartBaseline(t *testing.T, frame *image.Paletted, chart image.Rectangle) int {
	t.Helper()
	for y := chart.Min.Y; y < chart.Max.Y; y++ {
		if countIndexIn(frame, image.Rect(chart.Min.X, y, chart.Max.X, y+1), widget.PaperBlack) == chart.Dx() {
			return y
		}
	}
	t.Fatalf("no baseline in chart %v", chart)
	return -1
}

// topInkRow is the first row in r carrying PaperBlack in column x, or -1.
func topInkRow(frame *image.Paletted, x int, r image.Rectangle) int {
	for y := r.Min.Y; y < r.Max.Y; y++ {
		if frame.ColorIndexAt(x, y) == widget.PaperBlack {
			return y
		}
	}
	return -1
}

// One temperature scale is shared by all five columns, so a cold day's
// line sits visibly lower than a warm one's. Per-column scales would
// stretch every day to fill its own chart and draw both lines at the
// same height.
func TestWidget_ColumnsShareOneTemperatureScale(t *testing.T) {
	f := dryForecast()
	for i := range f {
		temp := 0.0
		if i == 1 {
			temp = 25
		}
		f[i].High, f[i].Low = temp, temp
		for h := range f[i].Hourly {
			f[i].Hourly[h].Temperature = temp
		}
	}
	frame := renderToFrame(t, newWidget(t, nil, f))

	cols := computeColumns(image.Rect(0, 0, 800, 480))
	// Hour 9 is a quiet column of the chart: no now-marker, no label.
	lineY := func(col columnLayout) int {
		chart := columnChart(col)
		x := chart.Min.X + chart.Dx()*(9-6)/16 + 2
		return topInkRow(frame, x, chart)
	}
	cold, warm := lineY(cols[2]), lineY(cols[1])
	if cold < 0 || warm < 0 {
		t.Fatalf("no line found: cold=%d warm=%d", cold, warm)
	}
	if warm >= cold {
		t.Errorf("warm day's line at y=%d is not above the cold day's at y=%d", warm, cold)
	}
}

// belowHeader is where the example config places bold-five: under a
// fuzzy_clock header band and the rule beneath it.
var belowHeader = image.Rect(0, 48, 800, 480)

// busiestDay is a Monday packed with wrapped titles, more than the
// column can hold.
func busiestDay() []ical.Event {
	var out []ical.Event
	for i, title := range []string{
		"Platform architecture review", "Quarterly planning session",
		"Dentist appointment downtown", "Parent council meeting", "Swim lessons",
	} {
		e := ev(title, 16, 8+2*i)
		e.UID = title
		out = append(out, e)
	}
	return out
}

// Placed under a header band, the widget keeps every pixel inside its
// own bounds: the clock above and anything below are left untouched.
func TestWidget_StaysInsideBoundsBelowAHeaderBand(t *testing.T) {
	tests := []struct {
		label  string
		bounds image.Rectangle
	}{
		{"below a header band", belowHeader},
		{"above another widget", image.Rect(0, 0, 800, 440)},
		{"between two widgets", image.Rect(0, 48, 800, 440)},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			frame := image.NewPaletted(image.Rect(0, 0, 800, 480), widget.PaperPalette)
			testutil.PaintOutside(frame, tt.bounds)
			w := New(tt.bounds, daydata.InMemory(append(sampleEvents(), busiestDay()...), sampleForecast()),
				fixedClock(testTime), drawConfig(3, "C"))
			if err := w.Render(frame); err != nil {
				t.Fatalf("Render: %v", err)
			}
			for y := range 480 {
				if y >= tt.bounds.Min.Y && y < tt.bounds.Max.Y {
					continue
				}
				for x := range 800 {
					if frame.ColorIndexAt(x, y) != widget.PaperBlack {
						t.Fatalf("painted over a neighbouring widget at (%d,%d)", x, y)
					}
				}
			}
		})
	}
}

// Under the header band the example config uses, a column still fits
// three events whose titles wrap, plus the line counting the rest.
func TestWidget_BelowHeaderFitsThreeWrappedEventsAndTheCount(t *testing.T) {
	frame := image.NewPaletted(image.Rect(0, 0, 800, 480), widget.PaperPalette)
	w := New(belowHeader, daydata.InMemory(busiestDay(), sampleForecast()),
		fixedClock(testTime), drawConfig(3, "C"))
	if err := w.Render(frame); err != nil {
		t.Fatalf("Render: %v", err)
	}

	today := computeColumns(belowHeader)[0].Events
	lineH := drawkit.BodyLineH()
	const gap = 8 // the stacked list's paper between events
	moreY := today.Min.Y + eventsTopPad + drawkit.BodyAscent() + 3*(3*lineH+gap)
	if !moreLineAt(frame, today, moreY, "+2 MORE") {
		t.Errorf("no \"+2 MORE\" line under three events at baseline %d", moreY)
	}
}

// Golden renders of the whole panel. These are the regression net for
// the geometry: every constant in this package shows up in them.
func TestWidget_Golden(t *testing.T) {
	tests := []struct {
		label    string
		events   []ical.Event
		forecast []weather.DailyForecast
		cfg      func(*daydata.Config)
		// bounds defaults to the whole panel.
		bounds image.Rectangle
		// down serves the days with a feed down and nothing cached.
		down bool
	}{
		{
			label:    "full week",
			events:   sampleEvents(),
			forecast: sampleForecast(),
		},
		{
			label:    "no events at all",
			events:   nil,
			forecast: sampleForecast(),
		},
		{
			label:    "no weather at all",
			events:   sampleEvents(),
			forecast: nil,
		},
		{
			label: "locations shown",
			events: []ical.Event{
				{
					UID: "l", Summary: "Lunch", Location: "Cafe",
					Start: time.Date(2026, 3, 16, 12, 0, 0, 0, time.UTC),
					End:   time.Date(2026, 3, 16, 13, 0, 0, 0, time.UTC),
				},
			},
			forecast: sampleForecast(),
			cfg:      func(c *daydata.Config) { c.ShowLocation = true },
		},
		{
			label:    "fahrenheit",
			events:   sampleEvents(),
			forecast: sampleForecast(),
			cfg:      func(c *daydata.Config) { c.Weather.TempUnit = "F" },
		},
		// The example config's layout: under a fuzzy_clock header band,
		// three events a column.
		{
			label:    "below header dry day",
			events:   sampleEvents(),
			forecast: dryForecast(),
			cfg:      func(c *daydata.Config) { c.MaxEvents = 3 },
			bounds:   belowHeader,
		},
		{
			label:    "below header rainy day",
			events:   sampleEvents(),
			forecast: rainyForecast(),
			cfg:      func(c *daydata.Config) { c.MaxEvents = 3 },
			bounds:   belowHeader,
		},
		{
			label:    "below header busiest day",
			events:   append(busiestDay(), sampleEvents()[5:]...),
			forecast: sampleForecast(),
			cfg:      func(c *daydata.Config) { c.MaxEvents = 3 },
			bounds:   belowHeader,
		},
		// A feed down with nothing cached: every column says so where an
		// empty day says "--", and lists what the other feeds sent under it.
		{
			label:    "calendar unavailable",
			forecast: sampleForecast(),
			down:     true,
		},
		{
			label:    "calendar unavailable below header busiest day",
			events:   append(busiestDay(), sampleEvents()[5:]...),
			forecast: sampleForecast(),
			cfg:      func(c *daydata.Config) { c.MaxEvents = 3 },
			bounds:   belowHeader,
			down:     true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			cfg := drawConfig(defaultMaxEvents, "C")
			if tt.cfg != nil {
				tt.cfg(&cfg)
			}
			bounds := tt.bounds
			if bounds.Empty() {
				bounds = image.Rect(0, 0, 800, 480)
			}
			var opts []daydata.MemoryOption
			if tt.down {
				opts = append(opts, daydata.CalendarDown())
			}
			w := New(bounds, daydata.InMemory(tt.events, tt.forecast, opts...), fixedClock(testTime), cfg)
			testutil.AssertGoldenPNG(t, renderToFrame(t, w))
		})
	}
}

// Factory is the shared day-widget factory, tested in daydata: parsing
// the shared settings, building the day data and taking the clock. What is
// bold-five's own is its default event cap and the reasons it gives for settings it has no use for.
func TestFactory(t *testing.T) {
	deps := widget.Deps{
		Now:      fixedClock(testTime),
		Calendar: calendar.NewProvider(fakehttp.New(), fixedClock(testTime)),
		Weather:  weather.NewProvider(fakehttp.New(), time.Hour, fixedClock(testTime), weather.Settings{}),
	}

	tests := []struct {
		label   string
		config  map[string]any
		wantErr string
	}{
		{label: "caps events at its default", config: map[string]any{"feeds": []any{"https://example.com/a.ics"}}},
		{
			label:   "explains a retired calendar key",
			config:  map[string]any{"feeds": []any{"https://example.com/a.ics"}, "show_weather": false},
			wantErr: "bold-five: show_weather is not supported: the weather band is part of the layout; a day with no forecast already draws nothing",
		},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			w, err := Factory(image.Rect(0, 0, 800, 480), tt.config, deps)
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("err = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Factory: %v", err)
			}
			if got := w.(*Widget).Config.MaxEvents; got != defaultMaxEvents {
				t.Errorf("MaxEvents = %d, want %d", got, defaultMaxEvents)
			}
		})
	}
}

// Every renderer places content at a fixed offset from its band's top,
// and the draw helpers clip to the frame rather than to the widget's
// bounds. A widget given less room than the bands need would therefore
// paint over its neighbour on the shared frame — so it draws nothing
// instead. A blank region is a misconfiguration you can see; ink on top
// of another widget looks like a fault somewhere else entirely.
func TestWidget_TooShortDrawsNothing(t *testing.T) {
	frame := image.NewPaletted(image.Rect(0, 0, 800, 480), widget.PaperPalette)

	// A neighbour already on the frame, below where this widget sits.
	for y := 200; y < 480; y++ {
		for x := range 800 {
			frame.SetColorIndex(x, y, widget.PaperBlack)
		}
	}

	w := New(image.Rect(0, 0, 800, 150), daydata.InMemory(sampleEvents(), sampleForecast()),
		fixedClock(testTime), drawConfig(defaultMaxEvents, "C"))
	if err := w.Render(frame); err != nil {
		t.Fatalf("Render: %v", err)
	}

	// Its own region is blank, and the neighbour below is untouched.
	if got := countIndexIn(frame, image.Rect(0, 0, 800, 150), widget.PaperBlack); got != 0 {
		t.Errorf("drew %d px into a widget too short to draw into", got)
	}
	for y := 200; y < 480; y++ {
		for x := range 800 {
			if frame.ColorIndexAt(x, y) != widget.PaperBlack {
				t.Fatalf("painted over the neighbouring widget at (%d,%d)", x, y)
			}
		}
	}
}
