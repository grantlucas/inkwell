package inkwell

import (
	"errors"
	"fmt"
	"image"
	"maps"
	"math"
	"net/http"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/testutil"
	"github.com/grantlucas/inkwell/internal/inkwell/testutil/fakehttp"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
)

// The example config is the file people copy to build their own, so its
// screens are tested the way the app runs them: loaded by the real config
// loader, built by the real widget registry through NewApp, and drawn by
// the compositor. Only the network is faked.
const (
	exampleConfigPath = "../../inkwell.example.yaml"
	// exampleFeed is the calendar feed the example's screens list.
	exampleFeed = "https://example.com/my-calendar.ics"
	// exampleForecast is the endpoint of the example's weather model.
	exampleForecast = "https://api.open-meteo.com/v1/gem"
)

// exampleToday is the day the example screens are drawn on; the clock reads
// 10:40 in the example's zone, after the first event and before the rest.
var exampleToday = time.Date(2026, 10, 6, 0, 0, 0, 0, mustLoadZone("America/Toronto"))

func mustLoadZone(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return loc
}

// exampleDay is today's calendar: a finished game, three things still to
// come, and two of them overlapping, written as UTC instants the way most
// feeds serialize them.
func exampleDay() string {
	var b strings.Builder
	b.WriteString("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Inkwell//Example//EN\r\n")
	event := func(uid, summary string, startH, startM, mins int) {
		start := exampleToday.Add(time.Duration(startH)*time.Hour + time.Duration(startM)*time.Minute).UTC()
		end := start.Add(time.Duration(mins) * time.Minute)
		fmt.Fprintf(&b, "BEGIN:VEVENT\r\nUID:%s\r\nSUMMARY:%s\r\nDTSTART:%s\r\nDTEND:%s\r\nEND:VEVENT\r\n",
			uid, summary, start.Format("20060102T150405Z"), end.Format("20060102T150405Z"))
	}
	event("game", "Game vs Burlington", 8, 0, 120)
	event("lunch", "Lunch at Mom & Dad's", 12, 30, 90)
	event("call", "Call with Sam", 13, 0, 30)
	event("grocery", "Grocery pickup", 16, 0, 30)
	event("movie", "Family movie night", 18, 30, 120)
	b.WriteString("END:VCALENDAR\r\n")
	return b.String()
}

// exampleSky is a showery October week: today wet through the afternoon,
// then rain, a dry day, fog and drizzle. Temperatures rise to a peak mid
// afternoon.
func exampleSky(day, hour int) (temp, precip float64) {
	base := []float64{9, 7, 4, 3, 8, 10, 11, 9}[day]
	temp = base + 6*math.Sin(math.Pi*float64(hour-6)/18)
	if hour < 6 {
		temp = base
	}
	switch day {
	case 0:
		precip = 80 * math.Max(0, 1-math.Abs(float64(hour)-15)/4)
	case 1:
		precip = 40 + 2*float64(hour)
	case 3:
		precip = 20 * math.Max(0, 1-math.Abs(float64(hour)-8)/3)
	case 4:
		precip = 50 * math.Max(0, 1-math.Abs(float64(hour)-17)/5)
	}
	return math.Round(temp*10) / 10, math.Round(precip)
}

// exampleCodes are the week's WMO codes: showers, rain, partly cloudy,
// fog, drizzle.
var exampleCodes = []int{80, 63, 2, 45, 53, 3, 0, 0}

// exampleUpstream answers the example config's feed and forecast.
func exampleUpstream() *fakehttp.Client {
	client := fakehttp.New()
	client.Serve(exampleFeed, exampleDay())
	client.Handle(exampleForecast, fakehttp.OpenMeteo{
		Now:   exampleNow(),
		Site:  exampleToday.Location(),
		Sky:   exampleSky,
		Codes: exampleCodes,
	}.Reply)
	return client
}

func exampleNow() time.Time { return exampleToday.Add(10*time.Hour + 40*time.Minute) }

// newExampleApp loads the example config and builds the app on client.
func newExampleApp(t *testing.T, client HTTPClient) *App {
	t.Helper()
	f, err := os.Open(exampleConfigPath)
	if err != nil {
		t.Fatalf("open example config: %v", err)
	}
	defer func() { _ = f.Close() }()
	cfg, err := LoadConfig(f)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	app, err := NewApp(cfg, WithHardware(&MockHardware{}), WithHTTPClient(client),
		WithDeps(widget.Deps{Now: exampleNow}))
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	return app
}

// renderExampleScreen loads the example config, builds the app on client,
// and composes the screen named name.
func renderExampleScreen(t *testing.T, name string, client HTTPClient) (*Screen, *image.Paletted) {
	t.Helper()
	app := newExampleApp(t, client)
	for _, s := range app.dashboard.screens {
		if s.Name != name {
			continue
		}
		frame, err := app.comp.Render(s.Widgets())
		if err != nil {
			t.Fatalf("render %s: %v", name, err)
		}
		return s, frame
	}
	t.Fatalf("the example config has no active %q screen", name)
	return nil, nil
}

