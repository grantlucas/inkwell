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
Forecasts are requested in this zone too, so a forecast for a location in
another zone still lines up its days and hours with the panel's Today.

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
own `config:`; see any weather widget's table, such as
[day-timeline](#day-timeline)'s.

## `dashboard` — screens and rotation

```yaml
dashboard:
  rotate_interval: "30m"
  screens:
    - name: day-timeline
      widgets:
        # ...widgets for this screen
    - name: row-agenda
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

**A rotation reaches the panel on the cycle it happens.** It opens the
refresh gate by itself, so the new screen is pushed straight away rather
than waiting for one of its widgets to come due, even a screen made only
of `static` widgets. From then on the new screen's widgets refresh on
their own cadences as usual.

The [example config](../../inkwell.example.yaml) rotates through four
screens every 15 minutes: the
[day-timeline screen](#the-day-timeline-screen), then `bold-five` under a
fuzzy clock band, `today-hero` and `row-agenda`.

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
> **nested** `config.refresh` on a calendar widget (`bold-five`,
> `today-hero`, `row-agenda`, `day-timeline`) is its *data cache TTL* —
> how often the ICS feeds are re-fetched over the network. They
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
| `scale` | integer | `1` | `>= 1` | Whole-number size multiplier. The longest phrase the settings can produce must fit the bounds at this scale, or loading fails. `2` across the full 800 px width is the large header-band clock bold-five uses; `3` does not fit. |
<!-- markdownlint-enable MD013 -->

### `separator`

A solid rule between widgets. Purely structural. A horizontal rule runs
along the bottom of its bounds, under the widget above it; a vertical
one runs down the right edge of its bounds, between widgets side by
side.

<!-- markdownlint-disable MD013 -->
| Key | Type | Default | Accepted values | Impact |
|-----|------|---------|-----------------|--------|
| `thickness` | integer | `2` | Any positive integer | Line thickness in pixels. Drawn solid black — a gray hairline would vanish under the BW threshold. |
| `orientation` | string | `"horizontal"` | `horizontal`, `vertical` | Which way the rule runs. A horizontal rule fills the bottom `thickness` rows of the bounds, a vertical one the right-most `thickness` columns. |
<!-- markdownlint-enable MD013 -->

Always `refresh: "static"`. It never changes, so it should never be the
reason the panel flashes.

### `bold-five`

Five days of calendar and weather as columns, one per day starting with
today, with every element sized to be read from across the room rather
than from arm's length: the day numeral has a 6.2 mm cap, readable to
about 1.06 m. Each column carries its own date, so it needs no separate
`date` widget.

Every column draws the combined chart: precipitation bars with the
temperature line over them, on one temperature scale shared by the five
days. A dry day still shows its line, so no column's chart is empty,
and a cold day sits visibly lower than a warm one. Event titles get
about 14 characters a line, the ceiling for a 160 px column, so it
makes text bigger without making it fit better.

It draws no clock of its own. The example config puts a
[`fuzzy_clock`](#fuzzy_clock) at `scale: 2` in a 46 px header band,
a 2 px `separator` under it, and bold-five in the remaining
`[0, 48, 800, 480]`. In that layout a column fits three events even
when every title wraps, so the example sets `max_events: 3`. A day
with more events than fit ends with a `+N MORE` line counting the
ones not shown; when the last event that fits would leave no room for
that line, it gives way to it, so hidden events are always counted.

There is no today highlight. Today is always the leftmost column, so an
inverted header would spend ink restating what position already says.

<!-- markdownlint-disable MD013 -->
| Key | Type | Default | Accepted values | Impact |
|-----|------|---------|-----------------|--------|
| `feeds` | list | — | **Required**, non-empty | ICS feed URLs to merge. Each entry is a URL string, or an object with `url`, optional `name`, and optional `rules` — see [feed rules](#feed-rules). |
| `refresh` | duration | `"15m"` | `>= 1m` | **Calendar data cache TTL** — how often feeds are re-fetched. Not the render cadence. |
| `max_events` | integer | `4` | Positive | Cap on events shown per column, set by the tall line height. Under a clock header band, `3` is what fits. Events past the cap or past the column's height are counted in a `+N MORE` line. |
| `show_location` | bool | `false` | `true`, `false` | Appends the event's location to its title, when it has one and the line has room. |
| `latitude` | number | inherits `weather.latitude` | `[-90, 90]` | Per-widget forecast location override. |
| `longitude` | number | inherits `weather.longitude` | `[-180, 180]` | Per-widget forecast location override. |
| `temp_unit` | string | inherits `weather.temp_unit` | `C`, `F` | Per-widget unit override. |
| `weather_model` | string | inherits `weather.model` | `gfs`, `ecmwf`, `gem` | Per-widget model override. |
<!-- markdownlint-enable MD013 -->

Calendar keys this screen has no use for — `days`, `week_start`,
`show_weather`, `show_weather_label`, `highlight_hour`, all carried over
from the deprecated `weekly-calendar` — are **rejected with an
explanation** rather than ignored. Silently dropping
`show_weather: false` would draw a weather band you had turned off,
which reads as a bug in the widget rather than a key that did not carry
over. Any other key the widget does not take, such as a misspelt
`max_event`, stops the dashboard loading with the list of keys it
accepts.

A day the forecast does not cover draws an empty weather band rather
than a zero. A clear sky at 0°C is a plausible reading, so drawing one
would leave you unable to tell an outage from the weather.

### `today-hero`

Today at reading distance down the left of the panel, the rest of the
week as one-line rows down the right. The bet is that from across the
room you only ever want today, and the week is a glance you take once
you have walked up to it — so the panel is spent unevenly rather than
evenly. Today gets 42% of the width; four more days share the rest.

Every chart on the screen is a combined chart: precipitation bars with
the temperature line drawn over them, black over paper and white where
it crosses a bar, so it reads in both `gray4` and `bw`. Today's is the
widest of any screen — 15 px bars with a marker at the current hour,
wide enough that an afternoon band reads as a band rather than a
texture. Each day row carries a smaller one under its date. All five
charts share one temperature range, so a cold day's line sits visibly
lower than a warm day's, and a dry day still shows its temperature
rather than an empty cell.

The date, month and fuzzy clock are plain text above a rule. There is
no filled block marking today: a black area that lands in the same place
on every refresh is a burn-in risk, and today is already the left
column.

Today's agenda shows only what is left of the day. An event that
finished two hours ago is history, and this is the screen that spends
real estate on today; "DONE FOR TODAY" appears once nothing remains.
When not every remaining event fits, under the cap or in the height, the
agenda's last line is `+N MORE`, counting every event it left out — the
same line every calendar screen ends a crowded day with. Event times
stay precise (16:15, not "quarter past four") — they are data, not a
clock. Only the clock in the identity block is fuzzy,
because a precise one would change every minute against a panel that
refreshes every fifteen.

<!-- markdownlint-disable MD013 -->
| Key | Type | Default | Accepted values | Impact |
|-----|------|---------|-----------------|--------|
| `feeds` | list | — | **Required**, non-empty | ICS feed URLs to merge. Each entry is a URL string, or an object with `url`, optional `name`, and optional `rules` — see [feed rules](#feed-rules). |
| `refresh` | duration | `"15m"` | `>= 1m` | **Calendar data cache TTL** — how often feeds are re-fetched. Not the render cadence. |
| `max_events` | integer | `3` | Positive | Cap on today's agenda. The scaled time and wrapped title make each event tall, so three is what fits before the "+N MORE" line. |
| `show_location` | bool | `false` | `true`, `false` | Appends the event's location to its title, when it has one and the line has room. |
| `latitude` | number | inherits `weather.latitude` | `[-90, 90]` | Per-widget forecast location override. |
| `longitude` | number | inherits `weather.longitude` | `[-180, 180]` | Per-widget forecast location override. |
| `temp_unit` | string | inherits `weather.temp_unit` | `C`, `F` | Per-widget unit override. |
| `weather_model` | string | inherits `weather.model` | `gfs`, `ecmwf`, `gem` | Per-widget model override. |
<!-- markdownlint-enable MD013 -->

As with `bold-five`, the calendar keys this screen has no use for —
`days`, `week_start`, `show_weather`, `show_weather_label`,
`highlight_hour` — are rejected with an explanation rather than
ignored. Any other key the widget does not take stops the dashboard
loading with the list of keys it accepts.

**What it gives up:** the day rows' charts are small. They sit under
the date rather than beside the agenda, because a 462 px row cannot
carry a legible chart *and* a legible title side by side — titles
dropped to about 12 characters when that was tried. At 164 px they
show the shape of the day's rain and temperature, not hour-by-hour
detail. If that detail matters for the whole week, it is an argument
for `bold-five`.

Tomorrow's row is *tagged* "TOMORROW" rather than promoted into the
hero column. An earlier draft promoted it and started the rows at +2,
which quietly dropped a day; the panel keeps its full five-day span and
nothing appears twice.

### `row-agenda`

The axis swap. Days become full-width rows with one event column each,
and for text that is the trade that matters, because width is what
titles were starving for. Titles get 35-odd characters instead of 13,
which makes this the only one of the three new screens where nothing
truncates on a realistic week. "Platform architecture review" fits. So
does "Car in for service", with room left over.

Rows are sized to their events. A quiet day's row is short and a busy
day's row is tall, and no row is shorter than the height that keeps its
chart readable (three lines of agenda). There are always five rows, and
room the rows do not need is shared between them. When the rows would
not fit on the panel, the busiest rows give up events first, one line
at a time, and the last visible line of such a row becomes "+N MORE"
with the count of what it could not show. Quiet days are never squeezed
to make room; on a tie the later day gives up the line, so today keeps
its detail longest. An empty day says "Nothing scheduled".

Every row carries the combined chart: precipitation bars with the
temperature line over them, so a dry day still shows the shape of its
temperature. All five charts share one temperature scale, so a cold day
sits visibly lower than a warm one, and today's chart marks the current
hour.

Today is the first row, and position is the only thing marking it. The
date is the same plain numeral, weekday and month on every row: a
filled block in the same place every refresh is a burn-in risk on
e-paper.

<!-- markdownlint-disable MD013 -->
| Key | Type | Default | Accepted values | Impact |
|-----|------|---------|-----------------|--------|
| `feeds` | list | — | **Required**, non-empty | ICS feed URLs to merge. Each entry is a URL string, or an object with `url`, optional `name`, and optional `rules` — see [feed rules](#feed-rules). |
| `refresh` | duration | `"15m"` | `>= 1m` | **Calendar data cache TTL** — how often feeds are re-fetched. Not the render cadence. |
| `show_location` | bool | `false` | `true`, `false` | Appends the event's location to its title, when it has one and the line has room. |
| `latitude` | number | inherits `weather.latitude` | `[-90, 90]` | Per-widget forecast location override. |
| `longitude` | number | inherits `weather.longitude` | `[-180, 180]` | Per-widget forecast location override. |
| `temp_unit` | string | inherits `weather.temp_unit` | `C`, `F` | Per-widget unit override. |
| `weather_model` | string | inherits `weather.model` | `gfs`, `ecmwf`, `gem` | Per-widget model override. |
<!-- markdownlint-enable MD013 -->

There is deliberately no `max_events`: rows grow to fit their events,
and when the week is too full the busiest rows give up lines first. A
separate cap could only contradict that. It is rejected with that
explanation, as are the other calendar keys `bold-five` rejects. Any
other key the widget does not take stops the dashboard loading with the
list of keys it accepts.

**What it gives up:** the panel holds about twenty event lines across
the five rows, so a genuinely packed week loses detail from its busiest
days. This screen is better at reading what is there than at showing
everything. Its charts are also the narrowest of the three screens that
carry them, 110 px against `bold-five`'s 160. Still a shape, but a
cramped one.

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

### `day-timeline`

Today only, on an hourly grid. Each event is a block from its real
start to its real end, so a long meeting looks long and a short one
looks short. Events that have finished are drawn as outlines and events
still to come as solid blocks, so the past and the rest of the day read
apart without reading a time. A solid block is a large black fill, but
it moves with the schedule, so it does not burn in the way a fixed fill
would.

Each block is labelled with its start time and title. A block with room
for more than one line wraps the title onto the lines under the time, at
word boundaries, and cuts only the last line that fits with "»"; the
block's height and its continuation arrows say where the event ends. A
block is never
shorter than one line of text: an event too short for that, or with no
end time, still starts at its real time but is drawn a line tall so its
label reads in full.

Two events whose blocks would overlap share the column side by side,
each still at its own start and end, so a partial clash shows as two
blocks of different heights. Overlap is judged on the blocks as drawn,
so a short event's line-tall block running into the next event puts
the two side by side too, and nothing is printed over anything else. A
third event at the same time is not drawn: a small "+N" tag at the
right of the column, level with it, counts every event left out of that
clash. One event carried by two feeds, with the same title, start and
end, is drawn once.

All-day events are listed in a strip above the grid, one per line, up
to two lines; when there are more, the second line says "+N MORE". A
timed event running through the whole of today, such as the middle day
of a conference, is listed there as all day. A timed event that starts
or ends today, even one crossing midnight, is a block on the grid like
any other. The strip takes height only when it has something to list.

The grid shows a window of whole hours, labelled down its left edge.
An event crossing an edge of the window is cut off at the edge, with an
arrowhead pointing the way it carries on. Events wholly outside the
window are not drawn; a "+N EARLIER" note above the grid or a
"+N LATER" note below it counts them, and each note takes height only
when there is something to count.

The grid's top edge is ruled only when the all-day strip or the
"+N EARLIER" note sits above it. Otherwise the grid starts at the
widget's top with no rule, so a separator placed above the widget, as
under the clock band on the day-timeline screen, is the only line there.

Between the hour labels and the events runs the weather lane: today's
forecast for the same hours, one row per hour of the window. The chance
of precipitation each hour is a bar growing to the right, and the
temperature line runs down through the rows, colder to the left and
warmer to the right across today's own range. The line is black over
paper and white where it crosses a bar, so it reads in both color
modes. A dry day draws the line with no bars, and with no forecast the
lane is left blank.

A now marker crosses the lane and the events at the current time, so
what is past and what is still to come read apart at a glance. It is
paper where it crosses a solid block, and passes behind a block's label
rather than striking it through, or behind a "+N" tag. Outside the
window it is not drawn.

The widget is meant for the left of a screen, and needs at least
238 × 160 px. The [day-timeline screen](#the-day-timeline-screen) in
the example config gives it 532 × 432 px under a clock band, beside
`today-weather` and `weather-ahead`.

<!-- markdownlint-disable MD013 -->
| Key | Type | Default | Accepted values | Impact |
|-----|------|---------|-----------------|--------|
| `feeds` | list | — | **Required**, non-empty | ICS feed URLs to merge. Each entry is a URL string, or an object with `url`, optional `name`, and optional `rules` — see [feed rules](#feed-rules). |
| `start_hour` | int | `7` | `0`–`24`, whole hours | First hour the grid shows. |
| `end_hour` | int | `22` | `0`–`24`, whole hours | Hour the grid ends at; `24` is midnight. Must be after `start_hour`, and the window at least six hours. |
| `refresh` | duration | `"15m"` | `>= 1m` | **Calendar data cache TTL** — how often feeds are re-fetched. Not the render cadence. |
| `show_location` | bool | `false` | `true`, `false` | Appends the event's location to its label, when it has one and the block has room. |
| `latitude` | number | inherits `weather.latitude` | `[-90, 90]` | Per-widget forecast location override. |
| `longitude` | number | inherits `weather.longitude` | `[-180, 180]` | Per-widget forecast location override. |
| `temp_unit` | string | inherits `weather.temp_unit` | `C`, `F` | Per-widget unit override. |
| `weather_model` | string | inherits `weather.model` | `gfs`, `ecmwf`, `gem` | Per-widget model override. |
<!-- markdownlint-enable MD013 -->

There is no `max_events`: every event in the window is placed at its
time, and the rest are counted. It is rejected with that explanation. A
window of `start_hour: 7.5`, an `end_hour` before `start_hour`, or a
window under six hours stops the dashboard loading with a message
saying which rule it broke. Suggested panel refresh: `"15m"`, like the
other calendar screens.

### `today-weather`

Today's forecast as a block you can place anywhere on a screen: the
condition icon, the high as the headline number, the low beside it and
the condition name beneath. It is today-hero's weather block at the same
sizes, so the two read alike. It draws no events and no chart, and has
its own refresh cadence, separate from the widgets around it.

It needs at least 238 × 108 px, the room for the widest high there is
("-12°C"). Below that it logs and draws nothing rather than spilling
onto its neighbours. When the high and low are both wide, the low takes
its own line under the high. A third of the panel's width, 266 px, is
comfortably enough.

When no forecast reaches today, because the fetch failed and nothing is
cached yet or the forecast stops short, the widget says `NO FORECAST`
instead of drawing a number nobody forecast. A failed fetch after a good
one keeps showing the last good forecast. Neither stops the rest of the
screen drawing.

<!-- markdownlint-disable MD013 -->
| Key | Type | Default | Accepted values | Impact |
|-----|------|---------|-----------------|--------|
| `latitude` | number | inherits `weather.latitude` | `[-90, 90]` | Per-widget forecast location override. |
| `longitude` | number | inherits `weather.longitude` | `[-180, 180]` | Per-widget forecast location override. |
| `temp_unit` | string | inherits `weather.temp_unit` | `C`, `F` | Per-widget unit override. |
| `weather_model` | string | inherits `weather.model` | `gfs`, `ecmwf`, `gem` | Per-widget model override. |
<!-- markdownlint-enable MD013 -->

All four are optional, so a widget with no `config:` at all shows the
top-level `weather:` location. The calendar settings — `feeds`,
`refresh`, `show_location`, `max_events` — are rejected with the reason,
since the widget reads no calendar, and so is `days`, which belongs to
`weather-ahead`.

```yaml
- type: today-weather
  bounds: [534, 48, 800, 168]
  refresh: "1h"    # the forecast changes slowly
```

On the [day-timeline screen](#the-day-timeline-screen) it sits at the
top of the right-hand column, above `weather-ahead`.

### `weather-ahead`

The days after today as rows of weather only, four by default. Each row
has the weekday and date in bold, the condition icon with the condition
name beside it, the high and low under the name, and a combined chart
filling the rest of the row. Every chart plots against one temperature
range taken across the widget's own rows, so a cold day sits visibly
lower than a warm one. Today is left to `today-weather`, which usually
sits above it; the two are separate so each can be placed and scheduled
on its own.

The rows split the widget's height evenly, with a rule between them.
Each row needs at least 222 × 62 px, so four days need 222 × 248 px and
a week needs 222 × 434 px. Below that it logs and draws nothing rather
than spilling onto its neighbours. The right-hand column of the
[day-timeline screen](#the-day-timeline-screen) under `today-weather`,
266 × 311 px, holds four days.

A day the forecast doesn't reach says `NO FORECAST` under its date
instead of drawing numbers nobody forecast. A failed fetch doesn't stop
the rest of the screen drawing.

<!-- markdownlint-disable MD013 -->
| Key | Type | Default | Accepted values | Impact |
|-----|------|---------|-----------------|--------|
| `days` | int | `4` | `[1, 7]` | How many days after today to show, one row each. |
| `latitude` | number | inherits `weather.latitude` | `[-90, 90]` | Per-widget forecast location override. |
| `longitude` | number | inherits `weather.longitude` | `[-180, 180]` | Per-widget forecast location override. |
| `temp_unit` | string | inherits `weather.temp_unit` | `C`, `F` | Per-widget unit override. |
| `weather_model` | string | inherits `weather.model` | `gfs`, `ecmwf`, `gem` | Per-widget model override. |
<!-- markdownlint-enable MD013 -->

All of them are optional. The calendar settings — `feeds`, `refresh`,
`show_location`, `max_events` — are rejected with the reason, since the
widget reads no calendar.

```yaml
- type: weather-ahead
  bounds: [534, 169, 800, 480]
  refresh: "1h"    # the forecast changes slowly
  config:
    days: 4
```

### The day widgets: `day-badge`, `combined-chart`, `event-list`

bold-five, today-hero and row-agenda each draw a day the same way: a
day badge (the date and the day's weather in brief), a combined chart,
and an event list. Those three are also widgets of their own, so a
screen can be composed from them in any layout, with each piece's own
bounds and refresh cadence. They draw exactly what the full-screen
widgets draw, at the same sizes: a screen composed as bold-five places
them draws bold-five. See
[composing a screen from the day widgets](#composing-a-screen-from-the-day-widgets).

Each one is placed on a single day, named by `day`: `0` is today, `1`
tomorrow, up to `6`, a week out. A day the forecast doesn't reach still
has its badge's date and its events, but no weather is drawn for it.

#### `day-badge`

The day's date and its weather in brief, in one of the shapes the
full-screen widgets draw it in. The badge is drawn at the top left of
its bounds, which must be at least the style's size; smaller bounds
draw nothing rather than cut the text off.

<!-- markdownlint-disable MD013 -->
| `style` | Size | What it draws | Taken from |
|---------|------|---------------|------------|
| `column` | 160 × 156 | The weekday over the date numeral, centred, then the condition icon on the left and the high at 2x over the low on the right. | bold-five's columns |
| `row` | 234 × 76 | The date numeral at 3x with the weekday and month beside it, then the condition icon and the high over the low. | row-agenda's rows |
| `compact` | 176 × 64 | The weekday (or `TOMORROW`) over the date numeral at 2x, with the icon and the high and low on one line beside it. | today-hero's day rows |
| `hero` | 338 × 202 | The weekday and date at 3x, the month and, on today, the fuzzy clock, above a rule; then a large icon, the high at 3x, the low and the condition's name. | today-hero's today |
<!-- markdownlint-enable MD013 -->

The `column` and `compact` styles align the high and low to the right
edge of the bounds, and `column` centres the date, so they widen with
their bounds. The others draw at fixed offsets from the top left.

<!-- markdownlint-disable MD013 -->
| Key | Type | Default | Accepted values | Impact |
|-----|------|---------|-----------------|--------|
| `style` | string | `column` | `column`, `row`, `compact`, `hero` | The shape the badge is drawn in. |
| `day` | int | `0` | `[0, 6]` | Which day, counted from today. |
| `latitude` | number | inherits `weather.latitude` | `[-90, 90]` | Per-widget forecast location override. |
| `longitude` | number | inherits `weather.longitude` | `[-180, 180]` | Per-widget forecast location override. |
| `temp_unit` | string | inherits `weather.temp_unit` | `C`, `F` | Per-widget unit override. |
| `weather_model` | string | inherits `weather.model` | `gfs`, `ecmwf`, `gem` | Per-widget model override. |
<!-- markdownlint-enable MD013 -->

It reads no calendar, so the calendar settings (`feeds`, `refresh`,
`show_location`, `max_events`) are rejected with the reason. The
`hero` style's fuzzy clock moves every five minutes, so give a hero
badge on today a `refresh` of `"5m"` if the clock should keep up;
every other badge changes only with the forecast and the date, so
`"1h"` is plenty.

#### `combined-chart`

One day's combined chart filling its bounds: the chance of
precipitation as bars across 06:00 to 21:00, with the temperature line
over them, black over paper and white over a bar. Today's chart has a
now marker at the current hour. A dry day draws the line with no bars,
and a day the forecast doesn't reach draws nothing.

The line is plotted on a temperature range shared across `range_days`
days from today. Charts don't know about each other, so each asks for
its span of days and gets the range across them from the forecast every
widget shares: **give every chart on a screen the same `range_days`**
and they plot on one scale, so a cold day sits visibly lower than a
warm one. Charts with different spans, or different weather settings,
plot on different ranges.

The chart labels its hours along the bottom and sizes itself to its
bounds; below about 10 px either way it draws nothing. bold-five's
charts are 144 × 40 px, row-agenda's 106 × 68 and today-hero's
largest 312 × 68.

<!-- markdownlint-disable MD013 -->
| Key | Type | Default | Accepted values | Impact |
|-----|------|---------|-----------------|--------|
| `day` | int | `0` | `[0, 6]` | Which day's chart, counted from today. |
| `range_days` | int | `5` | `[1, 7]`, and more than `day` | How many days from today the temperature range spans. Use the same value on every chart of a screen. |
| `latitude` | number | inherits `weather.latitude` | `[-90, 90]` | Per-widget forecast location override. |
| `longitude` | number | inherits `weather.longitude` | `[-180, 180]` | Per-widget forecast location override. |
| `temp_unit` | string | inherits `weather.temp_unit` | `C`, `F` | Accepted for symmetry; the chart draws no temperatures as text. |
| `weather_model` | string | inherits `weather.model` | `gfs`, `ecmwf`, `gem` | Per-widget model override. |
<!-- markdownlint-enable MD013 -->

A `range_days` that doesn't reach the chart's own day, such as `day: 5`
with the default of five, stops the config loading with the value it
needs. The calendar settings are rejected, as on `day-badge`.

#### `event-list`

One day's events listed into its bounds, in one of the shapes the
full-screen widgets list in. The list fits whole events only: one that
doesn't fit is left off, and the last line says "+N MORE", counting
every event not shown. A list too small for even that line draws
nothing, so there is no minimum size.

<!-- markdownlint-disable MD013 -->
| `style` | What it draws | Empty day | `max_events` default | Taken from |
|---------|---------------|-----------|----------------------|------------|
| `stacked` | The time in bold over a title of up to two lines, 8 px between events. | `--`, centred | `4` | bold-five's columns |
| `large` | The time at 2x over the title, with a rule between events. | `NOTHING SCHEDULED` | `3` | today-hero's agenda |
| `inline` | The time and the title on one line. | `NOTHING SCHEDULED` | `3` | row-agenda's and today-hero's rows |
<!-- markdownlint-enable MD013 -->

<!-- markdownlint-disable MD013 -->
| Key | Type | Default | Accepted values | Impact |
|-----|------|---------|-----------------|--------|
| `feeds` | list | — | **Required**, non-empty | ICS feed URLs to merge. Each entry is a URL string, or an object with `url`, optional `name`, and optional `rules` — see [feed rules](#feed-rules). |
| `style` | string | `stacked` | `stacked`, `large`, `inline` | The shape the events are listed in. |
| `day` | int | `0` | `[0, 6]` | Which day's events, counted from today. |
| `max_events` | int | the style's | Positive | Most events listed before "+N MORE". The room may hold fewer. |
| `hide_finished` | bool | `false` | `true`, `false` | Leaves out events that have already ended, as today-hero's agenda does. An event still running stays. |
| `empty` | string | the style's | Any text; `""` says nothing | What a day with no events says. |
| `show_location` | bool | `false` | `true`, `false` | Appends " @ " and the event's location to its title. |
| `refresh` | duration | `"15m"` | `>= 1m` | **Calendar data cache TTL** — how often feeds are re-fetched. Not the render cadence. |
<!-- markdownlint-enable MD013 -->

It reads no forecast, so the weather settings (`latitude`, `longitude`,
`temp_unit`, `weather_model`) are rejected with the reason.
today-hero's agenda is `style: large`, `hide_finished: true` and
`empty: "DONE FOR TODAY"`, though that message is drawn at 2x there and
at body size here.

### `weekly-calendar`

> **Deprecated, pending removal.** weekly-calendar is no longer in the
> example config and will be deleted in a later release. Its filled
> today header lands in the same place on every refresh, which is the
> burn-in risk the newer screens avoid. Move to one of the screens that
> replaced it: [`day-timeline`](#the-day-timeline-screen) for today hour
> by hour, [`bold-five`](#bold-five) for the same five-column shape
> sized for across the room, [`today-hero`](#today-hero) or
> [`row-agenda`](#row-agenda). They take the same `feeds`, `refresh`,
> `show_location` and weather keys. A key below that the new screen has
> no equivalent for stops the config loading, with the reason where
> there is one, rather than being silently dropped.

A rolling calendar-and-weather view of up to seven days, one column
per day starting with today — set `days` to show fewer and get wider
columns.

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
calendar widget showing another city. If every widget wants the same
values, set them once under top-level [`weather`](#weather--shared-forecast-defaults)
and leave these out; overriding here defeats the shared cache.

Any key not in this table, such as a misspelt `max_event`, stops the
dashboard loading with the list of keys the widget accepts.

## Worked examples

### The day-timeline screen

Today hour by hour, composed from widgets rather than drawn as one. It
is the first screen in [`inkwell.example.yaml`](../../inkwell.example.yaml),
so it is the one shown at startup, and the one that stays if you remove
`rotate_interval`.

```text
+----------------------------------------------------+  0
|               fuzzy_clock, scale 2                 |
+================================+===================+  46
|                                #   today-weather   |
|          day-timeline          #-------------------+  168
|  hours | weather lane | events #   weather-ahead   |
|                                #  (next four days) |
+--------------------------------+-------------------+  480
0                               532                 800
```

<!-- markdownlint-disable MD013 -->
| Widget | Bounds | `refresh` | Notes |
|--------|--------|-----------|-------|
| [`fuzzy_clock`](#fuzzy_clock) | `[0, 0, 800, 46]` | `"5m"` | `scale: 2`, the largest whose longest phrase fits 800 px. |
| [`separator`](#separator) | `[0, 46, 800, 48]` | `"static"` | 2 px under the clock band. |
| [`day-timeline`](#day-timeline) | `[0, 48, 532, 480]` | `"15m"` | The agenda and the weather lane, 07:00–22:00 by default. With no all-day strip or earlier note showing it draws no rule along its top, so the separator above is the only one. |
| [`separator`](#separator) | `[532, 48, 534, 480]` | `"static"` | `orientation: vertical`, as heavy as the rule under the clock. |
| [`today-weather`](#today-weather) | `[534, 48, 800, 168]` | `"1h"` | Hourly also turns the day over at midnight. |
| [`separator`](#separator) | `[534, 168, 800, 169]` | `"static"` | 1 px, matching the rules between weather-ahead's rows; weather-ahead draws none above its first. |
| [`weather-ahead`](#weather-ahead) | `[534, 169, 800, 480]` | `"1h"` | `days: 4`, about 77 px a row. |
<!-- markdownlint-enable MD013 -->

The bounds tile the panel exactly: every pixel belongs to exactly one
widget. Each widget keeps its own cadence, so the clock
refreshes the panel every five minutes without the calendar or the
forecast asking for more. Cadences are wall-clock aligned, so the 15m
agenda and the 1h weather land on minutes the 5m clock is already
refreshing.

A widget whose data does not arrive draws what it can and leaves the
others alone. With the calendar feed down, the grid is drawn with no
events while the clock and the weather draw as usual. With the forecast
down, the weather lane is blank and both weather widgets say
`NO FORECAST`, while the agenda and the clock draw as usual. The screen
is tested in both cases, along with a golden of the whole screen, by
loading this entry from the example config.

### Composing a screen from the day widgets

The [day widgets](#the-day-widgets-day-badge-combined-chart-event-list)
let a screen lay days out however it likes. The example config ends
with one, commented out: today and tomorrow side by side, each half the
panel, under the clock band. Uncomment it to add it to the rotation.

```text
+---------------------------------------------------+  0
|               fuzzy_clock, scale 2                |
+=========================+=========================+  46
|     day-badge, today    |   day-badge, tomorrow   |
|     combined-chart      |     combined-chart      |  204
|-------------------------|-------------------------|  260
|       event-list        |       event-list        |
+-------------------------+-------------------------+  480
0                        400                       800
```

<!-- markdownlint-disable MD013 -->
| Widget | Bounds | `refresh` | Notes |
|--------|--------|-----------|-------|
| [`fuzzy_clock`](#fuzzy_clock) | `[0, 0, 800, 46]` | `"5m"` | `scale: 2`. |
| [`separator`](#separator) | `[0, 46, 800, 48]` | `"static"` | 2 px under the clock band. |
| [`day-badge`](#day-badge) | `[0, 48, 400, 204]` | `"1h"` | `style: column`, `day: 0`. |
| [`combined-chart`](#combined-chart) | `[8, 204, 392, 260]` | `"1h"` | `day: 0`, `range_days: 2`. |
| [`separator`](#separator) | `[0, 260, 400, 261]` | `"static"` | `thickness: 1`. |
| [`event-list`](#event-list) | `[8, 270, 392, 480]` | `"15m"` | `style: stacked`, `day: 0`. |
| [`separator`](#separator) | `[400, 48, 401, 480]` | `"static"` | `orientation: vertical`, `thickness: 1`. |
| the same four | from `x` 401 | | `day: 1`, and the same `range_days: 2`. |
<!-- markdownlint-enable MD013 -->

Two things make the pieces read as one screen:

- **The charts share a range** because each asks for the same span of
  days, `range_days: 2`, from the one forecast every widget shares. A
  chart with a different span would plot its line on a different scale,
  so tomorrow's line would no longer sit lower than today's when it is
  colder. (Why the range works this way:
  [ADR 0015](../adrs/0015-placed-charts-share-a-range-through-the-day-data-span.md).)
- **Each widget keeps its own cadence.** The badges and charts change
  with the forecast, so `"1h"` is enough; the event lists follow the
  calendar at `"15m"`; the clock moves every five minutes. Each draws
  only its own bounds, so the space between them stays paper.

The same pieces can rebuild the full-screen widgets: five `column`
badges at 160 px, five charts inset 8 px under them with
`range_days: 5`, a rule, and five `stacked` lists draw bold-five
exactly, but for the rays of the partly-cloudy icon, which a placed
badge clips at its edge.

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
    - name: bold-five
      widgets:
        - type: fuzzy_clock
          bounds: [0, 0, 800, 46]
          refresh: "5m"           # phrase only changes every ~5 min
          config:
            scale: 2
        - type: separator
          bounds: [0, 46, 800, 48]
          refresh: "static"       # never a reason to flash
        - type: bold-five
          bounds: [0, 48, 800, 480]
          refresh: "15m"          # coalesces with the 5m clock at :00/:15/:30
          config:
            feeds:
              - "https://example.com/personal.ics"
            refresh: "15m"        # data cache TTL, matched to the cadence
            show_location: true
            max_events: 3
```

### Two rotating screens

```yaml
dashboard:
  rotate_interval: "30m"
  screens:
    - name: week
      widgets:
        - type: row-agenda
          bounds: [0, 0, 800, 480]
          refresh: "15m"
          config:
            feeds: ["https://example.com/personal.ics"]
    - name: clock
      widgets:
        - type: fuzzy_clock
          bounds: [0, 180, 800, 300]
          refresh: "5m"
          config:
            scale: 2
```

Each rotation reaches the panel when it happens; between rotations,
each screen's widgets refresh on their own cadences.

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
| `bold-five: feeds is required` | A calendar widget needs at least one feed. |
| `today-hero: unsupported setting "x" (accepted: ...)` | A calendar widget was given a key it does not take, often a typo. The message lists the keys it does. |
| `row-agenda: max_events is not supported: ...` | A key another calendar widget takes but this one has no use for. The message says why. |
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
