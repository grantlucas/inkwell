package weather

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

func providerClock(t time.Time) func() time.Time {
	return func() time.Time { return t }
}

func TestProvider_ForecastFetchesAndCaches(t *testing.T) {
	now := time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
	client := &fakeOpenMeteo{now: now, siteLoc: time.UTC}
	p := NewProvider(client, time.Hour, providerClock(now), Settings{})

	loc := Location{Latitude: 43.244, Longitude: -79.837}
	fc1, err := p.Forecast(context.Background(), loc, ModelGEM, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fc1 == nil || len(fc1.Days) != 1 {
		t.Fatalf("got %v, want a 1-day forecast", fc1)
	}
	if !strings.Contains(client.lastURL(), "/v1/gem") {
		t.Errorf("URL = %q, want gem endpoint", client.lastURL())
	}

	// Second identical call is served from cache: no extra HTTP fetch.
	if _, err := p.Forecast(context.Background(), loc, ModelGEM, 1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(client.requests()) != 1 {
		t.Errorf("client called %d times, want 1 (cached)", len(client.requests()))
	}
}

func TestProvider_DistinctKeysCacheIndependently(t *testing.T) {
	now := time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
	client := &fakeOpenMeteo{now: now, siteLoc: time.UTC}
	p := NewProvider(client, time.Hour, providerClock(now), Settings{})

	loc1 := Location{Latitude: 43.244, Longitude: -79.837}
	loc2 := Location{Latitude: 49.283, Longitude: -123.121}
	ctx := context.Background()

	// Three distinct keys: loc1/gem, loc2/gem, loc1/ecmwf → three fetches.
	_, _ = p.Forecast(ctx, loc1, ModelGEM, 1)
	_, _ = p.Forecast(ctx, loc2, ModelGEM, 1)
	_, _ = p.Forecast(ctx, loc1, ModelECMWF, 1)
	// Repeats of each key are cached — no further fetches, no thrash.
	_, _ = p.Forecast(ctx, loc1, ModelGEM, 1)
	_, _ = p.Forecast(ctx, loc2, ModelGEM, 1)

	if len(client.requests()) != 3 {
		t.Errorf("client called %d times, want 3 (one per distinct key)", len(client.requests()))
	}
}

func TestProvider_SourceForModelSharesCache(t *testing.T) {
	now := time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
	client := &fakeOpenMeteo{now: now, siteLoc: time.UTC}
	p := NewProvider(client, time.Hour, providerClock(now), Settings{})
	loc := Location{Latitude: 43.244, Longitude: -79.837}

	src := p.SourceForModel(ModelECMWF)
	if _, err := src.Forecast(context.Background(), loc, 1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(client.lastURL(), "/v1/ecmwf") {
		t.Errorf("URL = %q, want ecmwf endpoint", client.lastURL())
	}
	// A direct Provider call for the same key reuses the bound source's cache.
	if _, err := p.Forecast(context.Background(), loc, ModelECMWF, 1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(client.requests()) != 1 {
		t.Errorf("client called %d times, want 1 (shared cache)", len(client.requests()))
	}
}

func TestProvider_Defaults(t *testing.T) {
	want := Settings{
		Location: Location{Latitude: 43.244, Longitude: -79.837},
		Model:    ModelGEM,
		TempUnit: "C",
	}
	p := NewProvider(&fakeOpenMeteo{}, time.Hour, nil, want)
	if got := p.Defaults(); got != want {
		t.Errorf("Defaults() = %+v, want %+v", got, want)
	}
}

func TestProvider_ConcurrentForecastDedupes(t *testing.T) {
	now := time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
	client := &fakeOpenMeteo{now: now, siteLoc: time.UTC}
	p := NewProvider(client, time.Hour, providerClock(now), Settings{})
	loc := Location{Latitude: 43.244, Longitude: -79.837}

	const goroutines = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			_, _ = p.Forecast(context.Background(), loc, ModelGEM, 1)
		}()
	}
	wg.Wait()

	// All concurrent callers for the same key share one cache entry, so the
	// upstream is hit exactly once.
	if got := len(client.requests()); got != 1 {
		t.Errorf("client served %d requests, want 1 (concurrent callers share one fetch)", got)
	}
}

// A composed screen asks one location for different spans — today-weather
// wants a day, weather-ahead four, the weather lane its own — and each is
// answered from one upstream fetch, cut to the span asked for.
func TestProvider_DifferentSpansShareOneFetch(t *testing.T) {
	toronto := mustZone(t, "America/Toronto")
	now := time.Date(2026, 10, 5, 9, 0, 0, 0, toronto)
	api := &fakeOpenMeteo{now: now, siteLoc: toronto}
	p := NewProvider(api, time.Hour, providerClock(now), Settings{})
	loc := Location{Latitude: 43.244, Longitude: -79.837}

	for _, days := range []int{1, 4, 5} {
		fc, err := p.Forecast(context.Background(), loc, ModelGEM, days)
		if err != nil {
			t.Fatalf("days=%d: unexpected error: %v", days, err)
		}
		if len(fc.Days) != days {
			t.Errorf("days=%d: got %d days", days, len(fc.Days))
		}
		if got := fc.Days[0].Date.Format("2006-01-02"); got != "2026-10-05" {
			t.Errorf("days=%d: first day = %s, want 2026-10-05", days, got)
		}
	}
	if got := len(api.requests()); got != 1 {
		t.Errorf("upstream served %d requests, want 1", got)
	}
}

func mustZone(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatalf("load zone %s: %v", name, err)
	}
	return loc
}

// The forecast is asked for in the dashboard's zone — the zone of the clock
// that decides Today and the now marker — not the forecast location's. Just
// after midnight in Toronto it is still the evening before in Vancouver, so a
// forecast in Vancouver's zone would show yesterday as Today and put the now
// marker three hours off.
func TestProvider_ForecastIsInTheDashboardZone(t *testing.T) {
	toronto := mustZone(t, "America/Toronto")
	vancouver := mustZone(t, "America/Vancouver")
	now := time.Date(2026, 10, 6, 1, 30, 0, 0, toronto)
	api := &fakeOpenMeteo{now: now, siteLoc: vancouver}
	p := NewProvider(api, time.Hour, providerClock(now), Settings{})
	vancouverSite := Location{Latitude: 49.283, Longitude: -123.121}

	fc, err := p.Forecast(context.Background(), vancouverSite, ModelGEM, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := api.requests()[0]; !strings.Contains(got, "timezone=America%2FToronto") {
		t.Errorf("request %q does not carry the dashboard zone", got)
	}
	today := fc.Days[0]
	if got := today.Date.Format("2006-01-02"); got != "2026-10-06" {
		t.Fatalf("Today's forecast is for %s, want the dashboard's Today 2026-10-06", got)
	}
	nowHour := time.Date(2026, 10, 6, now.Hour(), 0, 0, 0, toronto)
	for _, h := range today.Hourly {
		if h.Hour == now.Hour() {
			if h.Temperature != epochHour(nowHour) {
				t.Errorf("the now-marker hour %d reads the hour starting %v, want %v",
					h.Hour, time.Unix(int64(h.Temperature)*3600, 0).In(toronto), nowHour)
			}
			return
		}
	}
	t.Errorf("Today's forecast has no hour %d", now.Hour())
}

// With no timezone configured the dashboard's clock is time.Local, which
// Go names "Local" whatever zone it holds — a name Open-Meteo would reject.
// The request carries the host's real zone name instead, found the way Go
// finds it, and falls back to the forecast location's zone only when the
// host can't name its zone.
func TestProvider_HostLocalZoneIsNamed(t *testing.T) {
	noLink := func(string) (string, error) { return "", errors.New("not a link") }
	cases := []struct {
		label    string
		env      map[string]string
		readlink func(string) (string, error)
		want     string
	}{
		{"TZ names the zone", map[string]string{"TZ": "Europe/Paris"}, noLink, "Europe/Paris"},
		{"TZ with a leading colon", map[string]string{"TZ": ":Asia/Tokyo"}, noLink, "Asia/Tokyo"},
		{"TZ as a zoneinfo path", map[string]string{"TZ": "/usr/share/zoneinfo/America/Halifax"}, noLink, "America/Halifax"},
		{"empty TZ is UTC", map[string]string{"TZ": ""}, noLink, "UTC"},
		{"/etc/localtime links into zoneinfo", nil, func(string) (string, error) {
			return "/var/db/timezone/zoneinfo/America/Toronto", nil
		}, "America/Toronto"},
		{"/etc/localtime links elsewhere", nil, func(string) (string, error) {
			return "/etc/zones/mine", nil
		}, "auto"},
		{"no TZ and no link", nil, noLink, "auto"},
	}
	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			swapHostLookups(t, tc.env, tc.readlink)
			api := &fakeOpenMeteo{now: time.Now(), siteLoc: time.UTC}
			p := NewProvider(api, time.Hour, time.Now, Settings{})

			_, _ = p.Forecast(context.Background(), Location{}, ModelGEM, 1)

			got, err := url.Parse(api.requests()[0])
			if err != nil {
				t.Fatal(err)
			}
			if tz := got.Query().Get("timezone"); tz != tc.want {
				t.Errorf("timezone = %q, want %q", tz, tc.want)
			}
		})
	}
}

