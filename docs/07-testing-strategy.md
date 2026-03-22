# Testing Strategy

The entire display pipeline reduces to: **render an image -> pack it into
48,000 bytes -> send those bytes over SPI with the right command sequence.**

If we can verify the bytes are correct on any machine, hardware integration
becomes a deployment concern, not a development concern.

## Architecture for Testability

### The Two-Interface Pattern

```go
// Hardware — low-level transport, swappable per environment
type Hardware interface {
    SendCommand(cmd byte) error
    SendData(data []byte) error
    ReadBusy() bool
    Reset() error
    Close() error
}

// Display — high-level operations, built on Hardware
type Display interface {
    Init(mode InitMode) error
    Clear() error
    Display(buffer []byte) error
    DisplayPartial(buffer []byte, x, y, w, h int) error
    Sleep() error
    Close() error
}
```

### Four Backend Implementations

```text
┌─────────────────────────────────────────────────┐
│              Your Application Code              │
│         (widgets, compositor, scheduler)         │
├─────────────────────────────────────────────────┤
│               Display Interface                  │
├────────────┬──────────┬──────────┬──────────────┤
│  spiHW     │  mockHW  │  imageHW │  webPreview  │
│ (real Pi)  │ (tests)  │ (PNG)    │ (browser)    │
└────────────┴──────────┴──────────┴──────────────┘
```

1. **`spiHardware`** — Production on Pi. Uses periph.io SPI + GPIO.
2. **`mockHardware`** — Unit tests. Records every command/data call. Assert
   exact sequences.
3. **`imageBackend`** — Writes PNG to disk on each `Display()` call. For
   manual visual inspection during development.
4. **`webPreview`** — Serves live preview in browser. Fastest feedback loop.

## 1. Golden File Testing (Primary Verification)

Compare rendered buffers against known-good reference files.

### Raw Buffer Comparison

The authoritative test: compare the exact 48,000-byte buffer.

```go
var update = flag.Bool("update", false, "update golden files")

func TestClockWidget(t *testing.T) {
    frame := image.NewPaletted(
        image.Rect(0, 0, 800, 480),
        color.Palette{color.White, color.Black},
    )

    widget := NewClockWidget(fixedTime("14:30:00"))
    widget.Render(frame)

    buf := PackImage(frame)
    golden := filepath.Join("testdata", t.Name()+".bin")

    if *update {
        os.WriteFile(golden, buf, 0644)
        return
    }

    expected, _ := os.ReadFile(golden)
    if !bytes.Equal(buf, expected) {
        t.Errorf("buffer mismatch — run with -update to regenerate")
    }
}
```

### Visual Golden Files (For Code Review)

Store PNGs alongside the `.bin` files so reviewers can see what changed:

```go
func saveGoldenPNG(t *testing.T, buf []byte) {
    img := UnpackBuffer(buf) // 48k bytes -> image.Paletted
    golden := filepath.Join("testdata", t.Name()+".png")
    f, _ := os.Create(golden)
    defer f.Close()
    png.Encode(f, img)
}
```

### File Structure

```text
testdata/
├── TestClockWidget/14_30.bin       # Raw buffer (authoritative)
├── TestClockWidget/14_30.png       # Visual reference (for humans)
├── TestCalendarWidget/3_events.bin
├── TestCalendarWidget/3_events.png
├── TestFullDashboard/normal.bin
└── TestFullDashboard/normal.png
```

Update workflow:

```bash
# Regenerate all golden files after intentional changes
go test ./... -update

# Review the PNGs in a diff tool or git diff
git diff --stat testdata/
```

## 2. Command Sequence Testing

Verify the exact SPI commands sent during init, display, and sleep.

```go
func TestInitSequence(t *testing.T) {
    mock := &MockHardware{}
    epd := NewEPD(mock)
    epd.Init(InitFull)

    expected := []Call{
        {Type: "reset"},
        {Type: "cmd", Data: []byte{0x06}},
        {Type: "data", Data: []byte{0x17, 0x17, 0x28, 0x17}},
        {Type: "cmd", Data: []byte{0x01}},
        {Type: "data", Data: []byte{0x07, 0x07, 0x28, 0x17}},
        {Type: "cmd", Data: []byte{0x04}},
        {Type: "busy"},
        // ... etc
    }

    if diff := cmp.Diff(expected, mock.Calls); diff != "" {
        t.Errorf("init sequence mismatch:\n%s", diff)
    }
}
```

This catches regressions in the command sequences without needing hardware.

### periph.io's Built-in Test Helpers

periph.io provides `spitest.Record` and `spitest.Playback`:

```go
import "periph.io/x/conn/v3/spi/spitest"

// Record a real SPI session on the Pi, save to file
// Then replay in tests on any machine
playback := &spitest.Playback{
    Playback: conntest.Playback{
        Ops: []conntest.IO{
            {W: []byte{0x06}, R: nil},
            {W: []byte{0x17, 0x17, 0x28, 0x17}, R: nil},
        },
    },
}
```

## 3. Widget/Component Testing

Each widget is tested in isolation with its own golden files.

