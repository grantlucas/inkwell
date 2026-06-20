# Fuzzy Clock Widget

Renders the current time as a natural-language English phrase ("About half
past eight", "Quarter to nine") rather than precise digits. Registered
under the dashboard `type: fuzzy_clock`.

The phrase is a pure, deterministic function of the time and the configured
options: the same minute always produces the same string. Because it only
changes meaningfully every ~5 minutes, it is the prototypical "low-flash"
widget — pair it with a slow render cadence (e.g. `refresh: "5m"`) and the
panel stays quiet while the time stays glanceable.

The text is drawn in IBM Plex Mono **Bold 16pt** as solid black on a white
background, so it survives both the `bw` threshold and the `gray4`
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
<!-- markdownlint-enable MD013 -->

A wrong type for any key, an unsupported `language`, an invalid `style`, or an
`align` value outside the three accepted strings is a configuration error.

`align` is useful for corner placements: a centered phrase shifts horizontally
as its length changes minute-to-minute, so pinning it to an edge keeps a fixed
anchor (e.g. `align: right` in the top-right of the panel).

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
```
