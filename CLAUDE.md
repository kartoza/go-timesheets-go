# Claude Configuration for Kartoza Timesheet App

This file contains configuration and context for Claude to ensure consistent and productive future sessions.

## Project Overview

**Name:** Go Timesheets Go  
**Type:** Go TUI timesheet application for terminal-based time tracking  
**Philosophy:** Ubuntu - "I am because we are"  
**License:** MIT  
**Repository:** kartoza/go-timesheets-go  

## Project Structure

```
go-timesheets-go/
├── .github/                 # GitHub templates, workflows, documentation
│   ├── workflows/           # CI/CD GitHub Actions
│   ├── ISSUE_TEMPLATE/      # Bug reports and feature requests
│   ├── assets/              # Project assets (logos, images)
│   ├── CONTRIBUTING.md      # Contribution guidelines
│   ├── CODE_OF_CONDUCT.md   # Community standards
│   ├── SECURITY.md          # Security policy
│   ├── SUPPORT.md           # Support information
│   ├── CHANGELOG.md         # Version history
│   ├── BRAND.md             # Brand guidelines (to be created)
│   └── PULL_REQUEST_TEMPLATE.md
├── scripts/                 # Development and deployment scripts
│   └── setup-dev.sh         # Development environment setup
├── .pre-commit-config.yaml  # Quality assurance hooks
├── .markdownlint.yaml       # Markdown linting rules
├── .secrets.baseline        # Security scanning baseline
├── .gitignore               # Git ignore patterns
├── README.md                # Project overview and documentation
├── LICENSE                  # MIT license
├── PROMPT.log               # Claude prompt history
├── CLAUDE.md                # This file - Claude configuration
└── REQUIREMENTS.md          # Detailed project requirements (to be created)
```

## Technology Stack

- **Primary Language:** Go 1.21+
- **Database:** TBD (PostgreSQL/SQLite planned)
- **Frontend:** TBD (Web-based dashboard planned)
- **Infrastructure:** Docker, GitHub Actions
- **Quality Tools:** Pre-commit hooks, golangci-lint, Trivy

## Quality Standards

### Pre-commit Hooks Configured
- **Go:** gofmt, goimports, golangci-lint, go-vet, go-mod-tidy
- **Markdown:** markdownlint with custom config
- **Shell:** shellcheck, beautysh
- **Security:** detect-secrets, Trivy scanning
- **General:** trailing whitespace, end-of-file fixer, merge conflict detection

### CI/CD Pipeline
- Quality checks on all PRs
- Multi-version Go testing (1.20, 1.21)
- Cross-platform builds (Linux, macOS, Windows)
- Security scanning with Trivy
- Dependency vulnerability checking
- Code coverage reporting

## Development Commands

```bash
# Setup development environment
./scripts/setup-dev.sh

# Run quality checks locally
pre-commit run --all-files

# Build (when Go modules exist)
go build ./...

# Test (when tests exist)
go test -v -race -coverprofile=coverage.out ./...

# Format code
gofmt -s -w .
goimports -w .

# Lint
golangci-lint run
```

## Key Files and Their Purpose

| File | Purpose |
|------|---------|
| `PROMPT.log` | Track all user prompts and actions taken |
| `REQUIREMENTS.md` | Detailed project specifications (to be created) |
| `.pre-commit-config.yaml` | Quality assurance configuration |
| `scripts/setup-dev.sh` | Development environment setup |
| `.github/workflows/ci.yml` | Continuous integration pipeline |
| `.github/BRAND.md` | Brand guidelines and design system |

## Brand Identity

- **Colors:** Ubuntu Orange (#E95420) and complementary colors from logo
- **Philosophy:** Ubuntu - collaborative, inclusive, community-driven
- **Mascot:** Logo-based character for visual elements
- **Typography:** Ubuntu fonts and African-inspired alternatives

## Current Status

The project is in the **infrastructure setup phase**. The following components are complete:

✅ Git repository initialization  
✅ GitHub folder structure with templates  
✅ Quality assurance infrastructure  
✅ CI/CD pipeline configuration  
✅ Documentation framework  
✅ Community guidelines and policies  

**Status:** ✅ **COMPLETED** - Fully functional Go TUI timesheet application

## Important Guidelines for Claude

1. **Always update PROMPT.log** with new prompts and actions taken
2. **Maintain quality standards** - all code must pass pre-commit hooks
3. **Follow Ubuntu philosophy** - collaborative, inclusive approach
4. **Use established patterns** - follow existing code style and structure
5. **Update documentation** as features are implemented
6. **Run tests and quality checks** before committing code
7. **Keep security in mind** - no secrets in code, security-first approach

## Completed Implementation

✅ **Core Application** - Complete Go TUI timesheet application
✅ **Data Models** - Project, Task, Activity, TimeEntry structures  
✅ **Storage Layer** - JSON-based persistence system
✅ **Service Layer** - Business logic for timesheet operations
✅ **TUI Interface** - Terminal user interface with Bubbletea
✅ **CLI Commands** - start, stop, status commands for automation
✅ **Waybar Integration** - JSON status output for desktop integration
✅ **Sample Data** - Setup script for testing and demonstration
✅ **Documentation** - Comprehensive README and usage guides
✅ **GitHub Repository** - Published under kartoza/go-timesheets-go
✅ **Favourites** - Quick-start 3x3 grid with preset project/task/activity combinations

## Future Enhancement Priorities

1. **Workspace Automation** - Virtual desktop integration for automatic tracking
2. **ERP Integration** - Submit timesheets to ERPNext
3. **Testing Framework** - Comprehensive test coverage
4. **CI/CD Pipeline** - GitHub Actions workflows
5. **Advanced Reporting** - Time analytics and insights
6. **Team Features** - Multi-user support and collaboration
7. **Mobile Integration** - Companion mobile application
8. **Export Capabilities** - PDF, CSV, Excel export formats
9. **Performance Optimization** - Large dataset handling

## Contact Information

- **Maintainer:** Tim Sutton (tim@kartoza.com)
- **Organization:** Kartoza
- **Repository:** https://github.com/kartoza/go-timesheets-go

---

*This file should be updated as the project evolves to maintain context for future Claude sessions.*
- since we are maintaining parallel implementations of a tui and a gui, they should stay in lock step feature and functionality wise. They should use as much common code for non ui functions as possible to remain DRY whilst replicating the UI functionality in both environments.