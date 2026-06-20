# Weekly Calendar Widget

Renders a 7-day calendar-and-weather dashboard, one column per day starting
with **today**. Each column shows a day header, an optional weather block
(delegated to the [`weatherview`](../weatherview/README.md) component), and a
list of that day's events pulled from one or more iCalendar (ICS) feeds.
Registered under the dashboard `type: weekly-calendar`.

## Screenshot

Device view (`gray4`) — the canonical full-dashboard layout: a `date` and
`clock` header, a `separator`, and the weekly-calendar widget below with live
weather and events from a test calendar feed:

![Weekly calendar dashboard device preview](docs/device.png)

Per column, top to bottom: the day header (today is inverted), the weather
block (condition icon + label, hi/lo temps, an hourly temperature curve, and
precipitation bars with an hour axis), then the day's events. Columns are
divided by solid vertical rules.

## Data sources

- **Calendar:** the `feeds` URLs are fetched as ICS and cached. The cache TTL
  is the nested `config.refresh` value (see below). A fetch failure is logged
  and the dashboard renders with whatever events succeeded (or none) rather
  than blanking.
- **Weather:** when `show_weather` is true, the app injects an ensemble
  Open-Meteo source (GFS + ECMWF + GEM) for the configured latitude/longitude.
  A weather failure is logged and the rest of each day cell still renders.

## Configuration

Top-level keys (`type`, `bounds`, `refresh`) are required by every widget.

> **Two different `refresh` keys.** The **top-level** `refresh` is the widget's
> *render cadence* — how often a frame change is allowed to push to the panel
> (a duration `>= 1m`, or `"static"`). The **nested** `config.refresh` is the
> *calendar data cache TTL* — how often the ICS feeds are re-fetched. They are
> independent; see [`docs/tech-specs/08-refresh-strategy.md`](../../../../docs/tech-specs/08-refresh-strategy.md).

The widget-specific keys live under `config:`.

<!-- markdownlint-disable MD013 -->
| Key                  | Type            | Default   | Description                                                                                          |
|----------------------|-----------------|-----------|------------------------------------------------------------------------------------------------------|
| `feeds`              | list of strings | —         | **Required**, non-empty. ICS feed URLs to merge into the calendar.                                   |
| `refresh`            | string          | `"15m"`   | Calendar data cache TTL. A Go duration `>= 1m`. (Distinct from the top-level render cadence.)         |
| `max_events`         | integer         | `5`       | Maximum events shown per day column. Must be positive.                                                |
| `show_location`      | bool            | `false`   | Show each event's location line when present.                                                        |
| `show_weather`       | bool            | `true`    | Render the per-day weather block. When false, weather is omitted and event space expands.            |
| `show_weather_label` | bool            | `true`    | Show the condition label (e.g. `CLOUDY`) above the temps in the weather block.                       |
| `latitude`           | number          | `0`       | Weather location latitude, in `[-90, 90]`. Only meaningful when `show_weather` is true.              |
| `longitude`          | number          | `0`       | Weather location longitude, in `[-180, 180]`. Only meaningful when `show_weather` is true.           |
| `temp_unit`          | string          | `"C"`     | Temperature unit for displayed temps: `"C"` or `"F"`.                                                |
| `week_start`         | string          | `"monday"`| `"monday"` or `"sunday"`. **Validated but not yet applied** — the view always starts on the current day (see [inkwell-7gn](#known-gaps)). |
| `highlight_hour`     | integer         | `15`      | Hour `[0, 23]` to highlight in the hourly chart. **Validated but not yet applied** — the chart always highlights the current hour (see [inkwell-7gn](#known-gaps)). |
<!-- markdownlint-enable MD013 -->

Any value of the wrong type, an empty or missing `feeds`, or an out-of-range
number is a configuration error that fails `LoadConfig`.

## Known gaps

`week_start` and `highlight_hour` are parsed and validated but currently have
no effect on rendering: `Render` always begins the week on today and always
highlights `now.Hour()`. Tracked in `inkwell-7gn`.

## Example

```yaml
- type: weekly-calendar
  bounds: [0, 52, 800, 480]
  refresh: "15m"        # render cadence (top-level)
  config:
    feeds:
      - "https://example.com/my-calendar.ics"
    refresh: "15m"      # calendar data cache TTL (nested)
    latitude: 43.6532
    longitude: -79.3832
    temp_unit: C
    show_weather: true
    show_weather_label: true
    show_location: true
    max_events: 4
```
