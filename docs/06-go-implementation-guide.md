# Go Implementation Guide

## Recommended Libraries

### SPI + GPIO: periph.io (Primary Recommendation)

periph.io is pure Go, actively maintained (host v3.8.5, April 2025), and
confirmed working on Pi Zero 2 W since v3.6.4.

```text
go get periph.io/x/conn/v3
go get periph.io/x/host/v3
```

**Why periph.io:**

- Pure Go — no CGO, trivial cross-compilation
- `spitest.Record` / `spitest.Playback` for testing without hardware
- `gpiotest` for fake GPIO pins in unit tests
- Explicit Pi Zero 2 W support in v3
- Clean API with BCM pin naming

**SPI usage:**

```go
import (
    "periph.io/x/host/v3"
    "periph.io/x/conn/v3/physic"
    "periph.io/x/conn/v3/spi"
    "periph.io/x/conn/v3/spi/spireg"
)

host.Init()
port, _ := spireg.Open("SPI0.0")
defer port.Close()
conn, _ := port.Connect(4*physic.MegaHertz, spi.Mode0, 8)

// Full-duplex transfer
write := []byte{0x10, 0x00}
read := make([]byte, len(write))
conn.Tx(write, read)
```

**GPIO usage:**

```go
import "periph.io/x/conn/v3/gpio/gpioreg"

// Output pin
rstPin := gpioreg.ByName("GPIO17")
rstPin.Out(gpio.High)
rstPin.Out(gpio.Low)

// Input pin
busyPin := gpioreg.ByName("GPIO24")
busyPin.In(gpio.PullUp, gpio.NoEdge)
val := busyPin.Read() // gpio.High or gpio.Low
```

### Alternative: go-gpiocdev + Direct spidev

If you prefer the modern Linux GPIO character device API:

```text
go get github.com/warthog618/go-gpiocdev
```

