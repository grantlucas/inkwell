package weather

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type mockHTTPClient struct {
	response *http.Response
	err      error
	lastURL  string
}

func (m *mockHTTPClient) Get(url string) (*http.Response, error) {
	m.lastURL = url
	return m.response, m.err
}

const sampleResponse = `{
	"hourly": {
		"time": ["2026-04-28T06:00","2026-04-28T07:00","2026-04-28T08:00"],
		"temperature_2m": [9.0, 10.5, 12.0],
		"precipitation_probability": [0, 20, 50]
	},
	"daily": {
		"time": ["2026-04-28"],
		"temperature_2m_max": [17.0],
		"temperature_2m_min": [9.0],
		"weather_code": [2]
	}
}`

func TestOpenMeteoSource_Forecast(t *testing.T) {
	client := &mockHTTPClient{
		response: &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(sampleResponse)),
		},
	}

	src := NewOpenMeteoSource(ModelGFS, client)
	loc := Location{Latitude: 45.4215, Longitude: -75.6972}
	fc, err := src.Forecast(context.Background(), loc, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(fc.Days) != 1 {
		t.Fatalf("got %d days, want 1", len(fc.Days))
	}

	day := fc.Days[0]
	if day.High != 17.0 {
		t.Errorf("High = %v, want 17.0", day.High)
	}
	if day.Low != 9.0 {
		t.Errorf("Low = %v, want 9.0", day.Low)
	}
	if day.Condition != PartlyCloudy {
		t.Errorf("Condition = %v, want PartlyCloudy", day.Condition)
	}
	if len(day.Hourly) != 3 {
		t.Fatalf("got %d hourly points, want 3", len(day.Hourly))
	}
	if day.Hourly[0].Hour != 6 {
		t.Errorf("Hourly[0].Hour = %d, want 6", day.Hourly[0].Hour)
	}
	if day.Hourly[0].Temperature != 9.0 {
		t.Errorf("Hourly[0].Temperature = %v, want 9.0", day.Hourly[0].Temperature)
	}
	if day.Hourly[1].PrecipitationProb != 0.2 {
		t.Errorf("Hourly[1].PrecipitationProb = %v, want 0.2", day.Hourly[1].PrecipitationProb)
	}
}

func TestOpenMeteoSource_BuildURL(t *testing.T) {
	client := &mockHTTPClient{
		response: &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(sampleResponse)),
		},
	}

	src := NewOpenMeteoSource(ModelECMWF, client)
	loc := Location{Latitude: 45.4215, Longitude: -75.6972}
	_, _ = src.Forecast(context.Background(), loc, 7)

	if !strings.Contains(client.lastURL, "api.open-meteo.com/v1/ecmwf") {
		t.Errorf("URL = %q, want ecmwf endpoint", client.lastURL)
	}
	if !strings.Contains(client.lastURL, "latitude=45.4215") {
		t.Errorf("URL = %q, missing latitude", client.lastURL)
	}
	if !strings.Contains(client.lastURL, "forecast_days=7") {
		t.Errorf("URL = %q, missing forecast_days", client.lastURL)
	}
	if !strings.Contains(client.lastURL, "timezone=auto") {
		t.Errorf("URL = %q, missing timezone", client.lastURL)
	}
}

func TestOpenMeteoSource_HTTPError(t *testing.T) {
	client := &mockHTTPClient{
		err: errors.New("network error"),
	}

	src := NewOpenMeteoSource(ModelGFS, client)
	_, err := src.Forecast(context.Background(), Location{}, 1)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "network error") {
		t.Errorf("error = %q, want to contain 'network error'", err.Error())
	}
}

func TestOpenMeteoSource_BadStatus(t *testing.T) {
	client := &mockHTTPClient{
		response: &http.Response{
			StatusCode: 429,
			Body:       io.NopCloser(strings.NewReader("")),
		},
	}

	src := NewOpenMeteoSource(ModelGFS, client)
	_, err := src.Forecast(context.Background(), Location{}, 1)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "HTTP 429") {
		t.Errorf("error = %q, want to contain 'HTTP 429'", err.Error())
	}
}

func TestOpenMeteoSource_BadJSON(t *testing.T) {
	client := &mockHTTPClient{
		response: &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader("not json")),
		},
	}

	src := NewOpenMeteoSource(ModelGFS, client)
	_, err := src.Forecast(context.Background(), Location{}, 1)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "parse JSON") {
		t.Errorf("error = %q, want to contain 'parse JSON'", err.Error())
	}
}

func TestOpenMeteoSource_UnknownModel(t *testing.T) {
	client := &mockHTTPClient{
		response: &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(sampleResponse)),
		},
	}

	src := NewOpenMeteoSource(Model("unknown"), client)
	if src.baseURL != modelBaseURLs[ModelGFS] {
		t.Errorf("baseURL = %q, want GFS fallback", src.baseURL)
	}
}

func TestOpenMeteoSource_EmptyResponse(t *testing.T) {
	client := &mockHTTPClient{
		response: &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(`{"hourly":{"time":[],"temperature_2m":[],"precipitation_probability":[]},"daily":{"time":[],"temperature_2m_max":[],"temperature_2m_min":[],"weather_code":[]}}`)),
		},
	}

	src := NewOpenMeteoSource(ModelGFS, client)
	fc, err := src.Forecast(context.Background(), Location{}, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fc.Days) != 0 {
		t.Errorf("got %d days, want 0", len(fc.Days))
	}
}

