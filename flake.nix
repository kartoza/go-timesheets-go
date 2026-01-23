{
  description = "Development environment for Kartoza Timesheet App";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    let
      # Version - update this with each release
      version = "0.9.1";

      # Helper function to build for a specific platform
      mkPackage = { pkgs, system, GOOS, GOARCH }:
        let
          # For Windows, we need to add .exe extension
          binaryName = if GOOS == "windows" then "kartoza-timesheet.exe" else "kartoza-timesheet";
          platformSuffix = "${GOOS}-${GOARCH}";
        in
        pkgs.buildGoModule {
          pname = "kartoza-timesheet-${platformSuffix}";
          inherit version;
          src = ./.;

          vendorHash = "sha256-zgdlxgR3TQSaRS+npRL3wEhKZRizJZ6D4Y3Jj4O/Xhs=";

          # Use release build tag to disable debug logging and monitoring
          tags = [ "release" ];

          ldflags = [
            "-s"
            "-w"
            "-X main.version=${version}"
          ];

          # Set cross-compilation environment variables
          # These need to be in preBuild to avoid conflicts
          preBuild = ''
            export CGO_ENABLED=0
            export GOOS=${GOOS}
            export GOARCH=${GOARCH}
          '';

          # For cross-compilation, we need to skip tests
          doCheck = false;

          postInstall = ''
            # Handle Go's cross-compilation directory structure
            # Go sometimes creates platform-specific subdirectories
            if [ -d "$out/bin/${GOOS}_${GOARCH}" ]; then
              mv $out/bin/${GOOS}_${GOARCH}/* $out/bin/
              rmdir $out/bin/${GOOS}_${GOARCH}
            fi

            # Rename binary for consistency
            # Windows builds automatically add .exe extension
            if [ -f "$out/bin/go-timesheets-go.exe" ]; then
              mv $out/bin/go-timesheets-go.exe $out/bin/${binaryName}
            elif [ -f "$out/bin/go-timesheets-go" ]; then
              mv $out/bin/go-timesheets-go $out/bin/${binaryName}
            fi

            # Remove scripts binary (we only want the main app)
            rm -f $out/bin/scripts $out/bin/scripts.exe

            # Only install .desktop file and icon for Linux
            ${if GOOS == "linux" then ''
              mkdir -p $out/share/applications
              mkdir -p $out/share/icons/hicolor/256x256/apps
              mkdir -p $out/share/icons/hicolor/scalable/apps
              cp $src/kartoza-timesheet.desktop $out/share/applications/
              cp $src/resources/kartoza-logo.png $out/share/icons/hicolor/256x256/apps/kartoza-timesheet.png
            '' else ""}

            # Create a tarball for distribution
            mkdir -p $out/dist
            cd $out/bin

            # Only create tarball for the main binary
            if [ -f "${binaryName}" ]; then
              tar czf $out/dist/kartoza-timesheet-${version}-${platformSuffix}.tar.gz ${binaryName}
            fi
          '';

          meta = with pkgs.lib; {
            description = "A beautiful terminal-based timesheet application";
            homepage = "https://github.com/kartoza/go-timesheets-go";
            license = licenses.mit;
            maintainers = [ ];
            platforms = platforms.all;
          };
        };
    in
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs {
          inherit system;
          config = { allowUnfree = true; };
        };

        # Development tools
        devTools = with pkgs; [
          # Go ecosystem
          go
          gopls
          gotools
          go-tools
          golangci-lint
          gomodifytags
          gotests
          impl
          reftools

          # General development
          git
          gh
          pre-commit
          nodejs_20
          python311
          python311Packages.pip

          # Terminal and shell utilities
          zsh
          fish
          starship
          eza
          bat
          fd
          ripgrep
          fzf
          tree
          jq
          yq

          # Build tools
          gnumake
          gcc
          pkg-config

          # Fyne GUI dependencies (OpenGL, X11, Wayland)
          libGL
          libGL.dev
          mesa
          xorg.libX11
          xorg.libX11.dev
          xorg.libXcursor
          xorg.libXrandr
          xorg.libXinerama
          xorg.libXi
          xorg.libXxf86vm
          xorg.xorgproto
          wayland
          wayland.dev
          wayland-protocols
          libxkbcommon
          libxkbcommon.dev
          glfw

          # System tray dependencies (Linux)
          libayatana-appindicator
          gtk3
          glib

          # Documentation
          mdbook
          pandoc

          # Container tools
          docker
          docker-compose

          # Nix tools
          nil # Nix LSP
          nixpkgs-fmt
          nixfmt-classic

          # Security tools
          trivy

          # System monitoring
          htop
          btop

          # Text processing
          gawk
          gnused
        ];

        # Native package (for current system)
        kartoza-timesheet = mkPackage {
          inherit pkgs system;
          GOOS = if pkgs.stdenv.isLinux then "linux"
                 else if pkgs.stdenv.isDarwin then "darwin"
                 else throw "Unsupported system";
          GOARCH = if pkgs.stdenv.isx86_64 then "amd64"
                   else if pkgs.stdenv.isAarch64 then "arm64"
                   else throw "Unsupported architecture";
        };

        # Cross-compilation packages
        # Linux builds
        linux-amd64 = mkPackage {
          inherit pkgs system;
          GOOS = "linux";
          GOARCH = "amd64";
        };

        linux-arm64 = mkPackage {
          inherit pkgs system;
          GOOS = "linux";
          GOARCH = "arm64";
        };

        # macOS builds
        darwin-amd64 = mkPackage {
          inherit pkgs system;
          GOOS = "darwin";
          GOARCH = "amd64";
        };

        darwin-arm64 = mkPackage {
          inherit pkgs system;
          GOOS = "darwin";
          GOARCH = "arm64";
        };

        # Windows builds
        windows-amd64 = mkPackage {
          inherit pkgs system;
          GOOS = "windows";
          GOARCH = "amd64";
        };

      in {
        # Package outputs for installing the app
        packages = {
          default = kartoza-timesheet;
          kartoza-timesheet = kartoza-timesheet;

          # Cross-compilation packages
          linux-amd64 = linux-amd64;
          linux-arm64 = linux-arm64;
          darwin-amd64 = darwin-amd64;
          darwin-arm64 = darwin-arm64;
          windows-amd64 = windows-amd64;

          # Convenience package to build all release artifacts
          all-releases = pkgs.symlinkJoin {
            name = "kartoza-timesheet-all-releases";
            paths = [
              linux-amd64
              linux-arm64
              darwin-amd64
              darwin-arm64
              windows-amd64
            ];
          };
        };

        devShells.default = pkgs.mkShell {
          buildInputs = devTools;

          # Native build inputs for CGO (Fyne GUI)
          nativeBuildInputs = with pkgs; [
            pkg-config
          ];

          # Set library paths for CGO
          LD_LIBRARY_PATH = pkgs.lib.makeLibraryPath (with pkgs; [
            libGL
            mesa
            xorg.libX11
            xorg.libXcursor
            xorg.libXrandr
            xorg.libXinerama
            xorg.libXi
            xorg.libXxf86vm
            wayland
            libxkbcommon
            glfw
          ]);

          # Include paths for C headers
          CGO_CFLAGS = "-I${pkgs.xorg.libX11.dev}/include -I${pkgs.xorg.xorgproto}/include -I${pkgs.libGL.dev}/include";
          CGO_LDFLAGS = "-L${pkgs.xorg.libX11}/lib -L${pkgs.libGL}/lib";

          shellHook = ''
            echo "🚀 Kartoza Timesheet App Development Environment"
            echo "=============================================="
            echo "✅ Go toolchain with gopls and linting tools"
            echo "✅ Modern CLI tools (ripgrep, fd, eza, etc.)"
            echo "✅ Git and GitHub CLI tools"
            echo "✅ Pre-commit hooks ready"
            echo "✅ System Neovim available"
            echo "✅ expvarmon for monitoring"
            echo ""
            echo "🔧 Quick commands:"
            echo "  nvim .           # Open Neovim in current directory"
            echo "  go mod init      # Initialize Go module"
            echo "  go get github.com/charmbracelet/bubbletea  # Add Bubbletea"
            echo "  pre-commit install  # Set up quality hooks"
            echo "  make monitor     # Monitor app with expvarmon TUI"
            echo ""
            echo ""

            # Set up environment
            export EDITOR=nvim
            export VISUAL=nvim
            export GOPATH="$HOME/go"
            export PATH="$PATH:$GOPATH/bin"

            # Go environment
            export GO111MODULE=on
            export GOPROXY=https://proxy.golang.org,direct
            export GOSUMDB=sum.golang.org

            # CGO settings for Fyne GUI
            export CGO_ENABLED=1


            # Initialize Go module if it doesn't exist
            if [ ! -f "go.mod" ]; then
              echo "💡 Run 'go mod init github.com/timlinux/kartoza-timesheet-app' to initialize Go module"
            fi

            # Set up aliases for development
            alias ll='eza -la'
            alias la='eza -la'
            alias ls='eza'
            alias cat='bat'
            alias find='fd'
            alias grep='rg'
            alias vim='nvim'
            alias vi='nvim'

            # Go aliases
            alias gor='go run .'
            alias got='go test ./...'
            alias gob='go build .'
            alias gom='go mod tidy'
            alias gof='gofmt -s -w .'
            alias goi='goimports -w .'

            # Git aliases
            alias gs='git status'
            alias ga='git add'
            alias gc='git commit'
            alias gp='git push'
            alias gl='git pull'
            alias gd='git diff'

            # Check if Bubbletea is in go.mod
            if [ -f "go.mod" ] && grep -q "bubbletea" go.mod; then
              echo "🫧 Bubbletea is ready for TUI development!"
            elif [ -f "go.mod" ]; then
              echo "💡 Add Bubbletea: go get github.com/charmbracelet/bubbletea"
            fi

            # Install expvarmon if not already installed
            if ! command -v expvarmon >/dev/null 2>&1; then
              echo "📊 Installing expvarmon for monitoring..."
              go install github.com/divan/expvarmon@latest >/dev/null 2>&1 &
            fi
          '';

          # Environment variables
          GO111MODULE = "on";
          GOPROXY = "https://proxy.golang.org,direct";
          GOSUMDB = "sum.golang.org";
        };

        # Formatter for the flake
        formatter = pkgs.nixpkgs-fmt;

        # Apps for easy execution
        apps = {
          default = flake-utils.lib.mkApp {
            drv = kartoza-timesheet;
          };

          setup = flake-utils.lib.mkApp {
            drv = pkgs.writeShellScriptBin "setup" ''
              #!/usr/bin/env bash
              echo "Setting up development environment..."
              if [ ! -f "go.mod" ]; then
                go mod init github.com/timlinux/kartoza-timesheet-app
                echo "✅ Go module initialized"
              fi

              # Add Bubbletea and common dependencies
              go get github.com/charmbracelet/bubbletea@latest
              go get github.com/charmbracelet/lipgloss@latest
              go get github.com/charmbracelet/bubbles@latest

              echo "✅ Bubbletea ecosystem installed"
              echo "🚀 Ready to build amazing TUIs!"
            '';
          };

          # Build release artifacts for all platforms
          release = flake-utils.lib.mkApp {
            drv = pkgs.writeShellScriptBin "release" ''
              #!/usr/bin/env bash
              set -e

              VERSION="${version}"
              RELEASE_DIR="release"
              BINARY_NAME="kartoza-timesheet"

              echo "🚀 Building release binaries for version $VERSION..."
              mkdir -p "$RELEASE_DIR"

              echo "  Building linux-amd64..."
              CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-s -w -X main.version=$VERSION" -o "$RELEASE_DIR/$BINARY_NAME-linux-amd64" .

              echo "  Building linux-arm64..."
              CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags "-s -w -X main.version=$VERSION" -o "$RELEASE_DIR/$BINARY_NAME-linux-arm64" .

              echo "  Building darwin-amd64..."
              CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags "-s -w -X main.version=$VERSION" -o "$RELEASE_DIR/$BINARY_NAME-darwin-amd64" .

              echo "  Building darwin-arm64..."
              CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags "-s -w -X main.version=$VERSION" -o "$RELEASE_DIR/$BINARY_NAME-darwin-arm64" .

              echo "  Building windows-amd64..."
              CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "-s -w -X main.version=$VERSION" -o "$RELEASE_DIR/$BINARY_NAME-windows-amd64.exe" .

              echo "📦 Creating tarballs..."
              cd "$RELEASE_DIR"
              tar -czf "$BINARY_NAME-linux-amd64.tar.gz" "$BINARY_NAME-linux-amd64"
              tar -czf "$BINARY_NAME-linux-arm64.tar.gz" "$BINARY_NAME-linux-arm64"
              tar -czf "$BINARY_NAME-darwin-amd64.tar.gz" "$BINARY_NAME-darwin-amd64"
              tar -czf "$BINARY_NAME-darwin-arm64.tar.gz" "$BINARY_NAME-darwin-arm64"
              tar -czf "$BINARY_NAME-windows-amd64.tar.gz" "$BINARY_NAME-windows-amd64.exe"

              echo ""
              echo "✅ Release artifacts created in $RELEASE_DIR/:"
              ls -lh *.tar.gz
            '';
          };

          # Build and upload release to GitHub
          release-upload = flake-utils.lib.mkApp {
            drv = pkgs.writeShellScriptBin "release-upload" ''
              #!/usr/bin/env bash
              set -e

              TAG="''${1:-}"
              if [ -z "$TAG" ]; then
                echo "❌ Error: TAG is required"
                echo "Usage: nix run .#release-upload -- v0.3"
                exit 1
              fi

              VERSION="${version}"
              RELEASE_DIR="release"

              # Build release first
              echo "🔨 Building release..."
              nix run .#release

              echo ""
              echo "📤 Creating GitHub release $TAG..."
              gh release create "$TAG" --generate-notes || true

              echo "📤 Uploading release artifacts..."
              gh release upload "$TAG" "$RELEASE_DIR"/*.tar.gz --clobber

              echo ""
              echo "✅ Release $TAG published!"
              echo "🔗 https://github.com/kartoza/go-timesheets-go/releases/tag/$TAG"
            '';
          };
        };
      });
}

