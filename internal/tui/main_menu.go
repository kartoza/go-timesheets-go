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
	"github.com/kartoza/go-timesheets-go/internal/models"
	"github.com/kartoza/go-timesheets-go/internal/pow"
	"github.com/kartoza/go-timesheets-go/internal/storage"
)

// menuDebugLog is defined in debug_dev.go or debug_release.go

// Blink interval for timer indicators (also updates elapsed time display locally)
const blinkInterval = 1 * time.Second

// MainMenuItem represents a menu option
type MainMenuItem int

const (
	MenuSettings MainMenuItem = iota
	MenuOpenMonitor // Only visible in debug builds
	MenuViewAPILog  // Only visible in debug builds
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
	width     int
	height    int
	err       error
	statusMsg string // Informational status message (not an error)
	dashboard *TimerDashboard
	timerStartTime         time.Time
	blinkOn                bool // Blink state for timer indicators

	// Stop timer confirmation dialog state
	confirmStopTimer          bool
	stopTimerDescription      textarea.Model
	stopTimerConfirmSelection int // 0 = Save, 1 = Add Commits, 2 = Cancel

	// POW (Proof of Work) capturer for screenshot capture during timer sessions
	powCapturer *pow.Capturer

	// Favourites grid state (embedded in main menu)
	favourites         *models.FavouriteAssociations
	favSelectedSlot    int  // 0-8 for the 3x3 grid
	favStorage         *storage.Storage
	favSuccessMessage  string
	favStatusMessage   string
}

type menuItem struct {
	label   string
	enabled bool
	action  MainMenuItem
}

// NewMainMenu creates a new main menu
// powCapturer can be nil if POW mode is not enabled
func NewMainMenu(apiClient *api.Client, username string, powCapturer *pow.Capturer) *MainMenuModel {
	// Initialize storage and load favourites
	cfg, _ := config.LoadConfig()
	storageDir := cfg.GetStorageDir()
	store, _ := storage.New(storageDir)

	favourites, err := store.LoadFavouriteAssociations()
	if err != nil {
		favourites = models.NewFavouriteAssociations()
	}

	return &MainMenuModel{
		apiClient:       apiClient,
		username:        username,
		selectedItem:    0,
		width:           80,
		height:          24,
		dashboard:       NewTimerDashboard(),
		powCapturer:     powCapturer,
		favourites:      favourites,
		favSelectedSlot: 0,
		favStorage:      store,
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

	case tea.MouseMsg:
		// Handle mouse clicks on menu items, favourites grid, and dashboard button
		if msg.Action == tea.MouseActionRelease && msg.Button == tea.MouseButtonLeft {
			// Skip if dialogs are open
			if m.confirmStopTimer {
				return m, nil
			}

			// Check if click is on the dashboard Start/Stop button
			if m.isClickOnDashboardButton(msg.X, msg.Y) {
				return m, m.handleDashboardButtonClick()
			}

			// Check if click is on the dashboard History button
			if m.isClickOnHistoryButton(msg.X, msg.Y) {
				return m, func() tea.Msg { return launchHistoryViewMsg{} }
			}

			// First check if click is on favourites grid
			favSlotIndex := m.getFavSlotFromMousePosition(msg.X, msg.Y)
			if favSlotIndex >= 0 && favSlotIndex < 9 {
				m.favSelectedSlot = favSlotIndex
				return m, m.handleFavouriteSelect()
			}

			// Calculate which menu item was clicked
			menuItemIndex := m.getMenuItemFromMousePosition(msg.X, msg.Y)
			if menuItemIndex >= 0 && menuItemIndex < len(m.menuItems) {
				m.selectedItem = menuItemIndex
				if m.menuItems[menuItemIndex].enabled {
					return m, m.handleMenuSelection()
				}
			}
		}
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

		case key.Matches(msg, key.NewBinding(key.WithKeys("s"))):
			// Start/Stop timer shortcut
			return m, m.handleDashboardButtonClick()

		case key.Matches(msg, key.NewBinding(key.WithKeys("h"))):
			// History shortcut
			return m, func() tea.Msg { return launchHistoryViewMsg{} }

		case key.Matches(msg, key.NewBinding(key.WithKeys("a"))):
			// AI Assistant shortcut
			return m, func() tea.Msg { return launchAIAssistantMsg{} }

		case key.Matches(msg, key.NewBinding(key.WithKeys("1", "2", "3", "4", "5", "6", "7", "8", "9"))):
			// Quick select favourite slot by number
			slotNum := int(msg.String()[0] - '0')
			m.favSelectedSlot = slotNum - 1
			return m, m.handleFavouriteSelect()

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

	case favTimerReadyToStartMsg:
		// Previous timer stopped, now start the new favourite timer
		return m, m.startFavouriteTimer(msg.slotIndex)

	case favTimerStartedFromMainMsg:
		// Timer started from favourites grid, reload active timer
		m.favSuccessMessage = fmt.Sprintf("Timer started: %s", msg.projectName)
		m.err = nil
		return m, tea.Batch(
			m.loadActiveTimer(),
			m.loadHoursData(),
		)
	}

	return m, nil
}

