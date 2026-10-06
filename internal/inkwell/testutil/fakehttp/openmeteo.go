package fakehttp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"time"
)

// OpenMeteo answers forecast requests the way Open-Meteo does: it honours
// forecast_days and timezone, starting the forecast at today in the
// requested zone and labelling every hour in that zone. "auto" resolves to
// the forecast site's own zone.
//
// Each hour's temperature is EpochHour of the instant it describes, so a
// test can check that the hour a widget reads is the hour it means,
// whatever zone the labels were written in. Day d's high and low are 20+d
// and 10+d.
//
// Register it with Handle:
//
//	tr.Handle("https://api.open-meteo.com/v1/gem", fakehttp.OpenMeteo{...}.Reply)
type OpenMeteo struct {
	// Now is the server's idea of now, which decides its Today.
	Now time.Time
	// Site is the zone "auto" resolves to.
	Site *time.Location
	// Days caps the forecast, as a model whose horizon is shorter than
	// the request would; zero answers every day asked for.
	Days int
	// Sky, when set, is the weather instead of epoch hours, for a test
	// that has to show a day a person would recognise: the temperature
	// (°C) and chance of rain (percent, as Open-Meteo writes it) for the
	// hour starting at hour on the forecast's day-th day. Each day's high
	// and low are then the warmest and coldest of its hours.
	Sky func(day, hour int) (temp, precip float64)
	// Codes is each forecast day's WMO weather code; a day past its end
	// is clear (0).
	Codes []int
}

// Reply answers one forecast request.
func (f OpenMeteo) Reply(req *http.Request) Reply {
	q := req.URL.Query()
	days, err := strconv.Atoi(q.Get("forecast_days"))
	if err != nil {
		return badRequest("forecast_days: " + err.Error())
	}
	if f.Days > 0 {
		days = min(days, f.Days)
	}
	zone := f.Site
	if tz := q.Get("timezone"); tz != "auto" {
		if zone, err = time.LoadLocation(tz); err != nil {
			return badRequest("timezone: " + err.Error())
		}
	}

	local := f.Now.In(zone)
	today := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, zone)

	// The wire format is spelled out here rather than borrowed from the
	// weather parser's struct, so a wrong field name there can't hide
	// behind a matching one here.
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
		hi, lo, code := float64(20+d), float64(10+d), 0
		if d < len(f.Codes) {
			code = f.Codes[d]
		}
		var temps []float64
		for h := day; h.Before(day.AddDate(0, 0, 1)); h = h.Add(time.Hour) {
			temp, precip := EpochHour(h), 0.0
			if f.Sky != nil {
				temp, precip = f.Sky(d, h.Hour())
			}
			temps = append(temps, temp)
			body.Hourly.Time = append(body.Hourly.Time, h.Format("2006-01-02T15:04"))
			body.Hourly.Temperature2m = append(body.Hourly.Temperature2m, temp)
			body.Hourly.PrecipProb = append(body.Hourly.PrecipProb, precip)
		}
		if f.Sky != nil {
			hi, lo = slices.Max(temps), slices.Min(temps)
		}
		body.Daily.Time = append(body.Daily.Time, day.Format("2006-01-02"))
		body.Daily.Max = append(body.Daily.Max, hi)
		body.Daily.Min = append(body.Daily.Min, lo)
		body.Daily.WeatherCode = append(body.Daily.WeatherCode, code)
	}
	raw, _ := json.Marshal(body)
	return Reply{Body: string(raw)}
}

// EpochHour is the temperature OpenMeteo writes for the hour starting at
// t: hours since the Unix epoch.
func EpochHour(t time.Time) float64 { return float64(t.Unix() / 3600) }

func badRequest(msg string) Reply {
	return Reply{Status: http.StatusBadRequest, Body: fmt.Sprintf(`{"reason":%q}`, msg)}
}
