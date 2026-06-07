.PHONY: all test vet coverage build build-pi build-pi-hw run stop verify fix lint ci clean help

# Default target
all: ci

# Run all tests with race detection and generate coverage profile
test:
	go test -race -coverprofile=/tmp/coverage.out ./internal/...

# Run go vet
vet:
	go vet ./...

# Check 100% coverage on internal packages (requires test to run first)
coverage: test
	@COVERAGE=$$(go tool cover -func=/tmp/coverage.out | grep '^total:' | awk '{print $$3}' | tr -d '%'); \
	echo "Total coverage: $${COVERAGE}%"; \
	if awk "BEGIN {exit !($${COVERAGE} < 100)}"; then \
		echo "Coverage is below 100% ($${COVERAGE}%)"; \
		exit 1; \
	fi

# Build for the host platform
build:
	go build ./...

# Run locally with the preview backend (copy inkwell.example.yaml to inkwell.yaml, or pass CONFIG=<path>)
run: build
	go run ./cmd/inkwell $(CONFIG)

# Stop a running inkwell process.
#
# pkill -f matches against the full command line, so a previous
# implementation that searched for 'go run ./cmd/inkwell' would also
# kill any unrelated process that happened to contain that substring
# (an editor with the path open, a sibling tail/grep, etc.). Match
# only the compiled binary path that `go run` exec's — the temp
# directory tree under $GOPATH/.cache that ends in /cmd/inkwell/inkwell
# — and fall back to the source-form match if no binary is found.
stop:
	@pkill -x inkwell 2>/dev/null && echo "inkwell stopped" \
		|| pkill -f '[/]inkwell/inkwell( |$$)' 2>/dev/null && echo "inkwell stopped" \
		|| echo "inkwell is not running"

# Cross-compile for Raspberry Pi (linux/arm64) — no SPI backend, matches CI smoke build.
build-pi:
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build ./...

# Cross-compile a deployable Pi binary with the SPI backend wired in.
# Mirrors what the release pipeline produces; output: ./inkwell.
build-pi-hw:
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 \
		go build -tags hardware -o inkwell ./cmd/inkwell

# Verify module dependencies
verify:
	go mod verify

# Run go fix to modernize code
fix:
	go fix ./...

# Lint markdown files
lint:
	markdownlint '**/*.md'

# Remove build artifacts
clean:
	rm -f /tmp/coverage.out

# Full CI pipeline (mirrors GitHub Actions)
ci: verify vet coverage build-pi

help:
	@echo "Available targets:"
	@echo "  make test       - Run all tests with race detection and coverage profile"
	@echo "  make vet        - Run go vet"
	@echo "  make coverage   - Run tests and check 100% coverage on internal packages"
	@echo "  make build      - Build for host platform"
	@echo "  make build-pi   - Cross-compile for Raspberry Pi (linux/arm64, no SPI backend)"
	@echo "  make build-pi-hw - Cross-compile a Pi binary with the SPI backend (-tags hardware)"
	@echo "  make run        - Run locally (copy inkwell.example.yaml to inkwell.yaml, or CONFIG=path make run)"
	@echo "  make stop       - Stop a running inkwell process"
	@echo "  make verify     - Verify module dependencies"
	@echo "  make fix        - Run go fix to modernize code"
	@echo "  make lint       - Lint markdown files"
	@echo "  make ci         - Full CI pipeline (verify, vet, test, coverage, build-pi)"
	@echo "  make clean      - Remove build artifacts"
	@echo "  make help       - Show this help"
