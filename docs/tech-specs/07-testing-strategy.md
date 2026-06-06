# Testing Strategy

The display pipeline reduces to: **render an image → pack it into a
byte buffer → send those bytes over SPI with the right command
sequence.** All three steps are 100% testable without hardware, and
the project enforces 100% statement coverage on `internal/...` in CI
via `make coverage`. The hardware integration step then becomes a
*deployment* concern, not a development concern.

See [`internal/inkwell/`][pkg] for the live tests — every production
file has a `_test.go` neighbour, and every widget has its own
`testdata/` golden directory.

[pkg]: ../../internal/inkwell/

## Architecture for Testability

### The Hardware Interface

```go
type Hardware interface {
    SendCommand(cmd byte) error
    SendData(data []byte) error
    ReadBusy() bool
    Reset() error
    Close() error
}
```

The high-level driver ([`EPD`][epd]) sits on top of this interface and
has no awareness of SPI specifics, so swapping in a recording mock for
tests is trivial. See [`hardware.go`][hardware] for the full interface
(including the optional `FrameSink` capability for preview backends).

[hardware]: ../../internal/inkwell/hardware.go
[epd]: ../../internal/inkwell/epd.go

### Four Backend Implementations

```text
┌─────────────────────────────────────────────────┐
│              Your Application Code              │
│         (widgets, compositor, scheduler)         │
├─────────────────────────────────────────────────┤
│               Hardware Interface                 │
├────────────┬──────────┬──────────┬──────────────┤
│  spiHW     │  MockHW  │  ImageHW │  WebPreview  │
│ (real Pi)  │ (tests)  │ (PNG)    │ (HTTP + SSE) │
└────────────┴──────────┴──────────┴──────────────┘
```

1. **`spiHardware`** ([`spi_hardware.go`][spi]) — production on Pi.
   Build-tagged `//go:build hardware` so it never compiles into host
   builds. Tests use `WithSPIConn(...)` and `WithGPIOPins(...)`
   options to inject periph.io test doubles.
2. **`MockHardware`** ([`mock_hardware.go`][mock]) — appends every
   call (`command` / `data` / `busy` / `reset` / `close`) to a `Calls`
   slice. Exposes `Commands()` / `DataCalls()` helpers for quick
   assertions, and a `BusyCount` field that flips `ReadBusy` from
   `false → true` after N reads to simulate the real busy-wait loop.
3. **`ImageBackend`** ([`image_backend.go`][img]) — writes a sequenced
   PNG (`frame_NNN.png`) into the configured `output_dir` on each
   refresh cycle.
4. **`WebPreview`** ([`web_preview.go`][web]) — serves the live frame
   over HTTP with an SSE stream for browser auto-refresh.

[spi]: ../../internal/inkwell/spi_hardware.go
[mock]: ../../internal/inkwell/mock_hardware.go
[img]: ../../internal/inkwell/image_backend.go
[web]: ../../internal/inkwell/web_preview.go

## 1. Golden File Testing (Primary Verification)

The shared helper [`testutil.AssertGoldenPNG`][golden-go] compares a
rendered `image.Paletted` against a committed `testdata/<name>.png`.
Tests run with `-update` regenerate the golden file in place:

```bash
# Regenerate golden files after intentional changes
go test ./internal/inkwell/widgets/... -update
```

Then review the diff visually (`git diff testdata/`) before committing.

[golden-go]: ../../internal/inkwell/testutil/golden.go

### Where Golden Files Live

Each widget owns its `testdata/` directory:

```text
internal/inkwell/widgets/
  clock/testdata/...png
  weekly/testdata/...png
  weatherview/testdata/...png
  ...
```

The widgets compare rendered output to the committed PNGs. The same
helper is used in [`testutil/golden_test.go`][golden-test] so the
helper itself is covered.

[golden-test]: ../../internal/inkwell/testutil/golden_test.go

### Raw Buffer Comparison

When you need to assert exactly what bytes a profile would push to the
panel — distinct from what the source frame looked like — call
`PackImage(profile, frame)` and compare the returned byte slice
directly. See [`buffer_test.go`][buffer-test] for examples of asserting
specific bytes / dither patterns against fixed inputs.

[buffer-test]: ../../internal/inkwell/buffer_test.go

## 2. Command Sequence Testing

`MockHardware` records every SPI/GPIO call, so tests can assert exact
init / display / sleep sequences without hardware. See
[`epd_test.go`][epd-test] for the canonical pattern: drive `EPD.Init`
or `EPD.Display`, then assert on `mock.Calls` (full call log) or
`mock.Commands()` (just the command bytes).

```go
mock := &inkwell.MockHardware{}
epd := inkwell.NewEPD(mock, &inkwell.Waveshare7in5V2)
_ = epd.Init(inkwell.InitFull)

want := []byte{
    0x06, 0x01, 0x04, 0x00, 0x61, 0x15, 0x50, 0x60,
}
if got := mock.Commands(); !bytes.Equal(got, want) {
    t.Errorf("InitFull commands: got %x want %x", got, want)
}
```

[epd-test]: ../../internal/inkwell/epd_test.go

## 3. Widget Testing

Each widget package is self-contained under
[`internal/inkwell/widgets/`][widgets] and ships with both unit tests
and golden PNGs:

- Widgets take their data dependencies via `widget.Deps` (and small
  inline accessors like a `now func() time.Time`). Tests substitute a
  fixed clock or a stub `CalendarSource` / `weather.Source`.
