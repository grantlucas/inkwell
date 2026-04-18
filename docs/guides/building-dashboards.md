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

A dashboard is a collection of **widgets** arranged on an 800×480 pixel
canvas (matching the Waveshare 7.5" e-ink display). Each widget owns a
rectangular region of the screen and draws its own content.

The **compositor** walks through the widget list in order and asks each
one to render into the shared frame. The result is packed into the
display's binary format and sent to either the e-ink hardware or the
web preview, depending on your configuration.

```text
+-- Your Dashboard ----------------------------+
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

Create a new file `internal/inkwell/label_widget.go`:

```go
package inkwell

import (
    "image"
    "image/color"
    "image/draw"

    "golang.org/x/image/font"
    "golang.org/x/image/font/basicfont"
    "golang.org/x/image/math/fixed"
)

// LabelWidget displays a line of text.
type LabelWidget struct {
    bounds image.Rectangle
    text   string
}

func NewLabelWidget(bounds image.Rectangle, text string) *LabelWidget {
    return &LabelWidget{bounds: bounds, text: text}
}

func (w *LabelWidget) Bounds() image.Rectangle { return w.bounds }

func (w *LabelWidget) Render(frame *image.Paletted) error {
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

### Adding it to the compositor

Widgets are registered with the compositor, which renders them in order:

```go
comp := NewCompositor(profile)
comp.AddWidget(NewLabelWidget(image.Rect(0, 0, 800, 30), "Hello from Inkwell"))
comp.AddWidget(NewClockWidget(image.Rect(650, 0, 800, 30), time.Now))
```

## Laying Out a Dashboard

You position widgets by specifying pixel coordinates with
`image.Rect(left, top, right, bottom)`. The origin `(0, 0)` is the
top-left corner of the display.

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

Translated to code:

```go
// Title bar — full width, 50px tall
comp.AddWidget(NewLabelWidget(image.Rect(0, 0, 650, 50), "My Dashboard"))

// Clock — top right
comp.AddWidget(NewClockWidget(image.Rect(650, 0, 800, 50), time.Now))

// Main content area
comp.AddWidget(NewWeatherWidget(image.Rect(0, 50, 550, 480), fetchTemp))

// Sidebar
comp.AddWidget(NewCalendarWidget(image.Rect(550, 50, 800, 480), fetchEvents))
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

Look at the built-in `ClockWidget` — instead of calling `time.Now()`
directly, it accepts a `now func() time.Time` parameter:

```go
// In production
w := NewClockWidget(bounds, time.Now)

// In tests — deterministic, reproducible
fixed := time.Date(2024, 1, 1, 14, 30, 0, 0, time.UTC)
w := NewClockWidget(bounds, func() time.Time { return fixed })
```

Apply this pattern to any widget that depends on external data (API
calls, system time, file reads, etc.).

### Writing a basic widget test

```go
func TestLabelWidget_Render(t *testing.T) {
    w := NewLabelWidget(image.Rect(0, 0, 200, 30), "Test")

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
func TestLabelWidget_Golden(t *testing.T) {
    w := NewLabelWidget(image.Rect(0, 0, 200, 30), "Test")
    frame := image.NewPaletted(
        image.Rect(0, 0, 200, 30),
        color.Palette{color.White, color.Black},
    )
    _ = w.Render(frame)

    AssertGoldenPNG(t, frame)
}
```

The first time you run this (or after changing the widget's output):

```bash
go test ./internal/inkwell -run TestLabelWidget_Golden -update
```

This saves the rendered frame as a PNG in `internal/inkwell/testdata/`.
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

Here's the typical flow for building a new dashboard component:

1. **Plan the layout** — sketch where each widget goes on the 800×480
   canvas
2. **Create the widget** — implement `Bounds()` and `Render()` in a new
   file under `internal/inkwell/`
3. **Write tests** — at minimum a render test and a golden file test
4. **Wire it up** — add the widget to the compositor
5. **Preview it** — run Inkwell and check the browser
6. **Iterate** — adjust coordinates, font sizes, and content until it
   looks good
7. **Verify coverage** — the project requires 100% statement coverage
8. **Commit** — golden PNGs are committed to git for visual diffing
