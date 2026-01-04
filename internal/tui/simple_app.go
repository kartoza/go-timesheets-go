package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/kartoza/go-timesheets-go/internal/api"
	"github.com/kartoza/go-timesheets-go/internal/config"
	"github.com/kartoza/go-timesheets-go/internal/models"
	"github.com/kartoza/go-timesheets-go/internal/service"
	"github.com/kartoza/go-timesheets-go/internal/storage"
)

// Message types
type tickMsg time.Time
type activeEntryMsg *models.ActiveTimeEntry
type projectsLoadedMsg []api.ProjectListItem
type tasksLoadedMsg []api.TaskListItem
type activitiesLoadedMsg []api.ActivityListItem
type historyLoadedMsg []api.TimelogEntry
type errorMsg error

// Tab represents different views
type Tab int

const (
	TabProjects Tab = iota
	TabProjectDetails
	TabActivities
	TabHistory
)

// SimpleApp represents a simplified TUI application
type SimpleApp struct {
	service         *service.TimesheetService
	apiClient       *api.Client
	width           int
	height          int
	activeEntry     *models.ActiveTimeEntry
	currentTab      Tab

	// Data
	projects        []api.ProjectListItem
	tasks           []api.TaskListItem
	activities      []api.ActivityListItem
	history         []api.TimelogEntry

	// Selection state
	selectedProject    *api.ProjectListItem
	selectedTask       *api.TaskListItem
	selectedActivity   *api.ActivityListItem
	lastUsedActivityID int

	// Cursor positions for navigation
	projectCursor   int
	taskCursor      int
	activityCursor  int
	historyCursor   int

	// Search inputs
	projectSearchInput textinput.Model
	searchMode         bool // true when user is typing in search box

	err             error
}

// NewSimpleApp creates a simplified app
func NewSimpleApp() (*SimpleApp, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %w", err)
	}

	dataDir := filepath.Join(homeDir, ".kartoza-timesheet")
	storage, err := storage.New(dataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize storage: %w", err)
	}

	service := service.New(storage, "default-user")

	// Try to load API client with saved token
	var apiClient *api.Client
	token, err := config.LoadToken()
	if err == nil && token != nil {
		apiClient, err = api.NewClient(api.Config{
			BaseURL:   token.BaseURL,
			AuthToken: token.Token,
			Timeout:   30,
		})
		if err != nil {
			// Failed to create client, continue without API
			apiClient = nil
		}
	}

	// Initialize project search input
	projectSearchInput := textinput.New()
	projectSearchInput.Placeholder = "Type project name (min 2 chars)..."
	projectSearchInput.CharLimit = 100
	projectSearchInput.Width = 50
	projectSearchInput.Focus() // Focus the input so it accepts typing

	app := &SimpleApp{
		service:            service,
		apiClient:          apiClient,
		currentTab:         TabProjects,
		projectSearchInput: projectSearchInput,
		searchMode:         true, // Start in search mode on Tab 1
	}

	return app, nil
}

func (a *SimpleApp) Init() tea.Cmd {
	return tea.Batch(
		// Don't load projects initially - wait for user to type
		a.loadActivities(),
		a.loadActiveEntry(),
		textinput.Blink,
		tea.Tick(time.Second, func(t time.Time) tea.Msg {
			return tickMsg(t)
		}),
	)
}

