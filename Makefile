.PHONY: all build test lint clean install install-dev-tools deps verify-deps cross-build

# Build variables
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -s -w \
	-X github.com/xabinapal/patrol/internal/version.Version=$(VERSION) \
	-X github.com/xabinapal/patrol/internal/version.Commit=$(COMMIT) \
	-X github.com/xabinapal/patrol/internal/version.Date=$(DATE)

# Default target
all: lint test build

# Build the binary
build:
	go build -ldflags="$(LDFLAGS)" -o bin/patrol ./cmd/patrol

# Run unit tests
test:
	go test -v -race -coverprofile=coverage.out ./...

# Run integration tests (requires Docker)
test-integration: build
	go test -v -tags=integration -timeout=5m ./test/integration/...

# Run all tests
test-all: test test-integration

# Run tests with coverage report
coverage: test
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Start test infrastructure (Vault and OpenBao in Docker)
test-infra-up:
	docker compose -f test/docker-compose.yaml up -d
	@echo "Waiting for services to be ready..."
	@sleep 5
	@echo "Test infrastructure is ready"
	@echo "  Vault:   http://127.0.0.1:8200 (token: root-token)"
	@echo "  OpenBao: http://127.0.0.1:8210 (token: root-token)"

# Stop test infrastructure
test-infra-down:
	docker compose -f test/docker-compose.yaml down -v

# Run integration tests with infrastructure
integration: test-infra-up test-integration test-infra-down

# Run linter
# Scan all files regardless of build tags to ensure platform-agnostic linting
# Run linter for each platform separately since build tags are mutually exclusive
# Set GOOS to match build tags so typecheck works correctly
lint:
	@if command -v golangci-lint >/dev/null 2>&1; then \
		EXIT_CODE=0; \
		echo "Running linter for darwin..."; \
		GOOS=darwin golangci-lint run --build-tags "darwin" --timeout=5m || EXIT_CODE=$$?; \
		echo "Running linter for linux..."; \
		GOOS=linux golangci-lint run --build-tags "linux" --timeout=5m || EXIT_CODE=$$?; \
		echo "Running linter for windows..."; \
		GOOS=windows golangci-lint run --build-tags "windows" --timeout=5m || EXIT_CODE=$$?; \
		exit $$EXIT_CODE; \
	else \
		echo "golangci-lint not installed"; \
		echo "Install with: make install-dev-tools"; \
		exit 1; \
	fi

# Format code
fmt:
	gofmt -s -w .
	@if command -v goimports >/dev/null 2>&1; then \
		goimports -w .; \
	fi

# Clean build artifacts
clean:
	rm -rf bin/
	rm -f coverage.out coverage.html

# Install to GOPATH/bin
install:
	go install -ldflags="$(LDFLAGS)" ./cmd/patrol

# Run the application
run: build
	./bin/patrol

# Generate mocks (if needed in the future)
generate:
	go generate ./...

# Download dependencies
deps:
	go mod download

# Verify dependencies
verify-deps:
	go mod verify

# Tidy dependencies
tidy:
	go mod tidy

# Install development tools
install-dev-tools:
	@echo "Installing development tools..."
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@go install github.com/securego/gosec/v2/cmd/gosec@latest
	@echo "Development tools installed successfully"
	@echo "  - golangci-lint: $(shell golangci-lint version 2>/dev/null || echo 'installed')"
	@echo "  - gosec: $(shell gosec -version 2>/dev/null || echo 'installed')"

# Security scan
# Scan all files regardless of build tags to ensure platform-agnostic security checks
security:
	@if command -v gosec >/dev/null 2>&1; then \
		gosec -tags "darwin,linux,windows" -fmt text ./... || exit 1; \
	else \
		echo "gosec not installed"; \
		echo "Install with: make install-dev-tools"; \
		exit 1; \
	fi

# Cross-build for specific OS/ARCH (used by CI)
# Usage: make cross-build GOOS=linux GOARCH=amd64
cross-build:
	@if [ -z "$(GOOS)" ] || [ -z "$(GOARCH)" ]; then \
		echo "Usage: make cross-build GOOS=<os> GOARCH=<arch>"; \
		exit 1; \
	fi
	@BINARY_NAME=patrol; \
	if [ "$(GOOS)" = "windows" ]; then \
		BINARY_NAME=patrol.exe; \
	fi; \
	GOOS=$(GOOS) GOARCH=$(GOARCH) go build -ldflags="$(LDFLAGS)" -o bin/$$BINARY_NAME ./cmd/patrol

# Show help
help:
	@echo "Available targets:"
	@echo "  all              - Run lint, test, and build"
	@echo "  build            - Build the binary"
	@echo "  test             - Run unit tests"
	@echo "  test-integration - Run integration tests (requires Docker)"
	@echo "  test-all         - Run all tests"
	@echo "  integration      - Start infra, run integration tests, stop infra"
	@echo "  test-infra-up    - Start test infrastructure (Vault, OpenBao)"
	@echo "  test-infra-down  - Stop test infrastructure"
	@echo "  coverage         - Run tests with coverage report"
	@echo "  lint             - Run linter"
	@echo "  fmt              - Format code"
	@echo "  clean            - Clean build artifacts"
	@echo "  install          - Install to GOPATH/bin"
	@echo "  install-dev-tools - Install development tools (golangci-lint, gosec)"
	@echo "  deps              - Download dependencies"
	@echo "  verify-deps       - Verify dependencies"
	@echo "  tidy              - Tidy dependencies"
	@echo "  security          - Run security scan"
	@echo "  cross-build       - Cross-build for specific OS/ARCH (GOOS=linux GOARCH=amd64)"
	@echo "  help              - Show this help"
