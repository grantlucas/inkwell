# Architecture Decision Records

These record the decisions behind Inkwell's driver, rendering, and refresh
behaviour: what was chosen, what it was chosen over, and what it costs. Each one
is a standalone file, numbered in the order it was recorded and written in the
usual context / decision / consequences shape.

An ADR describes the decision as of its date. When the code and an ADR disagree,
the code wins, and the ADR should get a follow-up that supersedes it rather than
an edit that quietly rewrites history.

ADRs 0001 through 0011 were back-filled on 2026-09-18 from `docs/tech-specs/`,
which they replace. The specs mixed decisions with vendor reference material and
had drifted out of date (they still described Bayer dithering and TTF fonts,
neither of which the tree has). The vendor material is gone: the SPI command
sequences live in [`profile.go`](../../internal/inkwell/profile.go), which is
their only copy now.

## Index

| ADR | Decision |
|-----|----------|
| [0001](0001-target-waveshare-7in5-v2-on-a-pi-zero-2w.md) | Target the Waveshare 7.5" V2 on a Pi Zero 2 W |
| [0002](0002-data-driven-display-profiles.md) | Model each display as profile data, not its own driver |
| [0003](0003-periph-io-for-spi-and-gpio.md) | Use periph.io for SPI and GPIO |
| [0004](0004-swappable-hardware-backends.md) | Put every backend behind one `Hardware` interface |
| [0005](0005-hardware-free-tests-with-a-full-coverage-gate.md) | Test off-hardware, gated at 100% statement coverage |
| [0006](0006-composite-in-a-palette-and-quantize-at-pack-time.md) | Composite into a 12-level palette, quantize at pack time, never dither |
| [0007](0007-poll-the-busy-pin-in-waitidle.md) | Block on the BUSY pin with a polling `waitIdle` |
| [0008](0008-full-screen-fast-refresh-instead-of-windowed-partial.md) | One full-screen fast refresh per change, no windowed partial |
| [0009](0009-match-initfull-drive-strength-to-initfast.md) | Tune `InitFull` to drive as hard as `InitFast` |
| [0010](0010-re-init-before-every-gray4-push.md) | Re-run the hardware init before every Gray4 push |
| [0011](0011-require-a-per-widget-refresh-cadence.md) | Require a per-widget `refresh:` and gate pushes on a wall-clock queue |
| [0012](0012-sleep-the-panel-between-refreshes.md) | Sleep the panel after every push and re-init before the next (supersedes 0010) |
| [0013](0013-restore-the-vendor-init-sequence.md) | Restore the vendor `init()` sequence for the full refresh (supersedes 0009) |
| [0014](0014-shield-the-tft-backplane-from-light.md) | Shield the panel's TFT backplane from light |

## Writing a new one

Copy the shape of an existing file: a one-line title stating the decision, a
status and date, then context, decision, consequences. Number it next in
sequence and add a row above. Keep the reasoning that a reader can't recover
from the code, especially anything that only showed up on real hardware.
