# Go Timesheets Go - Makefile

# Variables
BINARY_NAME=kartoza-timesheet
BINARY_STATIC=$(BINARY_NAME)-static
BUILD_DIR=build
VERSION ?= $(shell git describe --tags --always --dirty)
LDFLAGS=-ldflags "-s -w -X main.version=$(VERSION)"
STATIC_LDFLAGS=-ldflags "-s -w -X main.version=$(VERSION)"

# Default target
.PHONY: all
all: test build

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
build-all: test build static

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
install: test static
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

# Monitor application metrics with expvarmon
.PHONY: monitor
monitor:
	@echo "Starting expvarmon monitoring..."
	@echo "Make sure the timesheet app is running in another terminal!"
	@echo ""
	@command -v expvarmon >/dev/null 2>&1 || { echo "Error: expvarmon not found. Install with: go install github.com/divan/expvarmon@latest"; echo "Or use 'nix develop' which includes expvarmon"; exit 1; }
	expvarmon -ports="localhost:6060" \
	  -vars="api.requests.total,api.requests.errors,api.requests.inflight,api.cache_hit_ratio,api.requests.duration_ms" \
	  -i 1s

# View API request logs (built-in monitor command)
.PHONY: logs
logs:
	@echo "Viewing API request logs..."
	@test -f $(BINARY_NAME) || { echo "Error: $(BINARY_NAME) not found. Run 'make build' first."; exit 1; }
	./$(BINARY_NAME) monitor --since 1h

# Follow API request logs in real-time
.PHONY: logs-follow
logs-follow:
	@echo "Following API request logs..."
	@test -f $(BINARY_NAME) || { echo "Error: $(BINARY_NAME) not found. Run 'make build' first."; exit 1; }
	./$(BINARY_NAME) monitor --follow

# Show help
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build          - Build dynamic binary"
	@echo "  static         - Build static binary"
	@echo "  build-all      - Build both dynamic and static binaries"
	@echo "  clean          - Clean build artifacts"
	@echo "  test           - Run tests"
	@echo "  deps           - Install dependencies"
	@echo "  fmt            - Format code"
	@echo "  lint           - Lint code"
	@echo "  check          - Run all quality checks (fmt, lint, test)"
	@echo "  install        - Install static binary to /usr/local/bin"
	@echo "  uninstall      - Remove binary from /usr/local/bin"
	@echo "  sample-data    - Setup sample data for testing"
	@echo "  dev            - Start development TUI"
	@echo "  monitor        - Monitor app metrics with expvarmon TUI (requires app running)"
	@echo "  logs           - View API request logs from last hour"
	@echo "  logs-follow    - Follow API request logs in real-time"
	@echo "  release        - Build release binaries for all platforms"
	@echo "  release-upload - Build and upload release to GitHub (requires TAG=vX.Y)"
	@echo "  release-clean  - Clean release artifacts"
	@echo ""
	@echo "Packaging targets:"
	@echo "  deb            - Build Debian package (requires dpkg-buildpackage)"
	@echo "  rpm            - Build RPM package (requires rpmbuild)"
	@echo "  snap           - Build Snap package (requires snapcraft)"
	@echo "  flatpak        - Build Flatpak package (requires flatpak-builder)"
	@echo "  packages       - Build all packages (deb, rpm, snap, flatpak)"
	@echo "  packages-clean - Clean packaging artifacts"
	@echo ""
	@echo "  help           - Show this help message"

# Release directory
RELEASE_DIR=release