func (a *SimpleApp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height

	case tea.KeyMsg:
		// Handle Tab 1 search mode separately
		if a.currentTab == TabProjects && a.searchMode {
			switch msg.String() {
			case "ctrl+c":
				return a, tea.Quit
			case "esc":
				// Exit search mode, go to list navigation
				if len(a.projects) > 0 {
					a.searchMode = false
					a.projectSearchInput.Blur()
				}
			case "tab":
				// Switch from search to list navigation
				if len(a.projects) > 0 {
					a.searchMode = false
					a.projectSearchInput.Blur()
				}
			case "enter":
				// If there's exactly one result or cursor on first result, select it
				if len(a.projects) > 0 {
					a.searchMode = false
					a.projectSearchInput.Blur()
				}
			default:
				// Update search input
				var cmd tea.Cmd
				a.projectSearchInput, cmd = a.projectSearchInput.Update(msg)
				cmds = append(cmds, cmd)

				// Trigger search if >= 2 characters
				query := a.projectSearchInput.Value()
				if len(query) >= 2 {
					cmds = append(cmds, a.searchProjects(query))
				} else if len(query) == 0 {
					// Clear results if query is empty
					a.projects = nil
					a.projectCursor = 0
				}
			}
			return a, tea.Batch(cmds...)
		}

		// Normal navigation mode
		switch msg.String() {
		case "ctrl+c":
			return a, tea.Quit

		case "ctrl+l":
			// Logout - delete token and quit
			config.DeleteToken()
			return a, tea.Quit

		case "1":
			a.currentTab = TabProjects
			a.selectedProject = nil
			a.selectedTask = nil
			a.selectedActivity = nil
			a.searchMode = true
			a.projectSearchInput.Focus()
			cmds = append(cmds, textinput.Blink)

		case "2":
			if a.selectedProject != nil {
				a.currentTab = TabProjectDetails
			}

		case "3":
			if a.selectedTask != nil {
				a.currentTab = TabActivities
			}

		case "4":
			if a.selectedActivity != nil {
				a.currentTab = TabHistory
			}

		case "backspace", "esc":
			// Navigate back or enter search mode on Tab 1
			switch a.currentTab {
			case TabProjects:
				if !a.searchMode {
					// Go back to search mode
					a.searchMode = true
					a.projectSearchInput.Focus()
					cmds = append(cmds, textinput.Blink)
				}
			case TabProjectDetails:
				a.currentTab = TabProjects
				a.selectedProject = nil
				a.selectedTask = nil
			case TabActivities:
				a.currentTab = TabProjectDetails
				a.selectedActivity = nil
			case TabHistory:
				a.currentTab = TabActivities
			}

		case "up", "k":
			a.moveCursorUp()

		case "down", "j":
			a.moveCursorDown()

		case "enter", " ":
			cmd := a.handleSelection()
			if cmd != nil {
				cmds = append(cmds, cmd)
			}

		case "/":
			// Enter search mode on Tab 1
			if a.currentTab == TabProjects {
				a.searchMode = true
				a.projectSearchInput.Focus()
				cmds = append(cmds, textinput.Blink)
			}
		}

	case tickMsg:
		cmds = append(cmds, tea.Tick(time.Second, func(t time.Time) tea.Msg {
			return tickMsg(t)
		}))

	case activeEntryMsg:
		a.activeEntry = (*models.ActiveTimeEntry)(msg)

	case projectsLoadedMsg:
		a.projects = []api.ProjectListItem(msg)

	case tasksLoadedMsg:
		a.tasks = []api.TaskListItem(msg)
		a.taskCursor = 0

	case activitiesLoadedMsg:
		a.activities = []api.ActivityListItem(msg)
		// Set cursor to last used activity if available
		if a.lastUsedActivityID > 0 {
			for i, act := range a.activities {
				if act.ID == a.lastUsedActivityID {
					a.activityCursor = i
					break
				}
			}
		}

	case historyLoadedMsg:
		a.history = []api.TimelogEntry(msg)
		// Sort by start time, most recent first
		sort.Slice(a.history, func(i, j int) bool {
			return a.history[i].StartTime.After(a.history[j].StartTime)
		})
		a.historyCursor = 0

	case errorMsg:
		a.err = error(msg)
	}

	return a, tea.Batch(cmds...)
}

