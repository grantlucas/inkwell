# ADR 0004: Put every backend behind one `Hardware` interface

- **Status:** Accepted
- **Recorded:** 2026-09-18

## Context

The panel is the slowest, least available part of the system. Developing against
it directly means a Pi in the loop for every change, and testing against it means
no tests in CI at all.

Everything the driver does to the panel is four operations: send a command byte,
send data bytes, read the busy pin, pulse reset. That's a small enough surface to
stand in for.

## Decision

Define the transport as an interface and swap implementations underneath it:

```go
type Hardware interface {
    SendCommand(cmd byte) error
    SendData(data []byte) error
    ReadBusy() bool
    Reset() error
    Close() error
}
```

Four implementations ship:

1. **`spiHardware`** ([`spi_hardware.go`](../../internal/inkwell/spi_hardware.go)),
   production on the Pi via periph.io, build-tagged `//go:build hardware` so it
   never compiles into host or CI builds.
2. **`MockHardware`** ([`mock_hardware.go`](../../internal/inkwell/mock_hardware.go)),
   which appends every call to a `Calls` slice and exposes `Commands()` /
   `DataCalls()` for assertions, plus a `BusyCount` that flips `ReadBusy` after
   N reads to simulate the real busy-wait.
3. **`ImageBackend`** ([`image_backend.go`](../../internal/inkwell/image_backend.go)),
   which writes a sequenced PNG into `output_dir` each refresh.
4. **`WebPreview`** ([`web_preview.go`](../../internal/inkwell/web_preview.go)),
   which serves the live frame over HTTP with an SSE stream so a browser can
   follow along.

An optional `FrameSink` interface ([`hardware.go`](../../internal/inkwell/hardware.go))
lets preview backends also receive the pre-pack source frame, which is what makes
the `?source=1` design-intent view possible alongside the device view.

The build tag is load-bearing: without it, host builds would need the periph.io
Linux drivers to link on macOS.

## Consequences

The default backend is the web preview, so the normal development loop is a
browser refresh rather than a deploy. Hardware becomes a deployment concern
instead of a development one.

Tests can assert on exact SPI command sequences without a panel
(see [ADR 0005](0005-hardware-free-tests-with-a-full-coverage-gate.md)).

The cost is that the SPI path is the one path CI never exercises. A build with
`-tags hardware` is checked, but only real hardware proves the panel behaves, so
waveform and refresh changes still need a Pi for sign-off.
