# ADR 0013: Keep the reset-default drive rails for the full refresh

- **Status:** Accepted
- **Recorded:** 2026-09-19
- **Amends:** [ADR 0009](0009-match-initfull-drive-strength-to-initfast.md)
  (same bytes, corrected rationale)

## Context

[ADR 0009](0009-match-initfull-drive-strength-to-initfast.md) dropped the
power-setting command (`0x01`) from `InitFull` and swapped its booster bytes
for the fast sequence's, reasoning that the vendor `init()` "clamps" VDH/VDL
below the controller's post-reset default and that the fast path, which never
sends `0x01`, was crisper because it inherited that higher default.

The UC8179c datasheet does not support that reading. The power-on default of
register `0x01` is `VDH = VDL = 0x3A`, ±14 V on both rails. The vendor's
`{07 07 28 17}` asks for VDH ≈ +10.5 V and VDL = −7 V: lower, and deliberately
asymmetric. Waveshare changed it from the older `{07 07 3F 3F}` (±15 V) in
September 2024 to suit a newer film batch. So omitting `0x01` does not "let the
panel inherit a clamp-free default"; it drives the film harder and
symmetrically than the current vendor bytes do. In register `0x06` bits [5:3]
of each byte are the booster drive strength (1 to 8): the vendor full-refresh
booster `{17 17 28 17}` runs the start-up phases at 3 and the sustain phase C1
at 6, while the fast/4-gray `{27 27 18 17}` runs start-up at 5 and sustain at 4.

An earlier revision of this ADR therefore restored the vendor `init()` byte for
byte. **That was deployed on 2026-09-19 and reverted the same day.** The first
full refresh on the panel came back visibly lighter than it had with the
reset-default rails. The observation is confounded with the per-push power-off
that shipped in the same build ([ADR 0012](0012-sleep-the-panel-between-refreshes.md)),
but the vendor demo run earlier that day with the same `0x01` bytes and no
power-off had also read slightly muted next to Inkwell's default-rail output,
so the vendor bytes were not kept.

The plausible physical reading: this panel (a post-Oct-2024 unit that spent
months powered continuously and partly lit from behind) needs more source
drive than Waveshare's current batch tuning provides. A harder drive packs the
pigment further and leaves less to relax after the pulse ends.

## Decision

`InitFull` in [`profile.go`](../../internal/inkwell/profile.go) keeps the
bytes ADR 0009 arrived at: booster `{27 27 18 17}`, no `0x01`, then power on,
panel setting, resolution, dual-SPI off, VCOM interval, TCON. ADR 0009's
*decision* stands; its *rationale* is replaced by this one. The other three
sequences are the vendor's byte for byte.

`TestInitSendsResetThenProfileCommands` pins the command *and* data bytes of
all four sequences, so no sequence can drift without a failing test that names
the byte.

## Consequences

`InitFull` is a documented, hardware-verified deviation from the vendor
`init()`. Anyone diffing the profile against `epd7in5_V2.py` will find it and
should read this ADR and the comment in `profile.go` before "fixing" it.

Two experiments remain open if contrast is ever short again, each one line:
the vendor booster `{17 17 28 17}` (stronger sustain phase) with the default
rails, and the older `{07 07 3F 3F}` power setting (±15 V). Both need the
photo protocol in [ADR 0014](0014-shield-the-tft-backplane-from-light.md).
