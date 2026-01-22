# Changelog

All notable changes to Go Timesheets Go will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [0.6.0] - 2026-01-22

### Added

#### Favourites Feature
- **Quick-start 3x3 grid** - New Favourites screen with 9 configurable slots for instant timer starting
- **Preset configurations** - Each favourite slot stores a project/task/activity combination for one-click timer starts
- **Edit mode** - Press `e` to enter edit mode and configure favourite slots with custom names and associations
- **Number key navigation** - Press `1-9` to quickly select any slot in the grid
- **Auto-stop previous timer** - When starting a timer from favourites, any running timer is automatically stopped
- **Git commit auto-fill** - When stopping a timer, if the project has a linked git repository, the description is automatically populated with commit messages from the work session
- **POW mode toggle** - Press `p` in the favourites screen to toggle Proof of Work screenshot capture mode

### Changed
- Main menu now includes "Favourites" option for quick access to the new feature

---

## [0.5.0] - 2026-01-20

### Added

#### Proof of Work Video Linking
- **POW videos now linked to timesheet entries** - When you stop a timer with POW mode enabled, the generated timelapse video is automatically linked to your timesheet entry
- **Visual indicator in history list** - Entries with POW videos display a 🎬 icon in the status column for quick identification
- **POW video details in entry view** - View the linked video path in the entry detail view
- **One-key video playback** - Press `p` in the detail view to instantly play the POW video with your system's default player (supports mpv, vlc, totem, celluloid, xdg-open)

#### Crash Recovery
- **Panic handler with crash logging** - Application crashes are now captured and written to `~/.local/share/kartoza-timesheet/crash.log` with full stack traces for easier debugging

### Changed

#### Waybar Integration
- **Nerd Font icons** - Replaced emoji icons with Nerd Font icons in waybar status output for better terminal compatibility and consistent rendering

### Fixed

- **Status command cache** - Fixed caching behavior for the status command to ensure accurate waybar updates
- **Exit splash screen** - Added proper splash screen on application exit

---

## [0.4.0] - 2026-01-19

### Added
- Waybar widget integration with JSON status output
- Splash screen with logo on startup
- Comprehensive test suite
- Exit splash screen animation

### Changed
- Improved overall TUI responsiveness

---

## [0.3.1] - 2026-01-18

### Fixed
- Updated vendorHash for go modules in Nix flake

---

## [0.3.0] - 2026-01-17

### Added
- Office view with avatar support
- POW mode toggle with friendly status messages
- Cross-platform screenshot support (grim, scrot, gnome-screenshot, import, screencapture, PowerShell)

### Changed
- Removed Ubuntu branding references
- Improved sprite colors in office view

---

## [0.2.0] - 2026-01-15

### Added
- Proof of Work (POW) screenshot capture mode
- Automatic timelapse video generation from screenshots
- Timer start/stop with POW integration

---

## [0.1.0] - 2026-01-10

### Added
- Initial release
- Beautiful TUI interface with Bubbletea
- Project, Task, and Activity selection
- Timer start/stop functionality
- Timesheet history view with editing
- API integration with ERPNext timesheets
- Weekly time charts and reports
- Timesheet submission workflow
- Celebration mode with fireworks
- Git commit integration for descriptions
- Code repository linking

---

[0.6.0]: https://github.com/kartoza/go-timesheets-go/compare/v0.5.0...v0.6.0
[0.5.0]: https://github.com/kartoza/go-timesheets-go/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/kartoza/go-timesheets-go/compare/v0.3.1...v0.4.0
[0.3.1]: https://github.com/kartoza/go-timesheets-go/compare/v0.3.0...v0.3.1
[0.3.0]: https://github.com/kartoza/go-timesheets-go/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/kartoza/go-timesheets-go/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/kartoza/go-timesheets-go/releases/tag/v0.1.0
