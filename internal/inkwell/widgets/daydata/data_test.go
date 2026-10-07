package daydata_test

import (
	"errors"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar"
	"github.com/grantlucas/inkwell/internal/inkwell/testutil/fakehttp"
	"github.com/grantlucas/inkwell/internal/inkwell/weather"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daydata"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/weatherview"
)

// The data seam: the day data module driven by one fake HTTP client
// serving ICS and Open-Meteo fixtures. The calendar module, the weather
// provider and the day data module all run for real behind it.

const (
	feedA  = "https://a.example/cal.ics"
	feedB  = "https://b.example/cal.ics"
	gemURL = "https://api.open-meteo.com/v1/gem"
)

func mustZone(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatalf("load zone %s: %v", name, err)
	}
	return loc
}

// ics wraps VEVENT blocks in a VCALENDAR.
func ics(events ...string) string {
	return "BEGIN:VCALENDAR\r\n" + strings.Join(events, "") + "END:VCALENDAR\r\n"
}

// oneOff is a single timed event.
func oneOff(uid, summary, start, end string) string {
	return "BEGIN:VEVENT\r\nUID:" + uid + "\r\nDTSTART:" + start + "\r\nDTEND:" + end +
		"\r\nSUMMARY:" + summary + "\r\nEND:VEVENT\r\n"
}

// seam is one dashboard's worth of shared dependencies, fetching through
// one fake client.
type seam struct {
	tr   *fakehttp.Client
	now  time.Time
	deps widget.Deps
}

// newSeam serves a forecast that starts at Today in whatever zone it is
// asked for, from a site in Toronto.
func newSeam(t *testing.T, now time.Time) *seam {
	t.Helper()
	tr := fakehttp.New()
	tr.Handle(gemURL, fakehttp.OpenMeteo{Now: now, Site: mustZone(t, "America/Toronto")}.Reply)
	s := &seam{tr: tr, now: now}
	// The clock reads s.now, so a test can move it on between renders.
	clock := func() time.Time { return s.now }
	s.deps = widget.Deps{
		Now:      clock,
		Calendar: calendar.NewProvider(tr, clock),
		Weather: weather.NewProvider(tr, time.Hour, clock, weather.Settings{
			Location: weather.Location{Latitude: 43.25, Longitude: -79.87},
			TempUnit: "C",
			Model:    weather.ModelGEM,
		}),
	}
	return s
}

// source builds a widget's day data from cfg.
func (s *seam) source(t *testing.T, cfg daydata.Config) daydata.Source {
	t.Helper()
	src, err := daydata.New("test-widget", cfg, s.deps)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return src
}

// gemConfig is a widget showing feeds with the forecast for Hamilton.
func gemConfig(feeds ...calendar.Feed) daydata.Config {
	return daydata.Config{
		Feeds:   feeds,
		Refresh: 15 * time.Minute,
		Weather: daydata.WeatherConfig{Latitude: 43.25, Longitude: -79.87, TempUnit: "C", Model: weather.ModelGEM},
	}
}

func summaries(events []calendar.Event) []string {
	out := []string{}
	for _, e := range events {
		out = append(out, e.Summary)
	}
	return out
}

