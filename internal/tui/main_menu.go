package tui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/kartoza/go-timesheets-go/internal/api"
	"github.com/kartoza/go-timesheets-go/internal/config"
	"github.com/kartoza/go-timesheets-go/internal/pow"
	"github.com/kartoza/go-timesheets-go/internal/storage"
)

// menuDebugLog is defined in debug_dev.go or debug_release.go

// Blink interval for timer indicators (also updates elapsed time display locally)
const blinkInterval = 1 * time.Second

// MainMenuItem represents a menu option
type MainMenuItem int

const (
	MenuCreateTimer MainMenuItem = iota
	MenuStopTimer
	MenuWorkspaceAssociations
	MenuCodeRepos
	MenuViewHistory
	MenuOpenMonitor // Only visible in debug builds
	MenuViewAPILog  // Only visible in debug builds
	MenuLogOut
	MenuQuit
)

// MainMenuModel represents the main menu screen
type MainMenuModel struct {
	apiClient              *api.Client
	username               string
	activeTimer            *api.TimelogEntry
	monthlyHours           float64
	todayHours             float64
	selectedItem           int
	menuItems              []menuItem
	width                  int
	height                 int
	err                    error
	statusMsg              string // Informational status message (not an error)
	confirmLogout          bool
	logoutConfirmSelection int // 0 = Yes, 1 = No
	dashboard              *TimerDashboard
	timerStartTime         time.Time
	blinkOn                bool // Blink state for timer indicators

	// Stop timer confirmation dialog state
	confirmStopTimer          bool
	stopTimerDescription      textarea.Model
	stopTimerConfirmSelection int // 0 = Save, 1 = Add Commits, 2 = Cancel

	// POW (Proof of Work) capturer for screenshot capture during timer sessions
	powCapturer *pow.Capturer
}

type menuItem struct {
	label   string
	enabled bool
	action  MainMenuItem
}

// NewMainMenu creates a new main menu
// powCapturer can be nil if POW mode is not enabled
func NewMainMenu(apiClient *api.Client, username string, powCapturer *pow.Capturer) *MainMenuModel {
	return &MainMenuModel{
		apiClient:    apiClient,
		username:     username,
		selectedItem: 0,
		width:        80,
		height:       24,
		dashboard:    NewTimerDashboard(),
		powCapturer:  powCapturer,
	}
}

// Init initializes the main menu
func (m *MainMenuModel) Init() tea.Cmd {
	return tea.Batch(
		m.loadActiveTimer(),
		m.loadHoursData(), // Single call for both monthly and today hours
	)
}