```go
func TestCalendarWidget(t *testing.T) {
    tests := []struct {
        name   string
        events []CalendarEvent
    }{
        {"empty", nil},
        {"single_event", []CalendarEvent{{Title: "Standup", Time: "09:00"}}},
        {"three_events", threeEvents()},
        {"overflow", tenEvents()}, // More than fit in the widget
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            widget := NewCalendarWidget(tt.events)
            bounds := widget.Bounds()

            // Create an image sized to just this widget
            frame := image.NewPaletted(bounds, bwPalette)
            widget.Render(frame)

            assertGolden(t, frame)
        })
    }
}
```

Widgets never need to know about SPI. They just draw pixels.

## 4. Web Preview Server (Development Feedback Loop)

The fastest way to iterate: change code, see the result in your browser
instantly.

### How It Works

```text
┌──────────────┐     SSE: "new frame"     ┌───────────────┐
│  Go process  │ ────────────────────────► │    Browser     │
│              │                           │                │
│  /frame.png  │ ◄──── GET /frame.png ──── │  <img> reload  │
└──────────────┘                           └───────────────┘
```

### Endpoints

- **`GET /`** — HTML page with `<img>` and SSE listener JavaScript
- **`GET /frame.png`** — Current display buffer rendered as PNG
- **`GET /events`** — SSE stream, pushes `"refresh"` when frame updates
- **`GET /frame.png?scale=3`** — Scaled up for high-DPI screens

### SSE Auto-Refresh

The server-sent events pattern is simpler than WebSocket for one-way
notifications:

```go
func (s *PreviewServer) handleEvents(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    flusher := w.(http.Flusher)

    ch := s.Subscribe()
    defer s.Unsubscribe(ch)

    for {
        select {
        case <-ch:
            fmt.Fprintf(w, "data: refresh\n\n")
            flusher.Flush()
        case <-r.Context().Done():
            return
        }
    }
}
```

Client-side JavaScript (~10 lines):

```javascript
const source = new EventSource('/events');
const img = document.getElementById('display');
source.onmessage = () => {
    img.src = '/frame.png?t=' + Date.now();
};
```

### Simulating Partial Refresh

The web preview can visually indicate partial refresh regions:

- Overlay a highlighted rectangle (colored border, semi-transparent tint) on
  the affected region
- Fade it after a short delay
- This makes it immediately obvious which region was updated vs. full refresh

### Build Tag Separation

```go
//go:build !hardware

// preview.go — web preview backend, included in dev builds
```

```go
//go:build hardware

// spi.go — real hardware backend, only compiled for Pi
```

## 5. Integration Testing on the Pi

For the rare cases where you need real hardware:

```go
//go:build hardware

func TestRealDisplay(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping hardware test in short mode")
    }

    epd := NewEPD(NewSPIHardware())
    defer epd.Close()

    epd.Init(InitFull)
    epd.Clear()

    // Display a test pattern
    buf := makeCheckerboard()
    epd.Display(buf)

    // Visual confirmation required
    t.Log("Check display shows checkerboard pattern")
    time.Sleep(5 * time.Second)

    epd.Sleep()
}
```

Run on the Pi:

```bash
go test ./... -tags hardware -run TestRealDisplay
```

## 6. CI Pipeline (GitHub Actions)

Everything runs without hardware:

```yaml
name: CI
on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.24'
      - run: go test ./...
      - run: go vet ./...

  cross-compile:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.24'
      - run: GOOS=linux GOARCH=arm64 go build ./...
```

### What Runs Where

<!-- markdownlint-disable MD013 -->
| Test Type | Dev Machine (macOS) | CI (ubuntu x86_64) | Pi (linux arm64) |
|-----------|--------------------|--------------------|------------------|
| Golden file / buffer tests | Yes | Yes | Yes |
| Command sequence tests | Yes | Yes | Yes |
| Widget rendering tests | Yes | Yes | Yes |
| Web preview | Yes (interactive) | No | No |
| Real hardware tests | No | No | Yes (`-tags hardware`) |
| Cross-compile check | Yes | Yes | N/A (native) |
<!-- markdownlint-enable MD013 -->

## 7. Development Workflow

### Daily Loop

```text
1. Write/modify widget code
2. Run `go test ./...` — golden file comparison
3. If golden files changed intentionally: `go test ./... -update`
4. Open web preview: `go run ./cmd/preview/`
5. See result in browser at localhost:8080
6. Iterate on rendering
7. When satisfied: cross-compile and scp to Pi
8. Run hardware integration test on Pi
```

### First-Time Setup on Pi

```bash
# One-time: verify SPI and GPIO work
scp go-e-ink pi@<ip>:~/
ssh pi@<ip>
./go-e-ink -test-hardware   # Quick init -> clear -> sleep cycle
```

After this works once, all further development happens on your dev machine
with the web preview and golden file tests.

## 8. Font Rendering

Use Go's built-in font rasterization (pure Go, no CGO):

```go
import (
    "golang.org/x/image/font"
    "golang.org/x/image/font/opentype"
    "golang.org/x/image/math/fixed"
)
```

Load a TTF/OTF font, create a `font.Face`, use `font.Drawer` to render text
into an `image.Paletted`. All platform-independent and testable via golden
files.
