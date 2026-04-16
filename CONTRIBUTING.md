# Contributing to Inkwell

## Development Setup

### Prerequisites

- Go 1.25 or later
- `markdownlint-cli` (for linting markdown files)

### Getting Started

```bash
git clone https://github.com/grantlucas/inkwell.git
cd inkwell
make ci
```

This runs the full CI pipeline (dependency verification, vet, 100% coverage
check, and ARM64 cross-compilation) to confirm everything works.

To run locally with the web preview backend:

```bash
go run ./cmd/inkwell
# Open http://localhost:8080 in your browser
```

## Project Structure

```text
cmd/inkwell/          Entry point (main.go)
internal/inkwell/     All framework code (single package)
  testdata/           Golden files for test assertions
docs/                 Hardware and architecture reference
inkwell.yaml          Default configuration
Makefile              Build, test, and CI targets
```

All source code lives in a single `internal/inkwell` package by design. The
`Hardware` interface is the extension point for new backends.

## Testing Requirements

**100% statement coverage is mandatory.** CI will fail if coverage drops below
100%.

### Running Tests

```bash
make test       # Tests with race detection
make coverage   # Tests + coverage enforcement (fails below 100%)
```

To check coverage manually:

<!-- markdownlint-disable MD013 -->
```bash
go test ./... -coverprofile=/tmp/coverage.out && go tool cover -func=/tmp/coverage.out | grep total
```
<!-- markdownlint-enable MD013 -->

To inspect coverage in a browser:

```bash
go tool cover -html=/tmp/coverage.out
```

### Golden File Testing

Tests use golden files to verify buffer and image output. The helpers
`AssertGoldenBuffer()` and `AssertGoldenPNG()` compare test output against
expected files in `testdata/`.

To update golden files after an intentional change:

```bash
go test ./internal/inkwell/ -update
```

### MockHardware

`MockHardware` is a test double that records all SPI and GPIO calls. Tests
assert against the recorded call sequence to verify that the EPD driver sends
the correct commands and data.

## Coding Conventions

- Run `go fix ./...` before submitting PRs to modernize code
- Run `go vet ./...` to catch common issues
- Run `markdownlint '**/*.md'` on any markdown changes; wrap tables with
  `<!-- markdownlint-disable MD013 -->` / `<!-- markdownlint-enable MD013 -->`
  comments to handle line length
- New displays should be added as a `DisplayProfile` entry, not as new driver
  code
- New backends implement the `Hardware` interface (5 methods: `SendCommand`,
  `SendData`, `ReadBusy`, `Reset`, `Close`)

## CI Pipeline

`make ci` mirrors what GitHub Actions runs on every PR and push to `main`:

1. `make verify` -- verify module dependencies
2. `make vet` -- static analysis
3. `make coverage` -- tests with 100% coverage enforcement
4. `make build-pi` -- cross-compile for linux/arm64

All four steps must pass before a PR can merge.

## PR Workflow

1. Branch from `main`
2. Make your changes following the conventions above
3. Ensure `make ci` passes locally
4. Open a pull request against `main`
5. CI must be green before merge