// A widget asks for its days and gets each day's date, whether it is
// Today, its events and its forecast.
func TestDayData_EachDayCarriesItsDateEventsAndForecast(t *testing.T) {
	toronto := mustZone(t, "America/Toronto")
	now := time.Date(2026, 10, 5, 8, 0, 0, 0, toronto) // a Monday
	s := newSeam(t, now)
	s.tr.Serve(feedA, ics(oneOff("dentist", "Dentist", "20261006T140000Z", "20261006T150000Z")))

	got := s.source(t, gemConfig(calendar.Feed{URL: feedA})).Days(now, 3)

	if len(got.Days) != 3 {
		t.Fatalf("got %d days, want 3", len(got.Days))
	}
	wantEvents := [][]string{{}, {"Dentist"}, {}}
	for i, day := range got.Days {
		wantDate := time.Date(2026, 10, 5+i, 0, 0, 0, 0, toronto)
		if !day.Start.Equal(wantDate) {
			t.Errorf("day %d starts %v, want %v", i, day.Start, wantDate)
		}
		if day.IsToday != (i == 0) {
			t.Errorf("day %d IsToday = %v", i, day.IsToday)
		}
		if got := strings.Join(summaries(day.Events), ","); got != strings.Join(wantEvents[i], ",") {
			t.Errorf("day %d events = %q, want %q", i, got, wantEvents[i])
		}
		if day.Forecast == nil {
			t.Fatalf("day %d has no forecast", i)
		}
		if got := day.Forecast.Date.Format("2006-01-02"); got != wantDate.Format("2006-01-02") {
			t.Errorf("day %d forecast is for %s", i, got)
		}
	}
}

// weeklySeries is a weekly series on Mondays at 09:00 UTC from January,
// months before any window these tests ask for.
const weeklySeries = "BEGIN:VEVENT\r\nUID:weekly-sync@example.com\r\nDTSTART:20260105T090000Z\r\n" +
	"DTEND:20260105T093000Z\r\nRRULE:FREQ=WEEKLY\r\nSUMMARY:Weekly Sync\r\nEND:VEVENT\r\n"

// override replaces the series' instance at recurrenceID; status may be
// CANCELLED.
func override(recurrenceID, start, end, status string) string {
	s := "BEGIN:VEVENT\r\nUID:weekly-sync@example.com\r\nRECURRENCE-ID:" + recurrenceID +
		"\r\nDTSTART:" + start + "\r\nDTEND:" + end + "\r\nSUMMARY:Weekly Sync\r\n"
	if status != "" {
		s += "STATUS:" + status + "\r\n"
	}
	return s + "END:VEVENT\r\n"
}

// dayEvents is each day's event summaries, comma-joined.
func dayEvents(data daydata.Data) []string {
	out := make([]string, len(data.Days))
	for i, d := range data.Days {
		out[i] = strings.Join(summaries(d.Events), ",")
	}
	return out
}

