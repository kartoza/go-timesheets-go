package tui

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SettingsMenuItem represents a settings menu option
type SettingsMenuItem int

const (
	SettingsEditFavourites SettingsMenuItem = iota
	SettingsCodeRepos
	SettingsCalendar
	SettingsImportCSV
	SettingsRefreshProjects
	SettingsLogOut
	SettingsBack
)

// SettingsModel represents the settings screen
type SettingsModel struct {
	width        int
	height       int
	selectedItem int
	menuItems    []settingsMenuItem
	headerState  *HeaderState

	// Logout confirmation
	confirmLogout          bool
	logoutConfirmSelection int // 0 = Yes, 1 = No

	// Transient status message shown beneath the menu (e.g. "Syncing…").
	statusMessage string
}

type settingsMenuItem struct {
	label  string
	action SettingsMenuItem
}

// NewSettingsModel creates a new settings model
func NewSettingsModel() *SettingsModel {
	m := &SettingsModel{
		width:        80,
		height:       24,
		selectedItem: 0,
	}
	m.updateMenuItems()
	return m
}

// Init initializes the settings view
func (m *SettingsModel) Init() tea.Cmd {
	return nil
}

// Update handles messages for the settings screen
func (m *SettingsModel) Update(msg tea.Msg) (*SettingsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case refreshProjectsResultMsg:
		if msg.err != nil {
			m.statusMessage = "Sync failed: " + msg.err.Error()
		} else {
			m.statusMessage = "Projects synced."
		}
		return m, nil

	case tea.MouseMsg:
		if msg.Action == tea.MouseActionRelease && msg.Button == tea.MouseButtonLeft {
			if m.confirmLogout {
				return m, nil
			}

			// Calculate which menu item was clicked
			menuItemIndex := m.getMenuItemFromMousePosition(msg.X, msg.Y)
			if menuItemIndex >= 0 && menuItemIndex < len(m.menuItems) {
				m.selectedItem = menuItemIndex
				return m.handleMenuSelection()
			}
		}
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
					return m, func() tea.Msg { return settingsLogoutConfirmedMsg{} }
				} else {
					m.confirmLogout = false
					return m, nil
				}
			case "n", "esc":
				m.confirmLogout = false
				return m, nil
			case "y":
				return m, func() tea.Msg { return settingsLogoutConfirmedMsg{} }
			}
			return m, nil
		}

		// Normal navigation
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("ctrl+c"))):
			return m, tea.Quit

		case key.Matches(msg, key.NewBinding(key.WithKeys("esc", "q"))):
			return m, func() tea.Msg { return backToMenuMsg{} }

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
			return m.handleMenuSelection()
		}
	}

	return m, nil
}

// View renders the settings screen
func (m *SettingsModel) View() string {
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

	// Combine parts - header at top, then menu content
	mainSection := lipgloss.JoinVertical(
		lipgloss.Center,
		header,
		"",
		"",
		mainContent,
	)

	// Place main content at top (not centered vertically)
	centered := lipgloss.Place(
		m.width,
		m.height-2,
		lipgloss.Center,
		lipgloss.Top,
		mainSection,
	)

	// Add help at bottom
	helpStyle := lipgloss.NewStyle().
		Width(m.width).
		Align(lipgloss.Center)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		centered,
		helpStyle.Render(help),
	)
}

// renderHeader renders the header
func (m *SettingsModel) renderHeader() string {
	if m.headerState != nil {
		return RenderHeader("Settings", m.headerState)
	}
	return RenderHeader("Settings", &HeaderState{})
}

// SetHeaderState sets the shared header state
func (m *SettingsModel) SetHeaderState(state *HeaderState) {
	m.headerState = state
}

