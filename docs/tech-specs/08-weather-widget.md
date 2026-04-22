# 08 — Weather Widget

## Context

Inkwell currently has a single built-in widget (clock). A weather widget is
the next planned addition (see execution plan). The sibling project
[sapcast](https://github.com/grantlucas/sapcast) demonstrates fetching
weather data from Open-Meteo (free, no API key) and Pirate Weather (requires
key). This spec covers adding a weather widget to Inkwell using **Open-Meteo**
as the sole data source.

## Goals

- Display current temperature, daily high/low, feels-like, humidity, wind
  speed, and weather condition description
- Use the free [Open-Meteo API](https://open-meteo.com/) (no API key)
- Cache weather data to avoid excessive API calls on every render tick
- Support Celsius and Fahrenheit via configuration
- Maintain 100% test coverage with no real HTTP calls in tests

## Data Source: Open-Meteo API

### Request

<!-- markdownlint-disable MD013 -->
```text
GET https://api.open-meteo.com/v1/forecast
  ?latitude={lat}
  &longitude={lon}
  &current=temperature_2m,apparent_temperature,weather_code,relative_humidity_2m,wind_speed_10m
  &daily=temperature_2m_max,temperature_2m_min
  &forecast_days=1
  &timezone=auto
  &temperature_unit=celsius|fahrenheit
```
<!-- markdownlint-enable MD013 -->

### Response (relevant fields)

```json
{
  "current": {
    "temperature_2m": 22.1,
    "apparent_temperature": 20.3,
    "weather_code": 2,
    "relative_humidity_2m": 65,
    "wind_speed_10m": 12.4
  },
  "current_units": {
    "temperature_2m": "°C",
    "wind_speed_10m": "km/h"
  },
  "daily": {
    "temperature_2m_max": [25.2],
    "temperature_2m_min": [17.0]
  }
}
```

### WMO Weather Codes

The `weather_code` field uses WMO code table 4677. Key mappings:

| Code | Description |
|------|-------------|
| 0 | Clear sky |
| 1 | Mainly clear |
| 2 | Partly cloudy |
| 3 | Overcast |
| 45, 48 | Fog |
| 51, 53, 55 | Drizzle |
| 61, 63, 65 | Rain |
| 71, 73, 75 | Snowfall |
| 77 | Snow grains |
| 80, 81, 82 | Rain showers |
| 85, 86 | Snow showers |
| 95 | Thunderstorm |
| 96, 99 | Thunderstorm with hail |

## Architecture

### New Packages

```text
internal/inkwell/
  weather/
    weather.go              # Conditions model
    wmo.go                  # WMO code -> description mapping
    openmeteo/
      client.go             # Open-Meteo API client
      client_test.go
  widgets/
    weather/
      weather.go            # Widget implementation + Factory
      weather_test.go
```

### Modified Files

- `internal/inkwell/widget/registry.go` — add `HTTPClient` to `Deps`
- `internal/inkwell/widgets/registry.go` — register `"weather"` type
- `internal/inkwell/app.go` — wire `http.DefaultClient` into `Deps`

## Design

### 1. HTTPClient Interface (Deps Extension)

```go
// widget/registry.go

// HTTPClient abstracts HTTP requests for testability.
type HTTPClient interface {
    Do(req *http.Request) (*http.Response, error)
}

type Deps struct {
    Now        func() time.Time
    HTTPClient HTTPClient
}
```

This enables any widget to make HTTP calls while allowing tests to inject
mock responses. The clock widget is unaffected — it ignores `HTTPClient`.

### 2. Weather Data Model

```go
// weather/weather.go

// Conditions holds the current weather state for a location.
type Conditions struct {
    CurrentTemp float64 // Current temperature
    FeelsLike   float64 // Apparent temperature
    High        float64 // Daily maximum
    Low         float64 // Daily minimum
    Humidity    int     // Relative humidity (%)
    WindSpeed   float64 // Wind speed (km/h or mph)
    Description string  // Human-readable condition ("Partly cloudy")
    WeatherCode int     // WMO weather code
    Units       string  // "celsius" or "fahrenheit"
}
```

### 3. Open-Meteo Client

```go
// weather/openmeteo/client.go

// Client fetches weather data from the Open-Meteo API.
type Client struct {
    httpClient widget.HTTPClient
    latitude   float64
    longitude  float64
    units      string // "celsius" or "fahrenheit"
}

func (c *Client) Fetch(ctx context.Context) (*weather.Conditions, error)
```

- Builds the request URL with configured lat/lon/units
- Parses JSON response into `weather.Conditions`
- Maps `weather_code` to description via `wmo.Description()`

### 4. WMO Code Mapping

```go
// weather/wmo.go

// Description returns a human-readable description for a WMO weather code.
func Description(code int) string
```

Simple map lookup with `"Unknown"` fallback.

### 5. Weather Widget

```go
// widgets/weather/weather.go

type Widget struct {
    bounds     image.Rectangle
    client     *openmeteo.Client
    cache      *weather.Conditions
    cachedAt   time.Time
    cacheTTL   time.Duration
    now        func() time.Time
}
```

#### Factory Config

```yaml
- type: weather
  bounds: [50, 50, 350, 230]
  config:
    latitude: 43.65
    longitude: -79.38
    units: fahrenheit   # default: celsius
    cache_ttl: 15m      # default: 15m
```

Required: `latitude`, `longitude`. Optional: `units`, `cache_ttl`.

#### Caching Strategy

On each `Render()` call:

1. If `cache` is nil or `now() - cachedAt > cacheTTL` → fetch fresh data
2. On fetch error with existing cache → use stale cache (graceful degradation)
3. On fetch error with no cache → render "Weather unavailable"

#### Render Layout (Option B — Prominent)

```text
┌────────────────────────────────────┐
│                                    │
│            22°C                    │  ← current temp (2x scaled)
│           Cloudy                   │  ← condition text
│                                    │
│     High: 25°    Low: 17°         │  ← hi/lo on one line
│     Feels like: 20°               │
│     Humidity: 65%   Wind: 12 kph  │
│                                    │
└────────────────────────────────────┘
```

The widget auto-layouts based on its bounds. The current temperature is
rendered at 2x pixel scale (each pixel drawn as a 2x2 block) for visual
hierarchy, using `basicfont.Face7x13` as the source face. All other text
renders at 1x.

#### Alternative Layouts

**Compact (~200x120)**:

```text
┌──────────────────────────┐
│      22°C  Cloudy        │
│    H: 25°  L: 17°       │
│  Feels 20° · 65% · 12kph│
└──────────────────────────┘
```

**Full-width banner (~780x50)**:

<!-- markdownlint-disable MD013 -->
```text
┌──────────────────────────────────────────────────────────────────────────┐
│  22°C  Cloudy     H: 25°  L: 17°     Feels: 20°  Humid: 65%  Wind: 12 │
└──────────────────────────────────────────────────────────────────────────┘
```
<!-- markdownlint-enable MD013 -->

**Dashboard card (~380x240)**:

```text
┌──────────────────────────────────────┐
│          WEATHER                     │
│  ────────────────────────────────    │
│                                      │
│           22°C                       │
│         Cloudy                       │
│                                      │
│  High       25°C                     │
│  Low        17°C                     │
│  Feels like 20°C                     │
│  Humidity   65%                      │
│  Wind       12 kph                   │
│                                      │
└──────────────────────────────────────┘
```

### Font Scaling Note

`basicfont.Face7x13` is the only font available without adding dependencies.
For the larger temperature display, the widget draws each font pixel as a 2x2
block. This provides visual hierarchy without new dependencies. A future
enhancement could add TTF font support.

## Testing Strategy

- **Open-Meteo client**: mock `HTTPClient` with canned JSON responses;
  test error handling, malformed JSON, missing fields
- **WMO mapping**: table-driven tests for all code ranges
- **Weather widget**: inject mock HTTP client + fixed `Now` function;
  verify cache TTL behavior, stale cache fallback, render output
- **Golden file tests**: capture rendered PNG for visual regression
- All packages must achieve 100% statement coverage

## Dependencies

Build order (each depends on the previous):

1. HTTPClient interface in `widget.Deps` (no external deps)
2. `weather/` package with model + WMO codes (no external deps)
3. `weather/openmeteo/` client (depends on 1 + 2)
4. `widgets/weather/` widget (depends on 1 + 2 + 3)
5. Registry + app wiring (depends on 4)
