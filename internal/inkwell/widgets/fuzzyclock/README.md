# Fuzzy Clock Widget

Renders the current time as a natural-language English phrase ("About half
past eight", "Quarter to nine") rather than precise digits. Registered
under the dashboard `type: fuzzy_clock`.

The phrase is a pure, deterministic function of the time and the configured
options: the same minute always produces the same string. Because it only
changes meaningfully every ~5 minutes, it is the prototypical "low-flash"
widget — pair it with a slow render cadence (e.g. `refresh: "5m"`) and the
panel stays quiet while the time stays glanceable.

The text is drawn in the bold 20 px Tamzen tier as solid black on a white
background, through the same `drawkit` text helpers the other screens use, so
it is a 1-bit mask that survives both the `bw` threshold and the `gray4`
quantization cleanly (see the rendering rules in the repository
[`CLAUDE.md`](../../../../CLAUDE.md)).

## Configuration

Top-level keys (`type`, `bounds`, `refresh`) are required by every widget;
`refresh` is the render cadence (a duration `>= 1m`, or `"static"`). A fuzzy
clock typically uses `refresh: "5m"`.

The widget-specific keys live under `config:`.

<!-- markdownlint-disable MD013 -->
| Key                               | Type   | Default      | Description                                                                                              |
|-----------------------------------|--------|--------------|----------------------------------------------------------------------------------------------------------|
| `style`                           | string | `"sentence"` | Letter casing of the phrase: `"sentence"` ("About half past eight"), `"title"`, or `"lower"`.            |
| `use_words_for_noon_and_midnight` | bool   | `true`       | Substitute "noon"/"midnight" for "twelve" in 12-hour mode. Ignored in 24-hour mode.                     |
| `use_24_hour`                     | bool   | `false`      | Spell the hour as 0..23 instead of 1..12.                                                                |
| `language`                        | string | `"en"`       | Only `"en"` is supported today; the key is a forward-looking localization hook. Any other value errors. |
| `align`                           | string | `"center"`   | Horizontal alignment within `bounds`: `"center"`, `"left"`, or `"right"`. Left/right inset 4px.          |
| `scale`                           | int    | `1`          | Whole-number size multiplier, at least 1. `1` is the original size. See [Scale](#scale).                 |
<!-- markdownlint-enable MD013 -->

A wrong type for any key, an unsupported `language`, an invalid `style`, an
`align` value outside the three accepted strings, or a `scale` that is not an
integer of at least 1 is a configuration error.

`align` is useful for corner placements: a centered phrase shifts horizontally
as its length changes minute-to-minute, so pinning it to an edge keeps a fixed
anchor (e.g. `align: right` in the top-right of the panel).

## Scale

`scale` blows the same bitmap glyphs up by a whole number, so a large clock is
still a 1-bit mask at every size. Above 1x the strokes are also thickened
(2 px at 3x and up, 1 px at 2x) so a large phrase keeps display weight rather
than the stroke weight of body text. The other screens draw at 2x and 3x.

The scale is something you set, never something derived from the bounds.
Auto-fitting would resize the clock whenever the phrase length changed, and on
this panel every change is a flash.

Because the phrase changes length through the day, the widget checks at
configuration time that the **longest** phrase for your `style` and hour format
fits `bounds` at the scale you chose, and refuses to load otherwise, so a screen
never clips its clock at 8:25 some later afternoon. The room needed is the
phrase's advance (10 px per glyph per scale step), plus the stroke thickening on
each side, plus the 4px inset for `left`/`right`, and the line height (20 px per
scale step plus thickening) vertically. Some reference points at `scale: 1`:

<!-- markdownlint-disable MD013 -->
| Configuration                                        | Longest phrase                      | Width |
|------------------------------------------------------|-------------------------------------|-------|
| 12-hour, noon and midnight words (default)           | "Just after half past midnight"     | 290   |
| 12-hour, `use_words_for_noon_and_midnight: false`    | "Just after half past eleven"       | 270   |
| `use_24_hour: true`                                  | "Just after half past twenty-three" | 330   |
<!-- markdownlint-enable MD013 -->

Multiply by the scale and add the thickening and inset: at `scale: 3` even the
shortest 12-hour case needs 814 px, wider than the 800 px panel, so a
full-width header band tops out at `scale: 2`.

## Example

```yaml
- type: fuzzy_clock
  bounds: [500, 0, 800, 50]
  refresh: "5m"
  config:
    style: sentence
    use_words_for_noon_and_midnight: true
    use_24_hour: false
    language: en
    align: right
    scale: 1
```