// Update handles messages for the main menu
func (m *MainMenuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		// Handle stop timer confirmation dialog
		if m.confirmStopTimer {
			switch msg.String() {
			case "tab":
				// Toggle between textarea and buttons
				if m.stopTimerDescription.Focused() {
					m.stopTimerDescription.Blur()
				} else {
					m.stopTimerDescription.Focus()
				}
				return m, nil
			case "left", "h":
				if !m.stopTimerDescription.Focused() {
					if m.stopTimerConfirmSelection > 0 {
						m.stopTimerConfirmSelection--
					}
				}
				return m, nil
			case "right", "l":
				if !m.stopTimerDescription.Focused() {
					if m.stopTimerConfirmSelection < 2 {
						m.stopTimerConfirmSelection++
					}
				}
				return m, nil
			case "enter":
				if !m.stopTimerDescription.Focused() {
					switch m.stopTimerConfirmSelection {
					case 0: // Save
						description := m.stopTimerDescription.Value()
						m.confirmStopTimer = false
						return m, m.performStopTimerWithDescription(description)
					case 1: // Add Commits
						return m, m.appendCommitsToDescription()
					case 2: // Cancel
						m.confirmStopTimer = false
						return m, nil
					}
				}
			case "esc":
				// Cancel dialog
				m.confirmStopTimer = false
				return m, nil
			case "ctrl+s":
				// Quick save shortcut
				description := m.stopTimerDescription.Value()
				m.confirmStopTimer = false
				return m, m.performStopTimerWithDescription(description)
			case "ctrl+g":
				// Quick add commits shortcut
				return m, m.appendCommitsToDescription()
			}
			// Pass key events to textarea if focused
			if m.stopTimerDescription.Focused() {
				var cmd tea.Cmd
				m.stopTimerDescription, cmd = m.stopTimerDescription.Update(msg)
				return m, cmd
			}
			return m, nil
		}

		// Handle logout confirmation dialog
		if m.confirmLogout {
			switch msg.String() {
			case "left", "h":
				m.logoutConfirmSelection = 0 // Yes
				return m, nil
			case "right", "l":
				m.logoutConfirmSelection = 1 // No
				return m, nil
			case "enter":
				if m.logoutConfirmSelection == 0 {
					// Yes, log out
					return m, m.performLogout()
				} else {
					// No, cancel
					m.confirmLogout = false
					return m, nil
				}
			case "n", "esc":
				// Cancel logout
				m.confirmLogout = false
				return m, nil
			case "y":
				// Confirm logout
				return m, m.performLogout()
			}
			return m, nil
		}

		// Normal menu navigation
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("ctrl+c", "q", "esc"))):
			return m, tea.Quit

		case key.Matches(msg, key.NewBinding(key.WithKeys("o"))):
			// Launch office view
			return m, func() tea.Msg {
				return launchOfficeViewMsg{}
			}

		case key.Matches(msg, key.NewBinding(key.WithKeys("p"))):
			// Toggle POW mode
			return m, m.togglePowMode()

		case key.Matches(msg, key.NewBinding(key.WithKeys("up", "k"))):
			m.selectedItem--
			if m.selectedItem < 0 {
				m.selectedItem = len(m.menuItems) - 1
			}
			return m, nil

		case key.Matches(msg, key.NewBinding(key.WithKeys("down", "j"))):
			m.selectedItem++
			if m.selectedItem >= len(m.menuItems) {
				m.selectedItem = 0
			}
			return m, nil

		case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
			if m.selectedItem >= 0 && m.selectedItem < len(m.menuItems) {
				if m.menuItems[m.selectedItem].enabled {
					return m, m.handleMenuSelection()
				}
			}
			return m, nil
		}

	case activeTimerLoadedMsg:
		previousTimer := m.activeTimer
		m.activeTimer = msg.timer
		m.updateMenuItems()
		m.updateDashboard()
		// If timer was stopped and we were on "Stop Running Timer", move focus to "Create New Timer"
		if msg.timer == nil && m.selectedItem == 1 {
			m.selectedItem = 0
		}
		// Start blink ticker if timer is active (for UI updates, no API calls)
		if m.activeTimer != nil {
			// Start POW session if timer just became active (wasn't active before)
			if previousTimer == nil && m.powCapturer != nil && m.powCapturer.IsEnabled() {
				if err := m.powCapturer.StartSession(
					m.activeTimer.ID,
					m.activeTimer.ProjectName,
					m.activeTimer.TaskName,
				); err != nil {
					menuDebugLog.Printf("Failed to start POW session: %v", err)
				} else {
					menuDebugLog.Printf("POW session started for timer %d", m.activeTimer.ID)
				}
			}
			return m, m.startBlinkTicker()
		}
		return m, nil

	case monthlyHoursLoadedMsg:
		m.monthlyHours = msg.hours
		return m, nil

	case todayHoursLoadedMsg:
		m.todayHours = msg.hours
		m.updateDashboard()
		return m, nil

	case hoursDataLoadedMsg:
		// Combined handler for both monthly and today hours (single API call)
		m.monthlyHours = msg.monthlyHours
		m.todayHours = msg.todayHours
		m.updateDashboard()
		return m, nil

	case timerStoppedMsg:
		// Timer was stopped, reload active timer and hours data
		m.err = nil // Clear any previous errors
		return m, tea.Batch(
			m.loadActiveTimer(),
			m.loadHoursData(), // Single call for both monthly and today hours
		)

	case stopTimerDescriptionReadyMsg:
		// Auto-generated description is ready, show confirmation dialog
		m.confirmStopTimer = true
		m.stopTimerConfirmSelection = 0 // Default to "Save"

		// Initialize the textarea with the auto-generated description
		ta := textarea.New()
		ta.SetValue(msg.description)
		ta.SetWidth(56)
		ta.SetHeight(8)
		ta.Focus()
		ta.ShowLineNumbers = false
		ta.CharLimit = 2000
		m.stopTimerDescription = ta

		return m, m.stopTimerDescription.Focus()

	case commitsAppendedMsg:
		if msg.noCommitsFound {
			m.err = fmt.Errorf("no commits found in the timer's time range")
		} else {
			// Append commits to the current description
			current := m.stopTimerDescription.Value()
			if current != "" {
				current += "\n\n"
			}
			current += msg.commits
			m.stopTimerDescription.SetValue(current)
			m.err = nil
		}
		return m, nil

	case powToggledMsg:
		m.err = nil
		m.statusMsg = ""
		if !msg.ok {
			m.err = fmt.Errorf("POW mode unavailable - missing screenshot tool or ffmpeg")
		} else if msg.enabled {
			m.statusMsg = "Proof of Work mode now enabled. Any timesheets you capture now will capture screenshots too."
			menuDebugLog.Printf("POW mode enabled via toggle")
		} else {
			m.statusMsg = "Proof of Work mode disabled."
			menuDebugLog.Printf("POW mode disabled via toggle")
		}
		return m, nil

	case errorMsg:
		m.err = error(msg)
		m.statusMsg = "" // Clear status message when error occurs
		return m, nil

	case blinkTickMsg:
		// Toggle blink state and update dashboard (local only, no API calls)
		if m.activeTimer != nil {
			m.blinkOn = !m.blinkOn
			m.dashboard.BlinkOn = m.blinkOn
			m.updateDashboard() // Updates elapsed time locally based on start time
			return m, m.startBlinkTicker()
		}
		return m, nil
	}

	return m, nil
}

