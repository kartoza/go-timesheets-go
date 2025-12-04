# Kartoza Timesheet App

![Project Logo](/.github/assets/LOGO.png)

> *"I am because we are"* - Ubuntu Philosophy

A modern timesheet application built with Go, designed to help teams track their work efficiently while embracing the Ubuntu philosophy of collaborative development.

## Project Status

[![CI](https://github.com/timlinux/kartoza-timesheet-app/workflows/CI/badge.svg)](https://github.com/timlinux/kartoza-timesheet-app/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/timlinux/kartoza-timesheet-app/branch/main/graph/badge.svg)](https://codecov.io/gh/timlinux/kartoza-timesheet-app)
[![Go Report Card](https://goreportcard.com/badge/github.com/timlinux/kartoza-timesheet-app)](https://goreportcard.com/report/github.com/timlinux/kartoza-timesheet-app)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/release/timlinux/kartoza-timesheet-app.svg)](https://github.com/timlinux/kartoza-timesheet-app/releases/latest)

### Quality Assurance

[![Pre-commit](https://img.shields.io/badge/pre--commit-enabled-brightgreen?logo=pre-commit&logoColor=white)](https://github.com/pre-commit/pre-commit)
[![Security Rating](https://sonarcloud.io/api/project_badges/measure?project=timlinux_kartoza-timesheet-app&metric=security_rating)](https://sonarcloud.io/dashboard?id=timlinux_kartoza-timesheet-app)
[![Maintainability Rating](https://sonarcloud.io/api/project_badges/measure?project=timlinux_kartoza-timesheet-app&metric=sqale_rating)](https://sonarcloud.io/dashboard?id=timlinux_kartoza-timesheet-app)
[![Reliability Rating](https://sonarcloud.io/api/project_badges/measure?project=timlinux_kartoza-timesheet-app&metric=reliability_rating)](https://sonarcloud.io/dashboard?id=timlinux_kartoza-timesheet-app)

## Features

- [ ] User authentication and authorization
- [ ] Project and task management
- [ ] Time tracking with start/stop functionality
- [ ] Reporting and analytics
- [ ] Team collaboration features
- [ ] Export capabilities (PDF, CSV, Excel)
- [ ] REST API for integrations
- [ ] Web-based dashboard
- [ ] Mobile-responsive design

## Quick Start

### Prerequisites

- Go 1.21 or later
- Git
- Pre-commit (for development)

### Installation

```bash
# Clone the repository
git clone https://github.com/timlinux/kartoza-timesheet-app.git
cd kartoza-timesheet-app

# Set up development environment
./scripts/setup-dev.sh

# Run the application (when implemented)
go run main.go
```

### Development Setup

1. **Clone and Setup:**
   ```bash
   git clone https://github.com/timlinux/kartoza-timesheet-app.git
   cd kartoza-timesheet-app
   ./scripts/setup-dev.sh
   ```

2. **Install Pre-commit Hooks:**
   ```bash
   pre-commit install
   ```

3. **Run Quality Checks:**
   ```bash
   pre-commit run --all-files
   ```

## Documentation

### Core Documentation
- [Contributing Guidelines](./.github/CONTRIBUTING.md) - How to contribute to the project
- [Code of Conduct](./.github/CODE_OF_CONDUCT.md) - Community standards and behavior
- [Brand Guidelines](./.github/BRAND.md) - Visual identity and design standards
- [Requirements](./REQUIREMENTS.md) - Detailed project requirements and specifications

### Development & Operations
- [Security Policy](./.github/SECURITY.md) - Security reporting and policies
- [Support](./.github/SUPPORT.md) - Getting help and support resources
- [Changelog](./.github/CHANGELOG.md) - Version history and changes
- [Roadmap](./.github/ROADMAP.md) - Future plans and development priorities

### Templates & Processes
- [Bug Report Template](./.github/ISSUE_TEMPLATE/bug_report.md)
- [Feature Request Template](./.github/ISSUE_TEMPLATE/feature_request.md)
- [Pull Request Template](./.github/PULL_REQUEST_TEMPLATE.md)

## Quality Assurance

This project maintains high quality standards through:

### Automated Quality Checks
- **Pre-commit hooks** for local development quality
- **GitHub Actions CI/CD** for automated testing and validation
- **Code formatting and linting** for all supported file types:
  - Go: `gofmt`, `goimports`, `golangci-lint`, `go vet`
  - Markdown: `markdownlint` with custom configuration
  - Shell scripts: `shellcheck` and `beautysh`
  - Nix files: `nixfmt` formatting
  - Python: `black`, `isort`, `flake8`, `mypy`
  - HTML/CSS: `djLint` and `prettier`

### Security & Compliance
- **Secret detection** with `detect-secrets`
- **Vulnerability scanning** with Trivy
- **Dependency checking** for known security issues
- **SARIF integration** for security findings

### Testing Strategy
- Unit tests with race detection
- Integration tests for API endpoints
- End-to-end testing for critical workflows
- Performance testing for scalability
- Security testing for vulnerability assessment

## Technology Stack

- **Backend:** Go 1.21+
- **Database:** TBD (PostgreSQL/SQLite planned)
- **Frontend:** TBD (Web-based dashboard)
- **Infrastructure:** Docker, GitHub Actions
- **Quality Tools:** Pre-commit, golangci-lint, Trivy

## Contributing

We welcome contributions! Please see our [Contributing Guidelines](./.github/CONTRIBUTING.md) for details on:

- Development setup and workflow
- Code style and quality standards
- Testing requirements
- Pull request process
- Community guidelines

### Quick Contribution Steps

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/your-feature-name`
3. Make your changes and commit with quality checks
4. Push to your fork and submit a pull request

## Community

- **Ubuntu Philosophy:** We embrace "Ubuntu" - the belief that "I am because we are"
- **Inclusive:** All contributors are welcome regardless of experience level
- **Collaborative:** We work together to build something amazing
- **Quality-focused:** We maintain high standards while being supportive

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Support

- **Issues:** [GitHub Issues](https://github.com/timlinux/kartoza-timesheet-app/issues)
- **Discussions:** [GitHub Discussions](https://github.com/timlinux/kartoza-timesheet-app/discussions)
- **Email:** [tim@kartoza.com](mailto:tim@kartoza.com)

---

*Built with ❤️ by the Kartoza team and community contributors*