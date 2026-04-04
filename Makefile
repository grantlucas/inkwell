.PHONY: all test vet coverage build build-pi verify fix lint ci help

# Default target
all: ci

# Run all tests with race detection
test:
	go test -race ./...

# Run go vet
vet:
	go vet ./...

# Check 100% coverage on internal packages
coverage:
	@go test -coverprofile=/tmp/coverage.out ./internal/...
	@COVERAGE=$$(go tool cover -func=/tmp/coverage.out | grep '^total:' | awk '{print $$3}' | tr -d '%'); \
	echo "Total coverage: $${COVERAGE}%"; \
	if [ "$$(echo "$${COVERAGE} < 100" | bc -l)" -eq 1 ]; then \
		echo "Coverage is below 100% ($${COVERAGE}%)"; \
		exit 1; \
	fi

# Build for the host platform
build:
	go build ./...

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

# Full CI pipeline (mirrors GitHub Actions)
ci: verify vet test coverage build-pi

help:
	@echo "Available targets:"
	@echo "  make test       - Run all tests with race detection"
	@echo "  make vet        - Run go vet"
	@echo "  make coverage   - Check 100% coverage on internal packages"
	@echo "  make build      - Build for host platform"
	@echo "  make build-pi   - Cross-compile for Raspberry Pi (linux/arm64)"
	@echo "  make verify     - Verify module dependencies"
	@echo "  make fix        - Run go fix to modernize code"
	@echo "  make lint       - Lint markdown files"
	@echo "  make ci         - Full CI pipeline (verify, vet, test, coverage, build-pi)"
	@echo "  make help       - Show this help"
