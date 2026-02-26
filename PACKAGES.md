# Go Timesheets Go - Package Structure

This document provides an annotated list of all packages in the software architecture, enabling developers to understand the codebase organization and locate functionality.

---

## Table of Contents

1. [Root Package](#root-package)
2. [Command Package (cmd/)](#command-package-cmd)
3. [Internal Packages](#internal-packages)
   - [AI Package (internal/ai/)](#ai-package-internalai)
   - [API Package (internal/api/)](#api-package-internalapi)
   - [Config Package (internal/config/)](#config-package-internalconfig)
   - [GUI Package (internal/gui/)](#gui-package-internalgui)
   - [Models Package (internal/models/)](#models-package-internalmodels)
   - [Monitoring Package (internal/monitoring/)](#monitoring-package-internalmonitoring)
   - [Office Package (internal/office/)](#office-package-internaloffice)
   - [POW Package (internal/pow/)](#pow-package-internalpow)
   - [Service Package (internal/service/)](#service-package-internalservice)
   - [Storage Package (internal/storage/)](#storage-package-internalstorage)
   - [TUI Package (internal/tui/)](#tui-package-internaltui)
4. [Scripts](#scripts)

---

## Root Package

### `main.go`
**Purpose:** Application entry point
**Responsibility:** Initializes the application and delegates to the command package for CLI handling.

---

## Command Package (cmd/)

The command package implements the CLI interface using Cobra. Each file represents a command or command group.

| File | Command | Description |
|------|---------|-------------|
| `root.go` | `kartoza-timesheet` | Root command, sets up global flags and configuration |
| `start.go` | `start` | Start a new time tracking session |
| `stop.go` | `stop` | Stop the current time tracking session |
| `status.go` | `status` | Show current tracking status (supports JSON/Waybar output) |
| `status_test.go` | - | Unit tests for status command |
| `current.go` | `current` | Display the currently running time entry |
| `list.go` | `list` | List time entries for a given date range |
| `today.go` | `today` | Show today's time entries |
| `projects.go` | `projects` | List available projects |
| `import.go` | `import` | Import time entries from CSV files |
| `pow.go` | `pow` | Proof-of-work recording commands |
| `list_pow.go` | `list-pow` | List proof-of-work recordings |
| `play_pow.go` | `play-pow` | Play back proof-of-work recordings |
| `monitor.go` | `monitor` | Start the monitoring/metrics server |
| `office.go` | `office` | Launch the virtual office visualization |
| `celebration.go` | `celebration` | Trigger celebration animations |

---

## Internal Packages

### AI Package (internal/ai/)

**Purpose:** Artificial intelligence and machine learning functionality for smart project matching and analysis.

| File | Description |
|------|-------------|
| `engine.go` | Core AI engine that coordinates analysis and matching |
| `analysis.go` | Code analysis utilities for understanding work context |
| `matcher.go` | Project/task matching algorithms using fuzzy matching |
| `types.go` | Shared type definitions for AI components |
| `domain_knowledge.go` | Domain-specific knowledge base for better matching |
| `ollama.go` | Integration with Ollama LLM for natural language processing |
| `embedded_llm.go` | Embedded LLM support (production build) |
| `embedded_llm_stub.go` | Stub for embedded LLM (development build) |
| `nn_model.go` | Neural network model definitions |
| `nn_tokenizer.go` | Text tokenization for neural network input |
| `nn_trainer.go` | Training utilities for the neural network |
| `csv_import.go` | AI-assisted CSV import with smart field mapping |
| `csv_import_test.go` | Tests for CSV import functionality |

---

### API Package (internal/api/)

**Purpose:** External API clients for communicating with remote services.

| File | Description |
|------|-------------|
| `client.go` | ERPNext API client for timesheet operations. Handles authentication, CRUD operations for time entries, projects, tasks, and activities. Implements request deduplication and caching. |
| `github.go` | GitHub API client for repository information and commit history |
| `git_local.go` | Local git repository utilities for reading commit history |
| `google_calendar.go` | Google Calendar API client with OAuth 2.0 authentication. Fetches calendar events for gap-filling suggestions. Handles token storage and refresh. |

---

### Config Package (internal/config/)

**Purpose:** Configuration management and persistence.

| File | Description |
|------|-------------|
| `config.go` | Configuration structures and loading/saving. Includes `GoogleCalendarConfig` for OAuth credentials and tokens. Handles config file paths and defaults. |

---

### GUI Package (internal/gui/)

**Purpose:** Fyne-based graphical user interface implementation.

#### Core Files

| File | Description |
|------|-------------|
| `app.go` | Main GUI application orchestrator. Manages window, navigation, screens, and state. |
| `detect.go` | Display environment detection (X11/Wayland) for auto-selecting GUI vs TUI |
| `systray.go` | System tray integration for background operation |
| `theme.go` | Kartoza-branded Fyne theme with custom colors |

#### Screens (internal/gui/screens/)

| File | Description |
|------|-------------|
| `login.go` | Login screen with ERPNext authentication |
| `dashboard.go` | Main dashboard with current status, timer, and quick actions |
| `history.go` | Time entry history with two-column master-detail layout |
| `gaps.go` | Gap analysis view showing 14-day visualization with clickable segments |
| `favourites.go` | Favourites management screen for quick-start presets |
| `settings.go` | Settings menu with navigation to sub-screens |
| `calendar_settings.go` | Google Calendar OAuth configuration screen |
| `code_repos.go` | Code repository management for git-based tracking |
| `ai_assistant.go` | AI-powered project matching assistant |
| `office.go` | Virtual office visualization screen |

#### Widgets (internal/gui/widgets/)

| File | Description |
|------|-------------|
| `entry_form_dialog.go` | Modal dialog for creating/editing time entries. Supports calendar panel integration. |
| `calendar_panel.go` | Google Calendar events panel for gap-filling. Shows day's appointments with click-to-fill functionality. |
| `time_entry_card.go` | Card widget displaying a single time entry |
| `timer_display.go` | Running timer display with elapsed time |
| `progress_gauge.go` | Daily/weekly progress gauge visualization |
| `favourite_button.go` | Favourite preset button widget |
| `project_picker.go` | Searchable project/task/activity picker |
| `bounded_select.go` | Bounded dropdown select widget |
| `color_picker.go` | Color selection widget |
| `fireworks.go` | Celebration fireworks animation |
| `isometric_office.go` | Isometric office visualization widget |

#### Utilities (internal/gui/util/)

| File | Description |
|------|-------------|
| `async.go` | Async operation utilities for non-blocking UI updates |

---

### Models Package (internal/models/)

**Purpose:** Data structures and domain models shared across the application.

| File | Description |
|------|-------------|
| `models.go` | Core data models: `TimeEntry`, `Project`, `Task`, `Activity`, `User` |
| `models_test.go` | Tests for core models |
| `favourites.go` | `Favourite` and `FavouritesStore` for quick-start presets |
| `code_repo.go` | `CodeRepo` model for git repository tracking |
| `code_repo_test.go` | Tests for code repo model |
| `calendar.go` | `CalendarEvent` model for Google Calendar integration. Contains event title, start/end times, and description. |

---

### Monitoring Package (internal/monitoring/)

**Purpose:** Prometheus metrics and monitoring server.

| File | Description |
|------|-------------|
| `metrics.go` | Prometheus metric definitions and collectors |
| `server.go` | HTTP server for metrics endpoint |

---

### Office Package (internal/office/)

**Purpose:** Virtual office visualization showing team status.

| File | Description |
|------|-------------|
| `run.go` | Office visualization entry point and main loop |
| `scene.go` | Scene rendering and layout management |
| `team.go` | Team member data and status fetching |
| `avatars.go` | Avatar sprite management |
| `sprites.go` | Sprite rendering utilities |

---

### POW Package (internal/pow/)

**Purpose:** Proof-of-work (screen recording) functionality.

| File | Description |
|------|-------------|
| `pow.go` | Screen recording for proof-of-work documentation. Records work sessions with configurable intervals. Supports pause/resume and session management. |

---

### Service Package (internal/service/)

**Purpose:** Business logic layer shared between GUI and TUI implementations.

| File | Description |
|------|-------------|
| `timesheet.go` | Core timesheet business logic: CRUD operations, validation, submission |
| `gaps.go` | Gap analysis service: calculates untracked time periods, generates 14-day visualizations |
| `calendar.go` | Calendar service layer: wraps Google Calendar API client, handles event fetching for specific dates, manages OAuth flow |

---

### Storage Package (internal/storage/)

**Purpose:** Local data persistence and caching.

| File | Description |
|------|-------------|
| `storage.go` | JSON-based local storage for config, cache, and favourites |
| `storage_test.go` | Tests for storage operations |

---

### TUI Package (internal/tui/)

**Purpose:** Bubbletea-based terminal user interface implementation.

#### Core Application Files

| File | Description |
|------|-------------|
| `app_with_auth.go` | Main TUI application with authentication state machine. Manages all screen states and navigation. |
| `enhanced_app.go` | Enhanced application wrapper with additional features |
| `main_menu.go` | Main menu screen with navigation options |
| `splash.go` | Splash screen with branding |
| `welcome.go` | Welcome/onboarding screen |

#### Screen Implementations

| File | Description |
|------|-------------|
| `dashboard.go` | Dashboard view with current status and quick actions |
| `timesheet.go` | Time entry form for creating/editing entries |
| `history.go` | Time entry history browser with master-detail layout |
| `gaps.go` | Gap analysis visualization with 14-day bars |
| `favourites.go` | Favourites management with 3x3 grid |
| `settings.go` | Settings menu with sub-screen navigation |
| `calendar_panel.go` | Google Calendar integration: `CalendarPanel` for displaying events, `CalendarSettingsModel` for OAuth configuration |
| `code_repos.go` | Code repository management |
| `ai_assistant.go` | AI-powered project matching assistant |
| `fireworks.go` | Celebration fireworks animation |

#### Utilities

| File | Description |
|------|-------------|
| `widgets.go` | Reusable TUI widget components |
| `debug_dev.go` | Debug utilities (development build) |
| `debug_release.go` | Debug utilities stub (release build) |

---

## Scripts

| File | Description |
|------|-------------|
| `scripts/setup-sample-data.go` | Generates sample data for testing and demonstration |

---

## Package Dependencies

```
main.go
  └── cmd/
       └── internal/
            ├── api/          (external API clients)
            ├── config/       (configuration)
            ├── models/       (shared data structures)
            ├── service/      (business logic)
            │    └── api/
            ├── storage/      (local persistence)
            ├── gui/          (Fyne GUI)
            │    ├── screens/
            │    ├── widgets/
            │    └── util/
            ├── tui/          (Bubbletea TUI)
            ├── ai/           (AI/ML functionality)
            ├── office/       (virtual office)
            ├── pow/          (proof-of-work)
            └── monitoring/   (metrics)
```

---

## Adding New Packages

When adding a new package:

1. Create the package directory under `internal/`
2. Add package documentation in the main file
3. Update this PACKAGES.md document
4. Ensure the package follows the layered architecture (UI → Service → API/Storage)
5. Add tests with `_test.go` suffix
6. Update SPECIFICATION.md if the package introduces new features

---

*Last updated: 2026-02-25*