// The day-timeline screen, as the example config lays it out, tiles the
// panel with its widgets and draws today's agenda, today's weather and
// the days ahead together.
func TestExampleConfig_DayTimelineScreen(t *testing.T) {
	screen, frame := renderExampleScreen(t, "day-timeline", exampleUpstream())

	assertTiles(t, screen.Widgets(), image.Rect(0, 0, 800, 480))
	testutil.AssertGoldenPNG(t, frame)
}

// The example rotates through the new screens, day-timeline first, and
// each one loads with the example's bounds, tiles the panel and draws in
// every widget. weekly-calendar is no longer part of it.
func TestExampleConfig_Rotation(t *testing.T) {
	want := []string{"day-timeline", "bold-five", "today-hero", "row-agenda"}

	app := newExampleApp(t, exampleUpstream())
	var got []string
	for _, s := range app.dashboard.screens {
		got = append(got, s.Name)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("example screens = %q, want %q", got, want)
	}
	if app.dashboard.rotateInterval <= 0 {
		t.Errorf("rotate_interval = %v, want the example to rotate", app.dashboard.rotateInterval)
	}

	for _, name := range want {
		t.Run(name, func(t *testing.T) {
			screen, frame := renderExampleScreen(t, name, exampleUpstream())
			assertTiles(t, screen.Widgets(), image.Rect(0, 0, 800, 480))
			for _, w := range screen.Widgets() {
				if !inked(frame, w.Bounds()) {
					t.Errorf("%T at %v drew nothing", w, w.Bounds())
				}
			}
		})
	}
}

// assertTiles checks that ws cover panel exactly: every pixel belongs to
// one widget, so no two widgets draw over each other and nothing is left
// undrawn.
func assertTiles(t *testing.T, ws []widget.Widget, panel image.Rectangle) {
	t.Helper()
	area := 0
	for i, w := range ws {
		area += w.Bounds().Dx() * w.Bounds().Dy()
		for _, o := range ws[i+1:] {
			if w.Bounds().Overlaps(o.Bounds()) {
				t.Errorf("%v overlaps %v", w.Bounds(), o.Bounds())
			}
		}
	}
	if want := panel.Dx() * panel.Dy(); area != want {
		t.Errorf("widgets cover %d px, the panel is %d px", area, want)
	}
}

// One widget losing its data never blanks the screen: the compositor stops
// at the first render error, so every widget has to draw what it can.
// The widgets that don't read the failed source draw exactly what they
// draw when nothing fails.
func TestExampleConfig_DayTimelineScreen_SourceDown(t *testing.T) {
	_, healthy := renderExampleScreen(t, "day-timeline", exampleUpstream())

	tests := []struct {
		label string
		url   string
		reply fakehttp.Reply
		// unaffected are the widgets that don't read the failed source.
		unaffected []string
	}{
		{
			label:      "calendar feed down",
			url:        exampleFeed,
			reply:      fakehttp.Reply{Status: http.StatusInternalServerError},
			unaffected: []string{"*fuzzyclock.Widget", "*separator.Widget", "*todayweather.Widget", "*weatherahead.Widget"},
		},
		{
			label:      "forecast unreachable",
			url:        exampleForecast,
			reply:      fakehttp.Reply{Err: errors.New("network is unreachable")},
			unaffected: []string{"*fuzzyclock.Widget", "*separator.Widget"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			client := exampleUpstream()
			client.Set(tt.url, tt.reply)
			screen, frame := renderExampleScreen(t, "day-timeline", client)

			unaffected := map[string]bool{}
			for _, w := range screen.Widgets() {
				kind := fmt.Sprintf("%T", w)
				if !inked(frame, w.Bounds()) {
					t.Errorf("%s at %v drew nothing", kind, w.Bounds())
				}
				if slices.Contains(tt.unaffected, kind) {
					unaffected[kind] = true
					if !samePixels(frame, healthy, w.Bounds()) {
						t.Errorf("%s at %v changed though its data arrived", kind, w.Bounds())
					}
				}
			}
			if len(unaffected) != len(tt.unaffected) {
				t.Errorf("checked %v unchanged, want all of %v", slices.Sorted(maps.Keys(unaffected)), tt.unaffected)
			}
			testutil.AssertGoldenPNG(t, frame)
		})
	}
}

// inked reports whether anything but paper is drawn in r.
func inked(frame *image.Paletted, r image.Rectangle) bool {
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			if frame.ColorIndexAt(x, y) != widget.PaperWhite {
				return true
			}
		}
	}
	return false
}

// samePixels reports whether a and b match everywhere in r.
func samePixels(a, b *image.Paletted, r image.Rectangle) bool {
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			if a.ColorIndexAt(x, y) != b.ColorIndexAt(x, y) {
				return false
			}
		}
	}
	return true
}