// renderMenu renders the menu items
func (m *SettingsModel) renderMenu() string {
	normalStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#569FC6")).
		Padding(0, 2)

	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#DDA036")).
		Bold(true).
		Padding(0, 2)

	var items []string
	for i, item := range m.menuItems {
		prefix := "  "
		if i == m.selectedItem {
			prefix = "▶ "
		}

		var rendered string
		if i == m.selectedItem {
			rendered = selectedStyle.Render(prefix + item.label)
		} else {
			rendered = normalStyle.Render(prefix + item.label)
		}

		items = append(items, rendered)
	}

	if m.statusMessage != "" {
		statusStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9A9EA0")).
			Italic(true).
			Padding(1, 2)
		items = append(items, "", statusStyle.Render(m.statusMessage))
	}

	return lipgloss.JoinVertical(lipgloss.Left, items...)
}

// renderHelp renders the help text
func (m *SettingsModel) renderHelp() string {
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9A9EA0")).
		Italic(true)

	if m.confirmLogout {
		return helpStyle.Render("←/→: Select • Enter/y: Confirm • n/Esc: Cancel")
	}

	return helpStyle.Render("↑/↓: Navigate • Enter: Select • Esc/q: Back")
}

// renderLogoutConfirmation renders the logout confirmation dialog
func (m *SettingsModel) renderLogoutConfirmation() string {
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

// getMenuItemFromMousePosition calculates which menu item was clicked
func (m *SettingsModel) getMenuItemFromMousePosition(x, y int) int {
	// Menu is anchored at top after header
	// Header takes ~3 lines, then 2 empty lines, then menu starts
	menuStartY := 5 // header (3) + empty lines (2)

	clickedIndex := y - menuStartY
	if clickedIndex < 0 || clickedIndex >= len(m.menuItems) {
		return -1
	}

	// Check X bounds - menu is centered horizontally
	menuWidth := 40
	menuStartX := (m.width - menuWidth) / 2
	menuEndX := menuStartX + menuWidth

	if x < menuStartX-10 || x > menuEndX+10 {
		return -1
	}

	return clickedIndex
}

// updateMenuItems updates the menu items
func (m *SettingsModel) updateMenuItems() {
	m.menuItems = []settingsMenuItem{
		{label: "Edit Favourites", action: SettingsEditFavourites},
		{label: "Manage Code Repos", action: SettingsCodeRepos},
		{label: "Google Calendar", action: SettingsCalendar},
		{label: "Import CSV History", action: SettingsImportCSV},
		{label: "Sync Projects from ERPNext", action: SettingsRefreshProjects},
		{label: "Log Out", action: SettingsLogOut},
		{label: "← Back to Main Menu", action: SettingsBack},
	}
}

// handleMenuSelection handles the selected menu item
func (m *SettingsModel) handleMenuSelection() (*SettingsModel, tea.Cmd) {
	if m.selectedItem < 0 || m.selectedItem >= len(m.menuItems) {
		return m, nil
	}

	action := m.menuItems[m.selectedItem].action

	switch action {
	case SettingsEditFavourites:
		return m, func() tea.Msg { return launchFavouritesMsg{} }

	case SettingsCodeRepos:
		return m, func() tea.Msg { return launchCodeReposMsg{} }

	case SettingsCalendar:
		return m, func() tea.Msg { return launchCalendarSettingsMsg{} }

	case SettingsImportCSV:
		return m, func() tea.Msg { return launchImportCSVMsg{} }

	case SettingsRefreshProjects:
		m.statusMessage = "Syncing projects from ERPNext…"
		return m, func() tea.Msg { return refreshProjectsRequestMsg{} }

	case SettingsLogOut:
		m.confirmLogout = true
		m.logoutConfirmSelection = 1 // Default to "No"
		return m, nil

	case SettingsBack:
		return m, func() tea.Msg { return backToMenuMsg{} }
	}

	return m, nil
}

// Message types for settings
type settingsLogoutConfirmedMsg struct{}
type launchImportCSVMsg struct{}
type launchCalendarSettingsMsg struct{}

// refreshProjectsRequestMsg is emitted when the user picks "Sync Projects
// from ERPNext"; the parent app handles it by calling the API client.
type refreshProjectsRequestMsg struct{}

// refreshProjectsResultMsg carries the outcome back to the settings model
// so it can render a status line.
type refreshProjectsResultMsg struct {
	err error
}
