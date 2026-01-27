# Changelog

All notable changes to Go Timesheets Go will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]

---

## [0.10.0] - 2026-01-27

### Added

#### AI Assistant
- **Multi-level AI engine** - 4-level fallback chain: Embedded LLM (llama.cpp) → Ollama (local LLM server) → Neural Network (TF-IDF similarity) → Rule-based fuzzy matching
- **Natural language project discovery** - Ask questions like "Which project for GeoServer Docker?" to find the right project/task/activity combination
- **Timesheet analytics** - Ask analytical questions: "Which projects did I work on last week?", "How consistent was my timekeeping?", "Which project did I spend the most time on?"
- **Consistency analysis** - Detects rounded entries (exact hours, half-hours, quarter-hours) indicating retroactive rather than as-you-work recording
- **One-click timer start** - Select an AI-suggested match to immediately start a timer with the correct project/task/activity
- **Neural network training** - TF-IDF cosine similarity model automatically trains from your timesheet history
- **Ollama integration** - Connects to local Ollama LLM server for enhanced AI responses (no API key required)
- **Embedded LLM support** - Optional llama.cpp integration via `llamacpp` build tag for fully offline AI
- **GUI AI screen** - Full AI assistant interface in the Fyne GUI with query input, example queries, and clickable match results
- **TUI AI screen** - Full AI assistant interface in the Bubbletea TUI with keyboard navigation and example query shortcuts
- **Dashboard access** - AI button on dashboard (GUI) and 'a' shortcut key (TUI) for quick access

#### GUI Timesheet Creator
- **Date and time editing** - GUI timesheet form now includes editable Date, Start Time, and End Time fields matching TUI functionality
- **Completed entry support** - Provide an end time to submit a completed entry with calculated duration, or leave it blank to start a running timer
- **Context-aware submit button** - Button label changes between "Start Timer" and "Submit Entry" based on whether an end time is provided

#### GUI Stop Timer Dialog
- **Themed stop dialog** - Stop timer dialog now uses the same gold-bordered dark themed style as the timesheet creator
- **Editable fields** - Project, Task, Activity, Start Date/Time, and End Date/Time are all editable when stopping a timer
- **Pre-populated values** - All fields pre-populate from the active timer with end time defaulting to now
- **Full-width description** - Description field spans the full dialog width with Fetch from Git support
- **Validation** - Start/end time parsing and ordering validation with inline error messages

#### GUI Entry Detail Dialog
- **Consistent themed dialog** - History entry detail view now uses the same gold-bordered dark themed modal popup as the create and stop dialogs
- **Editable entry fields** - Non-submitted entries can be edited: Project, Task, Activity, Description, Start/End Date and Time
- **Pre-populated values** - All fields pre-populate from the selected entry's current data
- **Fetch from Git** - Description can be auto-populated with git commits from the entry's time range
- **Save Changes** - Save edits back to the API via UpdateTimesheet
- **Read-only for submitted** - Submitted entries display the same form layout with all fields disabled
- **Status display** - Status (Running/Pending/Submitted) and hours shown inline
- **POW video playback** - Play POW video button shown for entries with proof-of-work videos
- **Delete support** - Non-submitted entries can be deleted from the detail dialog

#### GUI Navigation
- **ESC key navigation** - Pressing ESC returns to the previous screen from any page (History→Dashboard, Settings→Dashboard, Favourites→Settings, etc.)
- **History list descriptions** - Entry cards in the history list now show the description text (truncated with ellipsis) below the task name

#### Multi-Instance Consistency (API-Canonical Architecture)
- **API is canonical** - All timesheet data operations now treat the remote API as the single source of truth
- **CLI start uses API** - The `start` command now checks for active timers and creates entries via the API instead of local files, preventing duplicate timers across instances
- **CLI status is cache-only** - The `status` command reads exclusively from the local timelog cache, generating zero API traffic even when polled frequently by waybar
- **CLI stop updates cache** - The `stop` command now updates the local timelog cache after stopping, keeping waybar status in sync
- **Atomic file writes** - All local storage writes (cache, favourites, associations, etc.) now use write-to-temp-then-rename to prevent corruption from concurrent access or crashes
- **Minimal API calls** - API is only called for state changes (start, stop, edit, delete) or explicit user requests (e.g. viewing history); cache is updated as a side-effect of these mutations
- **BreakTimesheet cache invalidation** - The `BreakTimesheet` API method now properly invalidates the in-memory timelog cache so subsequent reads get fresh data

---

## [0.9.1] - 2026-01-23

### Added

