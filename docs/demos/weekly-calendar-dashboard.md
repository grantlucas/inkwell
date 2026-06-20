# Weekly Calendar + Weather Dashboard

<!-- markdownlint-disable-next-line MD036 -->
*2026-04-29T01:01:38Z by Showboat 0.6.1*
<!-- showboat-id: d452bf6c-04db-403a-8f30-67ff227488e8 -->

The weekly calendar+weather dashboard composes three widgets into a single
800×480 e-ink screen: a date header, a right-aligned clock, and a 7-day
calendar with weather forecasts. Each widget is independently configured via
YAML bounds and rendered by the compositor in order.

## Screen Configuration

The dashboard is defined entirely in `inkwell.yaml` (copy
`inkwell.example.yaml` to get started). The date widget spans the full 800px
header band, the clock overlays the right edge, and the weekly-calendar fills
the remaining 428px body:

```bash
cat inkwell.example.yaml
```

```output
display: waveshare_7in5_v2
backend: preview
preview:
  port: 8080

dashboard:
  screens:
    - name: weekly
      widgets:
        - type: date
          bounds: [0, 0, 800, 52]
          refresh: "24h"
          config:
            format: "Monday, January 2"
        - type: clock
          bounds: [700, 0, 800, 52]
          refresh: "1m"
          config:
            format: "15:04"
            align: right
        - type: weekly-calendar
          bounds: [0, 52, 800, 480]
          refresh: "15m"
          config:
            feeds:
              - "https://example.com/my-calendar.ics"
            latitude: 40.7128
            longitude: -74.0060
            temp_unit: C
            show_weather: true
            show_weather_label: true
            max_events: 5
```

The compositor renders widgets in declaration order — the clock paints over
the date widget's right edge, both sharing the 52px header band. The
weekly-calendar widget fetches events from the iCal feed configured in
`feeds:` (the example uses a placeholder `example.com` URL — point this at any
iCal endpoint) and weather from Open-Meteo, using the single model named by
`weather_model` (default `gem`; `gfs`, `ecmwf`, or `gem`).

## Widget Registry

Three widget types power this screen. The registry wires type names to factory functions:

```bash
cat internal/inkwell/widgets/registry.go
```

```output
package widgets

import (
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/clock"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/date"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/weekly"
)

// NewDefaultRegistry creates a Registry pre-loaded with all built-in widgets.
func NewDefaultRegistry() *widget.Registry {
	r := widget.NewRegistry()
	r.Register("clock", clock.Factory)
	r.Register("date", date.Factory)
	r.Register("weekly-calendar", weekly.Factory)
	return r
}
```

## Tests and Coverage

Most packages maintain 100% statement coverage; the only exception is
`weatherview` at 99.5% (see footnote below). The weekly widget alone has 57
tests covering layout computation, day headers, event rendering, weather
integration, config parsing, and error paths:

```bash
go test ./... -count=1 2>&1
```

```output
?   	github.com/grantlucas/inkwell/cmd/inkwell	[no test files]
ok  	github.com/grantlucas/inkwell/internal/inkwell	0.797s
ok  	github.com/grantlucas/inkwell/internal/inkwell/calendar	0.611s
ok  	github.com/grantlucas/inkwell/internal/inkwell/calendar/ical	0.814s
ok  	github.com/grantlucas/inkwell/internal/inkwell/testutil	1.880s
ok  	github.com/grantlucas/inkwell/internal/inkwell/weather	1.655s
ok  	github.com/grantlucas/inkwell/internal/inkwell/widget	1.026s
ok  	github.com/grantlucas/inkwell/internal/inkwell/widgets	2.278s
ok  	github.com/grantlucas/inkwell/internal/inkwell/widgets/clock	1.455s
ok  	github.com/grantlucas/inkwell/internal/inkwell/widgets/date	2.482s
ok  	github.com/grantlucas/inkwell/internal/inkwell/widgets/weatherview	1.223s
ok  	github.com/grantlucas/inkwell/internal/inkwell/widgets/weekly	2.077s
```

```bash
go test ./internal/inkwell/... -coverprofile=/tmp/coverage.out -count=1 2>&1 && go tool cover -func=/tmp/coverage.out | grep total
```

```output
ok  	github.com/grantlucas/inkwell/internal/inkwell	1.501s	coverage: 100.0% of statements
ok  	github.com/grantlucas/inkwell/internal/inkwell/calendar	0.480s	coverage: 100.0% of statements
ok  	github.com/grantlucas/inkwell/internal/inkwell/calendar/ical	0.255s	coverage: 100.0% of statements
ok  	github.com/grantlucas/inkwell/internal/inkwell/testutil	0.903s	coverage: 100.0% of statements
ok  	github.com/grantlucas/inkwell/internal/inkwell/weather	1.514s	coverage: 100.0% of statements
ok  	github.com/grantlucas/inkwell/internal/inkwell/widget	0.676s	coverage: 100.0% of statements
ok  	github.com/grantlucas/inkwell/internal/inkwell/widgets	1.308s	coverage: 100.0% of statements
ok  	github.com/grantlucas/inkwell/internal/inkwell/widgets/clock	1.926s	coverage: 100.0% of statements
ok  	github.com/grantlucas/inkwell/internal/inkwell/widgets/date	1.722s	coverage: 100.0% of statements
ok  	github.com/grantlucas/inkwell/internal/inkwell/widgets/weatherview	2.117s	coverage: 99.5% of statements
ok  	github.com/grantlucas/inkwell/internal/inkwell/widgets/weekly	2.340s	coverage: 100.0% of statements
total:											(statements)		99.9%
```

The 99.5% on weatherview is a single unreachable error path in the opentype
font parsing stdlib — the font is embedded at compile time and always valid.

## Live Preview

Starting the preview server renders the composed dashboard at
<http://localhost:8080>. The server fetches live weather data from Open-Meteo
and calendar events from the Blue Jays iCal feed:

```bash
go run ./cmd/inkwell &>/dev/null & sleep 5 && curl -s -o /tmp/weekly-frame.png http://localhost:8080/frame.png && echo 'Preview server started, frame captured (800x480, 1-bit B/W)' && file /tmp/weekly-frame.png
```

```output
Preview server started, frame captured (800x480, 1-bit B/W)
/tmp/weekly-frame.png: PNG image data, 800 x 480, 1-bit colormap, non-interlaced
```
