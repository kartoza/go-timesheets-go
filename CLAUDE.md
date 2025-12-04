# Claude Configuration for Kartoza Timesheet App

This file contains configuration and context for Claude to ensure consistent and productive future sessions.

## Project Overview

**Name:** Kartoza Timesheet App  
**Type:** Go application for timesheet management  
**Philosophy:** Ubuntu - "I am because we are"  
**License:** MIT  
**Repository:** timlinux/kartoza-timesheet-app  

## Project Structure

```
kartoza-timesheet-app/
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

**Next Phase:** Core Go application development

## Important Guidelines for Claude

1. **Always update PROMPT.log** with new prompts and actions taken
2. **Maintain quality standards** - all code must pass pre-commit hooks
3. **Follow Ubuntu philosophy** - collaborative, inclusive approach
4. **Use established patterns** - follow existing code style and structure
5. **Update documentation** as features are implemented
6. **Run tests and quality checks** before committing code
7. **Keep security in mind** - no secrets in code, security-first approach

## Future Development Priorities

1. **Requirements Documentation** - Create detailed REQUIREMENTS.md
2. **Brand Guidelines** - Complete .github/BRAND.md with logo processing
3. **Go Module Initialization** - Set up go.mod and basic project structure
4. **Core API Design** - Define REST API endpoints and data models
5. **Database Schema** - Design and implement database structure
6. **Authentication System** - User management and security
7. **Web Interface** - Frontend dashboard development
8. **Testing Framework** - Comprehensive test coverage
9. **Documentation** - API docs, user guides, deployment guides
10. **Deployment** - Docker, CI/CD, production setup

## Contact Information

- **Maintainer:** Tim Sutton (tim@kartoza.com)
- **Organization:** Kartoza
- **Repository:** https://github.com/timlinux/kartoza-timesheet-app

---

*This file should be updated as the project evolves to maintain context for future Claude sessions.*