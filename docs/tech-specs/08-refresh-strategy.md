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

**BW mode** cycles full → fast → partial:

- First cycle and every `full_every` cycles → **full** (clears ghosting,
  satisfies the 24 h rule even when content is static).
- Every `fast_every` cycles → **fast** (single flicker, clears more
  ghosting than partial).
- Otherwise, when content changed → **partial** (flicker-free).
- When content is unchanged → **skip** (don't reflash an identical frame).

**Gray4 mode** has no flicker-free waveform, so the only lever is *when* to
refresh:

- First cycle and every `full_every` cycles → **grayscale refresh**
  (periodic, burn-in protection).
- Otherwise, when content changed → **grayscale refresh**.
- When content is unchanged → **skip**.

### Configuration

The cadence is tunable via the `refresh` section (counts are in render
cycles, so at the default 60 s interval `full_every: 60` is roughly hourly):

```yaml
refresh:
  full_every: 60   # cycles between full / forced grayscale refreshes
  fast_every: 10   # cycles between fast refreshes (bw only; 0 = never)
```

Defaults are `full_every: 60`, `fast_every: 10`.

### Trade-offs

- **Partial refreshes accumulate ghosting** — that's why full/fast run on a
  cadence rather than never. Tightening `full_every` trades flicker
  frequency for cleaner contrast.
- **Full-window partial, not region-diff** — inkwell partial-refreshes the
  whole panel rather than computing the changed bounding box. This already
  removes the flicker for routine ticks; per-region partial updates (only
  redrawing the clock's few pixels) are a possible future optimization.
- **gray4 can't be made flicker-free** — if flicker matters more than the
  grayscale legibility, run `color_mode: bw`.

## Verification

The web preview reconstructs the device buffer from the captured planes, so
it can't show flicker — it renders a static post-pack image. **Sign-off is
on real hardware**: run both `color_mode: bw` and `gray4` and confirm BW
routine ticks no longer flash while still clearing on cadence, and that
gray4 stops re-flashing an unchanged dashboard.
