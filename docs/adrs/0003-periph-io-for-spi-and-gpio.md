# ADR 0003: Use periph.io for SPI and GPIO

- **Status:** Accepted
- **Recorded:** 2026-09-18

## Context

Driving the panel needs SPI writes plus four GPIO lines (DC, RST, BUSY, PWR) on
a Pi Zero 2 W. The Go options were periph.io, `go-rpio`, and
`golang.org/x/exp/io/spi`.

Cross-compiling from a workstation to the Pi is part of the normal workflow, so
a library that drags in CGO would mean either a Pi-side build or a cross
toolchain with system headers.

## Decision

Use [periph.io](https://periph.io): `periph.io/x/conn/v3` for the SPI connection
and pin interfaces, `periph.io/x/host/v3` to register the Linux SPI and GPIO
drivers. It's pure Go, so `CGO_ENABLED=0` cross-compiles work, and it ships
`spitest` and `gpiotest` doubles that let the SPI backend be tested off-hardware.

`host/v3` is what gives the process a `/dev/spidev0.0` opener and a resolver for
`GPIO17` / `GPIO18` / `GPIO24` / `GPIO25`, so
[`spiHardware.initRealHardware`](../../internal/inkwell/spi_hardware.go) depends
on it being registered.

Rejected:

- **`go-rpio`** (`github.com/stianeikeland/go-rpio`): unmaintained since December
  2021, relies on fragile memory mapping, and doesn't claim Pi Zero 2 W support.
- **`golang.org/x/exp/io/spi`**: deprecated, and its own docs point at periph.io.

## Consequences

No CGO anywhere in the build, so `GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build`
is the whole cross-compile story.

The periph.io test doubles are what make
[ADR 0005](0005-hardware-free-tests-with-a-full-coverage-gate.md) reachable for
the SPI backend itself, not just for the layers above it.