#### Cross-Platform Packages
- **Windows package** - ZIP archive with GUI executable (no console window)
- **macOS package** - Universal binary DMG supporting both Intel and Apple Silicon Macs
- **macOS tar.gz** - Separate Intel (amd64) and ARM64 archives for manual installation
- **Linux binary** - Standalone amd64 binary tar.gz for systems without package managers

### Changed
- Updated GitHub Actions workflow to build packages for all major platforms on release

---

## [0.9.0] - 2026-01-23

### Added

#### GUI Mode Enhancements
- **Consistent dialog styling** - Favourites editor now uses the same polished styling as the timesheet dialog with gold borders, dark backgrounds, and rounded corners
- **Height-bounded dropdowns** - Both task and activity selectors in the favourites editor now use `BoundedSelect` widget showing maximum 8 items to prevent overflow issues
- **Alphabetical activity sorting** - Activities are now sorted alphabetically in both the timesheet creator and favourites editor for easier navigation

#### Virtual Office Scene
- **Full-screen office view** - The isometric office scene now expands to fill the entire window instead of being a small centered area
- **Real team data integration** - Office scene fetches actual Kartoza team members from kartoza.com with proper fallback handling
- **Activity bubbles** - Each worker now displays a speech bubble showing what they're currently working on
- **Dynamic desk layout** - Desk positions automatically adjust based on screen size and number of team members
- **Floating back button** - Navigation overlays elegantly on the office scene

### Changed
- Favourites editor task/activity dropdowns changed from standard Select to BoundedSelect for better UX
- Office scene rendering now uses responsive layout calculations

### Fixed
- Fixed dropdown overflow in favourites editor when many options are available

---

## [0.8.0] - 2026-01-22

### Added

#### Streamlined Main Menu
- **Dashboard buttons** - Start/Stop Timer button in Active Timer box, History button in Daily Progress box
- **Settings submenu** - New Settings screen consolidating management options (Edit Favourites, Workspace Associations, Code Repos, Log Out)
- **Embedded favourites grid** - 3x3 favourites grid now displayed directly on main menu for instant access
- **New keyboard shortcuts** - `s` for Start/Stop timer, `h` for History, `1-9` for quick favourite selection

#### Responsive History View
- **Dynamic column widths** - History table now expands responsively to use available terminal width
- **Terminal resize support** - Layout adjusts automatically when terminal is resized
- **Improved table structure** - Project shown as column, Task as subrow with icon, optional Times column on wide terminals

### Changed
- Main menu simplified to just Settings (plus debug options in dev builds)
- All timer controls moved to dashboard buttons
- History access moved to dashboard button
- Quit removed from menu (use `q` keyboard shortcut)

---

## [0.7.0] - 2026-01-22

### Added

#### Pervasive Mouse Support
- **Full mouse interaction** - Click to interact with all screens throughout the application
- **Main Menu** - Click on menu items to select and open them
- **History View** - Click on timesheet entries to view details
- **Workspace Associations** - Click on Name column to edit, Edit button to open association popup, Clear button to remove associations
- **Code Repos** - Click on repository rows to edit associations
- **Timesheet Creator** - Click on form fields to focus them, click popover items to select, click Submit button to submit
- **Login Screen** - Click on input fields (API URL, Username, Password) to focus them
- **Favourites** - Click on favourite slots to start timers (already had mouse support)

### Fixed
- **Workspace associations edit mode** - Fixed bug where clicking Name column would crash due to referencing non-existent field

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
- **Running task indicator** - Currently running task is highlighted in green with "RUNNING" label and cannot be clicked again
- **Mouse support** - Click on favourite slots to select and start timers (mouse support enabled throughout app)

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

[0.10.0]: https://github.com/kartoza/go-timesheets-go/compare/v0.9.1...v0.10.0
[0.9.1]: https://github.com/kartoza/go-timesheets-go/compare/v0.9.0...v0.9.1
[0.9.0]: https://github.com/kartoza/go-timesheets-go/compare/v0.8.0...v0.9.0
[0.8.0]: https://github.com/kartoza/go-timesheets-go/compare/v0.7.0...v0.8.0
[0.7.0]: https://github.com/kartoza/go-timesheets-go/compare/v0.6.0...v0.7.0
[0.6.0]: https://github.com/kartoza/go-timesheets-go/compare/v0.5.0...v0.6.0
[0.5.0]: https://github.com/kartoza/go-timesheets-go/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/kartoza/go-timesheets-go/compare/v0.3.1...v0.4.0
[0.3.1]: https://github.com/kartoza/go-timesheets-go/compare/v0.3.0...v0.3.1
[0.3.0]: https://github.com/kartoza/go-timesheets-go/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/kartoza/go-timesheets-go/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/kartoza/go-timesheets-go/releases/tag/v0.1.0
