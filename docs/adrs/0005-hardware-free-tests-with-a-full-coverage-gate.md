# ADR 0005: Test the whole pipeline off-hardware, gated at 100% statement coverage

- **Status:** Accepted
- **Recorded:** 2026-09-18

## Context

The display pipeline reduces to three steps: render an image, pack it into a byte
buffer, send those bytes over SPI in the right command order. With the transport
behind an interface ([ADR 0004](0004-swappable-hardware-backends.md)), all three
are testable on a laptop.

A project that's mostly rendering has a specific failure mode: the code runs fine,
produces the wrong pixels, and nothing notices until it's on the panel.

## Decision

Run every test without hardware and hold `internal/...` at 100% statement coverage
in CI via `make coverage`. Three techniques cover the three steps:

- **Golden PNGs.** [`testutil.AssertGoldenPNG`](../../internal/inkwell/testutil/golden.go)
  compares a rendered `image.Paletted` against a committed `testdata/<name>.png`;
  `go test ./internal/inkwell/widgets/... -update` regenerates them, and the diff
  gets reviewed by eye before committing. Goldens are opt-in per widget rather
  than mandatory: `widgets/clock/` keeps one, while `date/`, `separator/`,
  `weatherview/` and `weekly/` assert against rendered pixels directly.
- **Command-sequence assertions.** `MockHardware` records every call, so a test
  drives `EPD.Init` or `EPD.Display` and asserts on `mock.Commands()`. See
  [`epd_test.go`](../../internal/inkwell/epd_test.go).
- **Raw buffer comparison.** Calling `PackImage(profile, frame)` and comparing
  bytes checks what would actually reach the panel, as distinct from what the
  source frame looked like. See
  [`buffer_test.go`](../../internal/inkwell/buffer_test.go).

The coverage gate, not a naming convention, is the bar. Coverage doesn't
follow a strict `<file>_test.go` layout: files like `hardware.go`,
`widget/widget.go`, and `weather/weather.go` are exercised through their
callers.

CI (`.github/workflows/ci.yml`) runs `make verify`, `make vet`, `make coverage`
(race detector on, fails below 100%), and `make build-pi`. The Go version comes
from `go.mod`.

## Consequences

Every new branch needs a test before it can merge, which is the point, and is why
the TDD loop in [`AGENTS.md`](../../AGENTS.md) is a hard rule rather than a
preference.

100% statement coverage says every line ran, not that the output is right. The
golden files and pixel assertions are what carry correctness, so a change that
keeps coverage green while quietly changing the rendering is still caught by a
golden diff, and only if the widget keeps one.

Real-hardware verification stays a manual step in
[`docs/guides/installation.md`](../guides/installation.md).
