# Kartoza Timesheet App


## Features

### ✅ Implemented

- **Time Tracking**: Start and stop time tracking with projects and activities
- **Project Management**: Create and manage projects and tasks
- **Data Persistence**: JSON-based local storage system
- **CLI Commands**: Command-line interface for automation
- **Waybar Integration**: Status output for desktop integration
- **Beautiful TUI**: Terminal user interface with responsive design

### 🚧 Core Functionality

#### Time Entry
- Start/stop time tracking for specific projects and activities
- Optional task association and descriptions
- Automatic duration calculation
- Active session indicators

#### Data Models
- **Projects**: Named containers with descriptions and tasks
- **Tasks**: Project-specific work items with time estimates
- **Activities**: Work types (Coding, Planning, Review, etc.)
- **Time Entries**: Individual time tracking sessions
- **Submissions**: Batched timesheet submissions

#### CLI Commands

```bash
# Start time tracking
kartoza-timesheet start "Project Name" "Activity" ["Optional Task"]

# Stop time tracking
kartoza-timesheet stop

# Get status for waybar/desktop integration
kartoza-timesheet status

# Launch interactive TUI
kartoza-timesheet
```

#### Waybar Integration

Add to your waybar configuration:

```json
{
    "custom/timesheet": {
        "exec": "kartoza-timesheet status",
        "return-type": "json",
        "interval": 5,
        "on-click": "kartoza-timesheet"
    }
}
```

## Installation

### Build from Source

```bash
# Clone the repository
git clone https://github.com/kartoza/go-timesheets-go.git
cd go-timesheets-go

# Build the application
go build -o go-timesheets-go .

# Setup sample data (optional)
go run scripts/setup-sample-data.go

# Install to system (optional)
sudo cp go-timesheets-go /usr/local/bin/
```

### Development Setup

```bash
# Install dependencies
go mod download

# Run with development hot reload
go run . --help

# Run sample data setup
go run scripts/setup-sample-data.go
```

## Usage Examples

### Basic Time Tracking

```bash
# Start working on a project
kartoza-timesheet start "WB GEEST 2 Enhancements" "Coding"

# Work on a specific task
kartoza-timesheet start "WB GEEST 2 Enhancements" "Coding" "Task 3: Improved Functionalities"

# Add description
kartoza-timesheet start "Planet Labs - Support" "Documentation" --description "API integration guide"

# Stop tracking
kartoza-timesheet stop
```

### Desktop Integration

The application provides JSON status output perfect for desktop bars:

```bash
$ kartoza-timesheet status
{"text":"🔴 01:23:45","alt":"recording","tooltip":"Recording time\nProject: WB GEEST 2\nActivity: Coding\nCurrent session: 01:23:45\nToday's total: 6.50h","class":"recording"}
```

Status classes:
- `recording`: Time is being tracked
- `idle`: No active tracking
- `error`: Error occurred

### Interactive TUI

Launch the terminal UI for full functionality:

```bash
kartoza-timesheet
```

Navigation:
- `1`, `2`, `3`: Switch between Entry, Listing, and Submission views
- `Enter`: Start/stop time tracking (Entry view)
- `h`, `?`: Toggle help
- `q`, `Ctrl+C`: Quit

## Project Structure

```
go-timesheets-go/
├── cmd/                    # CLI command definitions
│   ├── root.go            # Root command and flags
│   ├── start.go           # Time tracking start command
│   ├── stop.go            # Time tracking stop command
│   └── status.go          # Status command for waybar
├── internal/
│   ├── models/            # Data models
│   │   └── models.go      # Core data structures
│   ├── storage/           # Data persistence
│   │   └── storage.go     # JSON file storage
│   ├── service/           # Business logic
│   │   └── timesheet.go   # Timesheet service layer
│   └── tui/               # Terminal user interface
│       ├── simple_app.go  # Simplified TUI application
│       └── *.go.backup    # Complex TUI components (for future)
├── scripts/
│   └── setup-sample-data.go  # Sample data generator
├── main.go                # Application entry point
└── go.mod                # Go module definition
```

## Data Storage

The application stores data in `~/.kartoza-timesheet/`:

- `projects.json`: Project definitions
- `tasks.json`: Task definitions  
- `activities.json`: Activity types
- `time_entries.json`: Time tracking records
- `active_entry.json`: Currently active session

## Configuration

### Environment Variables

- `KARTOZA_DATA_DIR`: Override default data directory
- `KARTOZA_USER_ID`: Override default user identifier

### Command Line Flags

- `--data-dir`: Specify data directory
- `--user`: Specify user ID for time entries

## Technical Architecture

### Dependencies

- **Core**: Go 1.21+
- **TUI Framework**: github.com/charmbracelet/bubbletea
- **Forms**: github.com/charmbracelet/huh  
- **Styling**: github.com/charmbracelet/lipgloss
- **CLI**: github.com/spf13/cobra
- **UUID Generation**: github.com/google/uuid
- **YAML Support**: gopkg.in/yaml.v3

### Design Patterns

- **Service Layer**: Business logic separation
- **Repository Pattern**: Data access abstraction
- **Command Pattern**: CLI command structure
- **Model-View-Update**: Bubbletea architecture
- **Message Passing**: Event-driven UI updates

### Future Enhancements

1. **Advanced TUI**: Full implementation with complex tables and charts
2. **Workspace Automation**: Automatic time tracking based on virtual desktop
3. **ERP Integration**: Submit timesheets to ERPNext
4. **Reporting**: Time reports and analytics
5. **Team Features**: Multi-user support and collaboration
6. **Export Formats**: CSV, PDF, Excel export
7. **Time Estimation**: AI-powered time estimation
8. **Mobile Companion**: Mobile app integration

## Contributing

This project follows the Ubuntu philosophy of collaboration. Contributions are welcome!

### Development Guidelines

1. Follow Go conventions and best practices
2. Maintain test coverage for new features
3. Use conventional commit messages
4. Update documentation for new features
5. Ensure compatibility with existing data formats

### Roadmap

- [x] Basic time tracking functionality
- [x] CLI commands for automation
- [x] Waybar integration
- [x] Data persistence layer
- [x] Simple TUI interface
- [ ] Advanced TUI with charts and tables
- [ ] Workspace automation
- [ ] ERP integration
- [ ] Mobile companion app

## License

MIT License - see [LICENSE](LICENSE) for details.

## Acknowledgments

- Built with [Charm](https://charm.sh/) TUI libraries
- Designed for the Kartoza team 

---

