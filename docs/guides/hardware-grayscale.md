# Hardware Grayscale Ceilings

Inkwell renders into a 12-level grayscale [`PaperPalette`][palette] at the
compositor and then packs that down to whatever the target panel actually
supports. The panel — not the palette — sets the hard ceiling on how many
discrete tones can render without dithering.

This guide tells you what those ceilings are per Waveshare panel family so
you can design for the cleanest possible output on your specific hardware.

[palette]: ../../internal/inkwell/widget/palette.go

## TL;DR — pick the path that matches your panel

<!-- markdownlint-disable MD013 -->
| Your panel | Native ceiling | Best palette strategy |
|---|---|---|
| Waveshare 7.5" V2 (current default) | 1-bit (or 4-level via Init4Gray) | Use only `PaperWhite` + `PaperBlack` for flat tone; accept Bayer-4×4 stipple for intermediate grays |
| Other direct-SPI Waveshare panels (4.2", 5.83", 7.5" B/G/H, etc.) | 1-bit (most) or 4-level (Init4Gray-capable variants) | Same as above |
| IT8951-controller Waveshare panels (6" HD, 7.8", 9.7", 10.3", 13.3") | 16-level (4 bits per pixel) | Use the full ramp; tones map directly to native gray buckets |
<!-- markdownlint-enable MD013 -->

If your panel isn't listed here, check its Waveshare wiki page for the
phrase "16 gray scale" or "4 grayscale". The presence (or absence) of those
phrases tells you which family you're in.

## Why the panel sets the ceiling

E-paper panels are addressed at a per-pixel bit depth fixed by the
controller:

- **Direct-SPI panels** (the ones with the controller integrated into the
  panel) send 1 bit per pixel for normal refresh and optionally 2 bits per
  pixel for a slower 4-level grayscale mode driven by a different
  waveform LUT (`Init4Gray`). They do not support 4bpp grayscale at all.
- **IT8951-controller panels** (larger format — 6" HD and up) ship with an
  external IT8951 chip that maintains its own framebuffer and accepts up
  to 4bpp (16 levels) as the recommended refresh format.

Inkwell's `packBW` applies Bayer-4×4 ordered dithering so soft palette
grays survive as halftone stipple patterns on 1-bit panels — that's how
the dashboard's today-highlight, hour-band, soft separators, and gray
labels stay visible after packing. But dithering trades flat tone for
stipple texture. If you want flat tones (no stipple), your palette has to
stay within the panel's native level count.

## Strategy 1 — 1-bit panels (Waveshare 7.5" V2 and friends)

The active default. Two clean options:

**Option A: pure 1-bit, no dithering.** Use only `PaperWhite` and
`PaperBlack` in widget code. Every shape will be hard-edged but
absolutely flat. Recommended for: stark, high-contrast designs;
preserving the maximum refresh rate.

**Option B: 1-bit + Bayer dithering (current default).** Use any
`PaperGrayNN` value. Flat tonal regions become stipple textures the eye
reads as continuous gray at the panel's pixel pitch. Recommended for:
hierarchy via tone (muted secondary text, soft highlight tints, gray bar
charts). Trade-off: tiny regions (<12 px on a side) don't have enough
pixels for the stipple to read.

Either way, on the Waveshare 7.5" V2 you can also opt into the panel's
native 4-level grayscale mode via the upcoming `color_mode: gray4`
config knob (tracked in this project's beads workspace — run
`bd ready` to see the active Gray4 work). Once that lands, four
specific palette indices will map to flat native grays without dithering:

| Native level | Closest `PaperPalette` index |
|---|---|
| White | `PaperWhite` |
| Light gray | `PaperGray20` |
| Dark gray | `PaperGray60` |
| Black | `PaperBlack` |

Anything else in the palette will still dither (via the future
`packGray4` dither path) to fake the missing intermediates.

## Strategy 2 — IT8951-controller panels (6" HD, 7.8", 9.7", 10.3", 13.3")

You get **16 native grayscale levels at 4bpp**. With the right driver
(not yet wired in Inkwell — see follow-ups), every `PaperGrayNN` palette
entry can map directly onto a native gray bucket with no dithering at
all.

Bear in mind:

- Inkwell currently has no IT8951 backend. Wiring one is straightforward
  (the SPI hardware abstraction is already in place) but out of scope
  for the active 7.5" V2 work.
- 12 named palette entries is *fewer* than the 16 native levels the
  IT8951 can render. We may resize `PaperPalette` to 16 entries in the
  future — tracked as a follow-up — to match this ceiling exactly.

## Mapping between palette levels and what survives

A useful mental model when designing widgets:

```text
PaperPalette (12 design-time levels)
        │
        ▼
   ┌────────┐   1-bit BW panel (Waveshare 7.5" V2 default)
   │packBW  │ → 1 device level, Bayer-4×4 stipple gives ~16 perceived steps
   └────────┘
        │
        ▼
   ┌────────┐   4-level Init4Gray panel (Waveshare 7.5" V2 opt-in, future)
   │packGray4│→ 4 device levels, future Bayer dither extends to ~16 perceived
   └────────┘
        │
        ▼
   ┌────────┐   IT8951-controller panel (6" HD and up, future)
   │ pack16  │ → 16 device levels, no dithering needed
   │ (TBD)   │
   └────────┘
```

The 12-step `PaperPalette` is a deliberate compromise: enough granularity
to express clear visual hierarchy in widget code, but small enough to
keep palette indices memorable as named constants. It is not tied to any
specific panel's native level count.

## Quick guidance

- **Designing for the 7.5" V2 today?** Use the full ramp freely.
  Bayer-4×4 makes everything readable on the device. Reach for
  `PaperWhite` / `PaperBlack` only where you specifically want flat
  high-contrast tone (large fill blocks, hard borders).
- **Designing for a 7.5" V2 in future Gray4 mode?** Prefer `PaperWhite`,
  `PaperGray20`, `PaperGray60`, and `PaperBlack` for flat tones; use
  other palette entries only when you want a stipple texture instead.
- **Designing for an IT8951 panel (when supported)?** Use the full ramp
  freely; no need to think about device collapse.

If you're not sure which panel family you have, [check Waveshare's wiki
for your panel model][waveshare-wiki] — the product page lists the
controller and whether 4bpp / 16-grayscale refresh is supported.

[waveshare-wiki]: https://www.waveshare.com/wiki/Main_Page
