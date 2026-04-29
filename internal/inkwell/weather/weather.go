// Package weather provides weather forecast data from multiple sources
// using an ensemble approach for improved accuracy.
package weather

import (
	"context"
	"time"
)

// Condition represents a weather condition type.
type Condition int

const (
	Clear Condition = iota
	PartlyCloudy
	Cloudy
	Rain
	Snow
	Thunderstorm
	Fog
	Drizzle
)

// HourlyPoint holds weather data for a single hour.
type HourlyPoint struct {
	Hour              int
	Temperature       float64
	PrecipitationProb float64
}

// DailyForecast holds a single day's weather forecast.
type DailyForecast struct {
	Date      time.Time
	High      float64
	Low       float64
	Condition Condition
	Hourly    []HourlyPoint
}

// Forecast holds a multi-day weather forecast for a location.
type Forecast struct {
	Location Location
	Days     []DailyForecast
}

// Location identifies a geographic position for weather queries.
type Location struct {
	Latitude  float64
	Longitude float64
}

// Source provides weather forecast data.
type Source interface {
	Forecast(ctx context.Context, loc Location, days int) (*Forecast, error)
}
