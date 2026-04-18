# Building Dashboards

This guide covers how to create custom dashboards for your e-ink display
using Inkwell's widget system, and how to preview them in real time from
your browser.

## Before You Begin

Make sure you can run Inkwell locally:

```bash
go run ./cmd/inkwell
```

Then open <http://localhost:8080> in your browser. You should see a dark
page with the display preview. Every time the display updates, the
browser refreshes automatically — no need to hit reload.

## How Dashboards Work

Inkwell organizes what appears on your display using three concepts:

- **Widget** — a Go type that renders content into a rectangular
  region of the display (e.g., a clock, weather summary, calendar).
  Widgets are code.
- **Screen** — a named layout of widgets with positions. Each screen
  defines what the display looks like at a given moment. Screens are
  defined in YAML.
- **Dashboard** — a collection of one or more screens. A dashboard
  with a single screen shows that screen forever. A dashboard with
  multiple screens rotates through them on a configurable interval.
  Dashboards are defined in YAML.

```text
Dashboard
  ├── Screen "home"
  │     ├── clock widget    @ [650, 0, 800, 50]
  │     ├── weather widget  @ [0, 50, 550, 480]
  │     └── calendar widget @ [550, 50, 800, 480]
  │
  └── Screen "detail"
        └── clock widget    @ [300, 210, 500, 270]
```

On each render tick, the dashboard picks the current screen, the
compositor draws that screen's widgets into a frame, and the frame
is sent to the display (hardware or web preview).

### Screen rotation

If you define multiple screens and set `rotate_interval`, the
dashboard automatically cycles through them:

```yaml
dashboard:
  rotate_interval: 5m
  screens:
    - name: home
      widgets: [...]
    - name: detail
      widgets: [...]
```

With a single screen, omit `rotate_interval` — or don't set it —
and the dashboard stays on that screen permanently.

### What a single screen looks like

```text
+-- Screen "home" ----------------------------+
|                                              |
|  +-------------------+ +------------------+  |
|  | Widget A          | | Widget B         |  |
|  | (e.g. weather)    | | (e.g. clock)     |  |
|  +-------------------+ +------------------+  |
|                                              |
|  +----------------------------------------+  |
|  | Widget C (e.g. calendar)               |  |
|  |                                        |  |
|  +----------------------------------------+  |
|                                              |
+----------------------------------------------+
```

## Creating a Widget

A widget is any Go type that implements two methods:

```go
type Widget interface {
    Bounds() image.Rectangle
    Render(frame *image.Paletted) error
}
```

- **`Bounds()`** — returns the rectangle where this widget lives on the
  display.
- **`Render(frame)`** — draws the widget's content onto the frame.

### Your first widget: a static label

Create a new file `internal/inkwell/widgets/label/label.go`:

```go
package label

import (
    "image"
    "image/color"
    "image/draw"

    "golang.org/x/image/font"
    "golang.org/x/image/font/basicfont"
    "golang.org/x/image/math/fixed"

    "github.com/grantlucas/inkwell/internal/inkwell/widget"
)

// Compile-time interface check.
var _ widget.Widget = (*Widget)(nil)

// Widget displays a line of text.
type Widget struct {
    bounds image.Rectangle
    text   string
}

func New(bounds image.Rectangle, text string) *Widget {
    return &Widget{bounds: bounds, text: text}
}

func (w *Widget) Bounds() image.Rectangle { return w.bounds }

func (w *Widget) Render(frame *image.Paletted) error {
    // White background
    draw.Draw(frame, w.bounds,
        image.NewUniform(color.White), image.Point{}, draw.Src)

    // Draw text
    face := basicfont.Face7x13
    d := &font.Drawer{
        Dst:  frame,
        Src:  image.NewUniform(color.Black),
        Face: face,
        Dot:  fixed.P(w.bounds.Min.X+4, w.bounds.Min.Y+face.Ascent+2),
    }
    d.DrawString(w.text)
    return nil
}
```

### Registering with the widget factory