func (a *SimpleApp) View() string {
	if a.width == 0 {
		return "Loading..."
	}

	var content string

	switch a.currentTab {
	case TabProjects:
		content = a.renderProjectsView()
	case TabProjectDetails:
		content = a.renderProjectDetailsView()
	case TabActivities:
		content = a.renderActivitiesView()
	case TabHistory:
		content = a.renderHistoryView()
	}

	nav := a.renderNav()

	if a.err != nil {
		errorMsg := lipgloss.NewStyle().
			Foreground(lipgloss.Color("9")).
			Render(fmt.Sprintf("Error: %v", a.err))
		content = lipgloss.JoinVertical(lipgloss.Top, content, errorMsg)
	}

	return lipgloss.JoinVertical(lipgloss.Top, nav, content)
}

func (a *SimpleApp) renderNav() string {
	activeStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#DF9E2F")).
		Foreground(lipgloss.Color("15")).
		Padding(0, 1).
		Bold(true)

	inactiveStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("8")).
		Foreground(lipgloss.Color("15")).
		Padding(0, 1)

	disabledStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("236")).
		Foreground(lipgloss.Color("240")).
		Padding(0, 1)

	var tab1, tab2, tab3, tab4 string

	// Tab 1: Projects
	if a.currentTab == TabProjects {
		tab1 = activeStyle.Render("1. Projects")
	} else {
		tab1 = inactiveStyle.Render("1. Projects")
	}

	// Tab 2: Project Details (enabled only if project selected)
	if a.selectedProject != nil {
		if a.currentTab == TabProjectDetails {
			tab2 = activeStyle.Render("2. Task")
		} else {
			tab2 = inactiveStyle.Render("2. Task")
		}
	} else {
		tab2 = disabledStyle.Render("2. Task")
	}

	// Tab 3: Activities (enabled only if task selected)
	if a.selectedTask != nil {
		if a.currentTab == TabActivities {
			tab3 = activeStyle.Render("3. Activity")
		} else {
			tab3 = inactiveStyle.Render("3. Activity")
		}
	} else {
		tab3 = disabledStyle.Render("3. Activity")
	}

	// Tab 4: History (enabled only if activity selected)
	if a.selectedActivity != nil {
		if a.currentTab == TabHistory {
			tab4 = activeStyle.Render("4. Current")
		} else {
			tab4 = inactiveStyle.Render("4. Current")
		}
	} else {
		tab4 = disabledStyle.Render("4. Current")
	}

	status := "● Idle"
	if a.activeEntry != nil {
		elapsed := time.Since(a.activeEntry.StartTime)
		hours := int(elapsed.Hours())
		minutes := int(elapsed.Minutes()) % 60
		seconds := int(elapsed.Seconds()) % 60
		status = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#51CF66")).
			Bold(true).
			Render(fmt.Sprintf("● Recording %02d:%02d:%02d", hours, minutes, seconds))
	}

	nav := lipgloss.JoinHorizontal(lipgloss.Top, tab1, " ", tab2, " ", tab3, " ", tab4)
	return lipgloss.JoinHorizontal(lipgloss.Top, nav, "    ", status)
}

