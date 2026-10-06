# ADR 0015: Placed charts share a temperature range through the day data span

- **Status:** Accepted
- **Recorded:** 2026-10-06

## Context

Every day screen plots its combined charts on one shared temperature range,
so a cold day sits visibly lower than a warm one. Inside a full-screen widget
such as bold-five that is easy: the widget asks the day data module for its
five days once, and the module hands back the range across them with the days.

Lifting the combined chart out into a widget of its own (#128) breaks that.
A screen composed from five `combined-chart` widgets has five widgets that
know nothing about each other. Each is built, configured and rendered alone,
with its own bounds and refresh cadence, and nothing in the compositor passes
state between them.

The options considered:

- **A screen-level range.** The compositor, or a screen setting, computes the
  range and hands it to every chart. That gives widgets a dependency on their
  neighbours and on the screen, the one thing the widget model avoids, and it
  needs a new channel through the compositor that only charts would use.
- **A fixed range in config** (`min_temp`, `max_temp`). Simple, but it clips
  every day outside it and needs retuning each season.
- **Each chart on its own day's range.** Every line fills its chart, so the
  comparison between days that the shared range exists for is lost.
- **Ask the day data module for the same span.** Each chart names its day and
  the span of days its range covers, counted from today, and asks the module
  for that span. The module computes the range across the span, as it already
  does for the full-screen widgets.

## Decision

A `combined-chart` widget takes `day` (which day it draws, 0 for today) and
`range_days` (how many days from today its range spans, five by default). On
each render it asks its day data module for `range_days` days and plots its
own day on the range that comes back.

Charts with the same `range_days` and the same weather settings get the same
range. The forecast is fetched once per model and location for a fixed
horizon and cached for every widget ([`weather.Provider`](../../internal/inkwell/weather/provider.go)),
so each chart's module computes the range from the same forecast days. No
chart knows about another, and nothing is added to the compositor.

`TestComposedScreen_DrawsBoldFive` holds this to account: five day badges,
five combined charts and five event lists placed as bold-five places them,
built through the registry from config, draw bold-five's pixels.

## Consequences

Sharing a range is a matter of config: give every chart on a screen the same
`range_days`. Charts that differ in it, or in location or model, plot on
different ranges, which is right when they show different places and a
mistake otherwise. The docs say so beside the setting.

The range always runs from today. A chart can't share a range that leaves
today out, the way `weather-ahead` takes its range across its own rows only;
a screen that wants that uses `weather-ahead`.

Each chart computes the range itself, so a cache refresh that lands between
two charts' renders in the same cycle could, in principle, give them ranges
from different fetches. The forecast cache lives three hours and the charts
render milliseconds apart, so the window is small and closes on the next
cycle.
