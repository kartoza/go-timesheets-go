# Nix Flake for Kartoza Timesheet

This project provides both a development environment and an installable package via Nix flakes.

## Table of Contents
- [Installing the Package](#installing-the-package)
- [Using the Package in Your System](#using-the-package-in-your-system)
- [Development Environment](#development-environment)
- [Development Tools](#development-tools)

---

## Installing the Package

The Kartoza Timesheet application is available as a Nix flake package that can be installed directly.

### Try Without Installing

Test the application without installing it:

```bash
nix run github:kartoza/go-timesheets-go
```

### Install to User Profile

Install the package to your user profile:

```bash
nix profile install github:kartoza/go-timesheets-go
```

After installation, you can run:
```bash
kartoza-timesheet
```

### Install from Local Directory

If you've cloned the repository:

```bash
# From within the repository directory
nix profile install .

# Or try it directly
nix run .
```

### Uninstall

To remove the package from your profile:

```bash
nix profile remove kartoza-timesheet
```

## Using the Package in Your System

### NixOS Configuration

Add to your NixOS configuration (`/etc/nixos/configuration.nix` or similar):

```nix
{
  description = "My NixOS configuration";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    kartoza-timesheet.url = "github:kartoza/go-timesheets-go";
  };

  outputs = { self, nixpkgs, kartoza-timesheet, ... }: {
    nixosConfigurations.your-hostname = nixpkgs.lib.nixosSystem {
      system = "x86_64-linux";
      modules = [
        {
          # Add the package to system packages
          environment.systemPackages = [
            kartoza-timesheet.packages.x86_64-linux.default
          ];
        }
      ];
    };
  };
}
```

Then rebuild your system:
```bash
sudo nixos-rebuild switch
```

### Home Manager Configuration

Add to your home-manager configuration:

```nix
{
  description = "Home Manager configuration";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    home-manager.url = "github:nix-community/home-manager";
    kartoza-timesheet.url = "github:kartoza/go-timesheets-go";
  };

  outputs = { self, nixpkgs, home-manager, kartoza-timesheet, ... }: {
    homeConfigurations.your-username = home-manager.lib.homeManagerConfiguration {
      pkgs = import nixpkgs {
        system = "x86_64-linux";
        config.allowUnfree = true;
      };

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

Then apply the configuration:
```bash
home-manager switch
```

### Flake Configuration File

You can also add it to your `flake.nix` as an input:

```nix
{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    kartoza-timesheet.url = "github:kartoza/go-timesheets-go";
  };

  outputs = { self, nixpkgs, kartoza-timesheet, ... }@inputs:
    let
      system = "x86_64-linux";
      pkgs = import nixpkgs { inherit system; };
    in {
      # Your other outputs...

      devShells.${system}.default = pkgs.mkShell {
        buildInputs = [
          kartoza-timesheet.packages.${system}.default
        ];
      };
    };
}
```

### Desktop Integration

The package includes a `.desktop` file that is automatically installed. After installation, you can:

1. **Launch from Application Menu**: Look for "Kartoza Timesheet" in your application launcher
2. **Launch from Terminal**: Run `kartoza-timesheet`
3. **Pin to Favorites**: Pin the application to your dock/favorites for quick access

The desktop file is configured to launch the application in a terminal, as it's a TUI (Terminal User Interface) application.

### Application Usage

Once installed, you can use the application:

```bash
# Launch interactive TUI
kartoza-timesheet

# Start time tracking
kartoza-timesheet start "Project Name" "Activity"

# Start with task
kartoza-timesheet start "WB GEEST 2" "Coding" "Task 3: Improved Functionalities"

# Stop tracking
kartoza-timesheet stop

# Get status (for waybar/desktop integration)
kartoza-timesheet status

# View help
kartoza-timesheet --help
```

---

## Development Environment

For developing the Kartoza Timesheet application, the flake provides a comprehensive development environment.

## Quick Start

### Option 1: Nix Flakes (Recommended)

```bash
# Enter the development environment
nix develop

# Or use direnv for automatic loading
echo "use flake" > .envrc
direnv allow
```

### Option 2: Initialize Everything at Once

```bash
# Enter dev environment and set up Go project
nix develop
nix run .#setup
```

## What's Included

### 🚀 Core Development Tools
- **Go 1.21+** with full toolchain
- **Neovim** with comprehensive Go configuration
- **Git** and **GitHub CLI** for version control
- **Pre-commit** hooks for quality assurance

### 🎨 Neovim Configuration
- **LSP Support**: `gopls` for Go language server
- **Treesitter**: Syntax highlighting for Go, Lua, Markdown, JSON, YAML
- **Copilot**: AI-powered code completion
- **File Explorer**: `nvim-tree` for project navigation
- **Fuzzy Finder**: Telescope for fast file/text search
- **Auto-completion**: Full completion with snippets

### 🫧 Bubbletea Ecosystem
- **bubbletea**: TUI framework for building terminal apps
- **lipgloss**: Styling and layout for beautiful TUIs
- **bubbles**: Pre-built components for common UI elements

### 🛠️ Go Development Tools
- `gopls` - Go language server
- `golangci-lint` - Comprehensive linting
- `gotools` - Standard Go tools
- `gomodifytags` - Struct tag modification
- `gotests` - Test generation
- `impl` - Interface implementation generation

### 📟 Modern CLI Tools
- `eza` - Better `ls` with icons and colors
- `bat` - Better `cat` with syntax highlighting  
- `fd` - Better `find` with intuitive syntax
- `ripgrep` - Faster `grep` with better output
- `fzf` - Fuzzy finder for everything
- `jq`/`yq` - JSON/YAML processing

## Key Mappings in Neovim

### File Navigation
- `<Space>ff` - Find files (Telescope)
- `<Space>fg` - Live grep (search in files)
- `<Space>fb` - Browse buffers
- `<Space>e` - Toggle file explorer

### Go Development
- `<Space>gr` - Run Go program
- `<Space>gt` - Run Go tests
- `<Space>gb` - Build Go program
- `<Space>gf` - Format Go code
- `<Space>gi` - Organize imports

### LSP Features
- `<Space>gd` - Go to definition
- `<Space>gD` - Go to declaration
- `<Space>gr` - Find references
- `<Space>gi` - Go to implementation
- `<Space>K` - Show hover documentation
- `<Space>rn` - Rename symbol
- `<Space>ca` - Code actions
- `<Space>f` - Format code

### Copilot Integration
- `<C-J>` - Accept Copilot suggestion
- `<C-]>` - Next suggestion
- `<C-[>` - Previous suggestion
- `<C-\>` - Trigger suggestion

## Environment Setup

The flake automatically sets up:

```bash
# Environment variables
export EDITOR=nvim
export VISUAL=nvim
export GOPATH="$HOME/go"
export GO111MODULE=on

# Useful aliases
alias ll='eza -la'
alias cat='bat'
alias find='fd'
alias grep='rg'
alias vim='nvim'

# Go shortcuts
alias gor='go run .'
alias got='go test ./...'
alias gob='go build .'
alias gom='go mod tidy'
```

## Getting Started with Bubbletea

After entering the environment:

1. **Initialize the project:**
   ```bash
   nix run .#setup
   ```

2. **Create a simple TUI app:**
   ```bash
   nvim main.go
   ```

3. **Example Bubbletea program:**
   ```go
   package main

   import (
       "fmt"
       tea "github.com/charmbracelet/bubbletea"
   )

   func main() {
       p := tea.NewProgram(initialModel())
       if _, err := p.Run(); err != nil {
           fmt.Printf("Error: %v", err)
       }
   }
   ```

## Direnv Integration

For automatic environment loading, use direnv:

```bash
# Install direnv if not available
nix-env -iA nixpkgs.direnv

# Set up direnv (one time)
echo 'eval "$(direnv hook bash)"' >> ~/.bashrc
# or for zsh:
echo 'eval "$(direnv hook zsh)"' >> ~/.zshrc

# Allow direnv in project
direnv allow
```

Now the environment automatically loads when you `cd` into the project directory!

## Troubleshooting

### Copilot Setup
1. Open Neovim: `nvim`
2. Run: `:Copilot setup`
3. Follow the authentication instructions

### LSP Not Working
- Ensure `gopls` is in PATH: `which gopls`
- Check LSP status in Neovim: `:LspInfo`
- Restart LSP: `:LspRestart`

### Flake Updates
```bash
# Update flake inputs
nix flake update

# Rebuild environment
nix develop --refresh
```

## Project Structure for TUI Apps

Recommended structure for Bubbletea applications:

```
kartoza-timesheet-app/
├── cmd/                    # CLI commands
├── internal/
│   ├── ui/                # TUI components
│   │   ├── components/    # Reusable UI components  
│   │   ├── pages/         # Application pages/screens
│   │   └── styles/        # Lipgloss styles
│   ├── models/            # Data models
│   └── services/          # Business logic
├── pkg/                   # Public packages
└── main.go               # Application entry point
```

## Quality Assurance

The environment includes pre-configured quality tools:

```bash
# Run all quality checks
pre-commit run --all-files

# Format Go code
gofmt -s -w .
goimports -w .

# Lint Go code  
golangci-lint run

# Run tests
go test ./...
```

---

**Happy coding with the power of Nix, Neovim, and Bubbletea!** 🚀🫧