func (a *SimpleApp) renderProjectsView() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#DF9E2F")).
		Padding(1, 0)

	title := titleStyle.Render("📁 Projects")

	// Search box
	searchBoxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#DF9E2F")).
		Padding(0, 1).
		Margin(1, 0)

	var searchBoxBorder lipgloss.Color
	if a.searchMode {
		searchBoxBorder = lipgloss.Color("#DF9E2F") // Bright orange when focused
	} else {
		searchBoxBorder = lipgloss.Color("#8A8B8B") // Gray when not focused
	}
	searchBoxStyle = searchBoxStyle.BorderForeground(searchBoxBorder)

	searchBox := searchBoxStyle.Render(a.projectSearchInput.View())

	// Results section
	var resultsContent string

	if len(a.projectSearchInput.Value()) == 0 {
		hintStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Italic(true)
		resultsContent = hintStyle.Render("Start typing to search for projects...")
	} else if len(a.projectSearchInput.Value()) < 2 {
		hintStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Italic(true)
		resultsContent = hintStyle.Render("Type at least 2 characters to search...")
	} else if len(a.projects) == 0 {
		noResults := lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Render("No projects found. Try a different search term.")
		resultsContent = noResults
	} else {
		// Calculate viewport for projects list
		reservedLines := 12
		availableHeight := a.height - reservedLines
		if availableHeight < 5 {
			availableHeight = 5
		}

		// Calculate scroll offset
		scrollOffset := 0
		if a.projectCursor >= availableHeight {
			scrollOffset = a.projectCursor - availableHeight + 1
		}

		// Determine visible range
		visibleStart := scrollOffset
		visibleEnd := scrollOffset + availableHeight
		if visibleEnd > len(a.projects) {
			visibleEnd = len(a.projects)
		}

		// Show results list
		var items []string

		selectedStyle := lipgloss.NewStyle().
			Background(lipgloss.Color("#DF9E2F")).
			Foreground(lipgloss.Color("15")).
			Padding(0, 1).
			Bold(true)

		normalStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("15")).
			Padding(0, 1)

		for i := visibleStart; i < visibleEnd; i++ {
			project := a.projects[i]
			prefix := "  "
			if i == a.projectCursor && !a.searchMode {
				prefix = "▶ "
				items = append(items, selectedStyle.Render(prefix+project.Label))
			} else {
				items = append(items, normalStyle.Render(prefix+project.Label))
			}
		}

		listBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#DF9E2F")).
			Padding(1, 2).
			Margin(1, 0)

		listContent := lipgloss.JoinVertical(lipgloss.Left, items...)

		// Add scroll indicator
		if len(a.projects) > availableHeight {
			scrollInfo := lipgloss.NewStyle().
				Foreground(lipgloss.Color("240")).
				Italic(true).
				Render(fmt.Sprintf("  Showing %d-%d of %d", visibleStart+1, visibleEnd, len(a.projects)))
			resultsContent = scrollInfo + "\n" + listBox.Render(listContent)
		} else {
			resultsContent = listBox.Render(listContent)
		}
	}

	// Help text
	var helpText string
	if a.searchMode {
		helpText = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Render("Tab/Esc: Navigate results | Enter: Select | Ctrl+C: Quit")
	} else {
		helpText = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Render("↑/↓: Navigate | Enter: Select | /: Search | Esc: Back to search | Ctrl+C: Quit")
	}

	return lipgloss.JoinVertical(lipgloss.Top, title, searchBox, resultsContent, helpText)
}

func (a *SimpleApp) renderProjectDetailsView() string {
	if a.selectedProject == nil {
		return "No project selected"
	}

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#DF9E2F")).
		Padding(1, 0)

	title := titleStyle.Render(fmt.Sprintf("📋 %s", a.selectedProject.Label))

	subtitleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Italic(true)

	subtitle := subtitleStyle.Render("Project Tasks")

	if len(a.tasks) == 0 {
		noTasks := lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Render("No tasks found for this project")
		return lipgloss.JoinVertical(lipgloss.Top, title, "", subtitle, "", noTasks)
	}

	// Calculate viewport for tasks list
	reservedLines := 12
	availableHeight := a.height - reservedLines
	if availableHeight < 5 {
		availableHeight = 5
	}

	// Calculate scroll offset
	scrollOffset := 0
	if a.taskCursor >= availableHeight {
		scrollOffset = a.taskCursor - availableHeight + 1
	}

	// Determine visible range
	visibleStart := scrollOffset
	visibleEnd := scrollOffset + availableHeight
	if visibleEnd > len(a.tasks) {
		visibleEnd = len(a.tasks)
	}

	var items []string

	selectedStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#DF9E2F")).
		Foreground(lipgloss.Color("15")).
		Padding(0, 1).
		Bold(true)

	normalStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("15")).
		Padding(0, 1)

	for i := visibleStart; i < visibleEnd; i++ {
		task := a.tasks[i]
		prefix := "  "
		taskInfo := fmt.Sprintf("%s (Expected: %.1fh", task.Label, task.ExpectedTime)
		if task.ActualTime > 0 {
			taskInfo += fmt.Sprintf(" | Actual: %.1fh", task.ActualTime)
		}
		taskInfo += ")"

		if i == a.taskCursor {
			prefix = "▶ "
			items = append(items, selectedStyle.Render(prefix+taskInfo))
		} else {
			items = append(items, normalStyle.Render(prefix+taskInfo))
		}
	}

	listBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#DF9E2F")).
		Padding(1, 2).
		Margin(1, 0)

	list := listBox.Render(lipgloss.JoinVertical(lipgloss.Left, items...))

	// Add scroll indicator
	var scrollInfo string
	if len(a.tasks) > availableHeight {
		scrollInfo = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Italic(true).
			Render(fmt.Sprintf("  Showing %d-%d of %d", visibleStart+1, visibleEnd, len(a.tasks)))
	}

	helpText := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Render("↑/↓: Navigate | Enter: Select | Esc: Back | Ctrl+C: Quit")

	if scrollInfo != "" {
		return lipgloss.JoinVertical(lipgloss.Top, title, subtitle, scrollInfo, list, helpText)
	}
	return lipgloss.JoinVertical(lipgloss.Top, title, subtitle, list, helpText)
}