// View renders the main menu
func (m *MainMenuModel) View() string {
	// Render header
	header := m.renderHeader()

	// Render dashboard
	dashboard := m.dashboard.Render()

	// Render menu items or confirmation dialog
	var mainContent string
	if m.confirmStopTimer {
		mainContent = m.renderStopTimerConfirmation()
	} else if m.confirmLogout {
		mainContent = m.renderLogoutConfirmation()
	} else {
		mainContent = m.renderMenu()
	}

	// Render status or error message if any
	var messageContent string
	if m.statusMsg != "" {
		statusStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#2ECC71")).
			Bold(true).
			Align(lipgloss.Center)
		messageContent = statusStyle.Render(m.statusMsg)
	} else if m.err != nil {
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF6B6B")).
			Bold(true).
			Align(lipgloss.Center)
		messageContent = errorStyle.Render("Error: " + m.err.Error())
	}

	// Render help text
	help := m.renderHelp()

	// Combine main content (without help)
	parts := []string{header, "", dashboard, "", mainContent}
	if messageContent != "" {
		parts = append(parts, "", messageContent)
	}
	mainSection := lipgloss.JoinVertical(
		lipgloss.Center,
		parts...,
	)

	// Center main content at top
	centeredMain := lipgloss.Place(
		m.width,
		m.height-2, // Leave room for help at bottom
		lipgloss.Center,
		lipgloss.Top,
		mainSection,
	)

	// Place help text at the bottom
	helpStyle := lipgloss.NewStyle().
		Width(m.width).
		Align(lipgloss.Center)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		centeredMain,
		helpStyle.Render(help),
	)
}

// renderHeader renders the consistent header across all screens
func (m *MainMenuModel) renderHeader() string {
	state := &HeaderState{
		Username:     m.username,
		IsActive:     m.activeTimer != nil,
		MonthlyHours: m.monthlyHours,
		BlinkOn:      m.blinkOn,
	}
	return RenderHeader("Main Menu", state)
}

