package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/kartoza/go-timesheets-go/internal/api"
	"github.com/kartoza/go-timesheets-go/internal/config"
)

// menuDebugLog is defined in debug_dev.go or debug_release.go

// Polling interval for active timer updates
const timerPollInterval = 1 * time.Minute

// Blink interval for timer indicators
const blinkInterval = 1 * time.Second

// MainMenuItem represents a menu option
type MainMenuItem int

const (
	MenuCreateTimer MainMenuItem = iota
	MenuStopTimer
	MenuWorkspaceAssociations
	MenuCodeRepos
	MenuViewHistory
	MenuOpenMonitor  // Only visible in debug builds
	MenuViewAPILog   // Only visible in debug builds
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
	confirmLogout          bool
	logoutConfirmSelection int // 0 = Yes, 1 = No
	dashboard              *TimerDashboard
	timerStartTime         time.Time
	blinkOn                bool // Blink state for timer indicators
}

type menuItem struct {
	label   string
	enabled bool
	action  MainMenuItem
}

// NewMainMenu creates a new main menu
func NewMainMenu(apiClient *api.Client, username string) *MainMenuModel {
	return &MainMenuModel{
		apiClient:    apiClient,
		username:     username,
		selectedItem: 0,
		width:        80,
		height:       24,
		dashboard:    NewTimerDashboard(),
	}
}

// Init initializes the main menu
func (m *MainMenuModel) Init() tea.Cmd {
	return tea.Batch(
		m.loadActiveTimer(),
		m.loadMonthlyHours(),
		m.loadTodayHours(),
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
		m.activeTimer = msg.timer
		m.updateMenuItems()
		m.updateDashboard()
		// If timer was stopped and we were on "Stop Running Timer", move focus to "Create New Timer"
		if msg.timer == nil && m.selectedItem == 1 {
			m.selectedItem = 0
		}
		// Start polling and blinking if timer is active
		if m.activeTimer != nil {
			return m, tea.Batch(
				m.startTimerPolling(),
				m.startBlinkTicker(),
			)
		}
		return m, nil

	case monthlyHoursLoadedMsg:
		m.monthlyHours = msg.hours
		return m, nil

	case todayHoursLoadedMsg:
		m.todayHours = msg.hours
		m.updateDashboard()
		return m, nil

	case timerPollTickMsg:
		// Refresh active timer data
		if m.activeTimer != nil {
			m.updateDashboard()
			return m, tea.Batch(
				m.loadActiveTimer(),
				m.loadTodayHours(),
				m.startTimerPolling(),
			)
		}
		return m, nil

	case timerStoppedMsg:
		// Timer was stopped, reload active timer and monthly hours
		m.err = nil // Clear any previous errors
		return m, tea.Batch(
			m.loadActiveTimer(),
			m.loadMonthlyHours(),
			m.loadTodayHours(),
		)

	case errorMsg:
		m.err = error(msg)
		return m, nil

	case blinkTickMsg:
		// Toggle blink state and update dashboard
		if m.activeTimer != nil {
			m.blinkOn = !m.blinkOn
			m.dashboard.BlinkOn = m.blinkOn
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
	if m.confirmLogout {
		mainContent = m.renderLogoutConfirmation()
	} else {
		mainContent = m.renderMenu()
	}

	// Render error message if any
	var errorContent string
	if m.err != nil {
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF6B6B")).
			Bold(true).
			Align(lipgloss.Center)
		errorContent = errorStyle.Render("Error: " + m.err.Error())
	}

	// Render help text
	help := m.renderHelp()

	// Combine main content (without help)
	parts := []string{header, "", dashboard, "", mainContent}
	if errorContent != "" {
		parts = append(parts, "", errorContent)
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

	if m.confirmLogout {
		return helpStyle.Render("←/→: Select • Enter/y: Confirm • n/Esc: Cancel")
	}

	return helpStyle.Render("↑/↓: Navigate • Enter: Select • Esc/q: Quit")
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
			label:   "Manage Workspace Associations",
			enabled: true,
			action:  MenuWorkspaceAssociations,
		},
		{
			label:   "Code Repos",
			enabled: true,
			action:  MenuCodeRepos,
		},
		{
			label:   "View Timesheet History",
			enabled: true,
			action:  MenuViewHistory,
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

// loadMonthlyHours loads the total hours for the current month
func (m *MainMenuModel) loadMonthlyHours() tea.Cmd {
	return func() tea.Msg {
		now := time.Now()
		monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		monthEnd := monthStart.AddDate(0, 1, 0)

		timelogs, err := m.apiClient.GetTimelogs()
		if err != nil {
			return errorMsg(err)
		}

		var totalHours float64
		for _, entry := range timelogs {
			startTime, err := entry.GetFromTimeAsTime()
			if err != nil {
				continue
			}

			if (startTime.After(monthStart) || startTime.Equal(monthStart)) && startTime.Before(monthEnd) {
				totalHours += entry.GetHoursAsFloat()
			}
		}

		return monthlyHoursLoadedMsg{hours: totalHours}
	}
}

// stopActiveTimer stops the currently active timer using PUT to update with end_time
func (m *MainMenuModel) stopActiveTimer() tea.Cmd {
	if m.activeTimer == nil {
		return nil
	}

	return func() tea.Msg {
		err := m.apiClient.StopTimesheet(m.activeTimer)
		if err != nil {
			return errorMsg(err)
		}

		// Reload active timer and monthly hours
		return timerStoppedMsg{}
	}
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

// loadTodayHours loads the total hours worked today
func (m *MainMenuModel) loadTodayHours() tea.Cmd {
	return func() tea.Msg {
		now := time.Now()
		todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		todayEnd := todayStart.AddDate(0, 0, 1)

		timelogs, err := m.apiClient.GetTimelogs()
		if err != nil {
			return errorMsg(err)
		}

		var totalHours float64
		for _, entry := range timelogs {
			startTime, err := entry.GetFromTimeAsTime()
			if err != nil {
				continue
			}

			// Only count completed entries (with end time) for today
			if (startTime.After(todayStart) || startTime.Equal(todayStart)) && startTime.Before(todayEnd) {
				// Don't count running timer here - it's calculated separately
				if entry.ToTime != "" {
					totalHours += entry.GetHoursAsFloat()
				}
			}
		}

		return todayHoursLoadedMsg{hours: totalHours}
	}
}

// startTimerPolling starts the polling timer for active timer updates
func (m *MainMenuModel) startTimerPolling() tea.Cmd {
	return tea.Tick(timerPollInterval, func(t time.Time) tea.Msg {
		return timerPollTickMsg{}
	})
}

// startBlinkTicker starts the blink ticker for timer indicators
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

type timerPollTickMsg struct{}

type timerStoppedMsg struct{}

type blinkTickMsg struct{}
