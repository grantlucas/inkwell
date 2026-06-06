package weather

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// EnsembleSource combines forecasts from multiple sources by averaging
// their predictions. At least one source must succeed.
type EnsembleSource struct {
	sources []Source
}

// NewEnsembleSource creates a source that averages forecasts from the
// given sources. Sources are queried in parallel.
func NewEnsembleSource(sources ...Source) *EnsembleSource {
	return &EnsembleSource{sources: sources}
}

// Forecast queries all sources in parallel and averages the results.
func (e *EnsembleSource) Forecast(ctx context.Context, loc Location, days int) (*Forecast, error) {
	type result struct {
		forecast *Forecast
		err      error
	}

	results := make([]result, len(e.sources))
	var wg sync.WaitGroup
	for i, src := range e.sources {
		wg.Add(1)
		go func(idx int, s Source) {
			defer wg.Done()
			fc, err := s.Forecast(ctx, loc, days)
			results[idx] = result{fc, err}
		}(i, src)
	}
	wg.Wait()

	var forecasts []*Forecast
	for i, r := range results {
		if r.err != nil {
			// Log per-source failures so a degraded "1 of 3 succeeded"
			// result still surfaces which models dropped out, instead
			// of silently averaging fewer sources than expected.
			log.Printf("ensemble: source %d: %v", i, r.err)
			continue
		}
		if r.forecast != nil {
			forecasts = append(forecasts, r.forecast)
		}
	}

	if len(forecasts) == 0 {
		return nil, fmt.Errorf("ensemble: all %d sources failed", len(e.sources))
	}

	return average(forecasts, loc), nil
}

func average(forecasts []*Forecast, loc Location) *Forecast {
	maxDays := 0
	for _, fc := range forecasts {
		if len(fc.Days) > maxDays {
			maxDays = len(fc.Days)
		}
	}

	out := &Forecast{Location: loc}
	for d := range maxDays {
		var highs, lows []float64
		var cond Condition
		condSet := false
		hourlyByHour := make(map[int][]HourlyPoint)
		var date time.Time

		for _, fc := range forecasts {
			if d >= len(fc.Days) {
				continue
			}
			day := fc.Days[d]
			date = day.Date
			highs = append(highs, day.High)
			lows = append(lows, day.Low)
			if !condSet {
				cond = day.Condition
				condSet = true
			}
			for _, hp := range day.Hourly {
				hourlyByHour[hp.Hour] = append(hourlyByHour[hp.Hour], hp)
			}
		}

		// condSet is true here whenever d < maxDays: maxDays is the max
		// of len(fc.Days) across sources, so at least one source has a
		// Days[d] to populate condSet/date on the first hit. Skipping
		// when condSet is false would be dead code.

		df := DailyForecast{
			Date:      date,
			High:      avgFloat(highs),
			Low:       avgFloat(lows),
			Condition: cond,
		}

		for h := range 24 {
			points, ok := hourlyByHour[h]
			if !ok {
				continue
			}
			var temps, probs []float64
			for _, p := range points {
				temps = append(temps, p.Temperature)
				probs = append(probs, p.PrecipitationProb)
			}
			df.Hourly = append(df.Hourly, HourlyPoint{
				Hour:              h,
				Temperature:       avgFloat(temps),
				PrecipitationProb: avgFloat(probs),
			})
		}

		out.Days = append(out.Days, df)
	}

	return out
}

func avgFloat(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	var sum float64
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}
