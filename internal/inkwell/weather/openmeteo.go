package weather

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// HTTPClient is the subset of *http.Client needed for fetching weather data.
type HTTPClient interface {
	Get(url string) (*http.Response, error)
}

// Model identifies an Open-Meteo forecast model.
type Model string

const (
	ModelGFS   Model = "gfs"
	ModelECMWF Model = "ecmwf"
	ModelGEM   Model = "gem"
)

var modelBaseURLs = map[Model]string{
	ModelGFS:   "https://api.open-meteo.com/v1/forecast",
	ModelECMWF: "https://api.open-meteo.com/v1/ecmwf",
	ModelGEM:   "https://api.open-meteo.com/v1/gem",
}

// OpenMeteoSource fetches weather forecasts from a single Open-Meteo model.
type OpenMeteoSource struct {
	model   Model
	baseURL string
	client  HTTPClient
}

// NewOpenMeteoSource creates a source for the given model.
func NewOpenMeteoSource(model Model, client HTTPClient) *OpenMeteoSource {
	base, ok := modelBaseURLs[model]
	if !ok {
		base = modelBaseURLs[ModelGFS]
	}
	return &OpenMeteoSource{model: model, baseURL: base, client: client}
}

// Forecast fetches a weather forecast for the given location.
func (s *OpenMeteoSource) Forecast(_ context.Context, loc Location, days int) (*Forecast, error) {
	u, err := s.buildURL(loc, days)
	if err != nil {
		return nil, fmt.Errorf("openmeteo %s: build URL: %w", s.model, err)
	}

	resp, err := s.client.Get(u)
	if err != nil {
		return nil, fmt.Errorf("openmeteo %s: fetch: %w", s.model, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openmeteo %s: HTTP %d", s.model, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("openmeteo %s: read body: %w", s.model, err)
	}

	return s.parseResponse(body, loc)
}

func (s *OpenMeteoSource) buildURL(loc Location, days int) (string, error) {
	u, err := url.Parse(s.baseURL)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("latitude", strconv.FormatFloat(loc.Latitude, 'f', 4, 64))
	q.Set("longitude", strconv.FormatFloat(loc.Longitude, 'f', 4, 64))
	q.Set("hourly", "temperature_2m,precipitation_probability")
	q.Set("daily", "temperature_2m_max,temperature_2m_min,weather_code")
	q.Set("forecast_days", strconv.Itoa(days))
	q.Set("timezone", "auto")
	u.RawQuery = q.Encode()
	return u.String(), nil
}

type openMeteoResponse struct {
	Hourly struct {
		Time                     []string  `json:"time"`
		Temperature2m            []float64 `json:"temperature_2m"`
		PrecipitationProbability []float64 `json:"precipitation_probability"`
	} `json:"hourly"`
	Daily struct {
		Time             []string  `json:"time"`
		Temperature2mMax []float64 `json:"temperature_2m_max"`
		Temperature2mMin []float64 `json:"temperature_2m_min"`
		WeatherCode      []int     `json:"weather_code"`
	} `json:"daily"`
}

func (s *OpenMeteoSource) parseResponse(body []byte, loc Location) (*Forecast, error) {
	var resp openMeteoResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("openmeteo %s: parse JSON: %w", s.model, err)
	}

	dayMap := make(map[string]*DailyForecast)
	var dayOrder []string

	for i, dateStr := range resp.Daily.Time {
		d, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			continue
		}
		df := &DailyForecast{Date: d}
		if i < len(resp.Daily.Temperature2mMax) {
			df.High = resp.Daily.Temperature2mMax[i]
		}
		if i < len(resp.Daily.Temperature2mMin) {
			df.Low = resp.Daily.Temperature2mMin[i]
		}
		if i < len(resp.Daily.WeatherCode) {
			df.Condition = ConditionFromWMO(resp.Daily.WeatherCode[i])
		}
		dayMap[dateStr] = df
		dayOrder = append(dayOrder, dateStr)
	}

	for i, timeStr := range resp.Hourly.Time {
		t, err := time.Parse("2006-01-02T15:04", timeStr)
		if err != nil {
			continue
		}
		dateKey := t.Format("2006-01-02")
		df, ok := dayMap[dateKey]
		if !ok {
			continue
		}
		hp := HourlyPoint{Hour: t.Hour()}
		if i < len(resp.Hourly.Temperature2m) {
			hp.Temperature = resp.Hourly.Temperature2m[i]
		}
		if i < len(resp.Hourly.PrecipitationProbability) {
			hp.PrecipitationProb = resp.Hourly.PrecipitationProbability[i] / 100.0
		}
		df.Hourly = append(df.Hourly, hp)
	}

	forecast := &Forecast{Location: loc}
	for _, key := range dayOrder {
		forecast.Days = append(forecast.Days, *dayMap[key])
	}

	return forecast, nil
}