// swapHostLookups stands in env and readlink for the host's environment and
// /etc/localtime for the length of the test.
func swapHostLookups(t *testing.T, env map[string]string, link func(string) (string, error)) {
	t.Helper()
	origEnv, origLink := lookupEnv, readlink
	t.Cleanup(func() { lookupEnv, readlink = origEnv, origLink })
	lookupEnv = func(k string) (string, bool) { v, ok := env[k]; return v, ok }
	readlink = link
}

// The cache outlives midnight, so a forecast fetched late in the evening is
// still being served the next morning. It answers from the new Today, not
// from the day it was fetched on, and still covers the whole span.
func TestProvider_CachedForecastStartsAtTodayAfterMidnight(t *testing.T) {
	toronto := mustZone(t, "America/Toronto")
	evening := time.Date(2026, 10, 5, 23, 0, 0, 0, toronto)
	now := evening
	api := &fakeOpenMeteo{now: evening, siteLoc: toronto}
	p := NewProvider(api, 3*time.Hour, func() time.Time { return now }, Settings{})
	loc := Location{Latitude: 43.244, Longitude: -79.837}

	if _, err := p.Forecast(context.Background(), loc, ModelGEM, 5); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	now = evening.Add(2 * time.Hour) // 01:00 the next day, cache still fresh
	fc, err := p.Forecast(context.Background(), loc, ModelGEM, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := len(api.requests()); got != 1 {
		t.Fatalf("upstream served %d requests, want 1 (served from cache)", got)
	}
	var dates []string
	for _, d := range fc.Days {
		dates = append(dates, d.Date.Format("01-02"))
	}
	want := "10-06 10-07 10-08 10-09 10-10"
	if got := strings.Join(dates, " "); got != want {
		t.Errorf("days = %s, want %s", got, want)
	}
}

// A failed fetch with nothing cached returns no forecast. Once a forecast has
// arrived, a later failure serves it again, cut to the span asked for,
// alongside the error.
func TestProvider_FailedFetchServesTheLastGoodForecast(t *testing.T) {
	toronto := mustZone(t, "America/Toronto")
	now := time.Date(2026, 10, 5, 9, 0, 0, 0, toronto)
	api := &fakeOpenMeteo{now: now, siteLoc: toronto, down: true}
	p := NewProvider(api, time.Hour, func() time.Time { return now }, Settings{})
	loc := Location{Latitude: 43.244, Longitude: -79.837}
	ctx := context.Background()

	if fc, err := p.Forecast(ctx, loc, ModelGEM, 5); fc != nil || err == nil {
		t.Fatalf("first fetch down: got (%v, %v), want no forecast and an error", fc, err)
	}

	api.setDown(false)
	if _, err := p.Forecast(ctx, loc, ModelGEM, 5); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	api.setDown(true)
	now = now.Add(2 * time.Hour) // past the cache's hour
	fc, err := p.Forecast(ctx, loc, ModelGEM, 3)
	if err == nil {
		t.Error("expected the fetch error alongside the stale forecast")
	}
	if fc == nil || len(fc.Days) != 3 {
		t.Fatalf("got %v, want the last good forecast cut to 3 days", fc)
	}
}