// The calendar side of a widget's days, from feeds as their hosts serve
// them to the day each event lands on. The window is the eight days from
// Monday 5 October in Toronto, a zone behind UTC.
func TestDayData_EventsLandOnTheirDays(t *testing.T) {
	toronto := mustZone(t, "America/Toronto")
	now := time.Date(2026, 10, 5, 8, 0, 0, 0, toronto)

	tests := []struct {
		label  string
		served map[string]string
		feeds  []calendar.Feed
		want   []string
	}{
		{
			label:  "a series that began before the window",
			served: map[string]string{feedA: ics(weeklySeries)},
			feeds:  []calendar.Feed{{URL: feedA}},
			want:   []string{"Weekly Sync", "", "", "", "", "", "", "Weekly Sync"},
		},
		{
			label: "a moved and a cancelled instance",
			served: map[string]string{feedA: ics(weeklySeries,
				override("20261005T090000Z", "20261006T150000Z", "20261006T153000Z", ""),
				override("20261012T090000Z", "20261012T090000Z", "20261012T093000Z", "CANCELLED"),
			)},
			feeds: []calendar.Feed{{URL: feedA}},
			want:  []string{"", "Weekly Sync", "", "", "", "", "", ""},
		},
		{
			label: "one event in two feeds",
			served: map[string]string{
				feedA: ics(oneOff("kickoff@a.example", "Kickoff", "20261006T130000Z", "20261006T140000Z")),
				feedB: ics(oneOff("kickoff@b.example", "Kickoff", "20261006T130000Z", "20261006T140000Z")),
			},
			feeds: []calendar.Feed{{URL: feedA}, {URL: feedB}},
			want:  []string{"", "Kickoff", "", "", "", "", "", ""},
		},
		{
			// A DATE value is parsed at UTC midnight, which is the evening
			// before in Toronto; the event still lands on its own date.
			label: "an all-day event in a zone behind UTC",
			served: map[string]string{feedA: ics("BEGIN:VEVENT\r\nUID:trip@example.com\r\n" +
				"DTSTART;VALUE=DATE:20261008\r\nDTEND;VALUE=DATE:20261009\r\nSUMMARY:Trip\r\nEND:VEVENT\r\n")},
			feeds: []calendar.Feed{{URL: feedA}},
			want:  []string{"", "", "", "Trip", "", "", "", ""},
		},
		{
			// DTEND is exclusive, so a trip ending on the Sunday is over
			// by then.
			label: "a multi-day all-day event, on each date it spans",
			served: map[string]string{feedA: ics("BEGIN:VEVENT\r\nUID:trip@example.com\r\n" +
				"DTSTART;VALUE=DATE:20261008\r\nDTEND;VALUE=DATE:20261011\r\nSUMMARY:Trip\r\nEND:VEVENT\r\n")},
			feeds: []calendar.Feed{{URL: feedA}},
			want:  []string{"", "", "", "Trip", "Trip", "Trip", "", ""},
		},
		{
			label:  "a feed that fails to fetch",
			served: map[string]string{},
			feeds:  []calendar.Feed{{URL: feedA}},
			want:   []string{"", "", "", "", "", "", "", ""},
		},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			s := newSeam(t, now)
			for url, body := range tt.served {
				s.tr.Serve(url, body)
			}

			got := s.source(t, gemConfig(tt.feeds...)).Days(now, 8)

			if g := dayEvents(got); !slices.Equal(g, tt.want) {
				t.Errorf("events by day = %q, want %q", g, tt.want)
			}
			// The forecast is fetched alongside, whatever the calendar did.
			if got.Days[0].Forecast == nil {
				t.Error("no forecast for Today")
			}
		})
	}
}

// A day lists its all-day events first and then its timed ones by start.
// East of UTC an all-day event's UTC-midnight anchor falls after the
// morning's timed events, so ordering by instant alone would bury it
// mid-morning.
func TestDayData_AllDayEventsComeFirst(t *testing.T) {
	now := time.Date(2026, 10, 6, 7, 0, 0, 0, mustZone(t, "Asia/Tokyo"))
	s := newSeam(t, now)
	s.tr.Serve(feedA, ics(
		oneOff("lunch", "Lunch", "20261006T030000Z", "20261006T040000Z"),         // 12:00 in Tokyo
		oneOff("breakfast", "Breakfast", "20261005T230000Z", "20261006T000000Z"), // 08:00 in Tokyo
		"BEGIN:VEVENT\r\nUID:holiday\r\nDTSTART;VALUE=DATE:20261006\r\nDTEND;VALUE=DATE:20261007\r\n"+
			"SUMMARY:Holiday\r\nEND:VEVENT\r\n",
	))

	got := s.source(t, gemConfig(calendar.Feed{URL: feedA})).Days(now, 1)

	if g, want := dayEvents(got)[0], "Holiday,Breakfast,Lunch"; g != want {
		t.Errorf("Today lists %q, want %q", g, want)
	}
}

// Asking for no days gets none.
func TestDayData_NoDaysAskedFor(t *testing.T) {
	now := time.Date(2026, 10, 5, 8, 0, 0, 0, mustZone(t, "America/Toronto"))
	for _, n := range []int{0, -1} {
		if got := newSeam(t, now).source(t, gemConfig()).Days(now, n); len(got.Days) != 0 {
			t.Errorf("Days(now, %d) gave %d days, want none", n, len(got.Days))
		}
	}
}

