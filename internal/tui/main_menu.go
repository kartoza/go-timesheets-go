package tui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/kartoza/go-timesheets-go/internal/api"
	"github.com/kartoza/go-timesheets-go/internal/config"
)

// MainMenuItem represents a menu option
type MainMenuItem int

const (
	MenuCreateTimer MainMenuItem = iota
	MenuStopTimer
	MenuWorkspaceAssociations
	MenuViewHistory
	MenuLogOut
	MenuQuit
)

// MainMenuModel represents the main menu screen
type MainMenuModel struct {
	apiClient       *api.Client
	username        string
	activeTimer     *api.TimelogEntry
	monthlyHours    float64
	selectedItem    int
	menuItems       []menuItem
	width           int
	height          int
	err             error
	confirmLogout   bool
	logoutConfirmSelection int // 0 = Yes, 1 = No
}

type menuItem struct {
	label    string
	enabled  bool
	action   MainMenuItem
}

// NewMainMenu creates a new main menu
func NewMainMenu(apiClient *api.Client, username string) *MainMenuModel {
	return &MainMenuModel{
		apiClient:    apiClient,
		username:     username,
		selectedItem: 0,
		width:        80,
		height:       24,
	}
}

// Init initializes the main menu
func (m *MainMenuModel) Init() tea.Cmd {
	return tea.Batch(
		m.loadActiveTimer(),
		m.loadMonthlyHours(),
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
		case key.Matches(msg, key.NewBinding(key.WithKeys("ctrl+c", "q"))):
			if m.selectedItem == len(m.menuItems)-1 || msg.String() == "ctrl+c" {
				return m, tea.Quit
			}

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
		return m, nil

	case monthlyHoursLoadedMsg:
		m.monthlyHours = msg.hours
		return m, nil

	case timerStoppedMsg:
		// Timer was stopped, reload active timer and monthly hours
		return m, tea.Batch(
			m.loadActiveTimer(),
			m.loadMonthlyHours(),
		)

	case errorMsg:
		m.err = error(msg)
		return m, nil
	}

	return m, nil
}

// View renders the main menu
func (m *MainMenuModel) View() string {
	// Render header
	header := m.renderHeader()

	// Render menu items or confirmation dialog
	var mainContent string
	if m.confirmLogout {
		mainContent = m.renderLogoutConfirmation()
	} else {
		mainContent = m.renderMenu()
	}

	// Render help text
	help := m.renderHelp()

	// Combine all sections
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		"",
		mainContent,
		"",
		help,
	)

	// Center content
	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Top,
		content,
	)
}

// renderHeader renders the consistent header across all screens
func (m *MainMenuModel) renderHeader() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#E95420")).
		Align(lipgloss.Center).
		Width(60)

	mottoStyle := lipgloss.NewStyle().
		Italic(true).
		Foreground(lipgloss.Color("#888888")).
		Align(lipgloss.Center).
		Width(60)

	dividerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#666666")).
		Width(60)

	statusStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Width(60)

	title := titleStyle.Render("Kartoza Timesheets")
	motto := mottoStyle.Render("Time Flies")
	divider := dividerStyle.Render("────────────────────────────────────────────────────────────")

	// Build status line
	trackerState := "Inactive"
	stateColor := lipgloss.Color("#888888")
	if m.activeTimer != nil {
		trackerState = "● Active"
		stateColor = lipgloss.Color("#E95420")
	}

	trackerStateStyled := lipgloss.NewStyle().
		Foreground(stateColor).
		Bold(true).
		Render(trackerState)

	statusLine := fmt.Sprintf("User: %s  |  Tracker: %s  |  Monthly Hours: %.1fh",
		m.username,
		trackerStateStyled,
		m.monthlyHours,
	)

	status := statusStyle.Render(statusLine)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		motto,
		divider,
		status,
		divider,
	)
}

// renderMenu renders the menu items
func (m *MainMenuModel) renderMenu() string {
	m.updateMenuItems()

	normalStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Padding(0, 2)

	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#E95420")).
		Bold(true).
		Padding(0, 2)

	disabledStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#666666")).
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

	menuTitle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#E95420")).
		Padding(0, 2).
		Render("Main Menu")

	return lipgloss.JoinVertical(
		lipgloss.Left,
		menuTitle,
		"",
		lipgloss.JoinVertical(lipgloss.Left, items...),
	)
}

// renderHelp renders the help text
func (m *MainMenuModel) renderHelp() string {
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#666666")).
		Italic(true)

	if m.confirmLogout {
		return helpStyle.Render("←/→: Select • Enter/y: Confirm • n/Esc: Cancel")
	}

	return helpStyle.Render("↑/↓: Navigate • Enter: Select • q: Quit")
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
			label:   "View Timesheet History",
			enabled: true,
			action:  MenuViewHistory,
		},
		{
			label:   "Log Out",
			enabled: true,
			action:  MenuLogOut,
		},
		{
			label:   "Quit",
			enabled: true,
			action:  MenuQuit,
		},
	}
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

	case MenuViewHistory:
		// Launch history view
		return func() tea.Msg {
			return launchHistoryViewMsg{}
		}

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

// stopActiveTimer stops the currently active timer
func (m *MainMenuModel) stopActiveTimer() tea.Cmd {
	if m.activeTimer == nil {
		return nil
	}

	return func() tea.Msg {
		_, err := m.apiClient.BreakTimesheet(m.activeTimer.ID)
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
		BorderForeground(lipgloss.Color("#E95420")).
		Padding(1, 2).
		Width(50)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#E95420")).
		Align(lipgloss.Center).
		Width(46)

	messageStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Align(lipgloss.Center).
		Width(46)

	buttonYesStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(lipgloss.Color("#666666")).
		Padding(0, 2).
		Margin(0, 1)

	buttonYesSelectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(lipgloss.Color("#E95420")).
		Bold(true).
		Padding(0, 2).
		Margin(0, 1)

	buttonNoStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(lipgloss.Color("#666666")).
		Padding(0, 2).
		Margin(0, 1)

	buttonNoSelectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(lipgloss.Color("#51CF66")).
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

// Message types
type activeTimerLoadedMsg struct {
	timer *api.TimelogEntry
}

type monthlyHoursLoadedMsg struct {
	hours float64
}

type timerStoppedMsg struct{}