Each widget package exports a `Factory` function so it can be
instantiated from YAML config. The factory receives bounds, a config
map, and injectable dependencies:

<!-- markdownlint-disable MD013 -->

```go
func Factory(bounds image.Rectangle, config map[string]any, deps widget.Deps) (widget.Widget, error) {
    text, _ := config["text"].(string)
    return New(bounds, text), nil
}
```

<!-- markdownlint-enable MD013 -->

Register it inside the `NewDefaultRegistry()` function in
`internal/inkwell/widgets/registry.go` — this is the registry that
`NewApp` uses by default:

```go
r.Register("label", label.Factory)
```

For tests or embedding, you can pass a custom registry via
`WithRegistry(...)` when constructing the app.

## Configuring Dashboards in YAML

Screens and dashboards are defined in `inkwell.yaml`. You don't need
to write Go code to arrange widgets — just edit the config and
restart.

### Example config

```yaml
dashboard:
  rotate_interval: 5m  # optional, omit for single-screen
  screens:
    - name: main
      widgets:
        - type: clock
          bounds: [650, 0, 800, 50]
          config:
            format: "15:04"
        - type: label
          bounds: [0, 0, 650, 50]
          config:
            text: "My Dashboard"
    - name: detail
      widgets:
        - type: clock
          bounds: [0, 0, 200, 50]
```

### Bounds format

`bounds` is `[x0, y0, x1, y1]` matching Go's `image.Rect()`. The
origin `(0, 0)` is the top-left corner of the display.

### Planning your layout

Sketch your layout on paper or in comments first. Here's an example
three-panel dashboard:

```text
(0,0)                                      (800,0)
  +-----------------------------+----------+
  |  Title Bar                  |  Clock   |  0-50
  +-----------------------------+----------+
  |                             |          |
  |  Main Content               | Sidebar  |
  |  (0,50) -> (550,480)        |          |  50-480
  |                             |          |
  +-----------------------------+----------+
(0,480)                       (550)      (800,480)
```

Translated to YAML:

```yaml
dashboard:
  screens:
    - name: main
      widgets:
        - type: label
          bounds: [0, 0, 650, 50]
          config:
            text: "My Dashboard"
        - type: clock
          bounds: [650, 0, 800, 50]
          config:
            format: "15:04"
        - type: weather
          bounds: [0, 50, 550, 480]
          config:
            location: "Toronto, CA"
        - type: calendar
          bounds: [550, 50, 800, 480]
```

### Layout tips

- **Avoid overlapping regions.** Widgets draw in order, so a later
  widget will paint over an earlier one if they share pixels.
- **The frame starts white.** You don't need to clear your region
  unless you want a non-white background.
- **Stay within 800×480.** Anything outside is ignored by the display
  but wastes render time.

## Drawing Basics

The frame is a Go `image.Paletted` with a two-color palette (white at
index 0, black at index 1). You can use any standard Go image
operations.

### Filled rectangles

```go
// Draw a 1px horizontal line across the full width at y=50
draw.Draw(frame, image.Rect(0, 50, 800, 51),
    image.NewUniform(color.Black), image.Point{}, draw.Src)
```

### Text with the built-in font

The bundled `basicfont.Face7x13` is a 7×13 pixel monospace font. It
works well for small labels and data but is tiny on the full display.

```go
face := basicfont.Face7x13
d := &font.Drawer{
    Dst:  frame,
    Src:  image.NewUniform(color.Black),
    Face: face,
    Dot:  fixed.P(x, y),  // baseline position
}
d.DrawString("Hello")
```

### Larger text with custom fonts

For headings or prominent information, load a TrueType font:

```go
import "golang.org/x/image/font/opentype"

ttFont, err := opentype.Parse(myEmbeddedTTFBytes)
largeFace, err := opentype.NewFace(ttFont, &opentype.FaceOptions{
    Size: 48,
    DPI:  72,
})
```

Then use `largeFace` in a `font.Drawer` the same way as above.

## Making Widgets Testable

Inkwell widgets are designed to be testable without running the full
application. The key pattern is **dependency injection via functions**.

