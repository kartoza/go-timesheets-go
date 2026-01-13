# Go Timesheets Go

<div align="center">

![Go](https://img.shields.io/badge/go-%2300ADD8.svg?style=for-the-badge&logo=go&logoColor=white)
![Terminal](https://img.shields.io/badge/Terminal-TUI-green?style=for-the-badge)
![License](https://img.shields.io/github/license/kartoza/go-timesheets-go?style=for-the-badge)

**A beautiful terminal-based timesheet application built with Go and Bubbletea**


</div>

## ✨ Features

- 🚀 **Beautiful TUI** - Terminal user interface with responsive design
- ⏱️ **Time Tracking** - Start and stop time tracking for projects and activities
- 📊 **Project Management** - Create and manage projects with tasks
- 🖥️ **CLI Commands** - Command-line interface for automation
- 📱 **Waybar Integration** - Desktop status bar integration with JSON output
- 💾 **Data Persistence** - Local JSON-based storage
- 🎯 **Activity Types** - Categorize work with activities (Coding, Planning, etc.)

## 🚀 Quick Start

### Installation

#### Using Nix Flakes (Recommended)

##### Quick Start

```bash
# Try it without installing
nix run github:kartoza/go-timesheets-go

# Install to your profile
nix profile install github:kartoza/go-timesheets-go
```

##### Adding to Your System Configuration

Add the flake as an input to your NixOS or home-manager configuration:

```nix
{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    kartoza-timesheet.url = "github:kartoza/go-timesheets-go";
  };

  outputs = { self, nixpkgs, kartoza-timesheet, ... }@inputs: {
    # For NixOS system configuration
    nixosConfigurations.your-hostname = nixpkgs.lib.nixosSystem {
      system = "x86_64-linux";  # or "aarch64-linux" for ARM
      modules = [
        {
          environment.systemPackages = [
            kartoza-timesheet.packages.x86_64-linux.default
          ];
        }
      ];
    };

    # For home-manager configuration
    homeConfigurations.your-username = inputs.home-manager.lib.homeManagerConfiguration {
      pkgs = import nixpkgs { system = "x86_64-linux"; };
      modules = [
        {
          home.packages = [
            kartoza-timesheet.packages.x86_64-linux.default
          ];
        }
      ];
    };
  };
}
```

The package includes a `.desktop` file (on Linux) that will be automatically installed, allowing you to launch the application from your application menu.

##### Building Release Binaries Locally

You can build release binaries for any platform using the Nix flake:

```bash
# Build for Linux (AMD64)
nix build .#linux-amd64

# Build for Linux (ARM64)
nix build .#linux-arm64

# Build for macOS (Intel)
nix build .#darwin-amd64

# Build for macOS (Apple Silicon)
nix build .#darwin-arm64

# Build for Windows (AMD64)
nix build .#windows-amd64

# Build all release artifacts at once
nix build .#all-releases
```

The built binaries will be in `./result/bin/` and distribution tarballs in `./result/dist/`.

**Note:** These commands use the exact same toolchain as the GitHub Actions CI/CD pipeline, ensuring reproducible builds.

#### Traditional Installation

```bash
# Clone the repository
git clone https://github.com/kartoza/go-timesheets-go.git
cd go-timesheets-go

# Build the application
go build -o go-timesheets-go .

# Setup sample data (optional)
go run scripts/setup-sample-data.go

# Install to system (optional)
sudo cp go-timesheets-go /usr/local/bin/
```

### Usage

```bash
# Start time tracking
go-timesheets-go start "Project Name" "Activity"

# Start with task
go-timesheets-go start "WB GEEST 2" "Coding" "Task 3: Improved Functionalities"

# Stop tracking
go-timesheets-go stop

# Get status (for waybar/desktop integration)
go-timesheets-go status

# Launch interactive TUI
go-timesheets-go
```

## 🖥️ Desktop Integration

### Waybar Configuration

Add to your waybar config:

```json
{
    "custom/timesheet": {
        "exec": "go-timesheets-go status",
        "return-type": "json", 
        "interval": 5,
        "on-click": "go-timesheets-go"
    }
}
```

### Status Output

```json
{
    "text": "🔴 01:23:45",
    "alt": "recording", 
    "tooltip": "Recording: WB GEEST 2\nActivity: Coding\nSession: 01:23:45\nToday: 6.5h",
    "class": "recording"
}
```

## 📱 Screenshots

<div align="center">

### Time Entry Interface
![Timesheet Entry](timesheet-entry.png)

### Daily Listing View  
![Timesheet Listing](timesheet-listing.png)

### Submission Workflow
![Timesheet Submission](timesheet-submission.png)

</div>

## 🏗️ Project Structure

```
go-timesheets-go/
├── cmd/                    # CLI command definitions
├── internal/
│   ├── models/            # Data models and structures
│   ├── service/           # Business logic layer
│   ├── storage/           # Data persistence layer
│   └── tui/               # Terminal user interface
├── scripts/               # Utility scripts
└── main.go               # Application entry point
```

## 🔧 Development

### Requirements

- **Go 1.21+** - For traditional Go development
- **Nix with flakes** - For reproducible builds (highly recommended)
- **Terminal with color support** - For the TUI interface
- **Git** - For version control

### Quick Start for Developers

#### Option 1: Using Nix (Recommended)

```bash
# Clone the repository
git clone https://github.com/kartoza/go-timesheets-go.git
cd go-timesheets-go

# Enter development shell with all tools pre-configured
nix develop

# Build and run
go build -o kartoza-timesheet .
./kartoza-timesheet
```

The `nix develop` shell includes all dependencies and development tools.

#### Option 2: Traditional Go Setup

```bash
# Clone the repository
git clone https://github.com/kartoza/go-timesheets-go.git
cd go-timesheets-go

# Install dependencies
go mod download

# Build and run
go build -o kartoza-timesheet .
./kartoza-timesheet
```

### Development Workflow

```bash
# Run in development mode
go run . --help

# Run with specific command
go run . start "Project" "Activity"

# Build for testing
go build -o kartoza-timesheet .

# Run tests (when implemented)
go test ./...

# Format code
go fmt ./...

# Tidy dependencies
go mod tidy
```

### Building for Different Platforms

#### Using Nix (Reproducible Builds)

The project uses Nix flakes for reproducible cross-platform builds. This is the same toolchain used by CI/CD.

```bash
# Build for your current platform
nix build

# The binary will be in:
./result/bin/kartoza-timesheet

# Build for specific platforms
nix build .#linux-amd64      # Linux x86_64
nix build .#linux-arm64      # Linux ARM64 (Raspberry Pi, etc.)
nix build .#darwin-amd64     # macOS Intel
nix build .#darwin-arm64     # macOS Apple Silicon (M1/M2/M3)
nix build .#windows-amd64    # Windows x86_64

# Build all platforms at once
nix build .#all-releases

# After building, find your artifacts:
ls -lh result/bin/           # Compiled binaries
ls -lh result/dist/          # Distribution tarballs (.tar.gz)
```

#### Using Standard Go Cross-Compilation

If you don't have Nix, you can use Go's built-in cross-compilation:

```bash
# Linux AMD64
GOOS=linux GOARCH=amd64 go build -o kartoza-timesheet-linux-amd64 .

# Linux ARM64
GOOS=linux GOARCH=arm64 go build -o kartoza-timesheet-linux-arm64 .

# macOS Intel
GOOS=darwin GOARCH=amd64 go build -o kartoza-timesheet-darwin-amd64 .

# macOS Apple Silicon
GOOS=darwin GOARCH=arm64 go build -o kartoza-timesheet-darwin-arm64 .

# Windows
GOOS=windows GOARCH=amd64 go build -o kartoza-timesheet-windows-amd64.exe .
```

**Note:** Nix builds produce statically-linked binaries with CGO disabled, which are more portable than standard Go builds.

### Understanding Build Artifacts

After building with Nix, you'll get:

```
result/
├── bin/
│   ├── kartoza-timesheet      # Main executable (or .exe for Windows)
│   └── scripts                # Build scripts (can be ignored)
├── dist/
│   └── kartoza-timesheet-0.2.0-{platform}-{arch}.tar.gz
└── share/
    └── applications/
        └── kartoza-timesheet.desktop  # Linux desktop entry (Linux only)
```

### Testing Builds

```bash
# Test the default build
nix build
./result/bin/kartoza-timesheet --help

# Test a specific platform build
nix build .#linux-amd64
file ./result/bin/kartoza-timesheet  # Verify it's a Linux ELF binary

# Test Windows build (on Linux, you can check the file type)
nix build .#windows-amd64
file ./result/bin/kartoza-timesheet.exe  # Should show "PE32+ executable"

# Test macOS build
nix build .#darwin-arm64
file ./result/bin/kartoza-timesheet  # Should show "Mach-O 64-bit arm64 executable"
```

### Verifying Build Reproducibility

One of the benefits of Nix is reproducible builds:

```bash
# Build twice and compare
nix build .#linux-amd64
sha256sum result/bin/kartoza-timesheet > checksum1.txt

rm result

nix build .#linux-amd64
sha256sum result/bin/kartoza-timesheet > checksum2.txt

# Should show identical checksums
diff checksum1.txt checksum2.txt
```

### Updating the vendorHash

If you add or update Go dependencies, you'll need to update the `vendorHash` in `flake.nix`:

```bash
# 1. Update go.mod and go.sum
go get github.com/some/new-package
go mod tidy

# 2. Try building with Nix (it will fail with the wrong hash)
nix build

# 3. The error message will show the correct hash. Copy it and update flake.nix:
# Change this line in flake.nix:
#   vendorHash = "sha256-OLD_HASH_HERE";
# To the new hash shown in the error message

# 4. Build again to verify
nix build
```

## 📦 Release Process

### Automated Releases (CI/CD)

When you create a GitHub release, the CI/CD pipeline automatically:

1. **Builds binaries** for all platforms:
   - Linux (AMD64, ARM64)
   - macOS (Intel, Apple Silicon)
   - Windows (AMD64)

2. **Creates distribution artifacts**:
   - Compiled binaries for each platform
   - Compressed tarballs (.tar.gz)
   - SHA256 checksums file

3. **Uploads artifacts** to the GitHub release

All builds use Nix for reproducibility, ensuring the same output regardless of where they're built.

### Creating a Release

```bash
# 1. Update version in flake.nix
vim flake.nix  # Change: version = "0.2.0" to version = "0.3.0"

# 2. Commit version bump
git add flake.nix
git commit -m "Bump version to 0.3.0"
git push

# 3. Create and push tag
git tag v0.3.0
git push origin v0.3.0

# 4. Create GitHub release
gh release create v0.3.0 \
  --title "Release v0.3.0" \
  --generate-notes

# 5. Wait for GitHub Actions to build and upload artifacts
# View progress at: https://github.com/kartoza/go-timesheets-go/actions

# 6. Verify release artifacts
gh release view v0.3.0
```

### Building Releases Locally

You can build release artifacts locally using the **exact same toolchain** as CI:

```bash
# Build all platforms at once
nix build .#all-releases

# Check what was built
ls -lh result/bin/
# kartoza-timesheet-linux-amd64
# kartoza-timesheet-linux-arm64
# kartoza-timesheet-darwin-amd64
# kartoza-timesheet-darwin-arm64
# kartoza-timesheet-windows-amd64.exe

# Distribution tarballs
ls -lh result/dist/
# kartoza-timesheet-0.2.0-linux-amd64.tar.gz
# kartoza-timesheet-0.2.0-linux-arm64.tar.gz
# kartoza-timesheet-0.2.0-darwin-amd64.tar.gz
# kartoza-timesheet-0.2.0-darwin-arm64.tar.gz
# kartoza-timesheet-0.2.0-windows-amd64.tar.gz

# Generate checksums (like CI does)
cd result/bin
sha256sum * > checksums.txt
cat checksums.txt
```

### Manual Release Upload (if needed)

If you need to manually upload artifacts:

```bash
# Build all releases
nix build .#all-releases

# Generate checksums
cd result/bin
sha256sum * > checksums.txt
cd ../..

# Upload to existing release
gh release upload v0.3.0 \
  result/bin/kartoza-timesheet-* \
  result/dist/*.tar.gz \
  result/bin/checksums.txt

# Or create a new release with artifacts
gh release create v0.3.0 \
  --title "Release v0.3.0" \
  --notes "Release notes here" \
  result/bin/kartoza-timesheet-* \
  result/dist/*.tar.gz \
  result/bin/checksums.txt
```

### Testing Release Artifacts

Before publishing, test the artifacts:

```bash
# Extract and test a tarball
mkdir test-release
cd test-release
tar xzf ../result/dist/kartoza-timesheet-0.2.0-linux-amd64.tar.gz
./kartoza-timesheet --help
./kartoza-timesheet status
cd ..

# Test Windows binary (requires Wine on Linux)
wine result/bin/kartoza-timesheet.exe --help
```

### CI/CD Workflow Details

The `.github/workflows/release.yml` workflow:

1. **Triggers**: On release creation or manual workflow dispatch
2. **Uses**: Determinate Systems Nix installer with caching
3. **Builds**: All 5 platform variants in parallel
4. **Generates**: SHA256 checksums for verification
5. **Uploads**: Binaries, tarballs, and checksums to the release
6. **Stores**: Artifacts for 7 days if manually triggered (for testing)

View workflow runs: https://github.com/kartoza/go-timesheets-go/actions

## 📚 Documentation

- [Complete Feature Documentation](README-APP.md)
- [Installation Guide](README-APP.md#installation)
- [CLI Reference](README-APP.md#cli-commands)
- [Desktop Integration](README-APP.md#waybar-integration)

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests (when test framework exists)
5. Submit a pull request

## 📄 License

MIT License - see [LICENSE](LICENSE) for details.

## 🙏 Acknowledgments

- Built with [Charm](https://charm.sh/) TUI libraries
- Created for the Kartoza team 

## 🔮 Roadmap

- [x] Basic time tracking functionality
- [x] CLI commands for automation
- [x] Waybar integration
- [x] Simple TUI interface
- [ ] Advanced TUI with charts and tables
- [ ] Workspace automation
- [ ] ERP integration
- [ ] Mobile companion app

---

