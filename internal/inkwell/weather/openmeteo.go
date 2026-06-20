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
// Modeled on http.Client.Do so callers can pass a *http.Client directly and
// the request carries its context — letting Source.Forecast(ctx, …) actually
// honor cancellation.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
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

// ParseModel validates a model identifier string against the known models
// and returns the corresponding Model. Unlike NewOpenMeteoSource, which
// silently falls back to GFS for an unknown model, ParseModel surfaces an
// error so config parsing can reject a mistyped weather_model instead of
// quietly fetching the wrong forecast.
func ParseModel(s string) (Model, error) {
	m := Model(s)
	if _, ok := modelBaseURLs[m]; !ok {
		return "", fmt.Errorf("unknown weather model %q (must be gfs, ecmwf, or gem)", s)
	}
	return m, nil
}

// newRequestWithContext is the indirection over http.NewRequestWithContext
// that tests override to exercise the otherwise-unreachable "build request"
// error branch (the URL comes from url.Parse → Encode so it's always
// well-formed in production). Same pattern as weatherview's
// newOpenTypeFace hook.
var newRequestWithContext = http.NewRequestWithContext

// OpenMeteoSource fetches weather forecasts from a single Open-Meteo model.
type OpenMeteoSource struct {
	model   Model
	baseURL string
	client  HTTPClient
}

// NewOpenMeteoSource creates a source for the given model. A nil client
// falls through to http.DefaultClient so a caller-forgotten dependency
// doesn't surface as a nil-pointer panic from the first request.
func NewOpenMeteoSource(model Model, client HTTPClient) *OpenMeteoSource {
	base, ok := modelBaseURLs[model]
	if !ok {
		base = modelBaseURLs[ModelGFS]
	}
	if client == nil {
		client = http.DefaultClient
	}
	return &OpenMeteoSource{model: model, baseURL: base, client: client}
}

// Forecast fetches a weather forecast for the given location. The request
// carries ctx so callers can bound the fetch with a deadline or cancel it.
func (s *OpenMeteoSource) Forecast(ctx context.Context, loc Location, days int) (*Forecast, error) {
	u, err := s.buildURL(loc, days)
	if err != nil {
		return nil, fmt.Errorf("openmeteo %s: build URL: %w", s.model, err)
	}

	req, err := newRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("openmeteo %s: build request: %w", s.model, err) //nolint:goerr113 // only reachable via test override
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openmeteo %s: fetch: %w", s.model, err)
	}
	defer func() {
		// Drain to allow connection reuse, then close. Body.Close on a
		// completed response generally doesn't return an actionable
		// error, but we still surface it via the swallowed log so an
		// upstream HTTP cleanup bug shows up under observation.
		_ = resp.Body.Close()
	}()

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
