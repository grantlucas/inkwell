# ADR 0006: Composite into a 12-level palette, quantize at pack time, never dither

- **Status:** Accepted
- **Recorded:** 2026-09-18

## Context

The panel does 1-bit, or four grays in `Init4Gray` mode
([ADR 0001](0001-target-waveshare-7in5-v2-on-a-pi-zero-2w.md)). Widgets want more
tones than that: precipitation bars want a ramp, weather icons are anti-aliased
art, charts want a dim axis against dark data.

Two ways to bridge the gap. Widgets could draw directly in device levels, which
means every widget has to know which mode is active and rewrite its palette when
`color_mode` changes. Or the compositor could offer a richer palette and collapse
it once, at the boundary.

Ordered dithering (Bayer 4x4) was tried on the BW path as a way to preserve soft
tones as stipple, and it was dropped after a device photo settled it. Gray4
quantizes anything below roughly `Y=192` to white, so the `PaperGray05` through
`PaperGray30` tints meant to read as gentle structure disappeared on hardware
anyway. On BW the dither kept them visible, but as visibly noisy backgrounds that
made the panel look fuzzy next to the smooth source design, and the gray-on-white
text it was meant to rescue still read worse on the device than at the desk.

## Decision

Widgets draw into a 12-level `PaperPalette`
([`widget/palette.go`](../../internal/inkwell/widget/palette.go)), and
`PackImage` ([`buffer.go`](../../internal/inkwell/buffer.go)) collapses that frame
straight to the device's bit depth with no dithering:

- **`packBW`**: pure threshold. `Y > 128` is white, `Y <= 128` (at least half
  covered) is black.
- **`packGray4`**: four luminance buckets from `gray4Palette`. `Y > 192` white,
  `> 128` light gray, `> 64` dark gray, else black. Packed 4 px/byte and split
  into two 1bpp planes for commands 0x10 and 0x13.
- **`Color7`** is reserved in the `ColorDepth` enum with no packer yet;
  `PackImage` returns an error for a profile naming an unsupported depth, and
  callers treat that as fatal config.

## Consequences

Widgets stay mode-agnostic, and only one place knows how the collapse works.

The trap is that the source canvas is richer than any device output, so a design
can look right in the compositor and vanish on the panel. A `PaperGray20`
background tint lands in Gray4's light bucket and snaps to white under the BW
threshold, so it reads on neither mode. Three rules fall out of that, and they're
the ones [`AGENTS.md`](../../AGENTS.md) enforces on new visual work:

- Soft accents are solid `PaperBlack` strokes or inverted fills, not gray tints.
  Gray fills only pay off in regions big enough for a Gray4 bucket to read, which
  in practice means precipitation-bar interiors at `PaperGray70`.
- Text uses `PaperBlack`, and hierarchy comes from weight and size rather than
  colour.
- Sign-off is the device view at `http://localhost:8080/`, not the `?source=1`
  design-intent view.

Dropping dithering is also why body text moved to a bitmap typeface (Tamzen, via
the BDF parser in [`fonts/bdf.go`](../../internal/inkwell/fonts/bdf.go)): 1-bit
glyph masks have no anti-aliasing for the threshold to eat, so `fonts.Regular`
holds up at every shipped size.

`TestPaperPalette_BWBucket` pins which palette entries fall on which side of the
threshold, so adding a shade to the palette is a deliberate act with a test to
update.