func (a *SimpleApp) renderActivitiesView() string {
	if a.selectedTask == nil {
		return "No task selected"
	}

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#DF9E2F")).
		Padding(1, 0)

	title := titleStyle.Render(fmt.Sprintf("⚡ Activities for: %s", a.selectedTask.Label))

	if len(a.activities) == 0 {
		noActivities := lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Render("No activities found. Loading...")
		return lipgloss.JoinVertical(lipgloss.Top, title, "", noActivities)
	}

	// Calculate viewport - how many items can fit on screen
	// Reserve space for: title (3 lines), box borders (4 lines), help text (1 line), margins (4 lines)
	reservedLines := 12
	availableHeight := a.height - reservedLines
	if availableHeight < 5 {
		availableHeight = 5 // Minimum viewport size
	}

	// Calculate scroll offset to keep cursor visible
	scrollOffset := 0
	if a.activityCursor >= availableHeight {
		scrollOffset = a.activityCursor - availableHeight + 1
	}

	// Determine visible range
	visibleStart := scrollOffset
	visibleEnd := scrollOffset + availableHeight
	if visibleEnd > len(a.activities) {
		visibleEnd = len(a.activities)
	}

	var items []string

	selectedStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#DF9E2F")).
		Foreground(lipgloss.Color("15")).
		Padding(0, 1).
		Bold(true)

	lastUsedStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#51CF66")).
		Foreground(lipgloss.Color("0")).
		Padding(0, 1).
		Bold(true)

	normalStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("15")).
		Padding(0, 1)

	// Only render visible items
	for i := visibleStart; i < visibleEnd; i++ {
		activity := a.activities[i]
		prefix := "  "
		label := activity.Label

		// Highlight last used activity
		if activity.ID == a.lastUsedActivityID && i != a.activityCursor {
			label += " (last used)"
		}

		if i == a.activityCursor {
			prefix = "▶ "
			if activity.ID == a.lastUsedActivityID {
				items = append(items, lastUsedStyle.Render(prefix+label+" (last used)"))
			} else {
				items = append(items, selectedStyle.Render(prefix+label))
			}
		} else if activity.ID == a.lastUsedActivityID {
			items = append(items, lastUsedStyle.Render(prefix+label))
		} else {
			items = append(items, normalStyle.Render(prefix+label))
		}
	}

	listBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#DF9E2F")).
		Padding(1, 2).
		Margin(1, 0)

	list := listBox.Render(lipgloss.JoinVertical(lipgloss.Left, items...))

	// Add scroll indicator if there are more items
	var scrollInfo string
	if len(a.activities) > availableHeight {
		scrollInfo = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Italic(true).
			Render(fmt.Sprintf("  Showing %d-%d of %d", visibleStart+1, visibleEnd, len(a.activities)))
	}

	helpText := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Render("↑/↓: Navigate | Enter: Select | Esc: Back | Ctrl+C: Quit")

	if scrollInfo != "" {
		return lipgloss.JoinVertical(lipgloss.Top, title, scrollInfo, list, helpText)
	}
	return lipgloss.JoinVertical(lipgloss.Top, title, list, helpText)
}

