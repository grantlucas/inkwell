# Hardware Grayscale Ceilings

Inkwell renders into a 12-level grayscale [`PaperPalette`][palette] at the
compositor and then packs that down to whatever the target panel actually
supports. The panel — not the palette — sets the hard ceiling on how many
discrete tones can render. **There is no dithering**: the packer buckets
each pixel straight to the nearest device level, so a palette gray either
lands in a distinct native bucket or collapses to its nearest neighbour.

This guide tells you what those ceilings are per Waveshare panel family so
you can design for the cleanest possible output on your specific hardware.

[palette]: ../../internal/inkwell/widget/palette.go

## TL;DR — pick the path that matches your panel

<!-- markdownlint-disable MD013 -->
| Your panel | Native ceiling | Best palette strategy |
|---|---|---|
| Waveshare 7.5" V2 (current default, `gray4`) | 4-level via Init4Gray (also 1-bit via `color_mode: bw`) | Design to the 4 Gray4 buckets (see table below); on `bw` use only `PaperWhite` + `PaperBlack` for predictable tone |
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

Inkwell has no dither stage. Both packers (`packBW` and `packGray4` in
[`internal/inkwell/buffer.go`](../../internal/inkwell/buffer.go)) collapse
the 12-level compositor frame straight to the device's bit depth by
luminance. A palette gray that doesn't fall in its own native bucket snaps
to the nearest one — it does not survive as a halftone texture. So to get a
flat, intentional tone you must keep your palette choices inside the
panel's native level count.

## Strategy 1 — Waveshare 7.5" V2 (the default)

The 7.5" V2 supports two modes, both wired end-to-end and selected by
`color_mode` at the top of `inkwell.yaml`:

```yaml
display: waveshare_7in5_v2
color_mode: gray4   # default; "bw" for 1-bit black/white
```

### `gray4` (default): 4 native levels

`color_mode: gray4` drives the panel's `Init4Gray` waveform and pins
`PackImage` and the compositor onto the `packGray4` 2-bit path. The packer
buckets each pixel's luminance `Y` into one of four native levels:

<!-- markdownlint-disable MD013 -->
| Native level | Luminance range | Canonical `PaperPalette` entry | Other entries that collapse here |
|---|---|---|---|
| White | `Y > 192` | `PaperWhite` | `PaperGray05`, `PaperGray10`, `PaperGray20` |
| Light gray | `129 ≤ Y ≤ 192` | `PaperGray30` | `PaperGray40` |
| Dark gray | `65 ≤ Y ≤ 128` | `PaperGray70` | `PaperGray50`, `PaperGray60` |
| Black | `Y ≤ 64` | `PaperBlack` | `PaperGray80`, `PaperGray90` |
<!-- markdownlint-enable MD013 -->

For flat, predictable tone, design with the four canonical entries
(`PaperWhite`, `PaperGray30`, `PaperGray70`, `PaperBlack`). Note `PaperGray20`
reads as **white**, not light gray — it sits above the `Y > 192` cutoff.
The precip-bar interiors are the canonical dark-gray case: they use
`PaperGray70` so Gray4 renders a real dark gray.

Trade-offs: `gray4` has a slower refresh, a larger framebuffer, no partial
refresh, and no flicker-free waveform (every refresh flashes). It reads
noticeably better on hardware than 1-bit, which is why it's the default.

### `bw`: 1-bit black/white

`color_mode: bw` uses `packBW`, a pure threshold: any pixel with `Y <= 128`
("at least half covered") becomes black, everything lighter becomes white.
There is no stipple, so soft grays do not survive — they snap all-or-nothing
to black or white:

- `PaperWhite` and the light ramp up to `PaperGray40` → **white**
- `PaperGray50` (`Y = 128`) and darker → **black**

Design for `bw` with `PaperWhite` and `PaperBlack` only, and carry hierarchy
with font weight/size and solid strokes rather than gray fills. `bw` is
faster, has a smaller framebuffer, and supports partial/fast refresh
waveforms — pick it when refresh cadence matters more than tonal range.

## Strategy 2 — IT8951-controller panels (6" HD, 7.8", 9.7", 10.3", 13.3")

You get **16 native grayscale levels at 4bpp**. With the right driver
(not yet wired in Inkwell — see follow-ups), every `PaperGrayNN` palette
entry can map directly onto a native gray bucket.

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
   ┌────────┐   1-bit BW panel (Waveshare 7.5" V2, color_mode: bw)
   │packBW  │ → 2 device levels via Y<=128 threshold; grays snap to B/W
   └────────┘
        │
        ▼
   ┌─────────┐  4-level Init4Gray panel (Waveshare 7.5" V2, color_mode: gray4, default)
   │packGray4│→ 4 device levels by luminance bucket; nearest-level snap
   └─────────┘
        │
        ▼
   ┌────────┐   IT8951-controller panel (6" HD and up, future)
   │ pack16 │ → 16 device levels, direct palette mapping
   │ (TBD)  │
   └────────┘
```

The 12-step `PaperPalette` is a deliberate compromise: enough granularity
to express clear visual hierarchy in widget code, but small enough to
keep palette indices memorable as named constants. It is not tied to any
specific panel's native level count.

## Quick guidance

- **Designing for the 7.5" V2 with `color_mode: gray4` (the default)?**
  Prefer `PaperWhite`, `PaperGray30`, `PaperGray70`, and `PaperBlack` for
  flat tones. Other palette entries collapse into one of those four
  buckets — fine when you don't mind the snap, but don't expect a distinct
  shade from them.
- **Designing for the 7.5" V2 with `color_mode: bw`?** Use `PaperWhite`
  and `PaperBlack` only. Anything `PaperGray50` or darker reads as black;
  lighter grays read as white. Carry hierarchy with weight and size.
- **Designing for an IT8951 panel (when supported)?** Use the full ramp
  freely; no need to think about device collapse.

If you're not sure which panel family you have, [check Waveshare's wiki
for your panel model][waveshare-wiki] — the product page lists the
controller and whether 4bpp / 16-grayscale refresh is supported.

[waveshare-wiki]: https://www.waveshare.com/wiki/Main_Page
