# Go Timesheets Go

<div align="center">

![Go](https://img.shields.io/badge/go-%2300ADD8.svg?style=for-the-badge&logo=go&logoColor=white)
![Terminal](https://img.shields.io/badge/Terminal-TUI-green?style=for-the-badge)
![License](https://img.shields.io/github/license/kartoza/go-timesheets-go?style=for-the-badge)

**A beautiful terminal-based timesheet application built with Go and Bubbletea**


</div>

## ✨ Features

- 🚀 **Beautiful TUI** - Terminal user interface with responsive design
- ⏱️ **Time Tracking** - Start and stop time tracking for projects and activities
- 📊 **Project Management** - Create and manage projects with tasks
- 🖥️ **CLI Commands** - Command-line interface for automation
- 📱 **Waybar Integration** - Desktop status bar integration with JSON output
- 💾 **Data Persistence** - Local JSON-based storage
- 🎯 **Activity Types** - Categorize work with activities (Coding, Planning, etc.)

## 🚀 Quick Start

### Installation

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

### Usage

```bash
# Start time tracking
go-timesheets-go start "Project Name" "Activity"

# Start with task
go-timesheets-go start "WB GEEST 2" "Coding" "Task 3: Improved Functionalities"

# Stop tracking
go-timesheets-go stop

# Get status (for waybar/desktop integration)
go-timesheets-go status

# Launch interactive TUI
go-timesheets-go
```

## 🖥️ Desktop Integration

### Waybar Configuration

Add to your waybar config:

```json
{
    "custom/timesheet": {
        "exec": "go-timesheets-go status",
        "return-type": "json", 
        "interval": 5,
        "on-click": "go-timesheets-go"
    }
}
```

### Status Output

```json
{
    "text": "🔴 01:23:45",
    "alt": "recording", 
    "tooltip": "Recording: WB GEEST 2\nActivity: Coding\nSession: 01:23:45\nToday: 6.5h",
    "class": "recording"
}
```

## 📱 Screenshots

<div align="center">

### Time Entry Interface
![Timesheet Entry](timesheet-entry.png)

### Daily Listing View  
![Timesheet Listing](timesheet-listing.png)

### Submission Workflow
![Timesheet Submission](timesheet-submission.png)

</div>

## 🏗️ Project Structure

```
go-timesheets-go/
├── cmd/                    # CLI command definitions
├── internal/
│   ├── models/            # Data models and structures
│   ├── service/           # Business logic layer
│   ├── storage/           # Data persistence layer
│   └── tui/               # Terminal user interface
├── scripts/               # Utility scripts
└── main.go               # Application entry point
```

## 🔧 Development

### Requirements

- Go 1.21+
- Terminal with color support

### Building

```bash
# Install dependencies
go mod download

# Run development version
go run . --help

# Build for production
go build -o go-timesheets-go .

# Run tests (when implemented)
go test ./...
```

## 📚 Documentation

- [Complete Feature Documentation](README-APP.md)
- [Installation Guide](README-APP.md#installation)
- [CLI Reference](README-APP.md#cli-commands)
- [Desktop Integration](README-APP.md#waybar-integration)

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests (when test framework exists)
5. Submit a pull request

## 📄 License

MIT License - see [LICENSE](LICENSE) for details.

## 🙏 Acknowledgments

- Built with [Charm](https://charm.sh/) TUI libraries
- Created for the Kartoza team 

## 🔮 Roadmap

- [x] Basic time tracking functionality
- [x] CLI commands for automation
- [x] Waybar integration
- [x] Simple TUI interface
- [ ] Advanced TUI with charts and tables
- [ ] Workspace automation
- [ ] ERP integration
- [ ] Mobile companion app

---

