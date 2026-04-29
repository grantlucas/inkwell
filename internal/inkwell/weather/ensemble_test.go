package weather

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"
)

type stubSource struct {
	forecast *Forecast
	err      error
}

func (s *stubSource) Forecast(_ context.Context, loc Location, _ int) (*Forecast, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.forecast, nil
}

func TestEnsembleSource_AveragesTwoSources(t *testing.T) {
	date := time.Date(2026, 4, 28, 0, 0, 0, 0, time.UTC)
	s1 := &stubSource{forecast: &Forecast{Days: []DailyForecast{
		{Date: date, High: 20, Low: 10, Condition: Clear, Hourly: []HourlyPoint{
			{Hour: 6, Temperature: 10, PrecipitationProb: 0.0},
			{Hour: 12, Temperature: 18, PrecipitationProb: 0.2},
		}},
	}}}
	s2 := &stubSource{forecast: &Forecast{Days: []DailyForecast{
		{Date: date, High: 22, Low: 12, Condition: Rain, Hourly: []HourlyPoint{
			{Hour: 6, Temperature: 12, PrecipitationProb: 0.4},
			{Hour: 12, Temperature: 20, PrecipitationProb: 0.6},
		}},
	}}}

	ensemble := NewEnsembleSource(s1, s2)
	fc, err := ensemble.Forecast(context.Background(), Location{}, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(fc.Days) != 1 {
		t.Fatalf("got %d days, want 1", len(fc.Days))
	}

	day := fc.Days[0]
	if day.High != 21 {
		t.Errorf("High = %v, want 21", day.High)
	}
	if day.Low != 11 {
		t.Errorf("Low = %v, want 11", day.Low)
	}
	if day.Condition != Clear {
		t.Errorf("Condition = %v, want Clear (first source)", day.Condition)
	}
	if len(day.Hourly) != 2 {
		t.Fatalf("got %d hourly, want 2", len(day.Hourly))
	}
	if day.Hourly[0].Temperature != 11 {
		t.Errorf("Hourly[0].Temperature = %v, want 11", day.Hourly[0].Temperature)
	}
	if math.Abs(day.Hourly[1].PrecipitationProb-0.4) > 0.01 {
		t.Errorf("Hourly[1].PrecipitationProb = %v, want 0.4", day.Hourly[1].PrecipitationProb)
	}
}

func TestEnsembleSource_OneSourceFails(t *testing.T) {
	date := time.Date(2026, 4, 28, 0, 0, 0, 0, time.UTC)
	s1 := &stubSource{err: errors.New("timeout")}
	s2 := &stubSource{forecast: &Forecast{Days: []DailyForecast{
		{Date: date, High: 20, Low: 10},
	}}}

	ensemble := NewEnsembleSource(s1, s2)
	fc, err := ensemble.Forecast(context.Background(), Location{}, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fc.Days[0].High != 20 {
		t.Errorf("High = %v, want 20 (sole source)", fc.Days[0].High)
	}
}

func TestEnsembleSource_AllFail(t *testing.T) {
	s1 := &stubSource{err: errors.New("fail1")}
	s2 := &stubSource{err: errors.New("fail2")}

	ensemble := NewEnsembleSource(s1, s2)
	_, err := ensemble.Forecast(context.Background(), Location{}, 1)
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "ensemble: all 2 sources failed" {
		t.Errorf("error = %q", err.Error())
	}
}

func TestEnsembleSource_DifferentDayCounts(t *testing.T) {
	date1 := time.Date(2026, 4, 28, 0, 0, 0, 0, time.UTC)
	date2 := time.Date(2026, 4, 29, 0, 0, 0, 0, time.UTC)
	s1 := &stubSource{forecast: &Forecast{Days: []DailyForecast{
		{Date: date1, High: 20, Low: 10},
	}}}
	s2 := &stubSource{forecast: &Forecast{Days: []DailyForecast{
		{Date: date1, High: 22, Low: 12},
		{Date: date2, High: 18, Low: 8},
	}}}

	ensemble := NewEnsembleSource(s1, s2)
	fc, err := ensemble.Forecast(context.Background(), Location{}, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fc.Days) != 2 {
		t.Fatalf("got %d days, want 2", len(fc.Days))
	}
	if fc.Days[0].High != 21 {
		t.Errorf("Day[0].High = %v, want 21 (average)", fc.Days[0].High)
	}
	if fc.Days[1].High != 18 {
		t.Errorf("Day[1].High = %v, want 18 (sole source)", fc.Days[1].High)
	}
}

func TestEnsembleSource_PreservesLocation(t *testing.T) {
	s := &stubSource{forecast: &Forecast{Days: []DailyForecast{
		{Date: time.Now(), High: 20, Low: 10},
	}}}

	loc := Location{Latitude: 45.4, Longitude: -75.7}
	ensemble := NewEnsembleSource(s)
	fc, err := ensemble.Forecast(context.Background(), loc, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fc.Location != loc {
		t.Errorf("Location = %v, want %v", fc.Location, loc)
	}
}

func TestEnsembleSource_ThreeSources(t *testing.T) {
	date := time.Date(2026, 4, 28, 0, 0, 0, 0, time.UTC)
	mk := func(high, low float64) *stubSource {
		return &stubSource{forecast: &Forecast{Days: []DailyForecast{
			{Date: date, High: high, Low: low},
		}}}
	}

	ensemble := NewEnsembleSource(mk(18, 8), mk(20, 10), mk(22, 12))
	fc, err := ensemble.Forecast(context.Background(), Location{}, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fc.Days[0].High != 20 {
		t.Errorf("High = %v, want 20", fc.Days[0].High)
	}
	if fc.Days[0].Low != 10 {
		t.Errorf("Low = %v, want 10", fc.Days[0].Low)
	}
}

func TestAvgFloat_Empty(t *testing.T) {
	got := avgFloat(nil)
	if got != 0 {
		t.Errorf("avgFloat(nil) = %v, want 0", got)
	}
}

func TestEnsembleSource_HourlyOrdering(t *testing.T) {
	date := time.Date(2026, 4, 28, 0, 0, 0, 0, time.UTC)
	s := &stubSource{forecast: &Forecast{Days: []DailyForecast{
		{Date: date, High: 20, Low: 10, Hourly: []HourlyPoint{
			{Hour: 12, Temperature: 18, PrecipitationProb: 0.1},
			{Hour: 6, Temperature: 10, PrecipitationProb: 0.0},
		}},
	}}}

	ensemble := NewEnsembleSource(s)
	fc, err := ensemble.Forecast(context.Background(), Location{}, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fc.Days[0].Hourly[0].Hour != 6 {
		t.Errorf("Hourly[0].Hour = %d, want 6 (sorted)", fc.Days[0].Hourly[0].Hour)
	}
	if fc.Days[0].Hourly[1].Hour != 12 {
		t.Errorf("Hourly[1].Hour = %d, want 12 (sorted)", fc.Days[0].Hourly[1].Hour)
	}
}
