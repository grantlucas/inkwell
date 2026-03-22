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

### Interface Design

Two layers of abstraction keep hardware concerns separated from display logic:

```go
// Hardware is the low-level SPI/GPIO transport.
// This is what gets swapped between real hardware, mock, and preview backends.
type Hardware interface {
    SendCommand(cmd byte) error
    SendData(data []byte) error
    ReadBusy() bool
    Reset() error
    Close() error
}

// Display is the high-level display operations.
// Built on top of a Hardware implementation.
type Display interface {
    Init(mode InitMode) error
    Clear() error
    Display(buffer []byte) error
    DisplayPartial(buffer []byte, x, y, w, h int) error
    Sleep() error
    Close() error
}
```

**Hardware implementations:**

1. **`spiHardware`** — Real Pi: periph.io SPI + GPIO
2. **`mockHardware`** — Testing: records all commands sent
3. **`previewHardware`** — Dev: renders to PNG / web preview

### InitMode Constants

```go
type InitMode int

const (
    InitFull    InitMode = iota // Standard full refresh (~4s)
    InitFast                    // Fast full refresh (~1.5s)
    InitPartial                 // Partial refresh mode (~0.4s)
    Init4Gray                   // 4-level grayscale mode
)
```

### Buffer Handling

The framebuffer is 48,000 bytes (800x480, 1 bit per pixel, 8 pixels per byte):

```go
const (
    Width      = 800
    Height     = 480
    BufferSize = Width * Height / 8 // 48,000 bytes
)

// PackImage converts a Go image to the display's packed buffer format.
// Bit = 1 means black, Bit = 0 means white (display convention).
// MSB of each byte is the leftmost pixel.
func PackImage(img image.Image) []byte {
    buf := make([]byte, BufferSize)
    for y := 0; y < Height; y++ {
        for x := 0; x < Width; x++ {
            byteIdx := (y*Width + x) / 8
            bitIdx := 7 - uint(x%8) // MSB first
            r, g, b, _ := img.At(x, y).RGBA()
            // Simple threshold: if dark, set bit (black)
            if (r+g+b)/3 < 0x8000 {
                buf[byteIdx] |= 1 << bitIdx
            }
        }
    }
    return buf
}
```

### Command Sequence (Port from Python)

The `send_command` / `send_data` pattern maps directly:

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

### Init Sequence (Standard Mode)

Direct translation of the Python `init()`:

```go
func (d *EPD) initFull() error {
    if err := d.hw.Reset(); err != nil {
        return err
    }

    d.hw.SendCommand(0x06) // Booster soft start
    d.hw.SendData([]byte{0x17, 0x17, 0x28, 0x17})

    d.hw.SendCommand(0x01) // Power setting
    d.hw.SendData([]byte{0x07, 0x07, 0x28, 0x17})

    d.hw.SendCommand(0x04) // Power on
    d.hw.ReadBusy()

    d.hw.SendCommand(0x00) // Panel setting
    d.hw.SendData([]byte{0x1F})

    d.hw.SendCommand(0x61) // Resolution: 800x480
    d.hw.SendData([]byte{0x03, 0x20, 0x01, 0xE0})

    d.hw.SendCommand(0x15) // Dual SPI off
    d.hw.SendData([]byte{0x00})

    d.hw.SendCommand(0x50) // VCOM interval
    d.hw.SendData([]byte{0x10, 0x07})

    d.hw.SendCommand(0x60) // TCON setting
    d.hw.SendData([]byte{0x22})

    return nil
}
```

### Widget/Component System

```go
// Widget renders into a rectangular region of the display.
type Widget interface {
    // Bounds returns the widget's position and size on the display.
    Bounds() image.Rectangle
    // Render draws the widget content into the provided image.
    // The image is the full display frame; use SubImage for the widget region.
    Render(frame *image.Paletted) error
}
```

Widgets render into sub-images of the full frame, then the compositor packs
the frame and sends it to the display. This enables per-widget golden file
testing.

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