// View renders the main menu
func (m *MainMenuModel) View() string {
	// Render header
	header := m.renderHeader()

	// Render dashboard
	dashboard := m.dashboard.Render()

	// Render favourites grid
	favouritesGrid := m.renderFavouritesGrid()

	// Render menu items or confirmation dialog
	var mainContent string
	if m.confirmStopTimer {
		mainContent = m.renderStopTimerConfirmation()
	} else {
		mainContent = m.renderMenu()
	}

	// Render status or error message if any
	var messageContent string
	if m.favSuccessMessage != "" {
		statusStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#2ECC71")).
			Bold(true).
			Align(lipgloss.Center)
		messageContent = statusStyle.Render(m.favSuccessMessage)
	} else if m.favStatusMessage != "" {
		statusStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#569FC6")).
			Italic(true).
			Align(lipgloss.Center)
		messageContent = statusStyle.Render(m.favStatusMessage)
	} else if m.statusMsg != "" {
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
	// Layout: header -> dashboard -> favourites grid -> menu
	parts := []string{header, "", dashboard, "", favouritesGrid, "", mainContent}
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

	powStatus := "off"
	if m.powCapturer != nil && m.powCapturer.IsEnabled() {
		powStatus = "on"
	}
	return helpStyle.Render(fmt.Sprintf("s: Start/Stop • h: History • a: AI • 1-9: Quick start • p: POW (%s) • q: Quit", powStatus))
}

// getMenuItemFromMousePosition calculates which menu item was clicked based on mouse coordinates
// Returns -1 if the click was outside the menu area
func (m *MainMenuModel) getMenuItemFromMousePosition(x, y int) int {
	// The menu is rendered centered. We need to calculate its position.
	// Layout: header (~3) + dashboard (~16 with borders) + favourites grid (~17) + spacing = ~38 lines before menu
	// Dashboard box height is now 14 + 2 borders = 16

	// Estimate menu start Y (header ~3 + spacing + dashboard ~16 + spacing + favourites ~17 + spacing)
	menuStartY := 38

	// Check if click is within menu Y range
	clickedIndex := y - menuStartY
	if clickedIndex < 0 || clickedIndex >= len(m.menuItems) {
		return -1
	}

	// Check if X is roughly in the center where menu is rendered
	// Menu is centered, so check if X is within reasonable bounds
	menuWidth := 40 // Approximate menu width
	menuStartX := (m.width - menuWidth) / 2
	menuEndX := menuStartX + menuWidth

	if x < menuStartX-10 || x > menuEndX+10 {
		return -1
	}

	return clickedIndex
}

// updateMenuItems updates the menu items based on current state
func (m *MainMenuModel) updateMenuItems() {
	m.menuItems = []menuItem{
		{
			label:   "Settings",
			enabled: true,
			action:  MenuSettings,
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
}

// handleMenuSelection handles the selected menu item
func (m *MainMenuModel) handleMenuSelection() tea.Cmd {
	if m.selectedItem < 0 || m.selectedItem >= len(m.menuItems) {
		return nil
	}

	action := m.menuItems[m.selectedItem].action

	switch action {
	case MenuSettings:
		// Launch settings screen
		return func() tea.Msg {
			return launchSettingsMsg{}
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
// It also saves the timelogs to cache for use by the status command
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

		// Save timelogs to cache for waybar status command
		cfg, cfgErr := config.LoadConfig()
		if cfgErr == nil {
			store, storeErr := storage.New(cfg.GetStorageDir())
			if storeErr == nil {
				_ = store.SaveTimelogCache(timelogs)
			}
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
		// Capture the entry ID before stopping (we need it for POW video mapping)
		entryID := m.activeTimer.ID

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
				// Save the POW video path mapping to the entry ID
				cfg, cfgErr := config.LoadConfig()
				if cfgErr == nil {
					store, storeErr := storage.New(cfg.GetStorageDir())
					if storeErr == nil {
						if saveErr := store.SavePowVideoPath(entryID, videoPath); saveErr != nil {
							menuDebugLog.Printf("Failed to save POW video mapping: %v", saveErr)
						} else {
							menuDebugLog.Printf("POW video mapped to entry %d: %s", entryID, videoPath)
						}
					}
				}
			}
		}

		// Update cache for waybar status command
		timelogs, err := m.apiClient.GetTimelogs()
		if err == nil {
			cfg, cfgErr := config.LoadConfig()
			if cfgErr == nil {
				store, storeErr := storage.New(cfg.GetStorageDir())
				if storeErr == nil {
					_ = store.SaveTimelogCache(timelogs)
				}
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
	endTime := time.Now().UTC()

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

// renderFavouritesGrid renders the embedded 3x3 favourites grid
func (m *MainMenuModel) renderFavouritesGrid() string {
	buttonWidth := 18
	buttonHeight := 3 // Compact height for main menu

	var rows []string

	for row := 0; row < 3; row++ {
		var cols []string
		for col := 0; col < 3; col++ {
			slotIndex := row*3 + col
			cols = append(cols, m.renderFavButton(slotIndex, buttonWidth, buttonHeight))
		}
		rowContent := lipgloss.JoinHorizontal(lipgloss.Center, cols...)
		rows = append(rows, rowContent)
	}

	return lipgloss.JoinVertical(lipgloss.Center, rows...)
}

// renderFavButton renders a single favourite button for the main menu
func (m *MainMenuModel) renderFavButton(slotIndex, width, height int) string {
	assoc := m.favourites.Associations[slotIndex]
	isSelected := slotIndex == m.favSelectedSlot
	isRunning := m.isFavSlotRunning(slotIndex)

	// Base style
	style := lipgloss.NewStyle().
		Width(width).
		Height(height).
		Align(lipgloss.Center, lipgloss.Center).
		Border(lipgloss.RoundedBorder()).
		Margin(0, 1)

	// Style based on state - running takes priority
	if isRunning {
		style = style.
			BorderForeground(ColorGreen).
			Foreground(ColorGreen).
			Bold(true)
	} else if isSelected {
		style = style.
			BorderForeground(ColorBlue).
			Foreground(ColorBlue).
			Bold(true)
	} else if assoc.HasAssociation() {
		style = style.
			BorderForeground(ColorGray).
			Foreground(ColorWhite)
	} else {
		style = style.
			BorderForeground(ColorDarkGray).
			Foreground(ColorGray)
	}

	// Button content - compact for main menu
	slotNum := fmt.Sprintf("[%d]", slotIndex+1)
	name := assoc.Name
	if name == "" {
		name = assoc.GetShortDisplayString(width - 4)
	} else if len(name) > width-4 {
		name = name[:width-5] + "…"
	}

	var content string
	if assoc.HasAssociation() {
		if isRunning {
			content = lipgloss.JoinVertical(
				lipgloss.Center,
				slotNum+" ▶",
				name,
			)
		} else {
			content = lipgloss.JoinVertical(
				lipgloss.Center,
				slotNum,
				name,
			)
		}
	} else {
		content = lipgloss.JoinVertical(
			lipgloss.Center,
			slotNum,
			"Empty",
		)
	}

	return style.Render(content)
}

// isFavSlotRunning checks if a slot's project/task/activity matches the currently running timer
func (m *MainMenuModel) isFavSlotRunning(slotIndex int) bool {
	if m.activeTimer == nil {
		return false
	}

	assoc := m.favourites.Associations[slotIndex]
	if !assoc.HasAssociation() {
		return false
	}

	// Compare project, task, and activity IDs
	if m.activeTimer.ProjectID != assoc.ProjectID {
		return false
	}

	// Compare task - both must match (or both be unset)
	activeTaskID := m.activeTimer.TaskID.Value
	if activeTaskID != assoc.TaskID {
		return false
	}

	// Compare activity
	if m.activeTimer.ActivityID != assoc.ActivityID {
		return false
	}

	return true
}

// getFavSlotFromMousePosition calculates which favourite slot was clicked
func (m *MainMenuModel) getFavSlotFromMousePosition(x, y int) int {
	// Grid layout constants (must match renderFavButton)
	buttonWidth := 18 + 2  // width + margin
	buttonHeight := 3 + 2  // height + borders (rounded border adds 2)

	// Calculate grid dimensions
	gridWidth := buttonWidth * 3
	gridHeight := buttonHeight * 3

	// Calculate actual vertical position based on rendered layout:
	// - Header: ~3 lines
	// - Empty line: 1
	// - Dashboard: boxHeight(14) + borders(2) = 16 lines
	// - Empty line: 1
	// Total before favourites grid: ~21 lines
	gridStartY := 21

	// Calculate grid start X position (centered)
	gridStartX := (m.width - gridWidth) / 2

	// Check if click is within grid bounds
	if x < gridStartX || x >= gridStartX+gridWidth {
		return -1
	}
	if y < gridStartY || y >= gridStartY+gridHeight {
		return -1
	}

	// Calculate column (0-2) and row (0-2)
	col := (x - gridStartX) / buttonWidth
	row := (y - gridStartY) / buttonHeight

	// Bounds check
	if col < 0 || col > 2 || row < 0 || row > 2 {
		return -1
	}

	return row*3 + col
}

// handleFavouriteSelect handles starting a timer from a favourite slot
func (m *MainMenuModel) handleFavouriteSelect() tea.Cmd {
	assoc := m.favourites.Associations[m.favSelectedSlot]
	if !assoc.HasAssociation() {
		m.err = fmt.Errorf("slot %d is not configured - use Favourites menu to edit", m.favSelectedSlot+1)
		return nil
	}

	// Check if this slot is already running
	if m.isFavSlotRunning(m.favSelectedSlot) {
		m.favStatusMessage = "This task is already running"
		return nil
	}

	// Check if there's an active timer that needs to be stopped first
	if m.activeTimer != nil {
		return m.stopCurrentAndStartFavourite(m.favSelectedSlot)
	}

	// No active timer, just start new one
	return m.startFavouriteTimer(m.favSelectedSlot)
}

// stopCurrentAndStartFavourite stops the current timer and starts a new one from favourite
func (m *MainMenuModel) stopCurrentAndStartFavourite(slotIndex int) tea.Cmd {
	return func() tea.Msg {
		if m.activeTimer == nil {
			return favTimerReadyToStartMsg{slotIndex: slotIndex}
		}

		// Capture entry ID for POW video mapping
		entryID := m.activeTimer.ID

		// Try to get git commits for description
		description := m.fetchGitHubCommitsForTimer()

		// Stop the timer
		var err error
		if description != "" {
			err = m.apiClient.StopTimesheetWithDescription(m.activeTimer, description)
		} else {
			err = m.apiClient.StopTimesheet(m.activeTimer)
		}

		if err != nil {
			return errorMsg(fmt.Errorf("failed to stop timer: %w", err))
		}

		// Stop POW session and generate video
		if m.powCapturer != nil && m.powCapturer.IsEnabled() {
			videoPath, err := m.powCapturer.StopSession()
			if err != nil {
				menuDebugLog.Printf("Failed to stop POW session: %v", err)
			} else if videoPath != "" {
				menuDebugLog.Printf("POW video created: %s", videoPath)
				cfg, cfgErr := config.LoadConfig()
				if cfgErr == nil {
					store, storeErr := storage.New(cfg.GetStorageDir())
					if storeErr == nil {
						_ = store.SavePowVideoPath(entryID, videoPath)
					}
				}
			}
		}

		return favTimerReadyToStartMsg{slotIndex: slotIndex}
	}
}

// startFavouriteTimer starts a timer from a favourite slot
func (m *MainMenuModel) startFavouriteTimer(slotIndex int) tea.Cmd {
	return func() tea.Msg {
		assoc := m.favourites.Associations[slotIndex]
		if !assoc.HasAssociation() {
			return errorMsg(fmt.Errorf("slot %d is not configured", slotIndex+1))
		}

		// Build description from favourite name if available
		description := ""
		if assoc.Name != "" {
			description = assoc.Name
		}

		// Create the TimeEntry model
		entry := models.TimeEntry{
			ProjectID:   fmt.Sprintf("%d", assoc.ProjectID),
			ActivityID:  fmt.Sprintf("%d", assoc.ActivityID),
			Description: description,
			StartTime:   time.Now().UTC(),
		}

		// Set TaskID if configured
		if assoc.TaskID > 0 {
			taskID := fmt.Sprintf("%d", assoc.TaskID)
			entry.TaskID = &taskID
		}

		err := m.apiClient.CreateTimesheet(entry)
		if err != nil {
			return errorMsg(fmt.Errorf("failed to start timer: %w", err))
		}

		return favTimerStartedFromMainMsg{projectName: assoc.ProjectName}
	}
}

// isClickOnDashboardButton checks if a mouse click is on the dashboard Start/Stop button
func (m *MainMenuModel) isClickOnDashboardButton(x, y int) bool {
	// Dashboard layout:
	// - Header: ~3 lines
	// - Empty line: 1
	// - Dashboard box starts at ~4, height is 14 (increased for button)
	// - Button is at the bottom of the timer box (left box)
	// Timer box is 40 wide, centered with gauge box (also 40 wide) with 2 space gap
	// Total dashboard width: 40 + 2 + 40 = 82

	dashboardWidth := 82
	timerBoxWidth := 40
	dashboardStartX := (m.width - dashboardWidth) / 2
	timerBoxEndX := dashboardStartX + timerBoxWidth

	// Button is roughly at line 15-16 (header 3 + empty 1 + box content ~11-12)
	buttonY := 15

	// Check if click is within timer box horizontal bounds
	if x < dashboardStartX || x > timerBoxEndX {
		return false
	}

	// Check if click is on or near the button row
	if y >= buttonY-1 && y <= buttonY+1 {
		return true
	}

	return false
}

// handleDashboardButtonClick handles clicking the Start/Stop button in the dashboard
func (m *MainMenuModel) handleDashboardButtonClick() tea.Cmd {
	if m.activeTimer != nil {
		// Stop the timer
		return m.stopActiveTimer()
	}
	// Start a new timer - launch timer creation
	return func() tea.Msg {
		return launchTimerCreationMsg{}
	}
}

// isClickOnHistoryButton checks if a mouse click is on the History button in the Daily Progress box
func (m *MainMenuModel) isClickOnHistoryButton(x, y int) bool {
	// Dashboard layout:
	// - The Daily Progress box is to the right of the timer box
	// - Timer box is 40 wide, then 2 space gap, then gauge box 40 wide
	// Total dashboard width: 40 + 2 + 40 = 82

	dashboardWidth := 82
	timerBoxWidth := 40
	dashboardStartX := (m.width - dashboardWidth) / 2
	gaugeBoxStartX := dashboardStartX + timerBoxWidth + 2
	gaugeBoxEndX := gaugeBoxStartX + timerBoxWidth

	// Button is at similar position as the Start/Stop button (bottom of box)
	buttonY := 15

	// Check if click is within gauge box horizontal bounds
	if x < gaugeBoxStartX || x > gaugeBoxEndX {
		return false
	}

	// Check if click is on or near the button row
	if y >= buttonY-1 && y <= buttonY+1 {
		return true
	}

	return false
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

// favTimerReadyToStartMsg is sent when previous timer stopped and ready to start favourite
type favTimerReadyToStartMsg struct {
	slotIndex int
}

// favTimerStartedFromMainMsg is sent when a timer was started from the main menu favourites grid
type favTimerStartedFromMainMsg struct {
	projectName string
}