func (a *SimpleApp) renderHistoryView() string {
	if a.selectedActivity == nil {
		return "No activity selected"
	}

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#DF9E2F")).
		Padding(1, 0)

	title := titleStyle.Render(fmt.Sprintf("📊 History: %s - %s", a.selectedTask.Label, a.selectedActivity.Label))

	if len(a.history) == 0 {
		noHistory := lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Render("No time entries found for this task and activity")
		return lipgloss.JoinVertical(lipgloss.Top, title, "", noHistory)
	}

	var items []string

	for _, entry := range a.history {
		dateStr := entry.StartTime.Format("2006-01-02 15:04")
		durationStr := fmt.Sprintf("%.2fh", entry.Duration)

		entryText := fmt.Sprintf("%s | %s | %s",
			dateStr,
			durationStr,
			entry.Description)

		if entry.IsSubmitted {
			entryText += " ✓"
		}

		items = append(items, entryText)
	}

	listBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#DF9E2F")).
		Padding(1, 2).
		Margin(1, 0).
		MaxHeight(a.height - 10)

	list := listBox.Render(lipgloss.JoinVertical(lipgloss.Left, items...))

	var total float64
	for _, entry := range a.history {
		total += entry.Duration
	}

	summaryStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#51CF66")).
		Bold(true)

	summary := summaryStyle.Render(fmt.Sprintf("Total: %.2f hours (%d entries)", total, len(a.history)))

	helpText := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Render("Esc: Back | Ctrl+C: Quit")

	return lipgloss.JoinVertical(lipgloss.Top, title, list, summary, "", helpText)
}

// Navigation helpers
func (a *SimpleApp) moveCursorUp() {
	switch a.currentTab {
	case TabProjects:
		if a.projectCursor > 0 {
			a.projectCursor--
		}
	case TabProjectDetails:
		if a.taskCursor > 0 {
			a.taskCursor--
		}
	case TabActivities:
		if a.activityCursor > 0 {
			a.activityCursor--
		}
	case TabHistory:
		if a.historyCursor > 0 {
			a.historyCursor--
		}
	}
}

func (a *SimpleApp) moveCursorDown() {
	switch a.currentTab {
	case TabProjects:
		if a.projectCursor < len(a.projects)-1 {
			a.projectCursor++
		}
	case TabProjectDetails:
		if a.taskCursor < len(a.tasks)-1 {
			a.taskCursor++
		}
	case TabActivities:
		if a.activityCursor < len(a.activities)-1 {
			a.activityCursor++
		}
	case TabHistory:
		if a.historyCursor < len(a.history)-1 {
			a.historyCursor++
		}
	}
}

func (a *SimpleApp) handleSelection() tea.Cmd {
	switch a.currentTab {
	case TabProjects:
		if a.projectCursor < len(a.projects) {
			a.selectedProject = &a.projects[a.projectCursor]
			a.currentTab = TabProjectDetails
			a.taskCursor = 0
			a.selectedTask = nil
			a.selectedActivity = nil
			return a.loadTasks(a.selectedProject.ID)
		}

	case TabProjectDetails:
		if a.taskCursor < len(a.tasks) {
			a.selectedTask = &a.tasks[a.taskCursor]
			a.currentTab = TabActivities
			a.activityCursor = 0
			a.selectedActivity = nil

			// Load last used activity from config
			a.loadLastUsedActivity()

			return nil
		}

	case TabActivities:
		if a.activityCursor < len(a.activities) {
			a.selectedActivity = &a.activities[a.activityCursor]
			a.currentTab = TabHistory
			a.historyCursor = 0

			// Save as last used activity
			a.saveLastUsedActivity(a.selectedActivity.ID)

			return a.loadHistory()
		}
	}

	return nil
}

