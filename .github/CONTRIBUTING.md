# Contributing to Kartoza Timesheet App

Thank you for your interest in contributing to the Kartoza Timesheet App! We appreciate your help in making this project better.

## Getting Started

### Prerequisites

- Go 1.21 or later
- Git
- Pre-commit (for development)

### Development Setup

1. Clone the repository:
   ```bash
   git clone https://github.com/timlinux/kartoza-timesheet-app.git
   cd kartoza-timesheet-app
   ```

2. Run the development setup script:
   ```bash
   ./scripts/setup-dev.sh
   ```

This will install pre-commit hooks and set up your development environment.

## How to Contribute

### Reporting Bugs

1. Check if the bug has already been reported in [Issues](https://github.com/timlinux/kartoza-timesheet-app/issues)
2. If not, create a new issue using the bug report template
3. Provide as much detail as possible, including:
   - Operating system and version
   - Go version
   - Steps to reproduce
   - Expected vs actual behavior

### Suggesting Features

1. Check if the feature has already been suggested in [Issues](https://github.com/timlinux/kartoza-timesheet-app/issues)
2. If not, create a new issue using the feature request template
3. Explain the problem your feature would solve
4. Describe your proposed solution

### Pull Requests

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/your-feature-name`
3. Make your changes
4. Run the quality checks: `pre-commit run --all-files`
5. Commit your changes with a descriptive message
6. Push to your fork: `git push origin feature/your-feature-name`
7. Create a Pull Request using the PR template

## Development Guidelines

### Code Style

We use automated tools to maintain code quality:

- **Go**: `gofmt`, `goimports`, `golangci-lint`
- **Markdown**: `markdownlint`
- **General**: `detect-secrets` for security scanning

All code must pass pre-commit hooks before being committed.

### Commit Messages

Use conventional commit format:
- `feat:` for new features
- `fix:` for bug fixes
- `docs:` for documentation changes
- `refactor:` for code refactoring
- `test:` for adding tests

### Testing

- Write tests for new features
- Ensure all tests pass before submitting PR
- Aim for good test coverage

## Quality Assurance

This project maintains high quality standards through:

- Pre-commit hooks for local development
- GitHub Actions CI/CD for automated testing
- Code formatting and linting
- Security scanning
- Documentation standards

## Community

- Be respectful and inclusive
- Follow our [Code of Conduct](.github/CODE_OF_CONDUCT.md)
- Help others learn and grow
- Embrace the Ubuntu philosophy: "I am because we are"

## Questions?

If you have questions about contributing, feel free to:
- Open an issue
- Email the maintainers
- Join our community discussions

Thank you for contributing to Kartoza Timesheet App!