// The calendar and the forecast are fetched together under one deadline.
// Fetched in turn, a calendar that ate the whole budget handed the
// forecast an expired context and that render drew no weather (#111).
// Here the calendar answers only once the forecast has been asked for, so
// the render gets both only if the two run at once.
func TestDayData_CalendarAndForecastFetchTogether(t *testing.T) {
	toronto := mustZone(t, "America/Toronto")
	now := time.Date(2026, 10, 5, 8, 0, 0, 0, toronto)
	s := newSeam(t, now)

	var (
		mu        sync.Mutex
		deadlines = map[string]time.Time{}
		once      sync.Once
		asked     = make(chan struct{})
	)
	record := func(r *http.Request, side string) {
		d, _ := r.Context().Deadline()
		mu.Lock()
		deadlines[side] = d
		mu.Unlock()
	}
	forecast := fakehttp.OpenMeteo{Now: now, Site: toronto}
	s.tr.Handle(gemURL, func(r *http.Request) fakehttp.Reply {
		record(r, "forecast")
		once.Do(func() { close(asked) })
		return forecast.Reply(r)
	})
	s.tr.Handle(feedA, func(r *http.Request) fakehttp.Reply {
		record(r, "calendar")
		select {
		case <-asked:
			return fakehttp.Reply{Body: ics(weeklySeries)}
		case <-r.Context().Done():
			return fakehttp.Reply{Err: r.Context().Err()}
		}
	})

	src := s.source(t, gemConfig(calendar.Feed{URL: feedA}))
	before := time.Now()
	got := src.Days(now, 1)
	after := time.Now()

	if g := dayEvents(got)[0]; g != "Weekly Sync" {
		t.Errorf("Today lists %q, want the calendar's answer", g)
	}
	if got.Days[0].Forecast == nil {
		t.Error("no forecast for Today")
	}
	cal, fc := deadlines["calendar"], deadlines["forecast"]
	if cal.IsZero() || !cal.Equal(fc) {
		t.Errorf("deadlines: calendar %v, forecast %v; want one shared deadline", cal, fc)
	}
	// The one budget bounds the render: ten seconds from when it began.
	if cal.Before(before.Add(10*time.Second)) || cal.After(after.Add(10*time.Second)) {
		t.Errorf("deadline %v, want ten seconds after the render began (%v to %v)", cal, before, after)
	}
}

// Two widgets showing one feed with different rules each see their own
// rules applied, and the feed is fetched once.
func TestDayData_PerWidgetRulesOnASharedFeed(t *testing.T) {
	now := time.Date(2026, 10, 5, 8, 0, 0, 0, mustZone(t, "America/Toronto"))
	s := newSeam(t, now)
	s.tr.Serve(feedA, ics(weeklySeries))
	rename, err := calendar.NewRule("^Weekly ", "Team ", false)
	if err != nil {
		t.Fatal(err)
	}
	hide, err := calendar.NewRule("Sync", "", true)
	if err != nil {
		t.Fatal(err)
	}

	renamed := s.source(t, gemConfig(calendar.Feed{URL: feedA, Rules: []calendar.Rule{rename}})).Days(now, 1)
	hidden := s.source(t, gemConfig(calendar.Feed{URL: feedA, Rules: []calendar.Rule{hide}})).Days(now, 1)
	plain := s.source(t, gemConfig(calendar.Feed{URL: feedA})).Days(now, 1)

	for _, c := range []struct {
		label string
		data  daydata.Data
		want  string
	}{{"renamed", renamed, "Team Sync"}, {"hidden", hidden, ""}, {"plain", plain, "Weekly Sync"}} {
		if got := dayEvents(c.data)[0]; got != c.want {
			t.Errorf("%s widget sees %q, want %q", c.label, got, c.want)
		}
	}
	if got := s.tr.Requests(feedA); got != 1 {
		t.Errorf("feed fetched %d times, want 1", got)
	}
}