This uses `/dev/gpiochip0` (the kernel's character device interface) instead of
memory mapping. Pair with direct `/dev/spidev0.0` ioctl access for SPI.

**GPIO via go-gpiocdev:**

```go
import "github.com/warthog618/go-gpiocdev"

dc, _ := gpiocdev.RequestLine("gpiochip0", 25, gpiocdev.AsOutput(0))
defer dc.Close()
dc.SetValue(1) // HIGH
dc.SetValue(0) // LOW

busy, _ := gpiocdev.RequestLine("gpiochip0", 24,
    gpiocdev.AsInput, gpiocdev.WithPullUp)
defer busy.Close()
val, _ := busy.Value() // 0 or 1
```

### Libraries to Avoid

- **go-rpio** (`github.com/stianeikeland/go-rpio`): Unmaintained since Dec
  2021, uses fragile memory mapping, no explicit Pi Zero 2 W support.
- **golang.org/x/exp/io/spi**: Deprecated, explicitly recommends periph.io.

## Existing Go E-Paper Projects

None are complete or maintained for the 7.5" V2, but useful for reference:

<!-- markdownlint-disable MD013 -->
| Project | Display | Notes |
|---------|---------|-------|
| `periph.io/x/devices/v3/epd` | 1.54", 2.13" | Best architecture reference, part of periph.io |
| `periph.io/x/devices/v3/waveshare2in13v2` | 2.13" v2 | Dedicated driver, good init sequence reference |
| `nii236/go-waveshare-epaper` | 7.5" | Dead (2 commits), but targets same display family |
| `robertely/EDP-7.5_V2` | 7.5" V2 | Dead (3 commits, 2022), closest to our target |
<!-- markdownlint-enable MD013 -->

## Core Architecture

### Layer Diagram

```text
┌─────────────────────────────────────────────────────┐
│                   Application                        │
│          (widgets, compositor, scheduler)             │
├─────────────────────────────────────────────────────┤
│                  Display (EPD)                        │
│       Generic driver, parameterized by profile       │
├─────────────────────────────────────────────────────┤
│                DisplayProfile                        │
│    Resolution, init sequences, capabilities, LUTs    │
│    (data only — selected via app config)             │
├────────────┬──────────┬──────────┬──────────────────┤
│  spiHW     │  mockHW  │  imageHW │  webPreview      │
│ (real Pi)  │ (tests)  │ (PNG)    │ (browser)        │
└────────────┴──────────┴──────────┴──────────────────┘
```

### Display Profile (Data-Driven, Not Code-Driven)

Each supported display is a `DisplayProfile` — a struct of configuration data,
not a separate driver. The generic `EPD` struct interprets the profile to drive
any supported display. Adding a new display means adding a new profile (data),
not new logic.

```go
// Command is a single SPI command + its data payload.
type Command struct {
    Reg  byte   // Command register (sent with DC=LOW)
    Data []byte // Parameter bytes (sent with DC=HIGH), nil if none
}

// ColorDepth describes how pixels are packed into the buffer.
type ColorDepth int

const (
    BW     ColorDepth = iota // 1 bit per pixel, 8px per byte
    Gray4                    // 2 bits per pixel, 4px per byte
    Color7                   // 4 bits per pixel, 2px per byte
)

// Capabilities flags for what a display supports.
type Capabilities struct {
    FastRefresh    bool
    PartialRefresh bool
    Grayscale      bool
}

// DisplayProfile contains everything needed to drive a specific display.
// This is pure data — no methods, no logic, no interface implementations.
type DisplayProfile struct {
    Name   string // e.g. "waveshare_7in5_v2"
    Width  int
    Height int
    Color  ColorDepth

    Capabilities Capabilities

    // Init sequences per mode (nil = mode not supported)
    InitFull    []Command
    InitFast    []Command // nil if Capabilities.FastRefresh is false
    InitPartial []Command // nil if Capabilities.PartialRefresh is false
    Init4Gray   []Command // nil if Capabilities.Grayscale is false

    // Display data commands
    OldBufferCmd byte // 0x10 for most displays, 0x24 for some
    NewBufferCmd byte // 0x13 for most displays, 0x26 for some
    RefreshCmd   byte // 0x12 for all known displays

    // Partial refresh window command (0x90 for most)
    PartialWindowCmd byte
    PartialEnterCmd  byte // 0x91
    PartialVCOM      []byte

    // Sleep sequence
    SleepSequence []Command

    // Waveform LUT (nil if display has built-in LUTs)
    LUT []byte
}

// BufferSize returns the framebuffer size in bytes for this display.
func (p *DisplayProfile) BufferSize() int {
    switch p.Color {
    case Gray4:
        return p.Width * p.Height / 4
    case Color7:
        return p.Width * p.Height / 2
    default: // BW
        return p.Width * p.Height / 8
    }
}
```

### Built-In Profiles

Profiles are registered in a map. The app config selects one by name:

```go
// Built-in profiles. Only 7.5" V2 is implemented initially.
var Profiles = map[string]*DisplayProfile{
    "waveshare_7in5_v2": &Waveshare7in5V2,
    // Future:
    // "waveshare_4in2_v2":  &Waveshare4in2V2,
    // "waveshare_2in13_v4": &Waveshare2in13V4,
}

// Waveshare7in5V2 is the profile for the Waveshare 7.5" e-Paper V2.
var Waveshare7in5V2 = DisplayProfile{
    Name:   "waveshare_7in5_v2",
    Width:  800,
    Height: 480,
    Color:  BW,
    Capabilities: Capabilities{
        FastRefresh:    true,
        PartialRefresh: true,
        Grayscale:      true,
    },
    InitFull: []Command{
        {0x06, []byte{0x17, 0x17, 0x28, 0x17}}, // Booster soft start
        {0x01, []byte{0x07, 0x07, 0x28, 0x17}},  // Power setting
        {0x04, nil},                               // Power on (+ busy wait)
        {0x00, []byte{0x1F}},                      // Panel setting
        {0x61, []byte{0x03, 0x20, 0x01, 0xE0}},   // Resolution 800x480
        {0x15, []byte{0x00}},                      // Dual SPI off
        {0x50, []byte{0x10, 0x07}},                // VCOM interval
        {0x60, []byte{0x22}},                      // TCON setting
    },
    InitFast: []Command{
        {0x00, []byte{0x1F}},                      // Panel setting
        {0x50, []byte{0x10, 0x07}},                // VCOM interval
        {0x04, nil},                               // Power on (+ busy wait)
        {0x06, []byte{0x27, 0x27, 0x18, 0x17}},   // Booster
        {0xE0, []byte{0x02}},                      // Cascade setting
        {0xE5, []byte{0x5A}},                      // Force temperature
    },
    InitPartial: []Command{
        {0x00, []byte{0x1F}},                      // Panel setting
        {0x04, nil},                               // Power on (+ busy wait)
        {0xE0, []byte{0x02}},                      // Cascade setting
        {0xE5, []byte{0x6E}},                      // Force temperature
    },
    Init4Gray: []Command{
        {0x00, []byte{0x1F}},
        {0x50, []byte{0x10, 0x07}},
        {0x04, nil},
        {0x06, []byte{0x27, 0x27, 0x18, 0x17}},
        {0xE0, []byte{0x02}},
        {0xE5, []byte{0x5F}},
    },
    OldBufferCmd:     0x10,
    NewBufferCmd:     0x13,
    RefreshCmd:       0x12,
    PartialWindowCmd: 0x90,
    PartialEnterCmd:  0x91,
    PartialVCOM:      []byte{0xA9, 0x07},
    SleepSequence: []Command{
        {0x50, []byte{0xF7}},  // VCOM setting
        {0x02, nil},           // Power off (+ busy wait)
        {0x07, []byte{0xA5}},  // Deep sleep
    },
}
```

### App Configuration

The display profile is selected in the app's config file, not in code:

```yaml
# inkwell.yaml
display: waveshare_7in5_v2

# Future examples:
# display: waveshare_4in2_v2
# display: waveshare_2in13_v4
```

At startup:

```go
profile, ok := inkwell.Profiles[cfg.Display]
if !ok {
    log.Fatalf("unknown display profile: %s", cfg.Display)
}
epd := inkwell.NewEPD(hardware, profile)
```

### Hardware Interface

The transport layer — this is what gets swapped between real hardware, tests,
and preview:

```go
// Hardware is the low-level SPI/GPIO transport.
type Hardware interface {
    SendCommand(cmd byte) error
    SendData(data []byte) error
    ReadBusy() bool
    Reset() error
    Close() error
}
```

**Implementations:**

1. **`spiHardware`** — Real Pi: periph.io SPI + GPIO
2. **`mockHardware`** — Testing: records all commands sent
3. **`imageBackend`** — Writes PNG to disk on `Display()` calls
4. **`webPreview`** — Serves live preview in browser

### Generic EPD Driver

One driver handles all displays by interpreting the profile:

```go
type EPD struct {
    hw      Hardware
    profile *DisplayProfile
}

func NewEPD(hw Hardware, profile *DisplayProfile) *EPD {
    return &EPD{hw: hw, profile: profile}
}

func (d *EPD) Init(mode InitMode) error {
    if err := d.hw.Reset(); err != nil {
        return err
    }

    var seq []Command
    switch mode {
    case InitFull:
        seq = d.profile.InitFull
    case InitFast:
        seq = d.profile.InitFast
    case InitPartial:
        seq = d.profile.InitPartial
    case Init4Gray:
        seq = d.profile.Init4Gray
    }

    if seq == nil {
        return fmt.Errorf("display %s does not support mode %v",
            d.profile.Name, mode)
    }

    return d.execSequence(seq)
}

// execSequence sends a series of commands, waiting for busy after
// any command with nil data (power-on commands).
func (d *EPD) execSequence(seq []Command) error {
    for _, cmd := range seq {
        if err := d.hw.SendCommand(cmd.Reg); err != nil {
            return err
        }
        if cmd.Data != nil {
            if err := d.hw.SendData(cmd.Data); err != nil {
                return err
            }
        }
        // Commands with no data payload (like Power On 0x04) require
        // a busy wait afterward
        if cmd.Data == nil {
            d.hw.ReadBusy()
        }
    }
    return nil
}

func (d *EPD) Display(buffer []byte) error {
    // Send old buffer (inverted)
    inverted := make([]byte, len(buffer))
    for i, b := range buffer {
        inverted[i] = ^b
    }
    d.hw.SendCommand(d.profile.OldBufferCmd)
    d.hw.SendData(inverted)

    // Send new buffer
    d.hw.SendCommand(d.profile.NewBufferCmd)
    d.hw.SendData(buffer)

    // Trigger refresh
    d.hw.SendCommand(d.profile.RefreshCmd)
    d.hw.ReadBusy()
    return nil
}

// Width and Height come from the profile, not constants.
func (d *EPD) Width() int  { return d.profile.Width }
func (d *EPD) Height() int { return d.profile.Height }
```

### SPI Hardware Implementation

```go
func (h *spiHardware) SendCommand(cmd byte) error {
    h.dcPin.Out(gpio.Low)   // DC LOW = command mode
    h.csPin.Out(gpio.Low)   // Select device
    _, err := h.spi.Write([]byte{cmd})
    h.csPin.Out(gpio.High)  // Deselect
    return err
}

func (h *spiHardware) SendData(data []byte) error {
    h.dcPin.Out(gpio.High)  // DC HIGH = data mode
    h.csPin.Out(gpio.Low)
    _, err := h.spi.Write(data)
    h.csPin.Out(gpio.High)
    return err
}
```

### Buffer Handling

Buffer packing is driven by the profile's `ColorDepth` and resolution:

```go
// PackImage converts a Go image to the display's packed buffer format.
// The packing strategy is determined by the profile's ColorDepth.
func PackImage(profile *DisplayProfile, img image.Image) []byte {
    switch profile.Color {
    case Gray4:
        return packGray4(profile, img)
    case Color7:
        return packColor7(profile, img)
    default:
        return packBW(profile, img)
    }
}

func packBW(p *DisplayProfile, img image.Image) []byte {
    buf := make([]byte, p.BufferSize())
    for y := 0; y < p.Height; y++ {
        for x := 0; x < p.Width; x++ {
            byteIdx := (y*p.Width + x) / 8
            bitIdx := 7 - uint(x%8) // MSB first
            r, g, b, _ := img.At(x, y).RGBA()
            if (r+g+b)/3 < 0x8000 {
                buf[byteIdx] |= 1 << bitIdx
            }
        }
    }
    return buf
}
```

### InitMode Constants

```go
type InitMode int

const (
    InitFull    InitMode = iota // Standard full refresh
    InitFast                    // Fast full refresh (if supported)
    InitPartial                 // Partial refresh mode (if supported)
    Init4Gray                   // 4-level grayscale (if supported)
)
```

### Widget/Component System

Widgets receive the profile's resolution, so they adapt to any display:

```go
// Widget renders into a rectangular region of the display.
type Widget interface {
    // Bounds returns the widget's position and size on the display.
    Bounds() image.Rectangle
    // Render draws the widget content into the provided image.
    Render(frame *image.Paletted) error
}
```

Widgets render into sub-images of the full frame, then the compositor calls
`PackImage(profile, frame)` and sends it to the display. This enables
per-widget golden file testing.

## Cross-Compilation

### No CGO (recommended — periph.io is pure Go)

```bash
# For Pi Zero 2 W running 64-bit OS
GOOS=linux GOARCH=arm64 go build -o go-e-ink ./cmd/...

# For Pi Zero 2 W running 32-bit OS
GOOS=linux GOARCH=arm GOARM=7 go build -o go-e-ink ./cmd/...
```

That is the entire build step. No cross-compiler toolchain needed.

### If CGO is Required

Install a cross-compiler:

```bash
brew tap messense/macos-cross-toolchains
brew install aarch64-unknown-linux-gnu
```

Build:

```bash
CGO_ENABLED=1 \
CC=aarch64-unknown-linux-gnu-gcc \
GOOS=linux GOARCH=arm64 \
go build -o go-e-ink ./cmd/...
```

Avoid CGO if possible — it complicates CI and deployment.

## Deployment

The binary can be deployed via:

1. `scp go-e-ink pi@<ip>:~/` — simple copy
2. Systemd unit file for auto-start on boot
3. Future: Tailscale + tsnet for remote updates and web UI

### Systemd Service Example

```ini
[Unit]
Description=E-Ink Dashboard
After=network.target

[Service]
ExecStart=/home/pi/go-e-ink
Restart=on-failure
User=pi
Group=gpio

[Install]
WantedBy=multi-user.target
```

The `gpio` group grants access to `/dev/gpiochip0` and `/dev/spidev0.0`
without root.