# Build release binaries for all platforms
.PHONY: release
release:
	@echo "Building release binaries for version $(VERSION)..."
	@mkdir -p $(RELEASE_DIR)
	@echo "  Building linux-amd64..."
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(STATIC_LDFLAGS) -o $(RELEASE_DIR)/$(BINARY_NAME)-linux-amd64 .
	@echo "  Building linux-arm64..."
	@CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build $(STATIC_LDFLAGS) -o $(RELEASE_DIR)/$(BINARY_NAME)-linux-arm64 .
	@echo "  Building darwin-amd64..."
	@CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build $(STATIC_LDFLAGS) -o $(RELEASE_DIR)/$(BINARY_NAME)-darwin-amd64 .
	@echo "  Building darwin-arm64..."
	@CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build $(STATIC_LDFLAGS) -o $(RELEASE_DIR)/$(BINARY_NAME)-darwin-arm64 .
	@echo "  Building windows-amd64..."
	@CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build $(STATIC_LDFLAGS) -o $(RELEASE_DIR)/$(BINARY_NAME)-windows-amd64.exe .
	@echo "Creating tarballs..."
	@cd $(RELEASE_DIR) && tar -czf $(BINARY_NAME)-linux-amd64.tar.gz $(BINARY_NAME)-linux-amd64
	@cd $(RELEASE_DIR) && tar -czf $(BINARY_NAME)-linux-arm64.tar.gz $(BINARY_NAME)-linux-arm64
	@cd $(RELEASE_DIR) && tar -czf $(BINARY_NAME)-darwin-amd64.tar.gz $(BINARY_NAME)-darwin-amd64
	@cd $(RELEASE_DIR) && tar -czf $(BINARY_NAME)-darwin-arm64.tar.gz $(BINARY_NAME)-darwin-arm64
	@cd $(RELEASE_DIR) && tar -czf $(BINARY_NAME)-windows-amd64.tar.gz $(BINARY_NAME)-windows-amd64.exe
	@echo ""
	@echo "Release artifacts created in $(RELEASE_DIR)/:"
	@ls -lh $(RELEASE_DIR)/*.tar.gz

# Upload release to GitHub (requires gh CLI and a tag)
.PHONY: release-upload
release-upload: release
	@if [ -z "$(TAG)" ]; then echo "Error: TAG is required. Usage: make release-upload TAG=v0.2"; exit 1; fi
	@echo "Creating GitHub release $(TAG)..."
	@gh release create $(TAG) --generate-notes || true
	@echo "Uploading release artifacts..."
	@gh release upload $(TAG) $(RELEASE_DIR)/*.tar.gz --clobber
	@echo "Release $(TAG) published: https://github.com/kartoza/go-timesheets-go/releases/tag/$(TAG)"

# Clean release artifacts
.PHONY: release-clean
release-clean:
	@echo "Cleaning release artifacts..."
	rm -rf $(RELEASE_DIR)

# Build info
.PHONY: info
info:
	@echo "Binary name: $(BINARY_NAME)"
	@echo "Version: $(VERSION)"
	@echo "Build flags: $(LDFLAGS)"
	@echo "Static flags: $(STATIC_LDFLAGS)"

# =============================================================================
# Packaging Targets
# =============================================================================

PACKAGING_DIR=packaging

# Build Debian package
.PHONY: deb
deb: clean
	@echo "Building Debian package..."
	@mkdir -p $(RELEASE_DIR)/deb
	@# Create source tarball
	@tar --exclude='.git' --exclude='release' --exclude='build' \
		-czf $(RELEASE_DIR)/deb/$(BINARY_NAME)_$(VERSION).orig.tar.gz \
		--transform 's,^,$(BINARY_NAME)-$(VERSION)/,' .
	@# Extract and build
	@cd $(RELEASE_DIR)/deb && tar -xzf $(BINARY_NAME)_$(VERSION).orig.tar.gz
	@cp -r $(PACKAGING_DIR)/debian $(RELEASE_DIR)/deb/$(BINARY_NAME)-$(VERSION)/
	@cd $(RELEASE_DIR)/deb/$(BINARY_NAME)-$(VERSION) && dpkg-buildpackage -us -uc -b
	@echo "Debian package built: $(RELEASE_DIR)/deb/"
	@ls -lh $(RELEASE_DIR)/deb/*.deb 2>/dev/null || echo "No .deb files found"

# Build RPM package (requires rpmbuild)
.PHONY: rpm
rpm: clean
	@echo "Building RPM package..."
	@mkdir -p $(RELEASE_DIR)/rpm/{BUILD,RPMS,SOURCES,SPECS,SRPMS}
	@# Create source tarball
	@tar --exclude='.git' --exclude='release' --exclude='build' \
		-czf $(RELEASE_DIR)/rpm/SOURCES/$(BINARY_NAME)-$(VERSION).tar.gz \
		--transform 's,^,$(BINARY_NAME)-$(VERSION)/,' .
	@cp $(PACKAGING_DIR)/rpm/$(BINARY_NAME).spec $(RELEASE_DIR)/rpm/SPECS/
	@rpmbuild --define "_topdir $(CURDIR)/$(RELEASE_DIR)/rpm" \
		-bb $(RELEASE_DIR)/rpm/SPECS/$(BINARY_NAME).spec
	@echo "RPM package built: $(RELEASE_DIR)/rpm/RPMS/"
	@find $(RELEASE_DIR)/rpm/RPMS -name "*.rpm" -exec ls -lh {} \;

# Build Snap package (requires snapcraft)
.PHONY: snap
snap:
	@echo "Building Snap package..."
	@mkdir -p $(RELEASE_DIR)/snap
	@cp $(PACKAGING_DIR)/snap/snapcraft.yaml .
	@snapcraft --output $(RELEASE_DIR)/snap/$(BINARY_NAME)_$(VERSION)_amd64.snap
	@rm -f snapcraft.yaml
	@echo "Snap package built: $(RELEASE_DIR)/snap/"
	@ls -lh $(RELEASE_DIR)/snap/*.snap

# Build Flatpak package (requires flatpak-builder)
.PHONY: flatpak
flatpak:
	@echo "Building Flatpak package..."
	@mkdir -p $(RELEASE_DIR)/flatpak
	@cd $(PACKAGING_DIR)/flatpak && flatpak-builder --force-clean --repo=$(CURDIR)/$(RELEASE_DIR)/flatpak/repo \
		$(CURDIR)/$(RELEASE_DIR)/flatpak/build com.kartoza.Timesheet.yml
	@flatpak build-bundle $(RELEASE_DIR)/flatpak/repo \
		$(RELEASE_DIR)/flatpak/$(BINARY_NAME)-$(VERSION).flatpak com.kartoza.Timesheet
	@echo "Flatpak built: $(RELEASE_DIR)/flatpak/"
	@ls -lh $(RELEASE_DIR)/flatpak/*.flatpak

# Build all packages
.PHONY: packages
packages: deb rpm snap flatpak
	@echo ""
	@echo "All packages built in $(RELEASE_DIR)/"

# Clean packaging artifacts
.PHONY: packages-clean
packages-clean:
	@echo "Cleaning packaging artifacts..."
	rm -rf $(RELEASE_DIR)/deb $(RELEASE_DIR)/rpm $(RELEASE_DIR)/snap $(RELEASE_DIR)/flatpak
	rm -f snapcraft.yaml
