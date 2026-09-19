# ADR 0001: Target the Waveshare 7.5" V2 on a Pi Zero 2 W

- **Status:** Accepted
- **Recorded:** 2026-09-18

## Context

Inkwell needs one reference panel to develop against. Everything downstream
(colour depth, refresh behaviour, buffer layout, wiring docs) follows from that
choice, so it's worth pinning explicitly rather than leaving it implied by
whatever is plugged in.

The Waveshare 7.5" V2 is 800x480, driven over SPI through the E-Paper Driver
HAT (Rev2.3) sitting on a Pi Zero 2 W's 40-pin header. The HAT ships with a
resistor selection: 0.47 ohm (B) for newer panels including the 7.5" V2, 3 ohm
(A) for older variants.

The panel is 1-bit at heart, with a 4-level grayscale mode (`Init4Gray`) driven
by a different waveform LUT and a 2-bit-per-pixel buffer. There is no native
support for more than four grays in this panel family. Waveshare's larger
formats (6" HD, 7.8", 9.7", 10.3", 13.3") use an external IT8951 controller and
do 16 native grays at 4 bits per pixel, but they need a different transport
entirely.

## Decision

Target the Waveshare 7.5" V2 + Driver HAT Rev2.3 (0.47 ohm) + Pi Zero 2 W as
the reference hardware, and treat its ceilings as fixed constraints on the rest
of the system rather than something to design around.

Wiring, as the driver expects it:

<!-- markdownlint-disable MD013 -->
| Signal | Function | BCM GPIO | Board Pin | Description |
|--------|----------|----------|-----------|-------------|
| VCC | Power | - | 3.3V | Power supply |
| GND | Ground | - | GND | Ground |
| DIN | MOSI | 10 | 19 | SPI data in |
| CLK | SCLK | 11 | 23 | SPI clock |
| CS | CE0 | 8 | 24 | SPI chip select |
| DC | Data/Command | 25 | 22 | Low = command, High = data |
| RST | Reset | 17 | 11 | Hardware reset (active low) |
| BUSY | Busy status | 24 | 18 | Low = busy, High = idle |
| PWR | Power control | 18 | 12 | Display power on/off |
<!-- markdownlint-enable MD013 -->

SPI transport: `/dev/spidev0.0`, mode 0 (CPOL=0, CPHA=0), 4 MHz, MSB first,
8-bit words.

Panel limits that bind the rest of the design:

- Refresh times: roughly 4-5 s full, 1.5 s fast, 0.4 s partial, 2 s grayscale.
- Keep at least 180 s between full refreshes, and refresh at least once every
  24 hours to avoid burn-in.
- Operating range 0-50 C.
- Never leave the panel powered with static high voltage. Sleep or power down
  when idle.

## Consequences

Four gray levels is the hard ceiling, so the compositor's richer palette has to
collapse somewhere (see [ADR 0006](0006-composite-in-a-palette-and-quantize-at-pack-time.md)),
and the burn-in and spacing rules drive the refresh planner
(see [ADR 0008](0008-full-screen-fast-refresh-instead-of-windowed-partial.md)).

IT8951 panels aren't targeted, but nothing in the compositor or the widgets
assumes this panel: a 16-gray backend plugs in at the profile and packer layer
(see [ADR 0002](0002-data-driven-display-profiles.md)).

For the end-user implications of the grayscale ceiling, see
[`docs/guides/hardware-grayscale.md`](../guides/hardware-grayscale.md). For
install and wiring steps, see
[`docs/guides/installation.md`](../guides/installation.md).

Reference: [7.5inch e-Paper HAT Manual](https://www.waveshare.com/wiki/7.5inch_e-Paper_HAT_Manual),
[E-Paper Driver HAT Wiki](https://www.waveshare.com/wiki/E-Paper_Driver_HAT).
