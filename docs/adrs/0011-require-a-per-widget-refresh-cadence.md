# ADR 0011: Require a per-widget `refresh:` and gate pushes on a wall-clock queue

- **Status:** Accepted
- **Recorded:** 2026-09-18

## Context

The waveform cadence in
[ADR 0008](0008-full-screen-fast-refresh-instead-of-windowed-partial.md) decides
which waveform a push uses. A separate question is whether a content change
should push at all right now.

A dashboard's widgets change on uncorrelated cadences: the clock every minute,
the calendar every 15 minutes, weather every few hours. Because the loop pushed
the moment the composited frame differed, two widgets on the same period but
offset in phase produced two refreshes per period instead of one, which on gray4
means two flickers.

The alternative to config was a `RefreshEvery()`-style interface on the widget,
which puts the cadence in the widget's code where a dashboard author can't see or
change it.

## Decision

Every widget must set a top-level `refresh:` in config. There's no default, no
global setting, and no code-level fallback: `LoadConfig` errors if a widget omits
it or sets it below one minute. The value is a duration of at least a minute
(`"5m"`, `"24h"`), or the literal `"static"` (alias `"never"`) for a widget that
never changes, `separator` being the canonical case.

`refreshSchedule` ([`refresh_queue.go`](../../internal/inkwell/refresh_queue.go))
holds each screen's cadences and answers `anyDue(now)`: a widget with cadence *N*
minutes is due when the minute-of-day divides by *N*. Aligning to wall-clock
minute-of-day rather than to an arbitrary start time is what makes widgets sharing
a cadence fall due together, so two `5m` widgets both fire on :00/:05/:10 and
coalesce into one push. Static widgets contribute a cadence of `0` and never open
the gate.

The render loop then feeds `due && changed` to the planner instead of bare
`changed`.

## Consequences

A change landing on an undue minute is held, not dropped. The loop skips and
leaves the last-pushed buffer alone, so the change ships on the next due minute
alongside anything else due then.

The planner's periodic full or grayscale refresh still fires regardless of the
gate, so burn-in protection survives a screen that's entirely static. Such a
screen only ever sees that periodic refresh, which is correct, since nothing on
it changes.

Making it required means the config states outright what each widget does, at the
cost of more ceremony in every dashboard file.

One naming trap: a widget's top-level `refresh` (render cadence) and
`weekly-calendar`'s nested `config.refresh` (data cache TTL) are different
settings at different nesting levels.

```yaml
dashboard:
  screens:
    - name: weekly
      widgets:
        - type: clock
          bounds: [700, 0, 800, 50]
          refresh: "5m"      # only refresh the clock every 5 min
        - type: separator
          bounds: [0, 50, 800, 52]
          refresh: "static"  # never changes; never triggers a refresh
        - type: weekly-calendar
          bounds: [0, 52, 800, 480]
          refresh: "15m"     # how often this widget may refresh the panel
          config:
            refresh: "15m"   # DATA cache TTL, distinct from the cadence above
```
