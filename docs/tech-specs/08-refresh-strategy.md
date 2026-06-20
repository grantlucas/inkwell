# Refresh Strategy

## The problem

The Waveshare 7.5" V2 clears ghosting by running a full-refresh waveform
that inverts the whole panel black/white several times before settling on
the new image. That's correct for occasional updates, but a dashboard
re-renders often (a one-minute clock tick, a weather poll), and flashing
the entire panel for a change that's mostly identical looks chaotic.

Earlier, inkwell initialized the panel once at startup and then called
`Display()` on every render cycle — so every tick ran the full-refresh
flash, regardless of how little changed.

## What the panel can actually do

The 7.5" V2 firmware exposes four update waveforms, selected by which
`init_*` sequence loads the controller LUT. inkwell already models all
four in `profile.go` (`InitFull`, `InitFast`, `InitPartial`, `Init4Gray`):

<!-- markdownlint-disable MD013 -->
| Mode | inkwell sequence | Time | Flicker | Notes |
|------|------------------|------|---------|-------|
| Full | `InitFull` | 4–5 s | flashes several times | clears ghosting, best contrast |
| Fast | `InitFast` | ~1.5 s | flickers **once** (shows the inverted image first) | bilevel (BW) only |
| Partial | `InitPartial` | ~0.5 s | **none** | bilevel (BW) only; can target a window |
| Grayscale | `Init4Gray` | ~2 s | flickers | the **only** 4-gray waveform — no fast/partial variant |
<!-- markdownlint-enable MD013 -->

Two facts drive the whole design:

1. **Fast and partial are BW-only.** There is no flicker-free 4-gray
   waveform. In `gray4` mode the flash is unavoidable on any real change.
2. **Partial refresh needs the old plane fed.** The controller decides
   which pixels to flip by diffing `OldBufferCmd` (0x10) against
   `NewBufferCmd` (0x13), but a partial refresh never repopulates the old
   plane on its own. If you don't write the previous frame to 0x10 before
   each partial update, stale/degraded controller RAM produces visible
   noise. `EPD.DisplayPartial` therefore takes both the new frame and the
   frame currently on the panel.

## Reference implementations

- **Waveshare `epd7in5_V2.py`** exposes `init()`, `init_fast()`,
  `init_part()` and a `display_Partial()` that sets the window via 0x91 /
  0x90 — but the stock driver never re-feeds 0x10, which is the source of
  the partial-refresh noise others have hit.
- **gohu.org, "Fixing Waveshare e-paper partial updates" (2025)** diagnoses
  the 0x10 issue and lands on a concrete cadence: **full refresh hourly,
  fast every 10 minutes, partial otherwise** — so 9 of 10 updates are
  flicker-free while periodic full refreshes clear accumulated ghosting.
- **Waveshare wiki** recommends refreshing the panel **at least once per
  24 h** to avoid burn-in.

## inkwell's strategy

The decision lives in `refreshPlanner` (`refresh.go`), which picks an action
per render cycle from whether the packed frame changed and a cycle counter.
The render loop (`App.refresh` in `app.go`) dispatches it and re-initializes
the controller only when the waveform LUT actually changes.

**BW mode** does a full-screen fast refresh on each changed cycle:

- First cycle and every `defaultFullEvery` cycles → **full** (clears ghosting,
  satisfies the 24 h rule even when content is static).
- Otherwise, when content changed → **fast** (a single full-screen flicker via
  the proven `Display` path).
