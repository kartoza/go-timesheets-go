# Go Timesheets Go - Specification

This document provides a complete specification of the Go Timesheets Go application, including all user stories, business rules, and functional requirements. This specification enables the application to be rebuilt from scratch in any language while achieving functional parity.

---

## Table of Contents

1. [Overview](#1-overview)
2. [Architecture](#2-architecture)
3. [Data Models](#3-data-models)
4. [User Stories](#4-user-stories)
5. [Screens & Views](#5-screens--views)
6. [CLI Commands](#6-cli-commands)
7. [Business Rules](#7-business-rules)
8. [API Integration](#8-api-integration)
9. [Features](#9-features)
10. [Configuration](#10-configuration)
11. [Integration Points](#11-integration-points)

---

## 1. Overview

**Go Timesheets Go** is a professional timesheet tracking application with both Terminal User Interface (TUI) and Graphical User Interface (GUI) implementations. It integrates with ERPNext Timesheet API for data persistence and provides advanced features including AI-assisted project discovery, proof-of-work recording, and gap analysis visualization.

### Key Principles

- **Dual Interface**: Parallel TUI (Bubbletea) and GUI (Fyne) implementations with feature parity
- **Shared Logic**: Common service layer and API client shared between interfaces
- **Remote-First**: ERPNext API is the canonical source of truth
- **Offline Capable**: Local caching enables offline viewing and status display
- **Cross-Platform**: Supports Linux, macOS, and Windows

---

## 2. Architecture

### 2.1 Layer Structure

```
┌─────────────────────────────────────────────────────┐
│                  User Interface                      │
│         ┌─────────────┐    ┌─────────────┐          │
│         │  TUI (tui/) │    │  GUI (gui/) │          │
│         │  Bubbletea  │    │    Fyne     │          │
│         └─────────────┘    └─────────────┘          │
├─────────────────────────────────────────────────────┤
│                  Service Layer                       │
│    ┌──────────┐ ┌──────────┐ ┌──────────────────┐   │
│    │   Gaps   │ │    AI    │ │  Business Logic  │   │
│    │ Service  │ │  Engine  │ │                  │   │
│    └──────────┘ └──────────┘ └──────────────────┘   │
├─────────────────────────────────────────────────────┤
│                   API Client                         │
│    ┌──────────┐ ┌──────────┐ ┌──────────────────┐   │
│    │  HTTP    │ │  Cache   │ │  Request Queue   │   │
│    │  Client  │ │  Layer   │ │  & Deduplication │   │
│    └──────────┘ └──────────┘ └──────────────────┘   │
├─────────────────────────────────────────────────────┤
│                 Local Storage                        │
│    ┌──────────┐ ┌──────────┐ ┌──────────────────┐   │
│    │  Config  │ │  Cache   │ │    Favourites    │   │
│    │   JSON   │ │  Files   │ │    & Assocs      │   │
│    └──────────┘ └──────────┘ └──────────────────┘   │
└─────────────────────────────────────────────────────┘
```

### 2.2 Interface Auto-Detection

1. Check for `--tui` flag - if present, force TUI mode
2. Check for `--gui` flag - if present, force GUI mode
3. Check `DISPLAY` or `WAYLAND_DISPLAY` environment variables
4. If graphical environment detected, use GUI; otherwise use TUI

---

## 3. Data Models

### 3.1 TimeEntry

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| ID | int | Yes | Unique identifier from API |
| UserID | int | Yes | Owner user ID |
| ProjectID | int | Yes | Associated project |
| ProjectName | string | Yes | Project display name |
| TaskID | int | No | Associated task (optional) |
| TaskName | string | No | Task display name |
| ActivityID | int | Yes | Activity type |
| ActivityType | string | Yes | Activity display name |
| FromTime | string | Yes | Start time (ISO 8601) |
| ToTime | string | No | End time (empty if running) |
| Hours | float64 | Yes | Duration in hours |
| Description | string | No | Work description |
| Submitted | bool | Yes | Whether submitted to ERP |

### 3.2 Project

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| ID | int | Yes | Unique identifier |
| Name | string | Yes | Project name |
| Description | string | No | Project description |
| IsActive | bool | Yes | Active status |

### 3.3 Task

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| ID | int | Yes | Unique identifier |
| ProjectID | int | Yes | Parent project |
| Name | string | Yes | Task name |
| Description | string | No | Task description |
| ExpectedTime | float64 | No | Estimated hours |
| ActualTime | float64 | No | Logged hours |
| IsActive | bool | Yes | Active status |

### 3.4 Activity

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| ID | int | Yes | Unique identifier |
| Name | string | Yes | Activity name (e.g., "Coding", "Planning") |

### 3.5 FavouriteAssociation

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| SlotNumber | int | Yes | Grid position (1-9) |
| ProjectID | int | Yes | Associated project |
| ProjectName | string | Yes | Display name |
| TaskID | int | No | Associated task |
| TaskName | string | No | Task display name |
| ActivityID | int | Yes | Associated activity |
| ActivityName | string | Yes | Activity display name |
| CustomName | string | No | User-defined slot name |
| Color | string | No | Hex color (default: #E95420) |

### 3.6 CodeRepoAssociation

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| ProjectID | int | Yes | Linked project |
| ProjectName | string | Yes | Project display name |
| RepoURL | string | Yes | Git repository URL |
| RepoName | string | Yes | Repository name |
| RepoOwner | string | No | GitHub owner/org |

### 3.7 EntryDetail (Service Layer)

| Field | Type | Description |
|-------|------|-------------|
| ID | int | Entry identifier |
| TaskName | string | Task display name |
| ActivityName | string | Activity display name |
| Hours | float64 | Duration |
| Description | string | Work description |
| Submitted | bool | Submission status |

### 3.8 CalendarEvent

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| ID | string | Yes | Google Calendar event ID |
| Title | string | Yes | Event summary/title |
| Start | time.Time | Yes | Event start time |
| End | time.Time | Yes | Event end time |
| AllDay | bool | Yes | Whether all-day event |
| Location | string | No | Event location |
| ColorHex | string | No | Event color (hex) |

**Methods**:
- `FormatTimeRange()`: Returns "HH:MM - HH:MM" formatted string
- `DurationHours()`: Returns duration in hours as float64
- `OverlapsWith(start, end)`: Checks if event overlaps with time range

### 3.9 GoogleCalendarConfig

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| ClientID | string | Yes | OAuth 2.0 Client ID |
| ClientSecret | string | Yes | OAuth 2.0 Client Secret |
| AccessToken | string | No | Current access token |
| RefreshToken | string | No | Token for refresh |
| TokenExpiry | string | No | Token expiration (ISO 8601) |

---

## 4. User Stories

### 4.1 Authentication

**US-AUTH-01**: As a user, I want to log in with my ERPNext credentials so that I can access my timesheet data.
- Acceptance: Username/password authentication with token persistence
- Alternative: Direct token configuration in config file

**US-AUTH-02**: As a user, I want to remain logged in between sessions so that I don't have to re-authenticate.
- Acceptance: Token stored securely and reused on application start

**US-AUTH-03**: As a user, I want to log out when needed so that I can switch accounts or secure my session.
- Acceptance: Logout clears stored credentials and returns to login screen

### 4.2 Time Tracking

**US-TIME-01**: As a user, I want to start a timer for a project and activity so that I can track my work time.
- Acceptance: Timer starts immediately with selected project/activity
- Constraint: Only one timer can run at a time

**US-TIME-02**: As a user, I want to stop my running timer so that I can record my work session.
- Acceptance: Timer stops, duration calculated, entry saved to API

**US-TIME-03**: As a user, I want to add a description when stopping a timer so that I can document what I worked on.
- Acceptance: Description field available at stop time

**US-TIME-04**: As a user, I want to see my elapsed time while working so that I can monitor my session.
- Acceptance: Live timer display updates every second with visual indicator

**US-TIME-05**: As a user, I want to optionally select a task when starting a timer so that I can track work at task-level granularity.
- Acceptance: Task selection is optional; tasks filtered by selected project

**US-TIME-06**: As a user, I want to create completed entries with start and end times so that I can log past work.
- Acceptance: Entry form allows specifying both start and end times

### 4.3 History & Editing

**US-HIST-01**: As a user, I want to view my timesheet history so that I can review my logged work.
- Acceptance: Scrollable list showing all entries with key details

**US-HIST-02**: As a user, I want to edit unsubmitted entries so that I can correct mistakes.
- Acceptance: Edit form for project, task, activity, description, times
- Constraint: Only unsubmitted entries can be edited

**US-HIST-03**: As a user, I want to delete unsubmitted entries so that I can remove erroneous records.
- Acceptance: Delete confirmation dialog
- Constraint: Only unsubmitted entries can be deleted

**US-HIST-04**: As a user, I want to submit my pending entries to ERP so that they are officially recorded.
- Acceptance: Batch submission with success confirmation
- Constraint: All entries must have descriptions before submission

**US-HIST-05**: As a user, I want to see which entries are missing descriptions so that I can complete them before submission.
- Acceptance: Visual indicator (warning icon) on entries without descriptions

### 4.4 Favourites

**US-FAV-01**: As a user, I want to configure favourite project/task/activity combinations so that I can quickly start common timers.
- Acceptance: 3x3 grid with 9 configurable slots

**US-FAV-02**: As a user, I want to start a timer with one click/keypress from my favourites so that I can begin work instantly.
- Acceptance: Keyboard shortcuts 1-9 or mouse click starts timer

**US-FAV-03**: As a user, I want to customize the name and color of each favourite so that I can easily identify them.
- Acceptance: Custom name and color picker per slot

**US-FAV-04**: As a user, I want to see which favourite is currently running so that I know my active work context.
- Acceptance: Visual indicator (green, play icon) on active favourite

### 4.5 Gaps Analysis

**US-GAP-01**: As a user, I want to see a visualization of my logged hours vs target for the last 14 days so that I can identify gaps in my timesheet.
- Acceptance: Bar chart with daily hours, target line at 8 hours

**US-GAP-02**: As a user, I want to click on a gap to create a new entry for that day so that I can fill missing time quickly.
- Acceptance: Click opens entry form pre-filled with date and calculated times

**US-GAP-03**: As a user, I want the end time to be pre-filled based on the gap duration when creating entries from gaps view.
- Acceptance: End time calculated as start time + gap hours (max 8h)

**US-GAP-04**: As a user, I want to see which entries are missing descriptions in the gaps view so that I can identify incomplete records.
- Acceptance: Yellow exclamation mark indicator on segments with missing descriptions

**US-GAP-05**: As a user, I want to click on project segments in gaps view to edit those entries.
- Acceptance: Click shows entry selection dialog (if multiple) or opens edit form
- Constraint: Only unsubmitted entries can be edited

**US-GAP-06**: As a user, I want the gaps view to refresh after creating or editing entries so that I see current data.
- Acceptance: Automatic refresh after any entry modification

### 4.6 Proof of Work (POW)

**US-POW-01**: As a user, I want to capture screenshots during my work sessions so that I have proof of my work.
- Acceptance: Screenshots captured at configurable intervals (default 60s)

**US-POW-02**: As a user, I want my screenshots converted to a timelapse video when I stop the timer so that I have a compact proof-of-work artifact.
- Acceptance: ffmpeg generates video from screenshots

**US-POW-03**: As a user, I want to see which entries have POW videos so that I can review them.
- Acceptance: Movie icon indicator on entries with videos

**US-POW-04**: As a user, I want to play POW videos from the entry detail view so that I can review my work session.
- Acceptance: Video playback with system player

**US-POW-05**: As a user, I want to pause and resume POW capture so that I can take breaks without gaps.
- Acceptance: POW can be paused/resumed independently of timer

### 4.7 AI Assistant

**US-AI-01**: As a user, I want to ask natural language questions to find the right project/task/activity so that I don't have to manually search.
- Acceptance: Query like "Which project for Docker work?" returns matches

**US-AI-02**: As a user, I want to start a timer directly from AI suggestions so that I can begin work quickly.
- Acceptance: Click on suggestion starts timer with that configuration

**US-AI-03**: As a user, I want to ask analytics questions about my timesheets so that I can understand my work patterns.
- Acceptance: Questions like "How many hours did I work last week?" return answers

**US-AI-04**: As a user, I want AI to work offline so that I can use it without internet.
- Acceptance: Multi-level fallback: Embedded LLM → Ollama → Neural Network → Fuzzy matching

### 4.8 Git Integration

**US-GIT-01**: As a user, I want to link projects to GitHub repositories so that I can auto-fill descriptions from commits.
- Acceptance: Code repo association per project

**US-GIT-02**: As a user, I want to fetch git commit messages for the time period of my entry so that I can use them as descriptions.
- Acceptance: "Fetch from Git" button populates description field

### 4.9 Google Calendar Integration

**US-CAL-01**: As a user, I want to connect my Google Calendar so that I can see my appointments when filling timesheet gaps.
- Acceptance: OAuth 2.0 flow with user-provided credentials
- Storage: Tokens stored securely in config directory

**US-CAL-02**: As a user, I want to see my calendar events for a day when creating a timesheet entry so that I can recall what I worked on.
- Acceptance: Calendar panel shows events when creating entry from gaps view

**US-CAL-03**: As a user, I want to click a calendar event to populate the start/end times in my timesheet entry so that I can quickly match my logged time to calendar meetings.
- Acceptance: Clicking event fills start/end time fields in entry form

**US-CAL-04**: As a user, I want to configure my Google Calendar credentials in settings so that I can connect and disconnect as needed.
- Acceptance: Settings screen with Client ID/Secret fields, Connect/Disconnect buttons

**US-CAL-05**: As a user, I want all-day events filtered out when viewing calendar in gaps view so that only timed events are shown.
- Acceptance: All-day events excluded from calendar panel display

### 4.10 Desktop Integration

**US-DESK-01**: As a user, I want to see my timer status in my desktop panel (waybar) so that I always know my tracking state.
- Acceptance: JSON status output compatible with waybar custom modules

**US-DESK-02**: As a user, I want status checks to be fast and not make API calls so that frequent polling doesn't impact performance.
- Acceptance: Status command reads from local cache only

---

## 5. Screens & Views

### 5.1 TUI Screens

| Screen | Purpose | Key Actions |
|--------|---------|-------------|
| Login | Authentication | Enter credentials, submit |
| Dashboard | Main hub | View timer, access features, use favourites |
| History | View entries | Browse, select for detail |
| Entry Detail | View/edit entry | Edit fields, delete, play POW |
| Timesheet Creator | Create entry | Select project/task/activity, set times |
| Gaps View | 14-day analysis | Navigate days, create entries |
| Settings | Configuration | Navigate to sub-screens |
| Favourites Editor | Configure favourites | Edit slots, set properties |
| Code Repos | Manage git links | Add/edit/remove associations |
| AI Assistant | Natural language queries | Enter queries, select results |

### 5.2 GUI Screens

| Screen | Purpose | Key Actions |
|--------|---------|-------------|
| Login | Authentication | Enter credentials, submit |
| Dashboard | Main hub | View timer, navigate, use favourites |
| History | View entries | Browse cards, click for detail |
| Gaps View | 14-day analysis | Click bars, create entries |
| Settings | Configuration | Navigate to sub-screens |
| Favourites | Configure favourites | Click slots to edit |
| Code Repos | Manage git links | Add/edit/remove associations |
| AI Assistant | Natural language queries | Enter queries, click results |
| Office Scene | Virtual office | View team, ambient display |

### 5.3 Shared Dialogs

| Dialog | Purpose | Trigger |
|--------|---------|---------|
| Entry Form | Create/Edit/Stop entries | Multiple contexts |
| Confirmation | Confirm destructive actions | Delete, submit |
| Error | Display errors | API failures, validation |
| Celebration | Submission celebration | After successful submit |

---

## 6. CLI Commands

### 6.1 Interactive Commands

| Command | Description | Options |
|---------|-------------|---------|
| `kartoza-timesheet` | Launch interactive UI | `--tui`, `--gui` |
| `celebration` | Display fireworks | `--duration` (seconds) |
| `office` | Launch virtual office | None |

### 6.2 Timer Commands

| Command | Description | Options |
|---------|-------------|---------|
| `start` | Start timer | `--project`, `--activity`, `--task` |
| `stop` | Stop running timer | `--description` |
| `current` | Get active timer | None |

### 6.3 Query Commands

| Command | Description | Options |
|---------|-------------|---------|
| `status` | Waybar status (JSON) | None |
| `today` | Today's entries (JSON) | None |
| `list` | All entries | None |
| `projects` | List projects | None |

### 6.4 POW Commands

| Command | Description | Options |
|---------|-------------|---------|
| `pow` | Start POW capture | `--interval` |
| `list-pow` | List POW videos | `--full-paths` |
| `play-pow` | Play POW video | Video path argument |

### 6.5 Utility Commands

| Command | Description | Options |
|---------|-------------|---------|
| `monitor` | View API logs | `--since`, `--endpoint`, `--follow` |
| `import-history` | Import CSV data | CSV file argument |

---

## 7. Business Rules

### 7.1 Timer Rules

| Rule ID | Rule | Enforcement |
|---------|------|-------------|
| BR-TIM-01 | Only one timer may run at a time | API rejects; UI auto-stops previous |
| BR-TIM-02 | Timer must have project and activity | Validation on creation |
| BR-TIM-03 | Task is optional | No validation |
| BR-TIM-04 | End time must be after start time | Validation on stop/edit |

### 7.2 Entry Rules

| Rule ID | Rule | Enforcement |
|---------|------|-------------|
| BR-ENT-01 | Only unsubmitted entries can be edited | UI disables editing; API rejects |
| BR-ENT-02 | Only unsubmitted entries can be deleted | UI disables delete; API rejects |
| BR-ENT-03 | Description required before submission | Validation on submit action |
| BR-ENT-04 | Hours calculated from time difference | Automatic calculation |

### 7.3 Submission Rules

| Rule ID | Rule | Enforcement |
|---------|------|-------------|
| BR-SUB-01 | All pending entries must have descriptions | Validation with error dialog |
| BR-SUB-02 | Only stopped entries can be submitted | Running entries excluded |
| BR-SUB-03 | Submitted entries are read-only | UI read-only mode |

### 7.4 Favourite Rules

| Rule ID | Rule | Enforcement |
|---------|------|-------------|
| BR-FAV-01 | Maximum 9 favourites (3x3 grid) | UI constraint |
| BR-FAV-02 | Favourite must have project and activity | Validation on save |
| BR-FAV-03 | Task is optional | No validation |
| BR-FAV-04 | Starting favourite auto-stops running timer | Automatic behavior |

### 7.5 POW Rules

| Rule ID | Rule | Enforcement |
|---------|------|-------------|
| BR-POW-01 | POW capture interval minimum 10 seconds | Configuration validation |
| BR-POW-02 | POW videos linked to timesheet entries | Automatic association |
| BR-POW-03 | POW can be paused/resumed independently | UI controls |

---

## 8. API Integration

### 8.1 ERPNext Timesheet API

**Base URL**: Configured per installation (e.g., `https://timesheets.example.com`)

**Authentication**:
- Token-based: `Authorization: Token <token>`
- Session-based: Cookie with CSRF token

### 8.2 Endpoints

| Method | Endpoint | Purpose |
|--------|----------|---------|
| POST | `/api/method/login` | Authenticate |
| GET | `/api/resource/Timesheet Log` | List entries |
| POST | `/api/resource/Timesheet Log` | Create entry |
| PUT | `/api/resource/Timesheet Log/{id}` | Update entry |
| DELETE | `/api/resource/Timesheet Log/{id}` | Delete entry |
| GET | `/api/resource/Project` | List projects |
| GET | `/api/resource/Task` | List tasks |
| GET | `/api/resource/Activity Type` | List activities |
| POST | `/api/method/submit_timesheet` | Submit entries |
| POST | `/api/method/stop_timesheet` | Stop timer |
| POST | `/api/method/break_timesheet` | Split entry |

### 8.3 Caching Strategy

| Data Type | Cache Duration | Invalidation |
|-----------|----------------|--------------|
| Projects | 5 minutes | Manual refresh |
| Activities | 5 minutes | Manual refresh |
| Tasks | 5 minutes | Manual refresh |
| Timelogs | 2 seconds | On any mutation |
| Active Timer | 2 seconds | On start/stop |

### 8.4 Request Deduplication

- In-flight requests are tracked by endpoint + parameters
- Duplicate requests within window return cached promise
- Prevents UI double-clicks from causing duplicate API calls

---

## 9. Features

### 9.1 Gaps Analysis

**Purpose**: Visualize missing hours over 14 days

**Components**:
- Bar chart with daily hour totals
- Target line at configurable hours (default 8)
- Color coding: Green (met), White (gap)
- Project segments within each bar
- Warning indicators for missing descriptions

**Interactions**:
- Click gap segment: Create entry with pre-filled times
- Click project segment: Edit existing entries
- Navigate left/right: Select different days
- Navigate up/down: Select different segments within day

**Calculations**:
- Gap hours = Target hours - Total logged hours
- Start time = 9:00 AM + previously logged hours
- End time = Start time + Gap hours (max target hours)

### 9.2 AI Assistant

**Multi-Level Fallback**:
1. **Embedded LLM** (llama.cpp): Build-time optional, fully offline
2. **Ollama**: Local LLM server via HTTP
3. **Neural Network**: TF-IDF trained from user's history
4. **Fuzzy Matching**: Rule-based string similarity

**Query Types**:
- Project discovery: "Which project for X?"
- Analytics: "How many hours on project X?"
- Consistency: "How consistent is my timekeeping?"

**Analytics Capabilities** (17 types):
- Time period summaries
- Project comparisons
- Activity breakdowns
- Consistency analysis (detects rounded entries)
- Gap detection
- Utilization metrics
- Overtime detection
- Context switching analysis

### 9.3 Proof of Work

**Capture Process**:
1. Timer starts → POW capture begins (if enabled)
2. Screenshots taken at interval (default 60s)
3. Timer stops → ffmpeg generates timelapse video
4. Video linked to timesheet entry

**Screenshot Tools** (in order of preference):
- Wayland: grim
- X11: scrot, gnome-screenshot, import (ImageMagick)
- macOS: screencapture
- Windows: PowerShell

**Video Players** (in order of preference):
- mpv, vlc, totem, celluloid, xdg-open

### 9.4 Celebration Mode

**Trigger**: After successful timesheet submission

**Display**:
- Full-screen fireworks animation
- Kartoza brand colors (reds, oranges, golds)
- Configurable duration
- Keyboard exit (any key)

---

## 10. Configuration

### 10.1 Config File Location

- Linux: `~/.config/kartoza-timesheets/config.json`
- macOS: `~/Library/Application Support/kartoza-timesheets/config.json`
- Windows: `%APPDATA%\kartoza-timesheets\config.json`

### 10.2 Configuration Options

```json
{
  "api_url": "https://timesheets.example.com",
  "username": "user@example.com",
  "token": "api_token_here",
  "github_token": "optional_github_pat",
  "pow_enabled": true,
  "pow_interval": 60,
  "target_hours": 8.0,
  "theme": "dark"
}
```

### 10.3 Data Files

| File | Purpose |
|------|---------|
| `config.json` | User configuration |
| `favourites.json` | Favourite associations |
| `code_repos.json` | Repository associations |
| `projects_cache.json` | Cached projects |
| `activities_cache.json` | Cached activities |
| `timelogs_cache.json` | Cached entries |
| `api_requests_YYYY-MM-DD.log` | Daily API logs |

---

## 11. Integration Points

### 11.1 ERPNext Timesheet Server

- **Protocol**: HTTPS REST API
- **Authentication**: Token or session-based
- **Data Format**: JSON
- **Required**: Yes (core functionality)

### 11.2 Git/GitHub

- **Purpose**: Commit message auto-fill
- **Authentication**: Optional GitHub PAT
- **Required**: No (feature enhancement)

### 11.3 Waybar

- **Purpose**: Desktop panel integration
- **Protocol**: JSON stdout
- **Required**: No (desktop feature)

### 11.4 Ollama

- **Purpose**: Local LLM queries
- **Protocol**: HTTP (localhost:11434)
- **Required**: No (AI fallback)

### 11.5 llama.cpp

- **Purpose**: Embedded LLM
- **Integration**: Build-time linking
- **Required**: No (build flag optional)

### 11.6 Google Calendar

- **Purpose**: Display calendar events when filling timesheet gaps
- **Protocol**: Google Calendar API v3 with OAuth 2.0
- **Authentication**: User-provided OAuth client credentials
- **Required**: No (optional feature enhancement)

**Setup Process**:
1. User creates OAuth credentials in Google Cloud Console
2. User enters Client ID and Client Secret in Settings → Google Calendar
3. User clicks "Connect" to initiate OAuth flow
4. Browser opens for Google authorization
5. Local HTTP server (port 9876) receives callback
6. Tokens stored securely in `~/.config/kartoza-timesheets/google_calendar.json`

**Integration Points**:
- **Gaps View**: When clicking a gap to create entry, calendar panel shows events for that day
- **Entry Form**: Calendar panel displayed alongside entry form (right side panel)
- **Event Selection**: Clicking calendar event auto-fills start/end times in entry form

---

## Appendix A: Keyboard Shortcuts

### Dashboard/Main Menu (TUI)

| Key | Action |
|-----|--------|
| `s` | Start/Stop timer |
| `h` | History view |
| `g` | Gaps view |
| `a` | AI Assistant |
| `p` | Toggle POW |
| `1-9` | Start favourite |
| `e` | Edit mode (favourites) |
| `q` | Quit |

### History View (TUI)

| Key | Action |
|-----|--------|
| `↑/↓` | Navigate entries |
| `Enter` | View details |
| `s` | Submit all pending |
| `Esc` | Back |

### Gaps View (TUI)

| Key | Action |
|-----|--------|
| `←/→` | Navigate days |
| `↑/↓` | Navigate segments |
| `Enter` | Create/edit entry |
| `r` | Refresh |
| `Esc` | Back |

---

## Appendix B: Color Scheme

### Brand Colors

| Name | Hex | Usage |
|------|-----|-------|
| Kartoza Orange | #E95420 | Primary accent, favourites default |
| Gold | #DDA036 | Borders, highlights |
| Green | #2ECC71 | Success, running indicators |
| Red | #E74C3C | Errors, warnings |
| Blue | #569FC6 | Info, links |
| Dark Gray | #333333 | Text on light backgrounds |
| Light Gray | #9A9EA0 | Secondary text |

### Project Colors (Cycling)

```
#2ECC71 (Green), #3498DB (Blue), #9B59B6 (Purple),
#F39C12 (Orange), #1ABC9C (Teal), #E91E63 (Pink),
#00BCD4 (Cyan), #FFC107 (Amber)
```

---

## Appendix C: Error Handling

### API Errors

| Error | User Message | Recovery |
|-------|--------------|----------|
| 401 Unauthorized | Session expired | Re-authenticate |
| 404 Not Found | Entry not found | Refresh data |
| 500 Server Error | Server error | Retry with backoff |
| Network Error | Connection failed | Check connectivity |

### Validation Errors

| Error | User Message | Resolution |
|-------|--------------|------------|
| Missing project | Please select a project | Select from dropdown |
| Missing activity | Please select an activity | Select from dropdown |
| Invalid time range | End time must be after start | Correct times |
| Missing descriptions | X entries missing descriptions | Add descriptions |

---

*Last Updated: 2026-02-19*
*Version: 0.10.x (Unreleased)*
