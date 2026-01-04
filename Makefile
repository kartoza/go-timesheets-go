# Go Timesheets Go - Makefile
# Ubuntu: "I am because we are"

# Variables
BINARY_NAME=go-timesheets-go
BINARY_STATIC=$(BINARY_NAME)-static
BUILD_DIR=build
VERSION ?= $(shell git describe --tags --always --dirty)
LDFLAGS=-ldflags "-s -w -X main.version=$(VERSION)"
STATIC_LDFLAGS=-ldflags "-s -w -X main.version=$(VERSION)"

# Default target
.PHONY: all
all: build

# Build dynamic binary
.PHONY: build
build:
	@echo "Building $(BINARY_NAME)..."
	go build $(LDFLAGS) -o $(BINARY_NAME) .

# Build static binary
.PHONY: static
static:
	@echo "Building static $(BINARY_STATIC)..."
	CGO_ENABLED=0 go build $(STATIC_LDFLAGS) -o $(BINARY_STATIC) .

# Build both binaries
.PHONY: build-all
build-all: build static

# Clean build artifacts
.PHONY: clean
clean:
	@echo "Cleaning build artifacts..."
	rm -f $(BINARY_NAME) $(BINARY_STATIC)
	rm -rf $(BUILD_DIR)

# Run tests
.PHONY: test
test:
	@echo "Running tests..."
	go test -v -race -coverprofile=coverage.out ./...

# Install dependencies
.PHONY: deps
deps:
	@echo "Installing dependencies..."
	go mod download
	go mod tidy

# Format code
.PHONY: fmt
fmt:
	@echo "Formatting code..."
	gofmt -s -w .
	goimports -w .

# Lint code
.PHONY: lint
lint:
	@echo "Linting code..."
	golangci-lint run

# Run all quality checks
.PHONY: check
check: fmt lint test

# Install binary to system
.PHONY: install
install: static
	@echo "Installing $(BINARY_STATIC) to /usr/local/bin/$(BINARY_NAME)..."
	sudo cp $(BINARY_STATIC) /usr/local/bin/$(BINARY_NAME)

# Uninstall binary from system
.PHONY: uninstall
uninstall:
	@echo "Uninstalling $(BINARY_NAME) from /usr/local/bin..."
	sudo rm -f /usr/local/bin/$(BINARY_NAME)

# Setup sample data
.PHONY: sample-data
sample-data:
	@echo "Setting up sample data..."
	go run scripts/setup-sample-data.go

# Development server (TUI)
.PHONY: dev
dev:
	@echo "Starting development TUI..."
	go run .

# Show help
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build       - Build dynamic binary"
	@echo "  static      - Build static binary"
	@echo "  build-all   - Build both dynamic and static binaries"
	@echo "  clean       - Clean build artifacts"
	@echo "  test        - Run tests"
	@echo "  deps        - Install dependencies"
	@echo "  fmt         - Format code"
	@echo "  lint        - Lint code"
	@echo "  check       - Run all quality checks (fmt, lint, test)"
	@echo "  install     - Install static binary to /usr/local/bin"
	@echo "  uninstall   - Remove binary from /usr/local/bin"
	@echo "  sample-data - Setup sample data for testing"
	@echo "  dev         - Start development TUI"
	@echo "  help        - Show this help message"

# Build info
.PHONY: info
info:
	@echo "Binary name: $(BINARY_NAME)"
	@echo "Version: $(VERSION)"
	@echo "Build flags: $(LDFLAGS)"
	@echo "Static flags: $(STATIC_LDFLAGS)"