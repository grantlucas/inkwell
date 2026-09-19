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
- **Weather:** when `show_weather` is true, the widget draws an Open-Meteo
  forecast from the **shared weather provider** — built once from the top-level
  [`weather:`](#weather-configuration) config and reused across every weather
  widget, so they deduplicate fetches through one three-hour cache. Location,
  model, and unit are inherited from that block; this widget can override any of
  them (see the config table). A weather failure is logged and the rest of each
  day cell still renders.

## Calendar feeds

Each entry in `feeds` must be a URL that returns a raw **iCalendar (ICS)**
stream — a `text/calendar` body that begins with `BEGIN:VCALENDAR`. It is *not*
a link to a calendar's web page. A common mistake is pasting a
`https://calendar.google.com/calendar/u/0?cid=…` "add to my calendar" link:
that URL returns the Google Calendar **web app (HTML)**, which the parser can't
read — it fails with a confusing `bufio.Scanner: token too long` (the HTML has
lines longer than the parser's buffer). Use the ICS feed URL instead.

### Google Calendar

1. Open **Google Calendar → Settings** (gear icon → *Settings*).
2. Under **Settings for my calendars**, click the calendar you want in the left
   sidebar.
3. Scroll to **Integrate calendar**. Copy one of the two iCal addresses (each
   ends in `/basic.ics`):
   - **Secret address in iCal format** — works for a **private** calendar
     without sharing it publicly. This is the right choice for most personal
     calendars. Treat it like a password: anyone with the URL can read the
     calendar, so keep it out of screenshots, commits, and shared configs.
   - **Public address in iCal format** — only works if the calendar is set to
     "Make available to public". Requesting the public ICS of a private
     calendar returns `404`.
4. Paste that `…/basic.ics` URL into `feeds`.

> **Rotating a leaked secret address.** If a secret iCal URL is exposed, open
> the same **Integrate calendar** section and use **Reset** next to the secret
> address. The old URL stops working immediately; update `feeds` with the new
> one.

### Other providers

Apple iCloud (shared-calendar "Public Calendar" link, `webcal://…` — change the
scheme to `https://`), Outlook/Office 365 ("Publish a calendar" → ICS link), and
most other calendar apps expose an equivalent ICS/iCal URL. As long as the URL
returns a `BEGIN:VCALENDAR` body over HTTP(S), it works here.

## Feed rules

Some feeds prepend the same boilerplate to every event. A league team calendar
typically leads each summary with the player and team — every event on the feed
starts `Jane Doe / Ravens / …` — which is identical everywhere and crowds the
part that actually differs (*Practice* vs *Game* vs the opponent) out of a
narrow day column.

A feed entry can therefore be an object instead of a bare URL, carrying `rules`
that clean up that feed's events as they are parsed:

```yaml
feeds:
  # A bare URL string still works, and applies no rules.
  - "https://example.com/my-calendar.ics"

  # The object form adds a label and rules.
  - url: "https://example.com/team-calendar.ics"
    name: "Hockey"             # optional; names the feed in config errors
    rules:
      # Drop the "Jane Doe / Ravens" heading from every summary.
      - match: '^Jane Doe\n(Ravens\n)?'
      # Shorten the opponent prefix.
      - match: 'vs '
        replace: 'v '
      # Hide fundraisers entirely.
      - match: 'Bottle Drive'
        exclude: true
```

Rules apply to the event **summary** only, and only to the feed they are
declared on.

<!-- markdownlint-disable MD013 -->
| Key       | Type   | Description                                                                 |
|-----------|--------|-----------------------------------------------------------------------------|
| `match`   | string | **Required.** A [Go RE2](https://pkg.go.dev/regexp/syntax) regular expression. |
| `replace` | string | Replacement for each match. Supports `$1` capture-group expansion. Defaults to `""`, which deletes the matched text. |
| `exclude` | bool   | When true, an event whose summary matches is dropped from the calendar. Cannot be combined with `replace`. |
<!-- markdownlint-enable MD013 -->

Notes:

- **Rules run in order, as a pipeline.** Each rule sees the summary as the
  rules before it left it, so an `exclude` can match on text an earlier
  `replace` produced.
- **`^` anchors the whole summary, not each line.** Summaries often contain
  real newlines (the parser unescapes `\n` per RFC 5545), so `^Practice` only
  matches a summary that *starts* with `Practice`. Use Go's `(?m)` flag —
  `(?m)^Practice$` — for per-line anchoring.
- **A rewritten summary is trimmed.** Stripping a leading line otherwise leaves
  its separator behind as blank space at the top of the event.
- **An invalid regex fails `LoadConfig`**, naming the feed (by `name` when it
  has one) and the rule index, rather than silently skipping the rule.
- Rules are applied at the feed boundary, before deduplication and caching, so
  everything downstream only ever sees cleaned-up events.

## Configuration

Top-level keys (`type`, `bounds`, `refresh`) are required by every widget.

> **Two different `refresh` keys.** The **top-level** `refresh` is the widget's
> *render cadence* — how often a frame change is allowed to push to the panel
> (a duration `>= 1m`, or `"static"`). The **nested** `config.refresh` is the
> *calendar data cache TTL* — how often the ICS feeds are re-fetched. They are
> independent; see [ADR 0011](../../../../docs/adrs/0011-require-a-per-widget-refresh-cadence.md).

The widget-specific keys live under `config:`.

<!-- markdownlint-disable MD013 -->
| Key                  | Type            | Default   | Description                                                                                          |
|----------------------|-----------------|-----------|------------------------------------------------------------------------------------------------------|
| `feeds`              | list            | —         | **Required**, non-empty. Each entry is an ICS feed URL string, or a feed object (`url`, optional `name`, optional `rules`) — see [Feed rules](#feed-rules). |
| `refresh`            | string          | `"15m"`   | Calendar data cache TTL. A Go duration `>= 1m`. (Distinct from the top-level render cadence.)         |
| `max_events`         | integer         | `5`       | Maximum events shown per day column. Must be positive.                                                |
| `show_location`      | bool            | `false`   | Show each event's location line when present.                                                        |
| `show_weather`       | bool            | `true`    | Render the per-day weather block. When false, weather is omitted and event space expands.            |
| `show_weather_label` | bool            | `true`    | Show the condition label (e.g. `CLOUDY`) above the temps in the weather block.                       |
| `latitude`           | number          | top-level `weather.latitude`  | Override the shared weather location latitude, in `[-90, 90]`. Only meaningful when `show_weather` is true.   |
| `longitude`          | number          | top-level `weather.longitude` | Override the shared weather location longitude, in `[-180, 180]`. Only meaningful when `show_weather` is true. |
| `temp_unit`          | string          | top-level `weather.temp_unit` | Override the displayed temperature unit: `"C"` or `"F"`.                                              |
| `weather_model`      | string          | top-level `weather.model`     | Override the Open-Meteo forecast model: `"gfs"`, `"ecmwf"`, or `"gem"`. See [Weather models](#weather-models). Only meaningful when `show_weather` is true. |
| `week_start`         | string          | `"monday"`| `"monday"` or `"sunday"`. **Validated but not yet applied** — the view always starts on the current day (see [inkwell-7gn](#known-gaps)). |
| `highlight_hour`     | integer         | `15`      | Hour `[0, 23]` to highlight in the hourly chart. **Validated but not yet applied** — the chart always highlights the current hour (see [inkwell-7gn](#known-gaps)). |
<!-- markdownlint-enable MD013 -->

Any value of the wrong type, an empty or missing `feeds`, or an out-of-range
number is a configuration error that fails `LoadConfig`.

## Timezone

A feed does not express every event the same way. Google emits most
events as UTC instants (`DTSTART:20260920T130000Z`) and others as wall
times qualified by a zone (`DTSTART;TZID=America/Toronto:20260919T104500`).
The parser preserves whichever form it found, so an `Event.Start` is
always a correct *instant* but carries the feed's zone, not the
viewer's. Rendering it directly would print two neighbouring events on
the same panel in two different zones.

The widget converts every event into one display zone before drawing it.
That zone is **not** configured here — it is the dashboard-wide
top-level [`timezone`](../../../docs/guides/configuration.md#timezone)
key, which also governs the `clock`, `date` and `fuzzy_clock` widgets so
the whole panel agrees:

```yaml
timezone: "America/Toronto"   # top level, not under the widget's config:
```

The widget receives it as the zone on its injected clock, and derives
the day columns, the event labels, and the weather chart's highlighted
hour from that one value.

One caveat the parser cannot fix: a feed using a non-IANA `TZID` (Outlook
emits names like `"Eastern Standard Time"`) falls back to UTC with a log
line, because `VTIMEZONE` blocks are not yet read.

## Weather configuration

Weather settings are configured **once** at the top level of the config, not
per widget:

```yaml
weather:
  latitude: 43.244
  longitude: -79.837
  model: gem        # gfs | ecmwf | gem
  temp_unit: C      # C or F
```

These become the defaults for every weather widget, served through a single
shared, cached provider — so multiple widgets at the same location fetch the
forecast only once. Each widget may override any field (`latitude`,
`longitude`, `temp_unit`, `weather_model`) in its own `config:` block, e.g. to
show a second city. Omitted top-level fields default to `model: gem`,
`temp_unit: C`, and location `0,0`.

## Weather models

`weather_model` selects which Open-Meteo numerical model the forecast comes
from. The models have complementary strengths, so the best choice depends on
where you are and what you care about most. There is no blending — one model
drives the temperatures, the hourly chart, and the condition icon, so they
stay mutually consistent.

<!-- markdownlint-disable MD013 -->
| Model     | Source                       | Best for                                                                                     |
|-----------|------------------------------|----------------------------------------------------------------------------------------------|
| `gem`     | Environment Canada (GEM/HRDPS) | **Canada (default).** Best short-range precipitation; matches Canadian sources like The Weather Network. High-resolution over North America. |
| `ecmwf`   | ECMWF (IFS)                  | **Temperature accuracy / outside North America.** The best global model overall, but tends to over-forecast near-term precipitation in spot-checks. |
| `gfs`     | NOAA (GFS)                   | **United States / fallback.** US-centric; the noisiest of the three for precipitation.       |
<!-- markdownlint-enable MD013 -->

Rule of thumb: in Canada, keep the `gem` default — it tracks Environment
Canada closely and avoids the inflated rain bars the other models produce. If
you care most about temperature or live outside North America, try `ecmwf`.

## Known gaps

`week_start` and `highlight_hour` are parsed and validated but currently have
no effect on rendering: `Render` always begins the week on today and always
highlights `now.Hour()`. Tracked in `inkwell-7gn`.

## Example

```yaml
# Top-level: shared weather defaults for all weather widgets.
weather:
  latitude: 43.6532
  longitude: -79.3832
  model: gem          # gfs | ecmwf | gem
  temp_unit: C

dashboard:
  screens:
    - name: weekly
      widgets:
        - type: weekly-calendar
          bounds: [0, 52, 800, 480]
          refresh: "15m"        # render cadence (top-level)
          config:
            feeds:
              # Use the ICS feed URL, not a "?cid=" web link — see "Calendar feeds".
              - "https://example.com/my-calendar.ics"
              # Object form: strip a league feed's per-event boilerplate.
              - url: "https://example.com/team-calendar.ics"
                name: "Hockey"
                rules:
                  - match: '^Jane Doe\n(Ravens\n)?'
            refresh: "15m"      # calendar data cache TTL (nested)
            show_weather: true
            show_weather_label: true
            show_location: true
            max_events: 4
            # location / model / temp_unit inherited from top-level weather:;
            # override here per widget if needed, e.g. weather_model: ecmwf
```
