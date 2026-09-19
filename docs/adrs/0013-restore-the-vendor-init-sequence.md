# ADR 0013: Restore the vendor `init()` sequence for the full refresh

- **Status:** Accepted
- **Recorded:** 2026-09-19
- **Supersedes:** [ADR 0009](0009-match-initfull-drive-strength-to-initfast.md)

## Context

[ADR 0009](0009-match-initfull-drive-strength-to-initfast.md) dropped the
power-setting command (`0x01`) from `InitFull` and swapped its booster bytes
for the fast sequence's, reasoning that the vendor `init()` "clamps" VDH/VDL
below the controller's post-reset default and that the fast path, which never
sends `0x01`, was crisper because it inherited that higher default.

The UC8179c datasheet does not support either half of that. The power-on
default of register `0x01` is `VDH = VDL = 0x3A`, ±14 V on both rails. The
vendor's `{07 07 28 17}` asks for VDH ≈ +10.5 V and VDL = −7 V: lower, and
deliberately asymmetric. Waveshare changed it from the older `{07 07 3F 3F}`
(±15 V) in September 2024 to suit the newer film batch, and this panel is from
after that change. Omitting `0x01` is therefore not "letting the panel drive
harder"; it is running the film at a symmetric ±14 V it was not tuned for.

The booster swap pointed the same way. In register `0x06` bits [5:3] of each
byte are the drive strength (1 to 8). The vendor full-refresh booster
`{17 17 28 17}` runs the start-up phases A and B at strength 3 and the sustain
phase C1 at 6; the fast/4-gray booster `{27 27 18 17}` runs A and B at 5 and C1
at 4. Phase C1 is the phase that holds the rails for the whole multi-flash
waveform, so the swap made the full refresh's sustain weaker, not stronger.

The muted hourly full refresh that ADR 0009 was chasing turned out to be the
same fault as the rest of issue #77: light on the TFT backplane
([ADR 0014](0014-shield-the-tft-backplane-from-light.md)). It reproduced with
the vendor's own driver and init bytes, and stopped when the case was sealed.
ADR 0009 also shipped with "needs on-device sign-off", and no sign-off was ever
recorded.

`old=^new` on the BW path, which ADR 0009 called Inkwell's force-drive, is what
the vendor `display()` does too (`0x10 = ~image`, `0x13 = image`). It was never
a reason to diverge from the vendor init.

## Decision

`InitFull` in [`profile.go`](../../internal/inkwell/profile.go) is the current
vendor `init()` byte for byte: booster `{17 17 28 17}`, power setting
`{07 07 28 17}`, power on, panel setting, resolution, dual-SPI off, VCOM
interval, TCON. The other three sequences were already identical to the
vendor's and are unchanged.

`TestInitSendsResetThenProfileCommands` pins the command *and* data bytes of
all four sequences, so the profile cannot drift from the reference without a
failing test that names the byte.

If Waveshare changes `init()` again, change the profile to match and record
why. Do not tune the drive bytes by eye against the web preview, which
reconstructs the device buffer and cannot show drive strength.

## Consequences

The profile is once more a straight transcription of the vendor driver, which
is the state [ADR 0002](0002-data-driven-display-profiles.md) intends.

The older `{07 07 3F 3F}` power setting (the C reference and GxEPD2) remains a
documented one-line experiment if a future panel batch reads muted with the
vendor bytes and the light path has been ruled out first.
