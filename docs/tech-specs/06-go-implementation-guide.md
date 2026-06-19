# Go Implementation Guide

This is the architecture reference for Inkwell's Go driver. It mirrors
what is actually in the tree under [`internal/inkwell/`][pkg] — when
something in this document and the code disagree, the code wins.
Re-read the linked source when you need exact signatures, error
handling, or option semantics.

[pkg]: ../../internal/inkwell/

## Layered Architecture

```text
┌─────────────────────────────────────────────────────┐
│             Application Entry Point                  │
│         cmd/inkwell/main.go (Config + App.Run)       │
├─────────────────────────────────────────────────────┤
│         Dashboard / Screen / Widget Registry         │
│        (multi-screen rotation, widget factories)     │
├─────────────────────────────────────────────────────┤
│            Compositor (paletted frame builder)       │
├─────────────────────────────────────────────────────┤
│                  EPD (generic driver)                │
│       Parameterized by DisplayProfile data only      │
├─────────────────────────────────────────────────────┤
│                DisplayProfile (data)                 │
│    Resolution, color depth, init sequences, LUTs     │
├────────────┬──────────┬──────────┬──────────────────┤
│  spiHW     │  mockHW  │  imageHW │  WebPreview      │
│ (real Pi)  │ (tests)  │ (PNG)    │ (HTTP + SSE)     │
└────────────┴──────────┴──────────┴──────────────────┘
```

Source-of-truth files:

<!-- markdownlint-disable MD013 -->
| Layer | File |
|-------|------|
| Application wiring + render loop | [`internal/inkwell/app.go`][app] |
| Dashboard + screen rotation | [`internal/inkwell/dashboard.go`][dashboard], [`internal/inkwell/screen.go`][screen] |
| Widget interface (+ optional `RefreshCadence`) + factory registry | [`internal/inkwell/widget/`][widget] |
| Built-in widget implementations | [`internal/inkwell/widgets/`][widgets] |
| Compositor (frame assembly) | [`internal/inkwell/compositor.go`][compositor] |
| Generic EPD driver | [`internal/inkwell/epd.go`][epd] |
| Display profile (data) | [`internal/inkwell/profile.go`][profile] |
| Buffer packing (BW dither + Gray4) | [`internal/inkwell/buffer.go`][buffer] |
| Hardware transport interface | [`internal/inkwell/hardware.go`][hardware] |
| SPI/GPIO backend (build-tagged) | [`internal/inkwell/spi_hardware.go`][spi] |
| Mock hardware (test recorder) | [`internal/inkwell/mock_hardware.go`][mock] |
| Image backend (PNG output) | [`internal/inkwell/image_backend.go`][img] |
| Web preview backend (HTTP + SSE) | [`internal/inkwell/web_preview.go`][web] |
| YAML config + defaults | [`internal/inkwell/config.go`][config] |
<!-- markdownlint-enable MD013 -->

[app]: ../../internal/inkwell/app.go
[dashboard]: ../../internal/inkwell/dashboard.go
[screen]: ../../internal/inkwell/screen.go
[widget]: ../../internal/inkwell/widget/
[widgets]: ../../internal/inkwell/widgets/
[compositor]: ../../internal/inkwell/compositor.go
[epd]: ../../internal/inkwell/epd.go
[profile]: ../../internal/inkwell/profile.go
[buffer]: ../../internal/inkwell/buffer.go
[hardware]: ../../internal/inkwell/hardware.go
[spi]: ../../internal/inkwell/spi_hardware.go
[mock]: ../../internal/inkwell/mock_hardware.go
[img]: ../../internal/inkwell/image_backend.go
[web]: ../../internal/inkwell/web_preview.go
[config]: ../../internal/inkwell/config.go

## Recommended Libraries

### SPI + GPIO: periph.io

