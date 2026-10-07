# Weatherview Component

`weatherview` is a **reusable rendering component**, not a standalone
dashboard widget — there is no `type: weatherview`. It draws the weather
parts of a day: the combined precipitation-and-temperature chart, the
condition icon and the day's high and low. The calendar screens
(bold-five, today-hero, row-agenda) and the weather and day widgets
(`today-weather`, `weather-ahead`, `day-badge`, `combined-chart`, the
day-timeline's weather lane) all draw from it. It is
documented here so the rendering it produces (and the knobs it exposes)
are discoverable alongside the widgets.

It renders the condition glyph from the bundled **Weather Icons** font
(`weathericons.ttf`) and its hour labels in the panel's bitmap text face.

## What it draws

### Screenshots

One bold-five column (`gray4`, shown at 2x): the condition icon and the
high and low above the combined chart, with today's now marker at 10:40:

![One bold-five day column with its combined chart](docs/day-cell.png)

In context, one chart per day across the bold-five screen. The
precipitation bars land on `PaperGray70` so they read as dark gray on
`gray4` and solid black under the `bw` threshold:

![Weatherview charts across the bold-five screen](docs/in-context.png)

### Combined chart

`RenderCombinedChart` draws precipitation bars with the temperature line
over them, giving the bars the height they need to read at a distance.

- **Hours 06:00–21:00 inclusive**, so an evening shower lands on the chart
  rather than off the end of it.
- **Hour axis `6 12 18`** — 24-hour marks matching the `format: "15:04"` the
  rest of the panel is set in, rather than a 12-hour `6 9 12 3 8`, which
  mixes morning and afternoon on one axis.
- **The caller supplies only the rect.** The chart takes its own band for the
  hour labels at the bottom of the cell, sized from its own label face, so one
  renderer serves a 312 px hero cell and a 106 px row badge.
- **One range per screen, and it is required.** The caller hands every chart
  the same `TempRange` (°C), so a cold day sits lower than a warm one. The day
  data module (`daydata`) works it out across the days that have a forecast.
  The warmest value in the range touches the top of the plot and the coldest
  the row above the baseline; temperatures outside it clamp to the edge, and a
  collapsed range widens to one degree.
- **The line is black over paper and white over a bar.** Each pixel is chosen
  from what is already drawn underneath it — bar fill, bar cap or now marker
  give a white pixel, bare paper a black one — so the same rule reads in BW
  (solid black bar, white line) and Gray4 (dark-gray bar, white line) without
  asking which mode is active. The line is 2 px thick. `DrawContrastLine`
  exposes the rule for other drawing code that lays a line over its own fills.
- **A dry day is never blank.** Below a 15% peak across the window the bars
  are dropped, since a flat row of stubs reads as a broken widget, but the
  baseline, ticks and line are still drawn. Absent data (no hourly points in
  the window) draws nothing.

<!-- markdownlint-disable MD013 -->
| `CombinedOptions` field     | Description                                                       |
|-----------------------------|-------------------------------------------------------------------|
| `NowHour` / `ShowNowMarker` | Draws a 2 px solid `PaperBlack` stroke at the current hour.       |
<!-- markdownlint-enable MD013 -->

### Shared pieces

The screens draw their weather badges from the same parts, so a day reads the
same on every screen:

- `DrawIcon` draws the condition glyph. A glyph that will not draw is logged
  and the rest of the cell carries on, so callers have no error to handle.
- `NewHighLow(day, unit)` writes a day's high and low rounded in the
  configured unit: `High()` is `17°C`, `Low()` is `9°`, and `Pair()` is
  `17° 9°` for a cell too narrow to name the unit.

## Configuration

Weatherview has **no YAML config of its own**. The widgets that draw with
it pass the settings through: `temp_unit` picks the unit `NewHighLow`
writes in, and the shared `TempRange` comes from the day data module.

## Public API

- `RenderCombinedChart(frame, bounds, hourly, rng, combinedOpts)` — bars with
  the temperature line over them (see above).
- `DrawContrastLine(frame, points, run)` — a 2 px line, black over paper and
  white over anything drawn, thickened downward for a line that `RunsAcross`
  and to the right for one that `RunsDown`.
- `TempRange.X(temp, left, width)` — where a line running down a plot sits
  across it, coldest on the left. The day-timeline's weather lane uses it.
- `BarLength(prob, room)` and `Dry(points)` — the bar-length and dry-day
  rules, shared with the weather lane so its sideways bars read the same.
- `DrawIcon(frame, x, y, size, condition)` — just the condition glyph.
- `NewHighLow(day, unit)` — the day's high and low as text.
- `GlobalTempRange(days)` — the shared `TempRange` across days, for chart normalization.