// renderMenu renders the menu items
func (m *MainMenuModel) renderMenu() string {
	m.updateMenuItems()

	normalStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#569FC6")).
		Padding(0, 2)

	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#DDA036")).
		Bold(true).
		Padding(0, 2)

	disabledStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9A9EA0")).
		Padding(0, 2)

	var items []string
	for i, item := range m.menuItems {
		prefix := "  "
		if i == m.selectedItem {
			prefix = "▶ "
		}

		var rendered string
		if !item.enabled {
			rendered = disabledStyle.Render(prefix + item.label + " (disabled)")
		} else if i == m.selectedItem {
			rendered = selectedStyle.Render(prefix + item.label)
		} else {
			rendered = normalStyle.Render(prefix + item.label)
		}

		items = append(items, rendered)
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		lipgloss.JoinVertical(lipgloss.Left, items...),
	)
}

// renderHelp renders the help text
func (m *MainMenuModel) renderHelp() string {
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9A9EA0")).
		Italic(true)

	if m.confirmStopTimer {
		if m.stopTimerDescription.Focused() {
			return helpStyle.Render("Tab: Focus buttons • Ctrl+S: Save • Ctrl+G: Add Commits • Esc: Cancel")
		}
		return helpStyle.Render("Tab: Edit • ←/→: Select • Enter: Confirm • Ctrl+G: Add Commits • Esc: Cancel")
	}

	if m.confirmLogout {
		return helpStyle.Render("←/→: Select • Enter/y: Confirm • n/Esc: Cancel")
	}

	powStatus := "off"
	if m.powCapturer != nil && m.powCapturer.IsEnabled() {
		powStatus = "on"
	}
	return helpStyle.Render(fmt.Sprintf("↑/↓: Navigate • Enter: Select • o: Office • p: POW (%s) • Esc/q: Quit", powStatus))
}

// updateMenuItems updates the menu items based on current state
func (m *MainMenuModel) updateMenuItems() {
	hasActiveTimer := m.activeTimer != nil

	m.menuItems = []menuItem{
		{
			label:   "Create New Timer",
			enabled: !hasActiveTimer,
			action:  MenuCreateTimer,
		},
		{
			label:   "Stop Running Timer",
			enabled: hasActiveTimer,
			action:  MenuStopTimer,
		},
		{
			label:   "View Timesheet History",
			enabled: true,
			action:  MenuViewHistory,
		},
		{
			label:   "Manage Workspace Associations",
			enabled: true,
			action:  MenuWorkspaceAssociations,
		},
		{
			label:   "Manage Code Repos",
			enabled: true,
			action:  MenuCodeRepos,
		},
	}

	// Only show debug options in debug builds
	if DebugEnabled {
		m.menuItems = append(m.menuItems,
			menuItem{
				label:   "Open Monitor (Dev)",
				enabled: true,
				action:  MenuOpenMonitor,
			},
			menuItem{
				label:   "View API Log (Dev)",
				enabled: true,
				action:  MenuViewAPILog,
			},
		)
	}

	m.menuItems = append(m.menuItems,
		menuItem{
			label:   "Log Out",
			enabled: true,
			action:  MenuLogOut,
		},
		menuItem{
			label:   "Quit",
			enabled: true,
			action:  MenuQuit,
		},
	)
}

// handleMenuSelection handles the selected menu item
func (m *MainMenuModel) handleMenuSelection() tea.Cmd {
	if m.selectedItem < 0 || m.selectedItem >= len(m.menuItems) {
		return nil
	}

	action := m.menuItems[m.selectedItem].action

	switch action {
	case MenuCreateTimer:
		// Launch the timer creation wizard
		return func() tea.Msg {
			return launchTimerCreationMsg{}
		}

	case MenuStopTimer:
		return m.stopActiveTimer()

	case MenuWorkspaceAssociations:
		// Launch workspace associations screen
		return func() tea.Msg {
			return launchWorkspaceAssociationsMsg{}
		}

	case MenuCodeRepos:
		// Launch code repos screen
		return func() tea.Msg {
			return launchCodeReposMsg{}
		}

	case MenuViewHistory:
		// Launch history view
		return func() tea.Msg {
			return launchHistoryViewMsg{}
		}

	case MenuOpenMonitor:
		// Launch expvarmon in a new terminal (only available in debug builds)
		if err := LaunchMonitor(); err != nil {
			m.err = err
		}
		return nil

	case MenuViewAPILog:
		// Launch API log viewer in a new terminal (only available in debug builds)
		if err := LaunchAPILog(); err != nil {
			m.err = err
		}
		return nil

	case MenuLogOut:
		// Show confirmation dialog
		m.confirmLogout = true
		m.logoutConfirmSelection = 1 // Default to "No"
		return nil

	case MenuQuit:
		return tea.Quit
	}

	return nil
}

