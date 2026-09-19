# ADR 0008: One full-screen fast refresh per change, no windowed partial refresh

- **Status:** Accepted
- **Recorded:** 2026-09-18

## Context

The 7.5" V2 clears ghosting by running a full-refresh waveform that inverts the
whole panel several times before settling. That's fine occasionally, but a
dashboard re-renders on a one-minute tick, and flashing the whole panel for a
clock minute looks chaotic. Early Inkwell initialized once at startup and called
`Display()` every cycle, so every tick flashed.

The firmware exposes four waveforms, and Inkwell models all four in
[`profile.go`](../../internal/inkwell/profile.go):

<!-- markdownlint-disable MD013 -->
| Mode | Sequence | Time | Flicker | Notes |
|------|----------|------|---------|-------|
| Full | `InitFull` | 4-5 s | flashes several times | clears ghosting, best contrast |
| Fast | `InitFast` | ~1.5 s | flickers once (shows the inverted image first) | bilevel (BW) only |
| Partial | `InitPartial` | ~0.5 s | none | bilevel (BW) only, can target a window |
| Grayscale | `Init4Gray` | ~2 s | flickers | the only 4-gray waveform |
<!-- markdownlint-enable MD013 -->

Two facts shape everything. Fast and partial are BW-only, so in `gray4` the flash
is unavailable to avoid. And partial refresh needs the old plane fed: the
controller decides which pixels to flip by diffing `OldBufferCmd` (0x10) against
`NewBufferCmd` (0x13), and a partial pass never repopulates 0x10 on its own.
The stock Waveshare driver skips that, which is the source of the partial-refresh
noise other people hit. gohu.org's 2025 write-up on fixing Waveshare partial
updates diagnoses it and lands on full hourly, fast every 10 minutes, partial
otherwise.

A windowed partial refresh is the ideal on paper: only the changed box updates,
with no flash. Two attempts failed on real hardware.

1. A true partial (old plane = the real previous frame) leaves the controller to
   drive only the differing pixels. This panel's partial waveform under-drives
   isolated pixels, so the changed content, a clock minute for instance, came out
   faint or half-updated.
2. Force-driving the box (`old=^new` inside the changed region, so every pixel is
   driven) fixes the faintness, but `old=^new` only resolves toward the new image
   under the full or fast waveform, which shows the inverted image first and then
   settles. A windowed update enters partial mode (`0x91` plus the partial VCOM),
   which reverts the controller to the partial waveform regardless of the
   `InitFast` LUT that's loaded. The force-driven box never resolved and settled
   inverted: the date and fuzzy-clock box came back solid black with the text
   knocked out (inkwell-6jq).

So the windowed path and the force-drive it needs are incompatible on this panel.

## Decision

Drop the windowed per-change path. `refreshPlanner`
([`refresh.go`](../../internal/inkwell/refresh.go)) picks one action per render
cycle from whether the packed frame changed and a cycle counter, and
`App.refresh` ([`app.go`](../../internal/inkwell/app.go)) dispatches it,
re-initializing the controller only when the waveform actually changes.

**BW:** first cycle and every `defaultFullEvery` cycles do a full refresh
(clears ghosting, satisfies the 24 h rule even when content is static);
otherwise a change does a full-screen fast refresh through the proven `Display`
path; an unchanged frame skips.

**Gray4:** no flicker-free waveform exists, so the only lever is when to refresh.
Periodic and changed cycles both do a grayscale refresh, unchanged frames skip.

The cadence is fixed internally (`defaultFullEvery = 60` in `refresh.go`, roughly
hourly at the default interval), not user config. It's a property of the panel,
not a dashboard-author preference. `EPD.DisplayPartial` stays as a primitive for
a possible future region-diff optimization (inkwell-5ik) but is off the render
path.

## Consequences

Every due change costs one full-screen flash. That's the deliberate trade against
a windowed update that either under-drives or settles inverted.

Fast refreshes accumulate ghosting, which is why the periodic full refresh runs
on a fixed cadence rather than never.

`gray4` can't be made flicker-free at all. If flicker matters more than grayscale
legibility, `color_mode: bw` is the answer.

The web preview reconstructs the device buffer from captured planes, so it can't
show flicker. Sign-off for any change here is on real hardware, in both modes.

See also [ADR 0011](0011-require-a-per-widget-refresh-cadence.md), which
decides when a change is allowed to push at all, and
[ADR 0012](0012-sleep-the-panel-between-refreshes.md), which replaced the
"re-init only when the waveform changes" rule above with a full init, display,
sleep cycle on every push. The contrast problems first attributed to this
cadence (ADRs 0009 and 0010, both superseded) were light on the panel's
backplane; see [ADR 0014](0014-shield-the-tft-backplane-from-light.md).
