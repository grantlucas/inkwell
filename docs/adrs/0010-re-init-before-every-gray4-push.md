# ADR 0010: Re-run the hardware init before every Gray4 push

- **Status:** Superseded by [ADR 0012](0012-sleep-the-panel-between-refreshes.md)
- **Recorded:** 2026-09-18

## Context

`App.refresh` re-runs `EPD.Init` (hardware reset plus the mode's power-on and
booster sequence) when the resulting waveform label differs from what's applied,
or when the planner sets `forceInit`. The label check is a cheap way to skip
needless resets on BW, where a routine `refreshFast` and a periodic `refreshFull`
are different labels and trigger it naturally.

Gray4 has only one waveform, so the label never changes after the first cycle.
Gating on the label alone silently reduced the whole burn-in cadence to a single
`Init4Gray` at process start followed by nothing but `Display()` calls for the
life of the process. The periodic clearing flash never re-asserted power or
booster state, and contrast drifted over long runs, visible as fading and patchy
fills.

The first fix set `forceInit` only on the periodic tick. That cured the
"no re-init for the entire process lifetime" failure, but hardware testing found
it wasn't enough: the panel rendered crisp right after the forced periodic
re-init, then visibly faded on the very next routine push. The electrical state
drifts between consecutive Gray4 pushes, not only across the long burn-in window.

## Decision

In Gray4, set `forceInit` on every push that isn't skipped, not only the periodic
one. Gray4 has no cheaper steady state worth preserving, and this matches the
upstream Waveshare reference driver, which re-runs its 4-gray init before every
single 4-gray display call rather than amortizing it.

## Consequences

Each Gray4 push costs roughly its own duration again in reset and power-on time.
Since pushes are already gated behind each widget's `refresh:` cadence, which has
a one-minute floor ([ADR 0011](0011-require-a-per-widget-refresh-cadence.md)),
that's negligible against the cadence itself.

Nothing cheaper held up on hardware, so this is the settled answer rather than a
placeholder. BW keeps the label-diff shortcut.