// loadActiveTimer loads the currently active timer
func (m *MainMenuModel) loadActiveTimer() tea.Cmd {
	return func() tea.Msg {
		timer, err := m.apiClient.GetActiveTimesheet()
		if err != nil {
			return errorMsg(err)
		}
		return activeTimerLoadedMsg{timer: timer}
	}
}

// loadHoursData loads both monthly and today hours in a single API call
func (m *MainMenuModel) loadHoursData() tea.Cmd {
	return func() tea.Msg {
		now := time.Now()
		monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		monthEnd := monthStart.AddDate(0, 1, 0)
		todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		todayEnd := todayStart.AddDate(0, 0, 1)

		timelogs, err := m.apiClient.GetTimelogs()
		if err != nil {
			return errorMsg(err)
		}

		var monthlyHours, todayHours float64
		for _, entry := range timelogs {
			startTime, err := entry.GetFromTimeAsTime()
			if err != nil {
				continue
			}

			// Calculate monthly hours
			if (startTime.After(monthStart) || startTime.Equal(monthStart)) && startTime.Before(monthEnd) {
				monthlyHours += entry.GetHoursAsFloat()
			}

			// Calculate today hours (only completed entries)
			if (startTime.After(todayStart) || startTime.Equal(todayStart)) && startTime.Before(todayEnd) {
				if entry.ToTime != "" {
					todayHours += entry.GetHoursAsFloat()
				}
			}
		}

		return hoursDataLoadedMsg{
			monthlyHours: monthlyHours,
			todayHours:   todayHours,
		}
	}
}

// stopActiveTimer stops the currently active timer using PUT to update with end_time
// Always shows a confirmation dialog where the user can review/edit the description
// and optionally fetch commit messages from a linked repository
func (m *MainMenuModel) stopActiveTimer() tea.Cmd {
	if m.activeTimer == nil {
		return nil
	}

	return func() tea.Msg {
		// Get existing description or empty string
		existingDescription := ""
		if m.activeTimer.Description != nil {
			existingDescription = *m.activeTimer.Description
		}

		// If no description, try to auto-generate from commits
		if existingDescription == "" {
			existingDescription = m.fetchGitHubCommitsForTimer()
		}

		// Always show confirmation dialog for user to review/edit
		return stopTimerDescriptionReadyMsg{description: existingDescription}
	}
}

// performStopTimerWithDescription actually stops the timer with the given description
func (m *MainMenuModel) performStopTimerWithDescription(description string) tea.Cmd {
	return func() tea.Msg {
		var err error
		if description != "" {
			err = m.apiClient.StopTimesheetWithDescription(m.activeTimer, description)
		} else {
			err = m.apiClient.StopTimesheet(m.activeTimer)
		}

		if err != nil {
			return errorMsg(err)
		}

		// Stop POW session and generate video
		if m.powCapturer != nil && m.powCapturer.IsEnabled() {
			videoPath, err := m.powCapturer.StopSession()
			if err != nil {
				menuDebugLog.Printf("Failed to stop POW session: %v", err)
			} else if videoPath != "" {
				menuDebugLog.Printf("POW video created: %s", videoPath)
			}
		}

		return timerStoppedMsg{}
	}
}

// appendCommitsToDescription fetches commits and appends them to the current description
func (m *MainMenuModel) appendCommitsToDescription() tea.Cmd {
	return func() tea.Msg {
		commits := m.fetchGitHubCommitsForTimer()
		if commits == "" {
			// No commits found, return message to show info
			return commitsAppendedMsg{noCommitsFound: true}
		}
		return commitsAppendedMsg{commits: commits}
	}
}

