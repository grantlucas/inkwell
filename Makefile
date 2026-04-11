.PHONY: all test vet coverage build build-pi run verify fix lint ci clean help

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

# Run locally with the preview backend (uses inkwell.yaml, or pass CONFIG=<path>)
run: build
	go run ./cmd/inkwell $(CONFIG)

# Cross-compile for Raspberry Pi (linux/arm64)
build-pi:
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build ./...

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
	@echo "  make build-pi   - Cross-compile for Raspberry Pi (linux/arm64)"
	@echo "  make run        - Run locally (uses inkwell.yaml, or CONFIG=path make run)"
	@echo "  make verify     - Verify module dependencies"
	@echo "  make fix        - Run go fix to modernize code"
	@echo "  make lint       - Lint markdown files"
	@echo "  make ci         - Full CI pipeline (verify, vet, test, coverage, build-pi)"
	@echo "  make clean      - Remove build artifacts"
	@echo "  make help       - Show this help"