// Widgets sharing a feed and a weather location cause one fetch of each,
// whatever span of days each asks for.
func TestDayData_OneFetchPerSharedFeedAndWeatherLocation(t *testing.T) {
	now := time.Date(2026, 10, 5, 8, 0, 0, 0, mustZone(t, "America/Toronto"))
	s := newSeam(t, now)
	s.tr.Serve(feedA, ics(weeklySeries))

	for _, n := range []int{1, 4, 5} {
		got := s.source(t, gemConfig(calendar.Feed{URL: feedA})).Days(now, n)
		if len(got.Days) != n {
			t.Fatalf("asked for %d days, got %d", n, len(got.Days))
		}
		for i, day := range got.Days {
			if day.Forecast == nil {
				t.Errorf("span %d: day %d has no forecast", n, i)
			}
		}
	}

	if got := s.tr.Requests(feedA); got != 1 {
		t.Errorf("feed fetched %d times, want 1", got)
	}
	if got := s.tr.Requests(gemURL); got != 1 {
		t.Errorf("forecast fetched %d times, want 1", got)
	}
}

// The forecast is asked for in the dashboard's zone, the zone of the clock
// that decides Today and the now marker, not the forecast site's. Just
// after midnight in Toronto it is still the evening before in Vancouver,
// so a forecast in Vancouver's zone would show yesterday as Today and put
// the now marker three hours off.
func TestDayData_ForecastIsInTheDashboardZone(t *testing.T) {
	toronto := mustZone(t, "America/Toronto")
	now := time.Date(2026, 10, 6, 1, 30, 0, 0, toronto)
	s := newSeam(t, now)
	var asked string
	vancouver := fakehttp.OpenMeteo{Now: now, Site: mustZone(t, "America/Vancouver")}
	s.tr.Handle(gemURL, func(r *http.Request) fakehttp.Reply {
		asked = r.URL.Query().Get("timezone")
		return vancouver.Reply(r)
	})

	today := s.source(t, gemConfig()).Days(now, 1).Days[0]

	if asked != "America/Toronto" {
		t.Errorf("forecast asked for in %q, want the dashboard's America/Toronto", asked)
	}
	if today.Forecast == nil {
		t.Fatal("no forecast for Today")
	}
	if got := today.Forecast.Date.Format("2006-01-02"); got != "2026-10-06" {
		t.Errorf("Today's forecast is for %s, want 2026-10-06", got)
	}
	nowHour := time.Date(2026, 10, 6, 1, 0, 0, 0, toronto)
	for _, h := range today.Forecast.Hourly {
		if h.Hour == now.Hour() {
			if h.Temperature != fakehttp.EpochHour(nowHour) {
				t.Errorf("the now-marker hour reads the hour starting %v, want %v",
					time.Unix(int64(h.Temperature)*3600, 0).In(toronto), nowHour)
			}
			return
		}
	}
	t.Errorf("Today's forecast has no hour %d", now.Hour())
}

// A widget configured with no feeds gets its days with forecasts and no
// events, and asks no calendar for anything.
func TestDayData_NoFeeds(t *testing.T) {
	now := time.Date(2026, 10, 5, 8, 0, 0, 0, mustZone(t, "America/Toronto"))
	s := newSeam(t, now)

	got := s.source(t, gemConfig()).Days(now, 4)

	for i, day := range got.Days {
		if len(day.Events) != 0 {
			t.Errorf("day %d has events %v", i, summaries(day.Events))
		}
		if day.Forecast == nil {
			t.Errorf("day %d has no forecast", i)
		}
	}
	if got := s.tr.Total(); got != 1 {
		t.Errorf("upstream requests = %d, want only the forecast's", got)
	}
}

// A widget that shows no weather gets its days and events with no forecast,
// and asks for none rather than fetching one to throw away.
func TestDayData_NoWeather(t *testing.T) {
	now := time.Date(2026, 10, 5, 8, 0, 0, 0, mustZone(t, "America/Toronto"))
	s := newSeam(t, now)
	s.tr.Serve(feedA, ics(weeklySeries))
	src, err := daydata.New("test-widget", gemConfig(calendar.Feed{URL: feedA}), s.deps, daydata.WithoutWeather())
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	got := src.Days(now, 4)

	for i, day := range got.Days {
		if day.Forecast != nil {
			t.Errorf("day %d has a forecast", i)
		}
	}
	if got.ForecastArrived {
		t.Error("ForecastArrived with no forecast asked for")
	}
	if g := dayEvents(got); !slices.Contains(g, "Weekly Sync") {
		t.Errorf("events by day = %q, want the weekly series", g)
	}
	if got := s.tr.Requests(gemURL); got != 0 {
		t.Errorf("forecast fetched %d times, want none", got)
	}
}

