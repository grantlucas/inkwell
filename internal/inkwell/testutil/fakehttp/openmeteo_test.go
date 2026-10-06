package fakehttp_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/testutil/fakehttp"
)

const gem = "https://api.open-meteo.com/v1/gem"

// forecastReply is the slice of Open-Meteo's wire format the fake's tests
// read back.
type forecastReply struct {
	Hourly struct {
		Time        []string  `json:"time"`
		Temperature []float64 `json:"temperature_2m"`
		Precip      []float64 `json:"precipitation_probability"`
	} `json:"hourly"`
	Daily struct {
		Time []string  `json:"time"`
		Max  []float64 `json:"temperature_2m_max"`
		Min  []float64 `json:"temperature_2m_min"`
		Code []int     `json:"weather_code"`
	} `json:"daily"`
}

func askForecast(t *testing.T, om fakehttp.OpenMeteo, query string) (int, forecastReply) {
	t.Helper()
	tr := fakehttp.New()
	tr.Handle(gem, om.Reply)
	resp, err := get(t, context.Background(), tr, gem+"?"+query)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	status, body, _ := read(t, resp)
	var out forecastReply
	if status == http.StatusOK {
		if err := json.Unmarshal([]byte(body), &out); err != nil {
			t.Fatalf("unmarshal %q: %v", body, err)
		}
	}
	return status, out
}

func mustZone(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatalf("load zone %s: %v", name, err)
	}
	return loc
}

// The fake answers the way Open-Meteo does: forecast_days days starting at
// today in the zone asked for, every hour labelled in that zone, with "auto"
// meaning the site's own zone. Each hour's temperature is EpochHour of the
// instant it describes, so a test can tell which hour it is reading.
func TestOpenMeteo_AnswersLikeOpenMeteo(t *testing.T) {
	toronto := mustZone(t, "America/Toronto")
	vancouver := mustZone(t, "America/Vancouver")
	// 01:30 in Toronto is still the evening before in Vancouver.
	now := time.Date(2026, 10, 6, 1, 30, 0, 0, toronto)

	tests := []struct {
		label     string
		om        fakehttp.OpenMeteo
		query     string
		wantDays  []string
		firstHour time.Time
	}{
		{
			label:     "a named zone",
			om:        fakehttp.OpenMeteo{Now: now, Site: vancouver},
			query:     "forecast_days=2&timezone=America%2FToronto",
			wantDays:  []string{"2026-10-06", "2026-10-07"},
			firstHour: time.Date(2026, 10, 6, 0, 0, 0, 0, toronto),
		},
		{
			label:     "auto is the site's zone",
			om:        fakehttp.OpenMeteo{Now: now, Site: vancouver},
			query:     "forecast_days=1&timezone=auto",
			wantDays:  []string{"2026-10-05"},
			firstHour: time.Date(2026, 10, 5, 0, 0, 0, 0, vancouver),
		},
		{
			label:     "a forecast that stops short of the days asked",
			om:        fakehttp.OpenMeteo{Now: now, Site: toronto, Days: 1},
			query:     "forecast_days=8&timezone=auto",
			wantDays:  []string{"2026-10-06"},
			firstHour: time.Date(2026, 10, 6, 0, 0, 0, 0, toronto),
		},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			status, got := askForecast(t, tt.om, tt.query)
			if status != http.StatusOK {
				t.Fatalf("status = %d", status)
			}
			if len(got.Daily.Time) != len(tt.wantDays) {
				t.Fatalf("days = %v, want %v", got.Daily.Time, tt.wantDays)
			}
			for i, d := range tt.wantDays {
				if got.Daily.Time[i] != d {
					t.Errorf("day %d = %s, want %s", i, got.Daily.Time[i], d)
				}
			}
			if len(got.Hourly.Time) != 24*len(tt.wantDays) {
				t.Errorf("hours = %d, want %d", len(got.Hourly.Time), 24*len(tt.wantDays))
			}
			if got.Hourly.Temperature[0] != fakehttp.EpochHour(tt.firstHour) {
				t.Errorf("first hour reads %v, want the hour starting %v", got.Hourly.Temperature[0], tt.firstHour)
			}
		})
	}
}

// A request Open-Meteo would refuse is refused.
func TestOpenMeteo_RejectsBadQueries(t *testing.T) {
	om := fakehttp.OpenMeteo{Now: time.Now(), Site: time.UTC}
	for _, query := range []string{
		"timezone=auto",
		"forecast_days=two&timezone=auto",
		"forecast_days=1&timezone=Not%2FAZone",
	} {
		if status, _ := askForecast(t, om, query); status != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", query, status)
		}
	}
}

// A fake given a Sky answers with that weather instead of epoch hours, so
// a golden can show a day a person would recognise: each hour's
// temperature and chance of rain from Sky, each day's high and low the
// warmest and coldest of its hours, and its weather code from Codes.
func TestOpenMeteo_Sky(t *testing.T) {
	om := fakehttp.OpenMeteo{
		Now:  time.Date(2026, 10, 6, 10, 0, 0, 0, time.UTC),
		Site: time.UTC,
		Sky: func(day, hour int) (temp, precip float64) {
			return float64(10*day + hour%5), float64(hour)
		},
		Codes: []int{61},
	}
	status, got := askForecast(t, om, "forecast_days=2&timezone=auto")
	if status != http.StatusOK {
		t.Fatalf("status = %d", status)
	}
	if got.Hourly.Temperature[7] != 2 || got.Hourly.Temperature[24+3] != 13 {
		t.Errorf("hourly temperatures = %v, want Sky's", got.Hourly.Temperature)
	}
	if got.Hourly.Precip[23] != 23 {
		t.Errorf("hour 23's chance = %v, want 23", got.Hourly.Precip[23])
	}
	wantDaily := []struct {
		max, min float64
		code     int
	}{{4, 0, 61}, {14, 10, 0}}
	for d, want := range wantDaily {
		if got.Daily.Max[d] != want.max || got.Daily.Min[d] != want.min || got.Daily.Code[d] != want.code {
			t.Errorf("day %d = %v/%v code %d, want %v/%v code %d",
				d, got.Daily.Max[d], got.Daily.Min[d], got.Daily.Code[d], want.max, want.min, want.code)
		}
	}
}
