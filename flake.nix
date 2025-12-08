{
  description = "Development environment for Kartoza Timesheet App";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs {
          inherit system;
          config = {
            allowUnfree = true;
          };
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
        
      in
      {
        devShells.default = pkgs.mkShell {
          buildInputs = devTools;
          
          shellHook = ''
            echo "🚀 Kartoza Timesheet App Development Environment"
            echo "=============================================="
            echo "✅ Go toolchain with gopls and linting tools"
            echo "✅ Modern CLI tools (ripgrep, fd, eza, etc.)"
            echo "✅ Git and GitHub CLI tools"
            echo "✅ Pre-commit hooks ready"
            echo "✅ System Neovim available"
            echo ""
            echo "🔧 Quick commands:"
            echo "  nvim .           # Open Neovim in current directory"
            echo "  go mod init      # Initialize Go module"
            echo "  go get github.com/charmbracelet/bubbletea  # Add Bubbletea"
            echo "  pre-commit install  # Set up quality hooks"
            echo ""
            echo "🌟 Ubuntu Philosophy: 'I am because we are'"
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
          setup = flake-utils.lib.mkApp {
            drv = pkgs.writeShellScriptBin "setup" ''
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
        };
      }
    );
}