// A widget built without the calendar module, the weather provider or the
// dashboard's clock fails rather than building its own or reading the wall
// clock, and says which widget.
func TestNew_MissingDeps(t *testing.T) {
	s := newSeam(t, time.Now())
	noCalendar, noWeather, noClock := s.deps, s.deps, s.deps
	noCalendar.Calendar = nil
	noWeather.Weather = nil
	noClock.Now = nil

	for _, c := range []struct {
		label string
		deps  widget.Deps
		want  string
	}{
		{"no calendar module", noCalendar, "test-widget: no calendar module"},
		{"no weather provider", noWeather, "test-widget: no weather provider"},
		{"no clock", noClock, "test-widget: no clock"},
	} {
		t.Run(c.label, func(t *testing.T) {
			_, err := daydata.New("test-widget", gemConfig(), c.deps)
			if err == nil || err.Error() != c.want {
				t.Errorf("err = %v, want %q", err, c.want)
			}
		})
	}
}

// A day the forecast doesn't reach has no forecast rather than a zero one,
// and the shared temperature range is taken across the days that have one,
// so a missing day can't drag it to a 0°C nobody forecast. With no forecast
// at all the range falls back to 0-25°C. A forecast that arrived carrying
// no days is still reported as arrived, so a layout sized on its arrival
// doesn't reflow for a cycle. The calendar is drawn whatever the forecast
// did.
func TestDayData_DayWithNoForecast(t *testing.T) {
	toronto := mustZone(t, "America/Toronto")
	now := time.Date(2026, 10, 5, 8, 0, 0, 0, toronto)
	lastHour := time.Date(2026, 10, 6, 23, 0, 0, 0, toronto)

	tests := []struct {
		label        string
		reply        func(*fakehttp.Client)
		wantForecast []bool
		wantRange    weatherview.TempRange
		wantArrived  bool
	}{
		{
			label: "a forecast that stops short",
			reply: func(tr *fakehttp.Client) {
				tr.Handle(gemURL, fakehttp.OpenMeteo{Now: now, Site: toronto, Days: 2}.Reply)
			},
			wantForecast: []bool{true, true, false, false},
			// Day 0's low is 10°C; the warmest reading is the last hour
			// of day 1, which the fake writes as its epoch hour.
			wantRange:   weatherview.TempRange{Min: 10, Max: fakehttp.EpochHour(lastHour)},
			wantArrived: true,
		},
		{
			label: "a forecast carrying no days",
			reply: func(tr *fakehttp.Client) {
				tr.Serve(gemURL, `{"hourly":{"time":[],"temperature_2m":[],"precipitation_probability":[]},`+
					`"daily":{"time":[],"temperature_2m_max":[],"temperature_2m_min":[],"weather_code":[]}}`)
			},
			wantForecast: []bool{false, false, false, false},
			wantRange:    weatherview.TempRange{Min: 0, Max: 25},
			wantArrived:  true,
		},
		{
			label:        "no forecast at all",
			reply:        func(tr *fakehttp.Client) { tr.Set(gemURL, fakehttp.Reply{Status: http.StatusServiceUnavailable}) },
			wantForecast: []bool{false, false, false, false},
			wantRange:    weatherview.TempRange{Min: 0, Max: 25},
		},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			s := newSeam(t, now)
			s.tr.Serve(feedA, ics(weeklySeries))
			tt.reply(s.tr)

			got := s.source(t, gemConfig(calendar.Feed{URL: feedA})).Days(now, 4)

			if g := dayEvents(got)[0]; g != "Weekly Sync" {
				t.Errorf("Today lists %q, want the calendar's events", g)
			}
			if got.ForecastArrived != tt.wantArrived {
				t.Errorf("ForecastArrived = %v, want %v", got.ForecastArrived, tt.wantArrived)
			}

			for i, day := range got.Days {
				if has := day.Forecast != nil; has != tt.wantForecast[i] {
					t.Errorf("day %d has forecast = %v, want %v", i, has, tt.wantForecast[i])
				}
			}
			if got.TempRange != tt.wantRange {
				t.Errorf("TempRange = %+v, want %+v", got.TempRange, tt.wantRange)
			}
		})
	}
}