// togglePowMode toggles the POW (Proof of Work) screenshot capture mode
func (m *MainMenuModel) togglePowMode() tea.Cmd {
	return func() tea.Msg {
		if m.powCapturer == nil {
			// Create a new capturer if none exists
			cfg := pow.DefaultConfig()
			cfg.Enabled = true
			capturer, err := pow.New(cfg)
			if err != nil {
				return powToggledMsg{enabled: false, ok: false}
			}
			m.powCapturer = capturer
			return powToggledMsg{enabled: capturer.IsEnabled(), ok: capturer.IsEnabled()}
		}

		enabled, ok := m.powCapturer.Toggle()
		return powToggledMsg{enabled: enabled, ok: ok}
	}
}

// fetchGitCommitsForTimer fetches commit messages from a linked repo (local or GitHub)
// for the active timer's project and time range
func (m *MainMenuModel) fetchGitHubCommitsForTimer() string {
	if m.activeTimer == nil {
		return ""
	}

	// Load code repo associations from storage
	cfg, err := config.LoadConfig()
	if err != nil {
		menuDebugLog.Printf("fetchGitCommitsForTimer: failed to load config: %v", err)
		return ""
	}

	store, err := storage.New(cfg.GetStorageDir())
	if err != nil {
		menuDebugLog.Printf("fetchGitCommitsForTimer: failed to create storage: %v", err)
		return ""
	}

	associations, err := store.LoadCodeRepoAssociations()
	if err != nil {
		menuDebugLog.Printf("fetchGitCommitsForTimer: failed to load code repo associations: %v", err)
		return ""
	}

	// Find association for this project
	assoc := associations.GetAssociationByProject(m.activeTimer.ProjectID)
	if assoc == nil || !assoc.HasAssociation() {
		menuDebugLog.Printf("fetchGitCommitsForTimer: no repo linked to project %d", m.activeTimer.ProjectID)
		return ""
	}

	// Parse start time
	startTime, err := m.activeTimer.GetFromTimeAsTime()
	if err != nil {
		menuDebugLog.Printf("fetchGitCommitsForTimer: failed to parse start time: %v", err)
		return ""
	}

	// End time is now
	endTime := time.Now()

	// Check if this is a local path or a GitHub URL
	repoURL := assoc.RepoURL
	if api.IsLocalPath(repoURL) {
		// Local git repository
		return m.fetchLocalGitCommits(repoURL, startTime, endTime)
	}

	// GitHub repository
	return m.fetchGitHubCommits(assoc.RepoOwner, assoc.RepoName, startTime, endTime, cfg.GetGitHubToken())
}

// fetchLocalGitCommits fetches commits from a local git repository
func (m *MainMenuModel) fetchLocalGitCommits(repoPath string, startTime, endTime time.Time) string {
	// Expand ~ to home directory
	repoPath = api.ExpandPath(repoPath)

	menuDebugLog.Printf("fetchLocalGitCommits: fetching from local repo %s", repoPath)

	// Check if it's a valid git repository
	if !api.IsGitRepository(repoPath) {
		menuDebugLog.Printf("fetchLocalGitCommits: %s is not a git repository", repoPath)
		return ""
	}

	// Create local git client and fetch commits
	localGitClient := api.NewLocalGitClient()

	commits, err := localGitClient.GetCommitsInTimeRange(
		repoPath,
		startTime,
		endTime,
		"", // No author filter - get all commits
	)

	if err != nil {
		menuDebugLog.Printf("fetchLocalGitCommits: failed to fetch commits: %v", err)
		return ""
	}

	if len(commits) == 0 {
		menuDebugLog.Printf("fetchLocalGitCommits: no commits found in time range")
		return ""
	}

	menuDebugLog.Printf("fetchLocalGitCommits: found %d commits", len(commits))

	// Format commits as description
	return api.FormatLocalCommitsAsDescription(commits)
}

