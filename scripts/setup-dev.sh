#!/bin/bash

# Kartoza Timesheet App - Development Environment Setup
# This script sets up the development environment for contributors

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if running in CI
is_ci() {
    [[ "${CI:-}" == "true" ]]
}

# Check if a command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Install pre-commit if not available
install_precommit() {
    log_info "Installing pre-commit..."
    
    if command_exists python3; then
        python3 -m pip install --user pre-commit
    elif command_exists python; then
        python -m pip install --user pre-commit
    elif command_exists pip3; then
        pip3 install --user pre-commit
    elif command_exists pip; then
        pip install --user pre-commit
    else
        log_error "Python and pip not found. Please install Python first."
        log_info "Visit: https://www.python.org/downloads/"
        exit 1
    fi
}

# Setup pre-commit hooks
setup_precommit() {
    log_info "Setting up pre-commit hooks..."
    
    if ! command_exists pre-commit; then
        install_precommit
    fi
    
    # Install the hooks
    if pre-commit install; then
        log_success "Pre-commit hooks installed successfully"
    else
        log_error "Failed to install pre-commit hooks"
        exit 1
    fi
    
    # Install commit-msg hook for conventional commits
    if pre-commit install --hook-type commit-msg; then
        log_success "Commit message hooks installed successfully"
    else
        log_warning "Failed to install commit-msg hooks (non-critical)"
    fi
}

# Check Go installation
check_go() {
    log_info "Checking Go installation..."
    
    if ! command_exists go; then
        log_error "Go not found. Please install Go 1.21 or later."
        log_info "Visit: https://golang.org/doc/install"
        exit 1
    fi
    
    go_version=$(go version | cut -d' ' -f3 | sed 's/go//')
    log_success "Go ${go_version} found"
    
    # Check if version is 1.21 or later
    if ! go version | grep -qE 'go1\.(2[1-9]|[3-9][0-9])'; then
        log_warning "Go 1.21+ recommended. Current version: ${go_version}"
    fi
}

# Setup Go modules if main.go exists
setup_go() {
    if [[ -f "main.go" ]] && [[ ! -f "go.mod" ]]; then
        log_info "Initializing Go module..."
        go mod init github.com/timlinux/kartoza-timesheet-app
        go mod tidy
        log_success "Go module initialized"
    elif [[ -f "go.mod" ]]; then
        log_info "Updating Go dependencies..."
        go mod download
        go mod tidy
        log_success "Go dependencies updated"
    else
        log_info "No Go files found yet - Go module will be initialized when needed"
    fi
}

# Install development tools
install_dev_tools() {
    log_info "Installing Go development tools..."
    
    if command_exists go; then
        # Install golangci-lint if not available
        if ! command_exists golangci-lint; then
            log_info "Installing golangci-lint..."
            if is_ci; then
                # In CI, use the install script
                curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b "$(go env GOPATH)/bin"
            else
                # For local development, use go install
                go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
            fi
        fi
        
        # Install other useful tools
        go install golang.org/x/tools/cmd/goimports@latest
        go install golang.org/x/vuln/cmd/govulncheck@latest
        
        log_success "Go development tools installed"
    fi
}

# Setup Git hooks and configuration
setup_git() {
    log_info "Configuring Git settings..."
    
    # Set up commit template if it doesn't exist
    if [[ ! -f .gitmessage ]]; then
        cat > .gitmessage << 'EOF'
# <type>(<scope>): <subject>
#
# <body>
#
# <footer>
#
# Types: feat, fix, docs, style, refactor, test, chore
# Example: feat(auth): add user authentication system
EOF
        git config commit.template .gitmessage
        log_success "Git commit template configured"
    fi
}

# Create necessary directories
setup_directories() {
    log_info "Creating project directories..."
    
    directories=(
        "cmd"
        "internal"
        "pkg"
        "api"
        "web"
        "docs"
        "test"
        "deployments"
    )
    
    for dir in "${directories[@]}"; do
        if [[ ! -d "$dir" ]]; then
            mkdir -p "$dir"
        fi
    done
    
    log_success "Project directories created"
}

# Run initial quality checks
run_quality_checks() {
    log_info "Running initial quality checks..."
    
    if command_exists pre-commit; then
        if pre-commit run --all-files; then
            log_success "All quality checks passed"
        else
            log_warning "Some quality checks failed - this is normal for initial setup"
            log_info "Run 'pre-commit run --all-files' to see and fix issues"
        fi
    fi
}

# Print helpful information
print_next_steps() {
    echo ""
    echo "======================================"
    log_success "Development environment setup complete!"
    echo "======================================"
    echo ""
    echo "Next steps:"
    echo "  1. Start developing your Go application"
    echo "  2. Create main.go to initialize the project"
    echo "  3. Run 'pre-commit run --all-files' to check code quality"
    echo "  4. Make your first commit with quality checks"
    echo ""
    echo "Useful commands:"
    echo "  pre-commit run --all-files  # Run all quality checks"
    echo "  go mod tidy                 # Clean up dependencies"
    echo "  go test ./...               # Run tests"
    echo "  golangci-lint run           # Run Go linter"
    echo ""
    echo "Documentation:"
    echo "  README.md                   # Project overview"
    echo "  .github/CONTRIBUTING.md     # Contribution guidelines"
    echo "  CLAUDE.md                   # Claude configuration"
    echo ""
    log_info "Happy coding! Remember: 'I am because we are' - Ubuntu"
}

# Main execution
main() {
    echo ""
    log_info "Setting up Kartoza Timesheet App development environment..."
    echo ""
    
    # Run setup functions
    check_go
    setup_precommit
    setup_go
    install_dev_tools
    setup_git
    setup_directories
    run_quality_checks
    
    # Show next steps
    print_next_steps
}

# Handle interrupts gracefully
trap 'log_error "Setup interrupted by user"; exit 1' INT

# Run main function
main "$@"