// Data loading commands
func (a *SimpleApp) searchProjects(query string) tea.Cmd {
	return func() tea.Msg {
		if a.apiClient == nil {
			return errorMsg(fmt.Errorf("API client not available"))
		}

		projects, err := a.apiClient.GetProjects(query)
		if err != nil {
			return errorMsg(err)
		}
		return projectsLoadedMsg(projects)
	}
}

func (a *SimpleApp) loadTasks(projectID int) tea.Cmd {
	return func() tea.Msg {
		if a.apiClient == nil {
			return errorMsg(fmt.Errorf("API client not available"))
		}

		tasks, err := a.apiClient.GetTasks(fmt.Sprintf("%d", projectID))
		if err != nil {
			return errorMsg(err)
		}
		return tasksLoadedMsg(tasks)
	}
}

func (a *SimpleApp) loadActivities() tea.Cmd {
	return func() tea.Msg {
		if a.apiClient == nil {
			return errorMsg(fmt.Errorf("API client not available"))
		}

		activities, err := a.apiClient.GetActivities()
		if err != nil {
			return errorMsg(err)
		}
		return activitiesLoadedMsg(activities)
	}
}

func (a *SimpleApp) loadHistory() tea.Cmd {
	return func() tea.Msg {
		if a.apiClient == nil {
			return errorMsg(fmt.Errorf("API client not available"))
		}

		// Get all timelogs
		timelogs, err := a.apiClient.GetTimelogs()
		if err != nil {
			return errorMsg(err)
		}

		// Filter by selected task and activity
		var filtered []api.TimelogEntry
		activityName := a.selectedActivity.Label

		for _, entries := range timelogs {
			for _, entry := range entries {
				if entry.TaskName == a.selectedTask.Label && entry.Activity == activityName {
					filtered = append(filtered, entry)
				}
			}
		}

		return historyLoadedMsg(filtered)
	}
}

func (a *SimpleApp) loadActiveEntry() tea.Cmd {
	return func() tea.Msg {
		entry, err := a.service.GetActiveTimeEntry()
		if err != nil {
			return errorMsg(err)
		}
		return activeEntryMsg(entry)
	}
}

// Last used activity management
func (a *SimpleApp) loadLastUsedActivity() {
	// Try to load from a preferences file
	homeDir, _ := os.UserHomeDir()
	prefsPath := filepath.Join(homeDir, ".config", "kartoza-timesheets", "preferences.json")

	data, err := os.ReadFile(prefsPath)
	if err != nil {
		return
	}

	var prefs struct {
		LastUsedActivityID int `json:"last_used_activity_id"`
	}

	if err := json.Unmarshal(data, &prefs); err != nil {
		return
	}

	a.lastUsedActivityID = prefs.LastUsedActivityID

	// Set cursor to last used activity
	for i, act := range a.activities {
		if act.ID == a.lastUsedActivityID {
			a.activityCursor = i
			break
		}
	}
}

func (a *SimpleApp) saveLastUsedActivity(activityID int) {
	homeDir, _ := os.UserHomeDir()
	prefsDir := filepath.Join(homeDir, ".config", "kartoza-timesheets")
	prefsPath := filepath.Join(prefsDir, "preferences.json")

	// Create directory if it doesn't exist
	os.MkdirAll(prefsDir, 0755)

	prefs := struct {
		LastUsedActivityID int `json:"last_used_activity_id"`
	}{
		LastUsedActivityID: activityID,
	}

	data, err := json.MarshalIndent(prefs, "", "  ")
	if err != nil {
		return
	}

	os.WriteFile(prefsPath, data, 0644)

	a.lastUsedActivityID = activityID
}

func (a *SimpleApp) Run() error {
	p := tea.NewProgram(a, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