// fetchGitHubCommits fetches commits from a GitHub repository
func (m *MainMenuModel) fetchGitHubCommits(owner, repo string, startTime, endTime time.Time, githubToken string) string {
	menuDebugLog.Printf("fetchGitHubCommits: fetching from GitHub repo %s/%s", owner, repo)

	// Create GitHub client and fetch commits
	githubClient := api.NewGitHubClient(githubToken)

	commits, err := githubClient.GetCommitsInTimeRange(
		owner,
		repo,
		startTime,
		endTime,
		"", // No author filter - get all commits
	)

	if err != nil {
		menuDebugLog.Printf("fetchGitHubCommits: failed to fetch commits: %v", err)
		return ""
	}

	if len(commits) == 0 {
		menuDebugLog.Printf("fetchGitHubCommits: no commits found in time range")
		return ""
	}

	menuDebugLog.Printf("fetchGitHubCommits: found %d commits", len(commits))

	// Format commits as description
	return api.FormatCommitsAsDescription(commits)
}

// performLogout deletes the auth token and quits the application
func (m *MainMenuModel) performLogout() tea.Cmd {
	return func() tea.Msg {
		// Delete the auth token from disk
		if err := config.DeleteToken(); err != nil {
			return errorMsg(err)
		}

		return tea.Quit()
	}
}

// renderLogoutConfirmation renders the logout confirmation dialog
func (m *MainMenuModel) renderLogoutConfirmation() string {
	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#DDA036")).
		Padding(1, 2).
		Width(50)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#DDA036")).
		Align(lipgloss.Center).
		Width(46)

	messageStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Align(lipgloss.Center).
		Width(46)

	buttonYesStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(lipgloss.Color("#9A9EA0")).
		Padding(0, 2).
		Margin(0, 1)

	buttonYesSelectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(lipgloss.Color("#DDA036")).
		Bold(true).
		Padding(0, 2).
		Margin(0, 1)

	buttonNoStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(lipgloss.Color("#9A9EA0")).
		Padding(0, 2).
		Margin(0, 1)

	buttonNoSelectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(lipgloss.Color("#569FC6")).
		Bold(true).
		Padding(0, 2).
		Margin(0, 1)

	title := titleStyle.Render("Confirm Logout")
	message := messageStyle.Render("Are you sure you want to log out?")

	var yesButton, noButton string
	if m.logoutConfirmSelection == 0 {
		yesButton = buttonYesSelectedStyle.Render("Yes")
		noButton = buttonNoStyle.Render("No")
	} else {
		yesButton = buttonYesStyle.Render("Yes")
		noButton = buttonNoSelectedStyle.Render("No")
	}

	buttons := lipgloss.JoinHorizontal(lipgloss.Center, yesButton, noButton)
	buttonsAligned := lipgloss.NewStyle().
		Align(lipgloss.Center).
		Width(46).
		Render(buttons)

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		message,
		"",
		buttonsAligned,
	)

	return dialogStyle.Render(content)
}

// renderStopTimerConfirmation renders the stop timer confirmation dialog with editable description
func (m *MainMenuModel) renderStopTimerConfirmation() string {
	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#DDA036")).
		Padding(1, 2).
		Width(64)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#DDA036")).
		Align(lipgloss.Center).
		Width(60)

	messageStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Align(lipgloss.Center).
		Width(60)

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#569FC6")).
		Bold(true)

	buttonStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(lipgloss.Color("#9A9EA0")).
		Padding(0, 2).
		Margin(0, 1)

	buttonSaveSelectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(lipgloss.Color("#2ECC71")).
		Bold(true).
		Padding(0, 2).
		Margin(0, 1)

	buttonAddCommitsSelectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(lipgloss.Color("#3498DB")).
		Bold(true).
		Padding(0, 2).
		Margin(0, 1)

	buttonCancelSelectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(lipgloss.Color("#E74C3C")).
		Bold(true).
		Padding(0, 2).
		Margin(0, 1)

	title := titleStyle.Render("Stop Timer - Review Description")
	message := messageStyle.Render("Review and edit the description. Use 'Add Commits' to fetch git commits:")
	label := labelStyle.Render("Description:")

	// Render textarea
	textareaContent := m.stopTimerDescription.View()

	// Render buttons - 0 = Save, 1 = Add Commits, 2 = Cancel
	var saveButton, addCommitsButton, cancelButton string
	switch m.stopTimerConfirmSelection {
	case 0:
		saveButton = buttonSaveSelectedStyle.Render("Save & Stop")
		addCommitsButton = buttonStyle.Render("Add Commits")
		cancelButton = buttonStyle.Render("Cancel")
	case 1:
		saveButton = buttonStyle.Render("Save & Stop")
		addCommitsButton = buttonAddCommitsSelectedStyle.Render("Add Commits")
		cancelButton = buttonStyle.Render("Cancel")
	case 2:
		saveButton = buttonStyle.Render("Save & Stop")
		addCommitsButton = buttonStyle.Render("Add Commits")
		cancelButton = buttonCancelSelectedStyle.Render("Cancel")
	}

	buttons := lipgloss.JoinHorizontal(lipgloss.Center, saveButton, addCommitsButton, cancelButton)
	buttonsAligned := lipgloss.NewStyle().
		Align(lipgloss.Center).
		Width(60).
		Render(buttons)

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		message,
		"",
		label,
		textareaContent,
		"",
		buttonsAligned,
	)

	return dialogStyle.Render(content)
}

