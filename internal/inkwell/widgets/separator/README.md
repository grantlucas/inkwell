# Separator Widget

Draws a solid divider across its bounds. A horizontal one (the default)
runs the full width, anchored to the **bottom** of the region; a vertical
one runs the full height, anchored to the **right** edge, for dividing
widgets that sit side by side. Registered under the dashboard
`type: separator`.

Every row of the bar renders in solid `PaperBlack`. The `bw` packer
threshold-snaps (there is no dither), so a "soft" gray rule would simply
disappear — the separator is therefore a solid bar that reads identically on
both `bw` and `gray4`. See the rendering rules in the repository
[`CLAUDE.md`](../../../../CLAUDE.md).

## Screenshot

Device view (`bw`) showing dividers of thickness `1`, `2`, and `6` between
labeled sections:

![Separator widget device preview](docs/device.png)

## Configuration

Top-level keys (`type`, `bounds`, `refresh`) are required by every widget. A
separator never changes, so it should use `refresh: "static"` — that keeps it
from ever opening the per-screen refresh gate.

The bar is drawn at the bottom of `bounds` (the right edge, when vertical), so
size the region's height (width) to at least `thickness` and place the rule
with `bounds`, not with extra padding.

The widget-specific keys live under `config:`.

<!-- markdownlint-disable MD013 -->
| Key         | Type    | Default | Description                                                                 |
|-------------|---------|---------|-----------------------------------------------------------------------------|
| `thickness` | integer | `2`     | Height of the bar in pixels (width, when vertical). Must be positive. A float (e.g. `2.0`) is accepted and truncated to an integer. |
| `orientation` | string | `horizontal` | `horizontal` or `vertical`: which way the rule runs. |
<!-- markdownlint-enable MD013 -->

A non-numeric `thickness`, a value `<= 0`, or any other `orientation` is a
configuration error.

## Example

```yaml
- type: separator
  bounds: [0, 50, 800, 52]
  refresh: "static"
  config:
    thickness: 2

# Between a left and a right column, under a 48 px header band.
- type: separator
  bounds: [532, 48, 534, 480]
  refresh: "static"
  config:
    orientation: vertical
```