Look at the built-in clock widget — instead of calling `time.Now()`
directly, it accepts a `now func() time.Time` parameter:

```go
// In production
w := clock.New(bounds, time.Now, "15:04")

// In tests — deterministic, reproducible
fixed := time.Date(2024, 1, 1, 14, 30, 0, 0, time.UTC)
w := clock.New(bounds, func() time.Time { return fixed }, "15:04")
```

Apply this pattern to any widget that depends on external data (API
calls, system time, file reads, etc.).

### Writing a basic widget test

```go
func TestWidget_Render(t *testing.T) {
    w := label.New(image.Rect(0, 0, 200, 30), "Test")

    frame := image.NewPaletted(
        image.Rect(0, 0, 200, 30),
        color.Palette{color.White, color.Black},
    )

    if err := w.Render(frame); err != nil {
        t.Fatalf("Render: %v", err)
    }

    // Verify something was drawn
    for _, px := range frame.Pix {
        if px != 0 {
            return // found a black pixel — pass
        }
    }
    t.Error("expected at least one black pixel")
}
```

### Golden file tests for visual regression

Golden file tests capture the exact rendered output and compare it on
future runs. If the output changes, the test fails — catching
unintended visual regressions.

```go
func TestWidget_Golden(t *testing.T) {
    w := label.New(image.Rect(0, 0, 200, 30), "Test")
    frame := image.NewPaletted(
        image.Rect(0, 0, 200, 30),
        color.Palette{color.White, color.Black},
    )
    _ = w.Render(frame)

    testutil.AssertGoldenPNG(t, frame)
}
```

The first time you run this (or after changing the widget's output):

```bash
go test ./internal/inkwell/widgets/label -run TestWidget_Golden -update
```

This saves the rendered frame as a PNG in the widget's `testdata/` directory.
Commit the golden file to git so that future test runs compare against
it. When reviewing PRs, updated golden PNGs show you exactly what
changed visually.

## Using the Web Preview

### Configuration

The web preview is the default backend. Your `inkwell.yaml` should have:

```yaml
display: waveshare_7in5_v2
backend: preview
preview:
  port: 8080
```

### What you see in the browser

When you visit <http://localhost:8080>, you get a minimal dark-themed
page displaying the current frame at 2× scale. The page uses
Server-Sent Events to auto-refresh whenever the display updates.

### Available URLs

<!-- markdownlint-disable MD013 -->
| URL                    | What it does                                                   |
|------------------------|----------------------------------------------------------------|
| `/`                    | Preview page with auto-refresh                                 |
| `/frame.png`           | Raw PNG of the current frame                                   |
| `/frame.png?scale=4`   | Upscaled PNG (1–10×) for pixel-level detail                    |
| `/events`              | SSE stream — browsers use this for auto-refresh                |
<!-- markdownlint-enable MD013 -->

You can open multiple browser tabs or windows — each gets its own SSE
connection and updates independently.

### Development workflow

1. Edit your widget code
2. Restart `go run ./cmd/inkwell`
3. The browser auto-refreshes with the new frame
4. Repeat until it looks right
5. Write/update golden file tests to lock in the visual output

## Putting It All Together

Here's the typical flow for building a new dashboard:

1. **Plan your screens** — decide how many screens you need and
   sketch the widget layout for each one on the 800×480 canvas
2. **Create the widgets** — implement `Bounds()` and `Render()` in
   self-contained subpackages under `internal/inkwell/widgets/<name>/`
3. **Write tests** — at minimum a render test and a golden file test
   per widget
4. **Register factories** — add each widget's `Factory` to
   `NewDefaultRegistry()` in `widgets/registry.go`
5. **Configure in YAML** — define your screens, widget placements,
   and rotation interval in `inkwell.yaml`
6. **Preview it** — run Inkwell and check the browser
7. **Iterate** — adjust bounds, config, or add screens without
   recompiling — just edit YAML and restart
8. **Verify coverage** — the project requires 100% statement coverage
9. **Commit** — golden PNGs are committed to git for visual diffing
