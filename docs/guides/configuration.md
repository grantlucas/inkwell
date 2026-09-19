# Configuration Reference

Every Inkwell setting, what it accepts, and — the part a bare option list
leaves out — what it actually changes on the panel.

Inkwell reads a single YAML file. Pass its path as the only argument, or
let the binary look for `inkwell.yaml` in the working directory:

```bash
inkwell              # reads ./inkwell.yaml
inkwell /etc/inkwell/inkwell.yaml
```

Start from [`inkwell.example.yaml`](../../inkwell.example.yaml) in the
repository root — it is a working config with every option present and
commented.

**Configuration errors are fatal at startup, never silent.** An unknown
value, a bad type, a widget missing its `refresh`, or bounds that fall
off the display all stop the program with a message naming the offending
key. Nothing is skipped or defaulted past a mistake, so a config that
loads is a config that means what it says.

## Contents

- [The shortest useful config](#the-shortest-useful-config)
- [Top-level settings](#top-level-settings)
- [`preview` — the web preview server](#preview--the-web-preview-server)
- [`image` — the PNG backend](#image--the-png-backend)
- [`weather` — shared forecast defaults](#weather--shared-forecast-defaults)
- [`dashboard` — screens and rotation](#dashboard--screens-and-rotation)
- [Widget settings every widget shares](#widget-settings-every-widget-shares)
- [Widget reference](#widget-reference)
- [Worked examples](#worked-examples)
- [Troubleshooting config errors](#troubleshooting-config-errors)

## The shortest useful config

```yaml
backend: preview

dashboard:
  screens:
    - name: main
      widgets:
        - type: clock
          bounds: [0, 0, 800, 100]
          refresh: "1m"
```

Everything else has a default. Run that, open
<http://localhost:8080/>, and you have a clock.

## Top-level settings

<!-- markdownlint-disable MD013 -->
| Key | Type | Default | Accepted values |
|-----|------|---------|-----------------|
| `display` | string | `waveshare_7in5_v2` | `waveshare_7in5_v2` |
| `backend` | string | `preview` | `preview`, `image`, `spi` |
| `color_mode` | string | `gray4` | `gray4`, `bw` |
| `clear_on_shutdown` | bool | `true` | `true`, `false` |
| `timezone` | string | host system zone | Any IANA name (`America/Toronto`) |
<!-- markdownlint-enable MD013 -->

### `display`

The panel profile, which fixes the canvas size every widget's `bounds`
are validated against. Only `waveshare_7in5_v2` (800×480) ships today,
and an unrecognised name is rejected rather than guessed at.

### `backend`

Where frames go. This is the setting that decides whether you are
developing or deployed.

- **`preview`** serves the rendered panel at `http://localhost:<port>/`
  and pushes updates over SSE, so the browser reflects each render
  without a reload. Nothing touches hardware. This is the default and
  the right choice on a laptop.
- **`image`** writes each frame to a PNG file on disk. Useful for
  screenshots, golden-file comparisons, and CI.
- **`spi`** drives the real e-paper panel over SPI. Only meaningful on a
  Raspberry Pi wired to the display; see
  [installation.md](installation.md).

### `color_mode`

How the compositor's frame is packed down to what the panel can
physically show. **This is not a cosmetic preference** — it changes the
bit depth, the refresh behaviour, and the framebuffer size.

- **`gray4`** — 2 bits per pixel: white, light gray, dark gray, black.
  Gray fills survive as real grays, which is what keeps the precip bars
  and the anti-aliased weather icons looking intentional. The trade-off
  is a slower refresh, a larger framebuffer, and no fast waveform.
- **`bw`** — 1 bit per pixel via a `Y <= 128` threshold: any pixel at
  least half covered turns black, everything else white. Faster refresh
  and a smaller buffer, but every intermediate gray snaps to one
  extreme.

There is no dithering in either mode, so a soft gray does not
"partially" survive — it collapses. If you are adding visual elements,
read [hardware-grayscale.md](hardware-grayscale.md) before choosing
colours, and check your work in the device view rather than the source
view.

### `clear_on_shutdown`

On `SIGINT`/`SIGTERM`, whether to blank the panel to white before
sleeping the controller.

Leave it `true` for anything long-lived: e-paper holds its last image
with no power, and a frame left sitting for weeks can ghost
permanently. Set it `false` only when you want the last frame to stay
visible — debugging a render bug after the process exits is the usual
reason.

### `timezone`

The zone every widget renders in — clock and date text, the calendar's
day columns and event times, and the hour the weather chart highlights.

```yaml
timezone: "America/Toronto"
```

It defaults to the host's system zone, which is right on a workstation
and often wrong on an appliance. A Raspberry Pi imaged without a
timezone reports UTC, and the failure is quiet rather than obvious: the
clock widget shows UTC, and because calendar feeds serialize most events
as UTC instants (`DTSTART:20260920T130000Z`), those events render at
their raw UTC clock — hours late, and a whole day out near midnight.
Feeds are inconsistent about this, so some events on the same panel can
look correct while others do not.

Naming the zone here makes the panel independent of how the device was
provisioned. The value is any IANA zone name; an unknown one fails
`LoadConfig` at startup rather than falling back silently. The zone
database is compiled into the binary, so no system `tzdata` package is
required.

## `preview` — the web preview server

```yaml
preview:
  port: 8080
```

| Key | Type | Default | Impact |
|-----|------|---------|--------|
| `port` | integer | `8080` | Port the preview server listens on. |

Only used when `backend: preview`. The server exposes:

- `/` — the viewer page, which live-updates over SSE.
- `/frame.png` — the current frame as a PNG. Takes `?scale=2` to
  upscale, and `?source=1` to serve the pre-pack source frame.

**The default view is the post-pack device buffer** — what the panel
will really show, thresholded or quantised according to `color_mode`.
`?source=1` gives the full-fidelity frame the compositor drew, which is
useful for design review and misleading for sign-off. If a detail only
reads in the source view, it will not be on the panel.

## `image` — the PNG backend

```yaml
image:
  output_dir: output
```

| Key | Type | Default | Impact |
|-----|------|---------|--------|
| `output_dir` | string | `output` | Directory frames are written to. |

Only used when `backend: image`.

## `weather` — shared forecast defaults

Set the location, model, and unit **once**, at the top level. Every
weather-capable widget inherits them, and they share a single cached
provider — so two widgets at the same location cost one fetch, not two.

```yaml
weather:
  latitude: 43.2557
  longitude: -79.8711
  model: gem
  temp_unit: C
```

<!-- markdownlint-disable MD013 -->
| Key | Type | Default | Accepted values | Impact |
|-----|------|---------|-----------------|--------|
| `latitude` | number | `0` | `[-90, 90]` | Forecast location. Left at `0` you get the Gulf of Guinea, not an error — set it. |
| `longitude` | number | `0` | `[-180, 180]` | As above. |
| `model` | string | `gem` | `gfs`, `ecmwf`, `gem` | Which Open-Meteo forecast model to query. |
| `temp_unit` | string | `C` | `C`, `F` | Unit for every displayed temperature. |
<!-- markdownlint-enable MD013 -->

### Choosing a model

The three models are different national forecast systems, and they
disagree — most visibly about precipitation timing.

- **`gem`** — Environment Canada. The best match for Canadian locations,
  which is where its higher-resolution domain sits.
- **`ecmwf`** — the European model. Strong global skill, coarser
  resolution.
- **`gfs`** — the US model. Global coverage, updated frequently.

A partial `weather:` block is valid: omit `model` or `temp_unit` and the
defaults apply. Any individual widget can override any of these in its
own `config:`; see the [weekly-calendar](#weekly-calendar) table.

## `dashboard` — screens and rotation

```yaml
dashboard:
  rotate_interval: "30m"
  screens:
    - name: weekly
      widgets:
        # ...widgets for this screen
    - name: agenda
      widgets:
        # ...widgets for this screen
```

<!-- markdownlint-disable MD013 -->
| Key | Type | Default | Impact |
|-----|------|---------|--------|
| `rotate_interval` | duration | unset (no rotation) | How often the active screen advances to the next one. Must be non-negative. Omit it, or leave it `0`, to stay on the first screen forever. |
| `screens` | list | — | Named screens, each with its own widget layout. |
| `screens[].name` | string | — | Label for the screen. Appears in configuration error messages. |
| `screens[].widgets` | list | — | The widgets on that screen. |
<!-- markdownlint-enable MD013 -->

**Rotation interacts with the refresh gate**, and this catches people
out. Advancing the screen does not by itself push a frame: a change only
reaches the panel when some widget on the *newly active* screen is due
to refresh that minute. A screen of `static` widgets can therefore rotate
into place and not appear until something else forces a refresh. If you
rotate, give each screen at least one widget whose cadence divides the
rotation interval.

Setting `rotate_interval` with no `screens` configured is an error rather
than a no-op.

## Widget settings every widget shares

Every entry under `screens[].widgets` takes these four keys. The first
three are structural; `config` is where widget-specific options go.

```yaml
- type: clock
  bounds: [700, 0, 800, 50]
  refresh: "1m"
  config:
    format: "15:04"
```

### `type`

Which widget to build. Must name a registered widget — see the
[widget reference](#widget-reference). An unknown type fails at startup.

### `bounds`

`[x0, y0, x1, y1]` — the widget's rectangle in panel pixels, top-left
origin, `x1`/`y1` exclusive. `[0, 0, 800, 50]` is a full-width strip 50
pixels tall.

Two validations run before anything renders: the rectangle must not be
empty, and it must fit entirely inside the display. Overlaps between
widgets are *not* checked — widgets draw in config order, so a later
widget paints over an earlier one.

### `refresh` (required)

**Every widget must set this. There is no default and no global
fallback** — loading fails if any widget omits it. That is deliberate:
on e-paper, refreshing is a visible flash, so how often each widget may
cause one should be stated outright rather than inherited from something
far away in the file.

Accepted values:

- A duration of **at least one minute, in whole minutes** — `"1m"`,
  `"5m"`, `"24h"`. Seconds-resolution values like `"90s"` are rejected.
- **`"static"`** (or `"never"`) — the widget's content never changes, so
  it should never trigger a refresh. Correct for separators and fixed
  labels.

Cadences are **wall-clock aligned**, which is the point of the whole
mechanism: two `"5m"` widgets both come due at :00, :05, :10 and their
changes coalesce into one flash, instead of each flashing on its own
offset. Picking cadences that share factors is how you keep a dashboard
quiet.

A widget being due does not force a refresh — it *permits* one. If
nothing on screen actually changed, no frame is pushed. Conversely a
change that arrives while nothing is due is held until the next due
minute.

> **Two different `refresh` keys.** The **top-level** `refresh` above is
> the widget's *render cadence* — how often it may flash the panel. The
> **nested** `config.refresh` on `weekly-calendar` is its *data cache
> TTL* — how often the ICS feeds are re-fetched over the network. They
> are unrelated, and a feed fetched every 15 minutes on a widget allowed
> to refresh every hour will simply show data up to an hour stale.

Burn-in protection is *not* configurable: the periodic full-panel
clearing refresh is a hardware property, fixed internally. See
[ADR 0008](../adrs/0008-full-screen-fast-refresh-instead-of-windowed-partial.md).

## Widget reference

### `clock`

Current time, rendered large.

<!-- markdownlint-disable MD013 -->
| Key | Type | Default | Accepted values | Impact |
|-----|------|---------|-----------------|--------|
| `format` | string | `"15:04"` | Any Go time layout | `"15:04"` → `17:41`. `"3:04 PM"` → `5:41 PM`. Must not be empty. |
| `align` | string | `"center"` | `center`, `left`, `right` | Where the text sits in `bounds`. Left/right inset by 4px. |
<!-- markdownlint-enable MD013 -->

Pair with `refresh: "1m"` — a finer cadence is rejected, and a coarser
one means a visibly wrong clock.

### `date`

Current date.

<!-- markdownlint-disable MD013 -->
| Key | Type | Default | Accepted values | Impact |
|-----|------|---------|-----------------|--------|
| `format` | string | `"Monday, January 2"` | Any Go time layout | `"Mon Jan 2"` → `Fri Sep 18`. Must not be empty. |
<!-- markdownlint-enable MD013 -->

`refresh: "24h"` is right: the value only changes at midnight.

### `fuzzy_clock`

Approximate time in words — *"About half past eight"*, *"Nearly ten past
five"*. The low-flash alternative to `clock`: the phrase only changes
meaningfully every five minutes or so, so a `"5m"` cadence costs you
nothing and saves four flashes an hour.

<!-- markdownlint-disable MD013 -->
| Key | Type | Default | Accepted values | Impact |
|-----|------|---------|-----------------|--------|
| `style` | string | `"sentence"` | `sentence`, `title`, `lower` | `sentence` → `About half past eight`. `title` → `About Half Past Eight`. `lower` → `about half past eight`. |
| `use_words_for_noon_and_midnight` | bool | `true` | `true`, `false` | `true` → `noon` / `midnight`. `false` → `twelve`. |
| `use_24_hour` | bool | `false` | `true`, `false` | 24-hour phrasing instead of 12-hour. |
| `language` | string | `"en"` | `en` | Only English today; any other value is rejected rather than silently ignored. |
| `align` | string | `"center"` | `center`, `left`, `right` | Pins the phrase to an edge, so a corner placement keeps a fixed anchor as the phrase length changes. Left/right inset 4px. |
<!-- markdownlint-enable MD013 -->

### `separator`

A horizontal rule. Purely structural.

<!-- markdownlint-disable MD013 -->
| Key | Type | Default | Accepted values | Impact |
|-----|------|---------|-----------------|--------|
| `thickness` | integer | `2` | Any positive integer | Line thickness in pixels. Drawn solid black — a gray hairline would vanish under the BW threshold. |
<!-- markdownlint-enable MD013 -->

Always `refresh: "static"`. It never changes, so it should never be the
reason the panel flashes.

### `weekly-calendar`

A rolling calendar-and-weather dashboard of up to seven days, one column
per day starting with today — set `days` to show fewer and get wider
columns. The largest widget and the one with the most options; the
[widget README](../../internal/inkwell/widgets/weekly/README.md) covers
layout and feed setup in more depth.

<!-- markdownlint-disable MD013 -->
| Key | Type | Default | Accepted values | Impact |
|-----|------|---------|-----------------|--------|
| `feeds` | list | — | **Required**, non-empty | ICS feed URLs to merge. Each entry is a URL string, or an object with `url`, optional `name`, and optional `rules` — see [feed rules](#feed-rules). |
| `refresh` | duration | `"15m"` | `>= 1m` | **Calendar data cache TTL** — how often feeds are re-fetched. Not the render cadence. |
| `days` | integer | `7` | `[1, 7]` | Day columns to draw, counting from today. Fewer days means wider columns and more room for event text: across 800 px, `7` leaves 13 characters per line and `5` leaves 19. |
| `max_events` | integer | `5` | Positive | Cap on events shown per day column. Extra events are dropped, not scrolled. |
| `show_location` | bool | `false` | `true`, `false` | Draws each event's location on its own line below the title, when the event has one and the column has room. |
| `show_weather` | bool | `true` | `true`, `false` | Renders the per-day weather block. When false, the space is given back to events. |
| `show_weather_label` | bool | `true` | `true`, `false` | Shows the condition word (`CLOUDY`) above the temperatures. |
| `week_start` | string | `"monday"` | `monday`, `sunday` | **Validated but not yet applied** — the view always starts on today. |
| `highlight_hour` | integer | `15` | `[0, 23]` | **Validated but not yet applied** — the hourly chart always highlights the current hour. |
| `latitude` | number | inherits `weather.latitude` | `[-90, 90]` | Per-widget forecast location override. |
| `longitude` | number | inherits `weather.longitude` | `[-180, 180]` | Per-widget forecast location override. |
| `temp_unit` | string | inherits `weather.temp_unit` | `C`, `F` | Per-widget unit override. |
| `weather_model` | string | inherits `weather.model` | `gfs`, `ecmwf`, `gem` | Per-widget model override. |
<!-- markdownlint-enable MD013 -->

The four inheriting keys exist for the multi-location case — a second
weekly-calendar showing another city. If every widget wants the same
values, set them once under top-level [`weather`](#weather--shared-forecast-defaults)
and leave these out; overriding here defeats the shared cache.

#### Feed rules

A feed entry may be an object instead of a URL string, carrying rules
that rewrite or drop that feed's events as they are parsed. This exists
because some feeds prepend identical boilerplate to every event — a
league team calendar leading every summary with the player and team
name, for instance — which in a column roughly 17 characters wide
crowds out the part that differs.

```yaml
feeds:
  # A bare URL string applies no rules.
  - "https://example.com/personal.ics"

  - url: "https://example.com/team-calendar.ics"
    name: "Hockey"                          # names this feed in config errors
    rules:
      - match: '^Jane Doe\n(Ravens\n)?'     # no `replace` deletes the match
      - match: 'vs (.*)'
        replace: 'v $1'                     # $1 expands capture groups
      - match: 'Bottle Drive'
        exclude: true                       # drops the event entirely
```

<!-- markdownlint-disable MD013 -->
| Key | Type | Impact |
|-----|------|--------|
| `match` | string | **Required.** A [Go RE2](https://pkg.go.dev/regexp/syntax) regular expression, tested against the event summary. |
| `replace` | string | Replacement for each match, with `$1` capture-group expansion. Defaults to `""`, which deletes the matched text. |
| `exclude` | bool | Drops any event whose summary matches. Cannot be combined with `replace`. |
<!-- markdownlint-enable MD013 -->

Rules apply to the event **summary** only, and only to the feed they are
declared on. They run in order as a pipeline, so an `exclude` can match
text an earlier `replace` produced. `^` anchors the whole summary rather
than each line — summaries often contain real newlines, so use `(?m)`
for per-line anchoring. An invalid regex fails at startup, naming the
feed and rule index.

## Worked examples

### A quiet dashboard

Every cadence is a multiple of five minutes, so the whole panel flashes
once per interval instead of several times.

```yaml
color_mode: gray4
backend: spi

weather:
  latitude: 43.2557
  longitude: -79.8711
  model: gem

dashboard:
  screens:
    - name: weekly
      widgets:
        - type: date
          bounds: [0, 0, 500, 50]
          refresh: "24h"          # only changes at midnight
        - type: fuzzy_clock
          bounds: [500, 0, 800, 50]
          refresh: "5m"           # phrase only changes every ~5 min
          config:
            align: right
        - type: separator
          bounds: [0, 50, 800, 52]
          refresh: "static"       # never a reason to flash
        - type: weekly-calendar
          bounds: [0, 52, 800, 480]
          refresh: "15m"          # coalesces with the 5m clock at :00/:15/:30
          config:
            feeds:
              - "https://example.com/personal.ics"
            refresh: "15m"        # data cache TTL, matched to the cadence
            show_location: true
            max_events: 4
```

### Two rotating screens

```yaml
dashboard:
  rotate_interval: "30m"
  screens:
    - name: week
      widgets:
        - type: weekly-calendar
          bounds: [0, 0, 800, 480]
          refresh: "15m"          # due at :00/:15/:30/:45, so rotations land
          config:
            feeds: ["https://example.com/personal.ics"]
    - name: clock
      widgets:
        - type: fuzzy_clock
          bounds: [0, 180, 800, 300]
          refresh: "5m"
```

Both screens carry a widget whose cadence divides 30 minutes, so each
rotation coincides with a due minute and reaches the panel promptly.

### Rendering to PNG for a screenshot

```yaml
backend: image
image:
  output_dir: /tmp/inkwell-frames
```

## Troubleshooting config errors

<!-- markdownlint-disable MD013 -->
| Message | Cause |
|---------|-------|
| `widget "x": refresh is required` | A widget has no top-level `refresh`. There is no default; add a duration or `"static"`. |
| `widget "x": refresh must be a whole-minute duration >= 1m or "static"` | Sub-minute or fractional-minute cadence, such as `"30s"` or `"90s"`. |
| `screen "s": widget "x" bounds [...] exceed display [...]` | The rectangle falls off the panel. On a 7.5" V2 the canvas is 800×480. |
| `screen "s": widget "x" has empty bounds [...]` | `x1 <= x0` or `y1 <= y0` — usually width/height written where the second corner belongs. |
| `unknown display profile: "x"` | Only `waveshare_7in5_v2` exists. |
| `invalid backend: "x"` | Must be `preview`, `image`, or `spi`. |
| `invalid color_mode: "x"` | Must be `gray4` or `bw`. |
| `dashboard.rotate_interval is set but no screens are configured` | Rotation needs screens to rotate between. |
| `weekly-calendar: feeds is required` | The widget needs at least one feed. |
| `bufio.Scanner: token too long` on a feed | The URL returned HTML, not ICS — usually a `?cid=` "add to calendar" link instead of the `.ics` feed URL. |
<!-- markdownlint-enable MD013 -->

## See also

- [`inkwell.example.yaml`](../../inkwell.example.yaml) — a working config
  with every option commented.
- [Building dashboards](building-dashboards.md) — writing your own
  widget and planning a layout.
- [Hardware grayscale](hardware-grayscale.md) — what the panel can
  actually show, and why `color_mode` matters.
- [Installation](installation.md) — running on a Raspberry Pi with
  `backend: spi`.
- [ADR 0008](../adrs/0008-full-screen-fast-refresh-instead-of-windowed-partial.md)
  and [ADR 0011](../adrs/0011-require-a-per-widget-refresh-cadence.md) — the
  waveform and cadence machinery behind `refresh`.
- [weekly-calendar README](../../internal/inkwell/widgets/weekly/README.md)
  — feed setup and layout detail.