- The widget registry ([`widget/registry.go`][registry]) lets tests
  build a custom factory map and pass it via `inkwell.WithRegistry(...)`
  when constructing the `App`.
- Golden PNGs are stored in each widget's `testdata/` directory and
  regenerated via `-update`.

[widgets]: ../../internal/inkwell/widgets/
[registry]: ../../internal/inkwell/widget/registry.go

## 4. Web Preview Server (Dev Feedback Loop)

The web preview is the fastest iteration loop and is the default
backend in [`inkwell.example.yaml`][example]. See
[`web_preview.go`][web] and [`web_preview_test.go`][web-test] /
[`web_preview_sse_test.go`][web-sse-test] for the implementation and
its tests.

[example]: ../../inkwell.example.yaml
[web-test]: ../../internal/inkwell/web_preview_test.go
[web-sse-test]: ../../internal/inkwell/web_preview_sse_test.go

```text
┌──────────────┐     SSE: "refresh"        ┌───────────────┐
│  Go process  │ ────────────────────────► │    Browser    │
│              │                           │               │
│  /frame.png  │ ◄──── GET /frame.png ──── │ <img> reload  │
└──────────────┘                           └───────────────┘
```

### Endpoints

<!-- markdownlint-disable MD013 -->
| Endpoint | Purpose |
|----------|---------|
| `GET /` | HTML page with `<img>`, view toggle, and SSE listener |
| `GET /frame.png` | Current frame as PNG (device buffer by default) |
| `GET /frame.png?scale=N` | 1–10× nearest-neighbor upscale |
| `GET /frame.png?source=1` | Pre-pack grayscale source frame (design intent) |
| `GET /events` | SSE stream — pushes `"refresh"` on every frame update |
<!-- markdownlint-enable MD013 -->

### Device vs. source view

`WebPreview` implements `FrameSink` so [`app.go`][app] can hand it the
composited grayscale frame before `PackImage` runs. On every refresh
the backend stores both:

- **`current`** — the *post-dither* device buffer unpacked back to an
  `image.Paletted`. This is what `/frame.png` returns by default and
  matches what the panel would physically show.
- **`source`** — the high-fidelity grayscale paletted frame, served
  when the client requests `?source=1` (e.g. by toggling the radio in
  the browser UI).

This is what keeps the preview honest: design decisions that only read
in the source view will be visible to reviewers as a real regression
on the device view.

[app]: ../../internal/inkwell/app.go

## 5. Real-Hardware Integration

The SPI backend is built-tagged so it never compiles into host or CI
builds:

```bash
# Build for Pi with SPI compiled in
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 \
  go build -tags hardware -o inkwell ./cmd/inkwell

# Smoke test on the Pi
ssh pi@<ip> './inkwell inkwell.yaml'
```

`spi_hardware_test.go` exercises the SPI backend code paths using
`WithSPIConn` and `WithGPIOPins` option injection — those tests run on
your dev machine without hardware. The end-to-end *real-hardware*
verification step (driving the panel from a deployed binary) lives in
the install guide: see
[`docs/guides/installation.md`](../guides/installation.md).

## 6. CI Pipeline (GitHub Actions)

Everything runs without hardware. See
[`.github/workflows/ci.yml`](../../.github/workflows/ci.yml) for the
live workflow; the `make` targets it invokes are:

<!-- markdownlint-disable MD013 -->
| Target | What it does |
|--------|--------------|
| `make verify` | `go mod verify` |
| `make vet` | `go vet ./...` |
| `make coverage` | Runs tests with race detector and `-coverprofile`, fails if coverage drops below 100% |
| `make build-pi` | Cross-compiles for `linux/arm64` (no SPI tag) |
<!-- markdownlint-enable MD013 -->

The Go version is read from [`go.mod`](../../go.mod) (currently 1.25+).

### What Runs Where

<!-- markdownlint-disable MD013 -->
| Test Type | Dev Machine (macOS) | CI (Ubuntu x86_64) | Pi (linux arm64) |
|-----------|--------------------|--------------------|------------------|
| Golden file / buffer tests | Yes | Yes | Yes |
| Command sequence tests | Yes | Yes | Yes |
| Widget rendering tests | Yes | Yes | Yes |
| Web preview (live) | Yes (interactive) | No | Yes (after deploy) |
| Real hardware tests | No | No | Yes (`-tags hardware`) |
| Cross-compile check | Yes | Yes (`make build-pi`) | N/A (native) |
<!-- markdownlint-enable MD013 -->

## 7. Development Workflow (TDD)

Per `CLAUDE.md`, feature and bug fix work uses the `/tdd` skill
(red-green-refactor). The typical inner loop:

1. Write a failing test for the smallest behavioural change.
2. Make it green with the smallest possible implementation.
3. Run `make coverage` — keep statement coverage at 100%.
4. Open the live web preview at `http://localhost:8080/` to confirm
   the change reads correctly on the *device view* (post-dither).
5. If it's a visual change, regenerate the relevant golden PNGs with
   `go test ./... -update` and review the diff visually.
6. Run `make fix` to apply Go modernisations before committing.
7. Commit immediately to checkpoint progress.

## 8. Font Rendering

Pure Go via `golang.org/x/image/font` and
`golang.org/x/image/font/opentype` — no CGO. Widgets load TTFs from
[`internal/inkwell/fonts/`](../../internal/inkwell/fonts/) and render
into the compositor's paletted frame, so the entire glyph path is
testable with golden files.