// updateDashboard updates the dashboard with current timer state
func (m *MainMenuModel) updateDashboard() {
	if m.dashboard == nil {
		m.dashboard = NewTimerDashboard()
	}

	m.dashboard.TodayWorked = m.todayHours
	m.dashboard.ShiftTarget = 8.0

	if m.activeTimer != nil {
		menuDebugLog.Printf("updateDashboard: activeTimer found")
		menuDebugLog.Printf("  ProjectName: '%s'", m.activeTimer.ProjectName)
		menuDebugLog.Printf("  TaskName: '%s'", m.activeTimer.TaskName)
		menuDebugLog.Printf("  Project: '%s'", m.activeTimer.Project)
		menuDebugLog.Printf("  Task: '%s'", m.activeTimer.Task)

		m.dashboard.IsActive = true
		m.dashboard.ProjectName = m.activeTimer.ProjectName
		m.dashboard.TaskName = m.activeTimer.TaskName

		// Calculate elapsed time from timer start
		startTime := m.activeTimer.StartTime()
		if !startTime.IsZero() {
			elapsed := time.Since(startTime)
			totalMinutes := int(elapsed.Minutes())
			m.dashboard.Hours = totalMinutes / 60
			m.dashboard.Minutes = totalMinutes % 60

			// Add current timer's elapsed time to today's worked hours
			m.dashboard.TodayWorked = m.todayHours + elapsed.Hours()
		}
	} else {
		menuDebugLog.Printf("updateDashboard: no active timer")
		m.dashboard.IsActive = false
		m.dashboard.Hours = 0
		m.dashboard.Minutes = 0
		m.dashboard.ProjectName = ""
		m.dashboard.TaskName = ""
	}
}

// startBlinkTicker starts the blink ticker for timer indicators and elapsed time updates
func (m *MainMenuModel) startBlinkTicker() tea.Cmd {
	return tea.Tick(blinkInterval, func(t time.Time) tea.Msg {
		return blinkTickMsg{}
	})
}

// Message types
type activeTimerLoadedMsg struct {
	timer *api.TimelogEntry
}

type monthlyHoursLoadedMsg struct {
	hours float64
}

type todayHoursLoadedMsg struct {
	hours float64
}

// hoursDataLoadedMsg contains both monthly and today hours from a single API call
type hoursDataLoadedMsg struct {
	monthlyHours float64
	todayHours   float64
}

type timerStoppedMsg struct{}

type blinkTickMsg struct{}

// stopTimerDescriptionReadyMsg is sent when the auto-generated description is ready
type stopTimerDescriptionReadyMsg struct {
	description string
}

// confirmStopTimerMsg is sent when the user confirms stopping the timer
type confirmStopTimerMsg struct {
	description string
}

// commitsAppendedMsg is sent when commits have been fetched for appending
type commitsAppendedMsg struct {
	commits        string
	noCommitsFound bool
}

// powToggledMsg is sent when POW mode has been toggled
type powToggledMsg struct {
	enabled bool
	ok      bool
}
