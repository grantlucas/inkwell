# ADR 0009: Tune `InitFull` to drive as hard as `InitFast`

- **Status:** Superseded by [ADR 0013](0013-restore-the-vendor-init-sequence.md)
- **Recorded:** 2026-09-18

## Context

On the BW path, both the periodic full refresh and the routine fast refresh
force-drive every pixel (`old=^new`), so the only thing separating them is the
init sequence that loads the LUT.

The stock Waveshare sequences differ in power terms. `init()` clamps the power
setting (`0x01`) to a lower VDH/VDL and uses a weaker booster (`0x06`) than
`init_fast()`, which never sends `0x01` at all and so inherits the higher
post-reset default.

Left alone, that made the periodic full refresh drive softer than the fast
refreshes around it. The panel settled muted and blotchy on the full refresh, and
the next fast refresh visibly restored contrast, which is the opposite of what
the cadence in [ADR 0008](0008-full-screen-fast-refresh-instead-of-windowed-partial.md)
intends: the periodic refresh is supposed to be the clean one.

## Decision

`InitFull` adopts the fast booster settings and drops the `0x01` power clamp, so
full and fast drive identically.

It stays a proper multi-flash full refresh rather than becoming a second fast
refresh. The full/GC waveform is selected by *not* forcing temperature (`0xE5`),
which only `InitFast` does.

## Consequences

Contrast is stable across the cadence, so the periodic refresh no longer reads as
a dip in quality.

The profile now deliberately diverges from the vendor's `init()` sequence. Anyone
diffing [`profile.go`](../../internal/inkwell/profile.go) against
`epd7in5_V2.py` will find the difference and should leave it alone.
