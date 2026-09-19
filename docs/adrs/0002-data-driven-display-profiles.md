# ADR 0002: Model each display as profile data, not as its own driver

- **Status:** Accepted
- **Recorded:** 2026-09-18

## Context

The obvious way to support a second panel later is to write a second driver.
That path duplicates the whole command flow (reset, init, write old plane,
write new plane, refresh, wait on busy) for every panel, and the flow is
identical across the family. Only the numbers change: resolution, colour depth,
which bytes each init sequence sends, which command carries the old plane.

The Waveshare Python driver takes the opposite approach, one Python module per
panel, and the sequences are copy-pasted between them.

## Decision

Keep one generic `EPD` driver ([`epd.go`](../../internal/inkwell/epd.go)) and
describe each panel as a `DisplayProfile` struct of data
([`profile.go`](../../internal/inkwell/profile.go)). Adding a display means
adding a profile, not adding logic.

A profile carries `Width`, `Height`, `Color` (`BW` / `Gray4` / `Color7`),
`Capabilities` flags (`FastRefresh`, `PartialRefresh`, `Grayscale`), the per-mode
init sequences (`InitFull`, `InitFast`, `InitPartial`, `Init4Gray`), the buffer
and refresh commands (`OldBufferCmd`, `NewBufferCmd`, `RefreshCmd`), the
partial-refresh metadata (`PartialWindowCmd`, `PartialEnterCmd`, `PartialVCOM`),
a `SleepSequence`, and an optional waveform `LUT`. `BufferSize()` derives the
frame size from colour depth and resolution (8 px/byte BW, 4 px/byte Gray4,
2 px/byte Color7).

`Profiles` is a name-keyed map, and `display:` in `inkwell.yaml` selects the
active one.

## Consequences

`EPD.Init(mode)` can refuse a mode the profile doesn't declare, so unsupported
waveforms fail loudly instead of writing garbage to the panel.

The profile is also the spec: the init sequences encode the vendor's SPI command
order, so a profile that disagrees with the controller datasheet is a data bug
with no code to read around it. The former `05-spi-command-reference.md` duplicated
that table in prose, which meant two places to keep in sync, so the profile is now
the only copy.

Since the driver only reads data, a panel in a different family still needs new
transport work (the IT8951 parts, for instance, aren't SPI-command-compatible),
but nothing above the driver changes.