The codebase uses [periph.io](https://periph.io) for SPI and GPIO. It
is pure Go (no CGO), supports the Pi Zero 2 W, and ships with test
helpers (`spitest`, `gpiotest`) for hardware-free unit tests.

Direct dependencies (see [`go.mod`](../../go.mod)):

```text
periph.io/x/conn/v3
periph.io/x/host/v3
```

`host/v3` registers the Linux SPI/GPIO drivers that
[`spiHardware.initRealHardware`][spi] calls into; without it the
process would have no `/dev/spidev0.0` opener and no resolver for
`GPIO17`/`GPIO18`/`GPIO24`/`GPIO25`.

### Libraries to Avoid

- **`go-rpio`** (`github.com/stianeikeland/go-rpio`): unmaintained
  since Dec 2021, uses fragile memory mapping, no explicit Pi Zero 2 W
  support.
- **`golang.org/x/exp/io/spi`**: deprecated, recommends periph.io.

## Data-Driven Display Profiles

Each supported display is a `DisplayProfile` — a struct of
configuration data, not a separate driver. The generic `EPD` interprets
the profile to drive any supported display. Adding a new display means
adding a new profile (data), not new logic.

See [`internal/inkwell/profile.go`][profile] for the live struct and
the `Waveshare7in5V2` profile. The key fields are:

- `Width`, `Height`, `Color` (`BW` / `Gray4` / `Color7`)
- `Capabilities` flags (`FastRefresh`, `PartialRefresh`, `Grayscale`)
- Init sequences per mode: `InitFull`, `InitFast`, `InitPartial`,
  `Init4Gray`
- Buffer / refresh commands: `OldBufferCmd`, `NewBufferCmd`, `RefreshCmd`
- Partial-refresh metadata: `PartialWindowCmd`, `PartialEnterCmd`,
  `PartialVCOM`
- `SleepSequence`
- Optional waveform `LUT`

`DisplayProfile.BufferSize()` returns the per-frame buffer size as a
function of `Color` and resolution (8 px/byte for BW, 4 px/byte for
Gray4, 2 px/byte for Color7).

`Profiles` is a name-keyed map; the active profile is selected by the
`display:` field in `inkwell.yaml`.

## Init Modes

```go
const (
    InitFull    InitMode = iota // Standard full refresh
    InitFast                    // Fast full refresh (if supported)
    InitPartial                 // Partial refresh mode (if supported)
    Init4Gray                   // 4-level grayscale (if supported)
)
```

`EPD.Init(mode)` performs a hardware reset and then walks the matching
init sequence from the profile. Commands with `Data: nil` (e.g.
`0x04 Power On`) trigger a `ReadBusy()` between commands; commands with
a non-nil `Data` are sent as command-then-data pairs.

The application currently only invokes `InitFull` from the run loop in
[`app.go`][app]; switching dynamically to `InitFast` or `InitPartial`
is a future optimisation (see open beads issues for `Gray4` and partial
refresh work).

## Hardware Interface

The transport layer is the only piece that gets swapped between real
hardware, tests, and preview:

```go
type Hardware interface {
    SendCommand(cmd byte) error
    SendData(data []byte) error
    ReadBusy() bool
    Reset() error
    Close() error
}
```

An optional `FrameSink` interface ([`hardware.go`][hardware]) lets
preview-style backends receive the *pre-pack source frame* as well, so
the browser can render either the device buffer (post-dither, 1-bit)
or the design-intent grayscale source.

Implementations:

1. **`spiHardware`** ([`spi_hardware.go`][spi]) — production on the Pi
   via periph.io. Build-tagged `//go:build hardware`. Uses `spi.Conn.Tx`
   plus the `DC` pin to encode command-vs-data; `RST`, `BUSY`, and
   `PWR` are managed via `gpio.PinIO` handles.
2. **`MockHardware`** ([`mock_hardware.go`][mock]) — appends every
   call to a `Calls []Call` slice and exposes `Commands()` /
   `DataCalls()` helpers for terse assertions.
3. **`ImageBackend`** ([`image_backend.go`][img]) — writes a PNG to the
   configured `output_dir` on every refresh cycle. Useful for headless
   smoke tests where you want a visual artifact.
4. **`WebPreview`** ([`web_preview.go`][web]) — serves the live frame
   over HTTP with an SSE stream for browser auto-refresh. Also
   implements `FrameSink` so the `?source=1` query parameter can show
   the smooth grayscale source.

## Generic EPD Driver

One driver handles all displays by interpreting the active profile.
See [`epd.go`][epd]. Notable behaviour:

- `EPD.Init(mode)` — reset, then walk the per-mode init sequence.
  Returns an error if the profile does not support `mode`.
- `EPD.Display(buf)` — validates `len(buf) == profile.BufferSize()`,
  sends the inverted old buffer (`OldBufferCmd`), then the new buffer
  (`NewBufferCmd`), then triggers refresh (`RefreshCmd`) and waits on
  `ReadBusy`.
- `EPD.DisplayPartial(buf, region)` — byte-aligns the region X
  coordinate down and width up, writes the partial window via
  `PartialWindowCmd`, then sends the partial buffer and refreshes.
  Errors out when `!profile.Capabilities.PartialRefresh`.
- `EPD.Clear()` — fills a `BufferSize`-byte buffer with `0xFF` and
  pushes it through `Display` (the inversion path handles polarity).
- `EPD.Sleep()` / `EPD.Close()` — execute the profile's
  `SleepSequence`; `Close` then closes the hardware.

## Buffer Packing

`PackImage(profile, img) ([]byte, error)` dispatches on
`profile.Color`. See [`buffer.go`][buffer]:

- **`packBW`** (BW path, active): luminance threshold against the
  4×4 Bayer ordered-dither matrix, MSB-first packing. Soft palette
  grays from the compositor survive as halftone stipple on the
  device — see [`docs/guides/hardware-grayscale.md`][grayscale-guide]
  for the design rules.
- **`packGray4`** (Init4Gray path): four luminance buckets
  (`white`/`light gray`/`dark gray`/`black`) packed 4 pixels per byte.
  Selected when `color_mode: gray4` is set in `inkwell.yaml`; the
  Init4Gray device wiring (plane-split write, `Init4Gray` `InitMode`
  selection at startup, WebPreview unpacker) is tracked under beads
  issues `inkwell-usv`, `inkwell-101`, `inkwell-0p7`, `inkwell-0yd`.
- **`Color7`** is reserved in the `ColorDepth` enum for future panels;
  there is no packer for it yet.

`PackImage` returns `(nil, error)` if a profile names an unsupported
color depth; callers (e.g. [`app.go`][app]'s render loop) treat that
as a fatal config error.

[grayscale-guide]: ../guides/hardware-grayscale.md

## Application Lifecycle

[`App`][app] wires the pieces together:

1. `NewApp(cfg, opts...)` resolves the `DisplayProfile` from
   `cfg.Display`, builds the hardware backend (via `createBackend`,
   keyed off `cfg.Backend`: `preview` / `image` / `spi`), constructs
   the widget registry and dependency map, builds the `Dashboard`
   from `cfg.Dashboard`, and instantiates the `EPD` + `Compositor`.
2. `App.Run(ctx)` calls `EPD.Init(InitFull)`, starts an HTTP server
   if the backend implements `HTTPServer` (currently only
   `WebPreview` does), and signals `Ready`. Then it enters a render
   loop on `cfg`-derived interval (default 60s, overridable via
   `WithInterval`): compose → optional `SetSourceFrame` → `PackImage`
   → `EPD.Display` → wait for tick or context cancellation.

`Run` returns the result of `EPD.Close()` when ctx is cancelled.

## Cross-Compile for the Pi

`make build-pi` (Makefile target):

```bash
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build ./...
```

That covers the default *without* SPI. To include the SPI backend
that actually drives the panel, add the `hardware` build tag — this
is what the [release pipeline](../../.goreleaser.yaml) does for the
published `linux-arm64` / `linux-armv7` / `linux-armv6` binaries.

```bash
# Includes the SPI backend
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 \
  go build -tags hardware -o inkwell ./cmd/inkwell
```

No CGO is required because periph.io is pure Go. `CGO_ENABLED=0`
prevents accidental cross-compilation drag from system libraries.

## Known Gaps

These pieces are designed but not fully wired in the current tree.
The execution plan in [`inkwell-execution-plan.md`](../../inkwell-execution-plan.md)
and the active beads workspace track them.

- **Gray4 device path.** Fully wired host-side: `packGray4` produces the
  2bpp buffer, `EPD.Display` splits it into two 1bpp planes (low bit →
  `0x10`, high bit → `0x13`), `App.Run` selects `Init4Gray` for the
  panel waveform, `Clear` zeroes the buffer (which is all-white in both
  encodings), and the capture backends (`WebPreview`, `ImageBackend`)
  join the planes via `reconstructFrame` to render four distinct shades
  in PNG. Remaining work is on-device validation
  (beads `inkwell-0p7`) — needs a Raspberry Pi + Waveshare panel.
- **Partial refresh in the render loop.** `EPD.DisplayPartial` is
  implemented but the render loop in `App.Run` only calls
  `EPD.Display` (full refresh). Switching to partial selectively is a
  future optimisation.
