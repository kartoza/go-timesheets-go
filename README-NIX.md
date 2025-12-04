# Nix Development Environment

This project includes a comprehensive Nix flake for setting up a complete Go development environment with Neovim, Bubbletea, and all necessary tooling.

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

*Remember: "I am because we are" - Ubuntu Philosophy*