// A widget is told when a feed it shows is unavailable: its fetch failed
// and there is no earlier good copy. Its days then can't say "a free day",
// only that the calendar is unavailable. A feed that fails with a good copy
// cached is still usable, as the calendar module serves that copy.
func TestDayData_ReportsAnUnavailableCalendar(t *testing.T) {
	toronto := mustZone(t, "America/Toronto")
	now := time.Date(2026, 10, 5, 8, 0, 0, 0, toronto)
	dentist := fakehttp.Reply{Body: ics(oneOff("dentist", "Dentist", "20261005T140000Z", "20261005T150000Z"))}
	swim := fakehttp.Reply{Body: ics(oneOff("swim", "Swim", "20261005T220000Z", "20261005T230000Z"))}
	down := fakehttp.Reply{Status: http.StatusServiceUnavailable}
	unreachable := fakehttp.Reply{Err: errors.New("network is unreachable")}

	tests := []struct {
		label string
		// earlier is what each feed served on a render before this one,
		// longer ago than the refresh setting; nil for no earlier render.
		earlier map[string]fakehttp.Reply
		replies map[string]fakehttp.Reply
		want    string
		// wantUnavailable is whether the days report an unavailable feed.
		wantUnavailable bool
	}{
		{label: "every feed up", replies: map[string]fakehttp.Reply{feedA: dentist, feedB: swim}, want: "Dentist,Swim"},
		{label: "a feed failing with nothing cached", replies: map[string]fakehttp.Reply{feedA: down, feedB: swim}, want: "Swim", wantUnavailable: true},
		{label: "a feed unreachable with nothing cached", replies: map[string]fakehttp.Reply{feedA: dentist, feedB: unreachable}, want: "Dentist", wantUnavailable: true},
		{label: "every feed failing", replies: map[string]fakehttp.Reply{feedA: down, feedB: unreachable}, wantUnavailable: true},
		{
			label:   "a feed failing with a cached copy",
			earlier: map[string]fakehttp.Reply{feedA: dentist, feedB: swim},
			replies: map[string]fakehttp.Reply{feedA: down, feedB: swim},
			want:    "Dentist,Swim",
		},
		{
			label:           "a feed that has never answered beside one with a cached copy",
			earlier:         map[string]fakehttp.Reply{feedA: down, feedB: swim},
			replies:         map[string]fakehttp.Reply{feedA: down, feedB: down},
			want:            "Swim",
			wantUnavailable: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			s := newSeam(t, now)
			src := s.source(t, gemConfig(calendar.Feed{URL: feedA}, calendar.Feed{URL: feedB}))
			if tt.earlier != nil {
				for url, r := range tt.earlier {
					s.tr.Set(url, r)
				}
				src.Days(s.now, 1)
				s.now = s.now.Add(time.Hour)
			}
			for url, r := range tt.replies {
				s.tr.Set(url, r)
			}

			got := src.Days(s.now, 1)

			if g := dayEvents(got)[0]; g != tt.want {
				t.Errorf("Today lists %q, want %q", g, tt.want)
			}
			if got.CalendarUnavailable != tt.wantUnavailable {
				t.Errorf("CalendarUnavailable = %v, want %v", got.CalendarUnavailable, tt.wantUnavailable)
			}
		})
	}
}
