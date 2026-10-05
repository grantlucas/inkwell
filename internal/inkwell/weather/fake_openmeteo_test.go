package weather

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// fakeOpenMeteo answers forecast requests the way Open-Meteo does: it
// honours forecast_days and timezone, starting the forecast at today in
// the requested zone and labelling every hour in that zone. "auto"
// resolves to the forecast location's own zone, which the test sets.
//
// Each hour's temperature encodes the instant it describes (hours since
// the Unix epoch), so a test can check that the hour a widget reads is
// the hour it means, whatever zone the labels were written in.
type fakeOpenMeteo struct {
	now     time.Time      // the server's idea of now
	siteLoc *time.Location // what "auto" resolves to
	down    bool           // answer every request with a 503

	mu   sync.Mutex
	urls []string
}

func (f *fakeOpenMeteo) Do(req *http.Request) (*http.Response, error) {
	f.mu.Lock()
	f.urls = append(f.urls, req.URL.String())
	down := f.down
	f.mu.Unlock()
	if down {
		return &http.Response{StatusCode: http.StatusServiceUnavailable, Body: http.NoBody}, nil
	}

	q := req.URL.Query()
	days, err := strconv.Atoi(q.Get("forecast_days"))
	if err != nil {
		return badRequest("forecast_days: " + err.Error()), nil
	}
	zone := f.siteLoc
	if tz := q.Get("timezone"); tz != "auto" {
		if zone, err = time.LoadLocation(tz); err != nil {
			return badRequest("timezone: " + err.Error()), nil
		}
	}

	local := f.now.In(zone)
	today := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, zone)

	// The wire format is spelled out here rather than borrowed from the
	// parser's struct, so a wrong field name there can't hide behind a
	// matching one here.
	var body struct {
		Hourly struct {
			Time          []string  `json:"time"`
			Temperature2m []float64 `json:"temperature_2m"`
			PrecipProb    []float64 `json:"precipitation_probability"`
		} `json:"hourly"`
		Daily struct {
			Time        []string  `json:"time"`
			Max         []float64 `json:"temperature_2m_max"`
			Min         []float64 `json:"temperature_2m_min"`
			WeatherCode []int     `json:"weather_code"`
		} `json:"daily"`
	}
	for d := range days {
		day := today.AddDate(0, 0, d)
		body.Daily.Time = append(body.Daily.Time, day.Format("2006-01-02"))
		body.Daily.Max = append(body.Daily.Max, float64(20+d))
		body.Daily.Min = append(body.Daily.Min, float64(10+d))
		body.Daily.WeatherCode = append(body.Daily.WeatherCode, 0)
		for h := day; h.Before(day.AddDate(0, 0, 1)); h = h.Add(time.Hour) {
			body.Hourly.Time = append(body.Hourly.Time, h.Format("2006-01-02T15:04"))
			body.Hourly.Temperature2m = append(body.Hourly.Temperature2m, epochHour(h))
			body.Hourly.PrecipProb = append(body.Hourly.PrecipProb, 0)
		}
	}
	raw, _ := json.Marshal(body)
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(string(raw)))}, nil
}

// epochHour is the value the fake writes for the hour starting at t.
func epochHour(t time.Time) float64 { return float64(t.Unix() / 3600) }

func (f *fakeOpenMeteo) requests() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.urls...)
}

func (f *fakeOpenMeteo) setDown(down bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.down = down
}

func (f *fakeOpenMeteo) lastURL() string {
	urls := f.requests()
	return urls[len(urls)-1]
}

func badRequest(msg string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusBadRequest,
		Body:       io.NopCloser(strings.NewReader(fmt.Sprintf(`{"reason":%q}`, msg))),
	}
}