func TestOpenMeteoSource_Location(t *testing.T) {
	client := &mockHTTPClient{
		response: &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(sampleResponse)),
		},
	}

	loc := Location{Latitude: 45.4215, Longitude: -75.6972}
	src := NewOpenMeteoSource(ModelGFS, client)
	fc, err := src.Forecast(context.Background(), loc, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fc.Location != loc {
		t.Errorf("Location = %v, want %v", fc.Location, loc)
	}
}

func TestOpenMeteoSource_BadBaseURL(t *testing.T) {
	src := &OpenMeteoSource{model: "test", baseURL: "://bad", client: nil}
	_, err := src.Forecast(context.Background(), Location{}, 1)
	if err == nil {
		t.Fatal("expected error for bad base URL")
	}
}

func TestOpenMeteoSource_ReadBodyError(t *testing.T) {
	client := &mockHTTPClient{
		response: &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(&errReader{}),
		},
	}

	src := NewOpenMeteoSource(ModelGFS, client)
	_, err := src.Forecast(context.Background(), Location{}, 1)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "read body") {
		t.Errorf("error = %q, want to contain 'read body'", err.Error())
	}
}

func TestOpenMeteoSource_BadDailyDate(t *testing.T) {
	resp := `{
		"hourly":{"time":[],"temperature_2m":[],"precipitation_probability":[]},
		"daily":{"time":["not-a-date"],"temperature_2m_max":[17],"temperature_2m_min":[9],"weather_code":[0]}
	}`
	client := &mockHTTPClient{
		response: &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(resp)),
		},
	}

	src := NewOpenMeteoSource(ModelGFS, client)
	fc, err := src.Forecast(context.Background(), Location{}, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fc.Days) != 0 {
		t.Errorf("got %d days, want 0 (bad date skipped)", len(fc.Days))
	}
}

func TestOpenMeteoSource_BadHourlyTime(t *testing.T) {
	resp := `{
		"hourly":{"time":["bad-time"],"temperature_2m":[10],"precipitation_probability":[20]},
		"daily":{"time":["2026-04-28"],"temperature_2m_max":[17],"temperature_2m_min":[9],"weather_code":[0]}
	}`
	client := &mockHTTPClient{
		response: &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(resp)),
		},
	}

	src := NewOpenMeteoSource(ModelGFS, client)
	fc, err := src.Forecast(context.Background(), Location{}, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fc.Days[0].Hourly) != 0 {
		t.Errorf("got %d hourly, want 0 (bad time skipped)", len(fc.Days[0].Hourly))
	}
}

func TestOpenMeteoSource_HourlyForUnknownDay(t *testing.T) {
	resp := `{
		"hourly":{"time":["2026-04-29T06:00"],"temperature_2m":[10],"precipitation_probability":[0]},
		"daily":{"time":["2026-04-28"],"temperature_2m_max":[17],"temperature_2m_min":[9],"weather_code":[0]}
	}`
	client := &mockHTTPClient{
		response: &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(resp)),
		},
	}

	src := NewOpenMeteoSource(ModelGFS, client)
	fc, err := src.Forecast(context.Background(), Location{}, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fc.Days[0].Hourly) != 0 {
		t.Errorf("got %d hourly, want 0 (orphan hourly skipped)", len(fc.Days[0].Hourly))
	}
}

func TestOpenMeteoSource_SparseArrays(t *testing.T) {
	resp := `{
		"hourly":{"time":["2026-04-28T06:00","2026-04-28T07:00"],"temperature_2m":[10],"precipitation_probability":[]},
		"daily":{"time":["2026-04-28"],"temperature_2m_max":[],"temperature_2m_min":[],"weather_code":[]}
	}`
	client := &mockHTTPClient{
		response: &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(resp)),
		},
	}

	src := NewOpenMeteoSource(ModelGFS, client)
	fc, err := src.Forecast(context.Background(), Location{}, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fc.Days[0].High != 0 {
		t.Errorf("High = %v, want 0 (empty array)", fc.Days[0].High)
	}
	if len(fc.Days[0].Hourly) != 2 {
		t.Fatalf("got %d hourly, want 2", len(fc.Days[0].Hourly))
	}
	if fc.Days[0].Hourly[1].Temperature != 0 {
		t.Errorf("Hourly[1].Temperature = %v, want 0 (sparse)", fc.Days[0].Hourly[1].Temperature)
	}
}

type errReader struct{}

func (e *errReader) Read(_ []byte) (int, error) {
	return 0, errors.New("read error")
}

func TestOpenMeteoSource_GEMModel(t *testing.T) {
	client := &mockHTTPClient{
		response: &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(sampleResponse)),
		},
	}

	src := NewOpenMeteoSource(ModelGEM, client)
	_, _ = src.Forecast(context.Background(), Location{}, 1)
	if !strings.Contains(client.lastURL, "api.open-meteo.com/v1/gem") {
		t.Errorf("URL = %q, want gem endpoint", client.lastURL)
	}
}