- When content is unchanged → **skip** (don't reflash an identical frame).

> **Why per-change updates aren't windowed/flicker-free.** A windowed *partial*
> refresh would let only the changed box update with no flash, which is ideal in
> principle. Two attempts failed on real hardware:
>
> 1. A true partial (old plane = the real previous frame) leaves the controller
>    to drive only the differing pixels. The 7.5" V2 partial waveform
>    under-drives such isolated pixels, so changed content (e.g. the clock
>    minute) rendered faint / half-updated.
> 2. Force-driving the box (`old=^new` inside the changed box, so every pixel is
>    driven) fixes the faintness — but `old=^new` only resolves toward the new
>    image under the full/fast waveform, which shows the inverted image first and
>    then settles. A windowed update enters partial mode (`0x91` + the partial
>    VCOM), which reverts the controller to the partial waveform regardless of
>    the `InitFast` LUT loaded, so the force-driven box never resolves and
>    settles *inverted* — the date / fuzzy-clock box came back solid black with
>    the text knocked out (inkwell-6jq).
>
> A windowed partial refresh and the force-drive it needs are therefore
> incompatible on this panel. inkwell drops the windowed per-change path and does
> one full-screen fast refresh per due change instead: correct (it reuses the
> same `Display` sequence the periodic fast/full refreshes use), at the cost of
> a single full-screen flash. The generic `EPD.DisplayPartial` primitive is kept
> for the future region-diff optimization (inkwell-5ik) but is not on the render
> path.

**Gray4 mode** has no flicker-free waveform, so the only lever is *when* to
refresh:

- First cycle and every `defaultFullEvery` cycles → **grayscale refresh**
  (periodic, burn-in protection).
- Otherwise, when content changed → **grayscale refresh**.
- When content is unchanged → **skip**.

### Configuration

This burn-in / waveform cadence is **fixed internally**, not user-configurable
— it's a property of the panel hardware (how often it needs a full clearing
flash), not something a dashboard author tunes. The constants live next to the
planner (`defaultFullEvery = 60` in `refresh.go`): a full / forced-grayscale
refresh roughly hourly at the default interval, with BW fast refreshes on every
changed cycle in between. The only refresh setting in the config is the
per-widget cadence below.

### Trade-offs

- **Fast refreshes accumulate ghosting** — that's why a full refresh runs on a
  fixed cadence rather than never.
- **One full-screen flash per change, not flicker-free** — every due change does
  a single full-screen fast flash. A windowed, flicker-free per-change update was
  tried and abandoned: it either under-drives the changed pixels or settles the
  box inverted (see the note above).
- **gray4 can't be made flicker-free** — if flicker matters more than the
  grayscale legibility, run `color_mode: bw`.

## Per-widget refresh cadence (the refresh queue)

The burn-in cadence above decides *which waveform* a push uses. A separate axis
decides *whether a content change is allowed to push at all this minute*.

The problem: a dashboard's widgets change on uncorrelated cadences (the clock
every minute, the calendar every 15 min, weather every few hours). Because the
loop pushed the moment the composited frame differed, two widgets that update on
the same period but offset in phase produced *two* refreshes per period instead
of one — and on gray4 that's two flickers.

So **every widget must set a `refresh:` in its config** — there is no default,
no global setting, and no widget-code fallback. The value is one of:

- a duration of **at least one minute** (`"5m"`, `"24h"`) — how often the widget
  may refresh the panel; or
- the literal `"static"` (alias `"never"`) — the widget never changes and so
  never triggers a refresh on its own (`separator` is the canonical case).

Making it required and explicit means it's always clear from the config what
each widget does; `LoadConfig` errors if a widget omits `refresh` or sets it
below the one-minute floor. There is no `RefreshEvery()`-style code interface —
the config is the single source of truth, parsed into `WidgetConfig.Refresh`
(`config.go`).

`refreshSchedule` (`refresh_queue.go`) holds each screen's cadences (a static
widget contributes a cadence of `0`) and answers `anyDue(now)`: a widget with
cadence *N* minutes is due when the **minute-of-day** is divisible by *N*.
Aligning to the wall-clock minute-of-day (not to an arbitrary start time) is
what makes widgets sharing a cadence fall due *together* — two `5m` widgets both
fire on `:00/:05/:10` and coalesce. Static widgets (cadence `0`, or anything
below a minute) never open the gate.

The render loop (`App.refresh` in `app.go`) then feeds `due && changed` to the
planner instead of bare `changed`. Consequences:

- A change that lands on an **undue** minute is *held*, not dropped — the loop
  skips and leaves the last-pushed buffer untouched, so the change ships on the
  next due minute (alongside anything else due then).
- The planner's **periodic full/grayscale refresh still fires regardless** of
  the gate, preserving the burn-in / ghosting cadence above.

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
            refresh: "15m"   # DATA cache TTL — distinct from the cadence above
```

**Naming caveat:** a widget's top-level `refresh` (render cadence) and
`weekly-calendar`'s nested `config.refresh` (data cache TTL) are different
settings at different nesting levels. Don't conflate them.

A screen whose widgets are all static never satisfies the gate, so it only ever
sees the periodic burn-in refresh — which is correct, since nothing on it
changes.

## Verification

The web preview reconstructs the device buffer from the captured planes, so
it can't show flicker — it renders a static post-pack image. **Sign-off is
on real hardware**: run both `color_mode: bw` and `gray4` and confirm BW
routine ticks no longer flash while still clearing on cadence, and that
gray4 stops re-flashing an unchanged dashboard.
