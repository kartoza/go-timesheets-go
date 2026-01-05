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
type runningTimerDetectedMsg struct {
	entry    api.TimelogEntry
	project  api.ProjectListItem
	task     api.TaskListItem
	activity api.ActivityListItem
}
type timesheetSubmittedMsg struct {
	isRunning bool
	history   []api.TimelogEntry
}

// Tab represents different views
type Tab int

const (
	TabProjects Tab = iota
	TabProjectDetails
	TabActivities
	TabCurrent
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

	// Timer control form inputs (Tab 4)
	descriptionInput textinput.Model
	dateInput        textinput.Model
	startTimeInput   textinput.Model
	endTimeInput     textinput.Model
	focusedFormField int  // 0=description, 1=date, 2=start, 3=end, 4=submit
	formMode         bool // true when editing form
	editingEntryID   int  // ID of entry being edited, 0 for new entry
	showCalendar     bool // true when calendar popup is shown

	err            error
	successMessage string
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

	// Initialize timer form inputs
	descriptionInput := textinput.New()
	descriptionInput.Placeholder = "Description..."
	descriptionInput.CharLimit = 200
	descriptionInput.Width = 60

	dateInput := textinput.New()
	dateInput.Placeholder = "YYYY-MM-DD"
	dateInput.CharLimit = 10
	dateInput.Width = 15
	dateInput.SetValue(time.Now().Format("2006-01-02"))

	startTimeInput := textinput.New()
	startTimeInput.Placeholder = "HH:MM"
	startTimeInput.CharLimit = 5
	startTimeInput.Width = 10

	endTimeInput := textinput.New()
	endTimeInput.Placeholder = "HH:MM"
	endTimeInput.CharLimit = 5
	endTimeInput.Width = 10

	app := &SimpleApp{
		service:            service,
		apiClient:          apiClient,
		currentTab:         TabProjects,
		projectSearchInput: projectSearchInput,
		searchMode:         true, // Start in search mode on Tab 1
		descriptionInput:   descriptionInput,
		dateInput:          dateInput,
		startTimeInput:     startTimeInput,
		endTimeInput:       endTimeInput,
	}

	// Load session state to restore previous selections
	app.loadSessionState()

	return app, nil
}

func (a *SimpleApp) Init() tea.Cmd {
	return tea.Batch(
		// Don't load projects initially - wait for user to type
		a.loadActivities(),
		a.loadActiveEntry(),
		a.checkForRunningTimer(),
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

		// Handle Tab 4 form mode
		if a.currentTab == TabCurrent && a.formMode {
			switch msg.String() {
			case "ctrl+c":
				return a, tea.Quit
			case "esc":
				// Exit form mode
				a.formMode = false
				a.blurAllFormFields()
				return a, nil
			case "tab":
				// Move to next field
				a.blurAllFormFields()
				a.focusedFormField = (a.focusedFormField + 1) % 5
				a.focusFormField()
				return a, nil
			case "shift+tab":
				// Move to previous field
				a.blurAllFormFields()
				a.focusedFormField = (a.focusedFormField - 1 + 5) % 5
				a.focusFormField()
				return a, nil
			case "enter":
				// Submit form if on submit button
				if a.focusedFormField == 4 {
					cmd := a.submitTimeEntry()
					a.formMode = false
					a.blurAllFormFields()
					return a, cmd
				}
				// Move to next field on enter
				a.blurAllFormFields()
				a.focusedFormField = (a.focusedFormField + 1) % 5
				a.focusFormField()
				return a, nil
			default:
				// Update the focused input field
				var cmd tea.Cmd
				switch a.focusedFormField {
				case 0:
					a.descriptionInput, cmd = a.descriptionInput.Update(msg)
				case 1:
					a.dateInput, cmd = a.dateInput.Update(msg)
				case 2:
					a.startTimeInput, cmd = a.startTimeInput.Update(msg)
				case 3:
					a.endTimeInput, cmd = a.endTimeInput.Update(msg)
				}
				return a, cmd
			}
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
			a.clearMessages()
			a.currentTab = TabProjects
			a.selectedProject = nil
			a.selectedTask = nil
			a.selectedActivity = nil
			a.searchMode = true
			a.projectSearchInput.Focus()
			a.saveSessionState() // Save session state on tab switch
			cmds = append(cmds, textinput.Blink)

		case "2":
			if a.selectedProject != nil {
				a.clearMessages()
				a.currentTab = TabProjectDetails
				a.saveSessionState() // Save session state on tab switch
			}

		case "3":
			if a.selectedTask != nil {
				a.clearMessages()
				a.currentTab = TabActivities
				a.saveSessionState() // Save session state on tab switch
			}

		case "4":
			if a.selectedActivity != nil {
				a.clearMessages()
				a.currentTab = TabCurrent
				a.saveSessionState() // Save session state on tab switch
			}

		case "n":
			// New entry (only on Tab 4)
			if a.currentTab == TabCurrent {
				a.clearMessages()
				a.enterNewEntryMode()
				cmds = append(cmds, textinput.Blink)
			}

		case "e":
			// Edit selected entry (only on Tab 4)
			if a.currentTab == TabCurrent && len(a.history) > 0 && !a.history[a.historyCursor].IsSubmitted {
				a.clearMessages()
				a.enterEditMode()
				cmds = append(cmds, textinput.Blink)
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
				a.saveSessionState() // Save session state on tab navigation
			case TabActivities:
				a.currentTab = TabProjectDetails
				a.selectedActivity = nil
				a.saveSessionState() // Save session state on tab navigation
			case TabCurrent:
				a.currentTab = TabActivities
				a.saveSessionState() // Save session state on tab navigation
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
		// Exit form mode if it was active (after successful submission)
		a.formMode = false
		a.blurAllFormFields()
		a.saveSessionState() // Save session state after successful submission

	case timesheetSubmittedMsg:
		a.history = msg.history
		// Sort by start time, most recent first
		sort.Slice(a.history, func(i, j int) bool {
			return a.history[i].StartTime.After(a.history[j].StartTime)
		})
		a.historyCursor = 0
		// Exit form mode
		a.formMode = false
		a.blurAllFormFields()
		a.saveSessionState()

		// Clear any previous error and set success message based on running status
		a.err = nil
		if msg.isRunning {
			a.successMessage = "✓ Timesheet submitted and timer is running"
		} else {
			a.successMessage = "✓ Timesheet submitted and closed"
		}

	case runningTimerDetectedMsg:
		// Populate selections from running timer
		a.selectedProject = &msg.project
		a.selectedTask = &msg.task
		a.selectedActivity = &msg.activity

		// Populate data lists
		a.projects = []api.ProjectListItem{msg.project}
		a.tasks = []api.TaskListItem{msg.task}
		a.activities = []api.ActivityListItem{msg.activity}

		// Add the running entry to history
		a.history = []api.TimelogEntry{msg.entry}

		// Switch to Tab 4
		a.currentTab = TabCurrent
		a.searchMode = false

		// Save session state
		a.saveSessionState()

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
	case TabCurrent:
		content = a.renderCurrentView()
	}

	nav := a.renderNav()

	if a.err != nil {
		errorMsg := lipgloss.NewStyle().
			Foreground(lipgloss.Color("9")).
			Render(fmt.Sprintf("Error: %v", a.err))
		content = lipgloss.JoinVertical(lipgloss.Top, content, errorMsg)
	}

	if a.successMessage != "" {
		successMsg := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#51CF66")).
			Bold(true).
			Render(a.successMessage)
		content = lipgloss.JoinVertical(lipgloss.Top, content, successMsg)
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

	// Tab 4: Current (enabled only if activity selected)
	if a.selectedActivity != nil {
		if a.currentTab == TabCurrent {
			tab4 = activeStyle.Render("4. Current")
		} else {
			tab4 = inactiveStyle.Render("4. Current")
		}
	} else {
		tab4 = disabledStyle.Render("4. Current")
	}

	status := "● Idle"

	// Check for running entries in history
	var runningEntry *api.TimelogEntry
	for i := range a.history {
		if a.history[i].EndTime.IsZero() {
			runningEntry = &a.history[i]
			break
		}
	}

	// Display running status from history or active entry
	if runningEntry != nil {
		elapsed := time.Since(runningEntry.StartTime)
		hours := int(elapsed.Hours())
		minutes := int(elapsed.Minutes()) % 60
		seconds := int(elapsed.Seconds()) % 60
		status = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#51CF66")).
			Bold(true).
			Render(fmt.Sprintf("● Recording %02d:%02d:%02d", hours, minutes, seconds))
	} else if a.activeEntry != nil {
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

func (a *SimpleApp) renderCurrentView() string {
	if a.selectedActivity == nil {
		return "No activity selected"
	}

	// Render the timer form
	formView := a.renderTimerForm()

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#DF9E2F")).
		Padding(1, 0)

	title := titleStyle.Render(fmt.Sprintf("📊 History: %s, %s - %s", a.selectedProject.Label, a.selectedTask.Label, a.selectedActivity.Label))

	// Render the table
	tableView := a.renderEntriesTable()

	return lipgloss.JoinVertical(lipgloss.Top, formView, "", title, "", tableView)
}

func (a *SimpleApp) renderTimerForm() string {
	formStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#DF9E2F")).
		Padding(1, 2).
		Margin(0, 0)

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Width(15)

	focusedLabelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#DF9E2F")).
		Bold(true).
		Width(15)

	// Calculate elapsed time if start time is set
	var elapsedTime string
	if a.startTimeInput.Value() != "" {
		startStr := fmt.Sprintf("%s %s", a.dateInput.Value(), a.startTimeInput.Value())
		startTime, err := time.Parse("2006-01-02 15:04", startStr)
		if err == nil {
			var endTime time.Time
			if a.endTimeInput.Value() != "" {
				endStr := fmt.Sprintf("%s %s", a.dateInput.Value(), a.endTimeInput.Value())
				endTime, err = time.Parse("2006-01-02 15:04", endStr)
				if err != nil {
					endTime = time.Now()
				}
			} else {
				endTime = time.Now()
			}
			elapsed := endTime.Sub(startTime)
			hours := int(elapsed.Hours())
			minutes := int(elapsed.Minutes()) % 60
			elapsedTime = fmt.Sprintf("%dh %dm", hours, minutes)
		}
	}

	// Build form fields
	var formFields []string

	// Description field
	if a.focusedFormField == 0 && a.formMode {
		formFields = append(formFields, lipgloss.JoinHorizontal(lipgloss.Left,
			focusedLabelStyle.Render("Description:"),
			a.descriptionInput.View()))
	} else {
		formFields = append(formFields, lipgloss.JoinHorizontal(lipgloss.Left,
			labelStyle.Render("Description:"),
			a.descriptionInput.View()))
	}

	// Date field
	if a.focusedFormField == 1 && a.formMode {
		formFields = append(formFields, lipgloss.JoinHorizontal(lipgloss.Left,
			focusedLabelStyle.Render("Date:"),
			a.dateInput.View(),
			"  📅"))
	} else {
		formFields = append(formFields, lipgloss.JoinHorizontal(lipgloss.Left,
			labelStyle.Render("Date:"),
			a.dateInput.View()))
	}

	// Start time field
	if a.focusedFormField == 2 && a.formMode {
		formFields = append(formFields, lipgloss.JoinHorizontal(lipgloss.Left,
			focusedLabelStyle.Render("Start Time:"),
			a.startTimeInput.View()))
	} else {
		formFields = append(formFields, lipgloss.JoinHorizontal(lipgloss.Left,
			labelStyle.Render("Start Time:"),
			a.startTimeInput.View()))
	}

	// Elapsed time (read-only)
	formFields = append(formFields, lipgloss.JoinHorizontal(lipgloss.Left,
		labelStyle.Render("Elapsed:"),
		lipgloss.NewStyle().Foreground(lipgloss.Color("82")).Render(elapsedTime)))

	// End time field
	if a.focusedFormField == 3 && a.formMode {
		formFields = append(formFields, lipgloss.JoinHorizontal(lipgloss.Left,
			focusedLabelStyle.Render("End Time:"),
			a.endTimeInput.View()))
	} else {
		formFields = append(formFields, lipgloss.JoinHorizontal(lipgloss.Left,
			labelStyle.Render("End Time:"),
			a.endTimeInput.View()))
	}

	// Submit button
	submitStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("0")).
		Background(lipgloss.Color("82")).
		Padding(0, 2).
		Margin(0, 15)

	if a.focusedFormField == 4 && a.formMode {
		submitStyle = submitStyle.Bold(true).Background(lipgloss.Color("46"))
		formFields = append(formFields, submitStyle.Render("[ SUBMIT ]"))
	} else {
		formFields = append(formFields, submitStyle.Render("  SUBMIT  "))
	}

	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Italic(true).
		Margin(1, 0, 0, 0)

	if a.formMode {
		formFields = append(formFields, helpStyle.Render("Tab: Next Field | Shift+Tab: Previous | Enter: Submit | Esc: Exit Form"))
	} else {
		formFields = append(formFields, helpStyle.Render("n: New Entry | e: Edit Selected | ↑/↓: Navigate | Esc: Back"))
	}

	return formStyle.Render(lipgloss.JoinVertical(lipgloss.Left, formFields...))
}

func (a *SimpleApp) renderEntriesTable() string {
	if len(a.history) == 0 {
		noCurrent := lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Padding(2, 0).
			Render("No time entries found for this task and activity")
		return noCurrent
	}

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#DF9E2F")).
		Width(20).
		Align(lipgloss.Left)

	cellStyle := lipgloss.NewStyle().
		Width(20).
		Align(lipgloss.Left)

	selectedStyle := lipgloss.NewStyle().
		Width(20).
		Align(lipgloss.Left).
		Foreground(lipgloss.Color("0")).
		Background(lipgloss.Color("#DF9E2F"))

	// Table header
	header := lipgloss.JoinHorizontal(lipgloss.Top,
		headerStyle.Render("Date"),
		headerStyle.Render("Start"),
		headerStyle.Render("End"),
		headerStyle.Render("Elapsed"),
		headerStyle.Width(40).Render("Description"),
		headerStyle.Width(10).Render("Action"))

	// Calculate visible window for table rows
	maxTableHeight := 10
	visibleStart := 0
	visibleEnd := len(a.history)

	if len(a.history) > maxTableHeight {
		if a.historyCursor >= maxTableHeight/2 {
			visibleStart = a.historyCursor - maxTableHeight/2
		}
		visibleEnd = visibleStart + maxTableHeight
		if visibleEnd > len(a.history) {
			visibleEnd = len(a.history)
			visibleStart = visibleEnd - maxTableHeight
			if visibleStart < 0 {
				visibleStart = 0
			}
		}
	}

	// Table rows
	var rows []string
	for i := visibleStart; i < visibleEnd; i++ {
		entry := a.history[i]
		dateStr := entry.StartTime.Format("2006-01-02")
		startStr := entry.StartTime.Format("15:04")

		// Check if entry is running (no end time)
		isRunning := entry.EndTime.IsZero()

		var endStr string
		var elapsedStr string

		if isRunning {
			endStr = "--:--"
			// Calculate elapsed time from start to now
			elapsed := time.Since(entry.StartTime)
			hours := int(elapsed.Hours())
			minutes := int(elapsed.Minutes()) % 60
			elapsedStr = fmt.Sprintf("%dh %dm", hours, minutes)
		} else {
			endStr = entry.EndTime.Format("15:04")
			hours := int(entry.Duration)
			minutes := int((entry.Duration - float64(hours)) * 60)
			elapsedStr = fmt.Sprintf("%dh %dm", hours, minutes)
		}

		descStr := entry.Description
		if len(descStr) > 38 {
			descStr = descStr[:35] + "..."
		}

		var editAction string
		runningStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#51CF66")).
			Bold(true)

		if isRunning {
			editAction = runningStyle.Render("● Running")
		} else if entry.IsSubmitted {
			editAction = "✓ Done"
		} else {
			editAction = "[Edit]"
		}

		var row string
		if i == a.historyCursor && !a.formMode {
			// For selected row, don't style the action column if it's running (to preserve green color)
			actionCell := selectedStyle.Width(10).Render(editAction)
			if isRunning {
				// Keep the green color by not re-styling the already-styled running indicator
				actionCell = lipgloss.NewStyle().Width(10).Render(editAction)
			}
			row = lipgloss.JoinHorizontal(lipgloss.Top,
				selectedStyle.Render(dateStr),
				selectedStyle.Render(startStr),
				selectedStyle.Render(endStr),
				selectedStyle.Render(elapsedStr),
				selectedStyle.Width(40).Render(descStr),
				actionCell)
		} else {
			row = lipgloss.JoinHorizontal(lipgloss.Top,
				cellStyle.Render(dateStr),
				cellStyle.Render(startStr),
				cellStyle.Render(endStr),
				cellStyle.Render(elapsedStr),
				cellStyle.Width(40).Render(descStr),
				cellStyle.Width(10).Render(editAction))
		}
		rows = append(rows, row)
	}

	tableContent := lipgloss.JoinVertical(lipgloss.Left, append([]string{header}, rows...)...)

	tableStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#DF9E2F")).
		Padding(1, 2).
		Margin(0, 0)

	var total float64
	for _, entry := range a.history {
		total += entry.Duration
	}

	summaryStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#51CF66")).
		Bold(true).
		Margin(1, 0, 0, 0)

	summary := summaryStyle.Render(fmt.Sprintf("Total: %.2fh (%d entries)", total, len(a.history)))

	// Scroll indicator
	var scrollInfo string
	if len(a.history) > maxTableHeight {
		scrollInfo = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Italic(true).
			Render(fmt.Sprintf("  Showing %d-%d of %d", visibleStart+1, visibleEnd, len(a.history)))
	}

	result := tableStyle.Render(tableContent)
	if scrollInfo != "" {
		return lipgloss.JoinVertical(lipgloss.Top, result, summary, scrollInfo)
	}
	return lipgloss.JoinVertical(lipgloss.Top, result, summary)
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
	case TabCurrent:
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
	case TabCurrent:
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
			a.saveSessionState() // Save session state when project is selected
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

			a.saveSessionState() // Save session state when task is selected

			return nil
		}

	case TabActivities:
		if a.activityCursor < len(a.activities) {
			a.selectedActivity = &a.activities[a.activityCursor]
			a.currentTab = TabCurrent
			a.historyCursor = 0

			// Save as last used activity
			a.saveLastUsedActivity(a.selectedActivity.ID)

			return a.loadCurrent()
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

func (a *SimpleApp) loadCurrent() tea.Cmd {
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

		for _, entry := range timelogs {
			if entry.TaskName == a.selectedTask.Label && entry.Activity == activityName {
				filtered = append(filtered, entry)
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

func (a *SimpleApp) checkForRunningTimer() tea.Cmd {
	return func() tea.Msg {
		if a.apiClient == nil {
			return nil
		}

		// Get all timelogs
		timelogs, err := a.apiClient.GetTimelogs()
		if err != nil {
			return nil // Silently fail - don't block app startup
		}

		// Find first running timer (entry with zero end time)
		for _, entry := range timelogs {
			if entry.EndTime.IsZero() {
				// Found a running timer - need to load project, task, and activity details
				projects, err := a.apiClient.GetProjects(entry.ProjectName)
				if err != nil || len(projects) == 0 {
					continue
				}

				tasks, err := a.apiClient.GetTasks(fmt.Sprintf("%d", projects[0].ID))
				if err != nil {
					continue
				}

				// Find matching task
				var matchedTask *api.TaskListItem
				for _, task := range tasks {
					if task.Label == entry.TaskName {
						matchedTask = &task
						break
					}
				}
				if matchedTask == nil {
					continue
				}

				// Load activities
				activities, err := a.apiClient.GetActivities()
				if err != nil {
					continue
				}

				// Find matching activity
				var matchedActivity *api.ActivityListItem
				for _, activity := range activities {
					if activity.Label == entry.Activity {
						matchedActivity = &activity
						break
					}
				}
				if matchedActivity == nil {
					continue
				}

				// Return the running timer with all details
				return runningTimerDetectedMsg{
					entry:    entry,
					project:  projects[0],
					task:     *matchedTask,
					activity: *matchedActivity,
				}
			}
		}

		return nil
	}
}

// SessionState holds the user's last selections for persistence
type SessionState struct {
	LastUsedProjectID  int `json:"last_used_project_id"`
	LastUsedTaskID     int `json:"last_used_task_id"`
	LastUsedActivityID int `json:"last_used_activity_id"`
}

// Session state management
func (a *SimpleApp) loadSessionState() {
	// Try to load from a preferences file
	homeDir, _ := os.UserHomeDir()
	prefsPath := filepath.Join(homeDir, ".config", "kartoza-timesheets", "session.json")

	data, err := os.ReadFile(prefsPath)
	if err != nil {
		return
	}

	var session SessionState
	if err := json.Unmarshal(data, &session); err != nil {
		return
	}

	a.lastUsedActivityID = session.LastUsedActivityID

	// Note: We can't restore the full selections here because the data hasn't been loaded yet
	// The cursor positions will be set when the data is loaded and we can match IDs
}

func (a *SimpleApp) saveSessionState() {
	homeDir, _ := os.UserHomeDir()
	prefsDir := filepath.Join(homeDir, ".config", "kartoza-timesheets")
	prefsPath := filepath.Join(prefsDir, "session.json")

	// Create directory if it doesn't exist
	os.MkdirAll(prefsDir, 0755)

	session := SessionState{
		LastUsedActivityID: a.lastUsedActivityID,
	}

	// Save selected project ID if available
	if a.selectedProject != nil {
		session.LastUsedProjectID = a.selectedProject.ID
	}

	// Save selected task ID if available
	if a.selectedTask != nil {
		session.LastUsedTaskID = a.selectedTask.ID
	}

	// Save selected activity ID if available
	if a.selectedActivity != nil {
		session.LastUsedActivityID = a.selectedActivity.ID
	}

	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return
	}

	os.WriteFile(prefsPath, data, 0644)
}

// loadLastUsedActivity loads the last used activity from session state
func (a *SimpleApp) loadLastUsedActivity() {
	a.loadSessionState()

	// Set cursor to last used activity
	for i, act := range a.activities {
		if act.ID == a.lastUsedActivityID {
			a.activityCursor = i
			break
		}
	}
}

// saveLastUsedActivity saves the last used activity to session state
func (a *SimpleApp) saveLastUsedActivity(activityID int) {
	a.lastUsedActivityID = activityID
	a.saveSessionState()
}

// Form mode helpers
func (a *SimpleApp) enterNewEntryMode() {
	a.formMode = true
	a.editingEntryID = 0
	a.focusedFormField = 0

	// Set form fields with current date/time
	now := time.Now()
	a.descriptionInput.SetValue("")
	a.dateInput.SetValue(now.Format("2006-01-02"))
	a.startTimeInput.SetValue(now.Format("15:04")) // Default start time to now
	a.endTimeInput.SetValue("")

	a.focusFormField()
}

func (a *SimpleApp) enterEditMode() {
	if len(a.history) == 0 || a.historyCursor >= len(a.history) {
		return
	}

	entry := a.history[a.historyCursor]
	a.formMode = true
	a.editingEntryID = entry.ID
	a.focusedFormField = 0

	// Populate form fields with entry data
	a.descriptionInput.SetValue(entry.Description)
	a.dateInput.SetValue(entry.StartTime.Format("2006-01-02"))
	a.startTimeInput.SetValue(entry.StartTime.Format("15:04"))
	a.endTimeInput.SetValue(entry.EndTime.Format("15:04"))

	a.focusFormField()
}

func (a *SimpleApp) focusFormField() {
	switch a.focusedFormField {
	case 0:
		a.descriptionInput.Focus()
	case 1:
		a.dateInput.Focus()
	case 2:
		a.startTimeInput.Focus()
	case 3:
		a.endTimeInput.Focus()
	}
}

func (a *SimpleApp) blurAllFormFields() {
	a.descriptionInput.Blur()
	a.dateInput.Blur()
	a.startTimeInput.Blur()
	a.endTimeInput.Blur()
}

func (a *SimpleApp) submitTimeEntry() tea.Cmd {
	return func() tea.Msg {
		if a.apiClient == nil {
			return errorMsg(fmt.Errorf("API client not available"))
		}

		// Parse date and times
		dateStr := a.dateInput.Value()
		startTimeStr := a.startTimeInput.Value()
		endTimeStr := a.endTimeInput.Value()

		if dateStr == "" || startTimeStr == "" {
			return errorMsg(fmt.Errorf("date and start time are required"))
		}

		startStr := fmt.Sprintf("%s %s", dateStr, startTimeStr)
		startTime, err := time.Parse("2006-01-02 15:04", startStr)
		if err != nil {
			return errorMsg(fmt.Errorf("invalid start time format: %w", err))
		}

		var endTimePtr *time.Time
		var duration float64

		if endTimeStr != "" {
			endStr := fmt.Sprintf("%s %s", dateStr, endTimeStr)
			endTime, err := time.Parse("2006-01-02 15:04", endStr)
			if err != nil {
				return errorMsg(fmt.Errorf("invalid end time format: %w", err))
			}
			duration = endTime.Sub(startTime).Hours()

			if duration <= 0 {
				return errorMsg(fmt.Errorf("end time must be after start time"))
			}
			endTimePtr = &endTime
		} else {
			// If no end time, leave it nil - this creates a running timer
			endTimePtr = nil
			duration = 0
		}

		// Create time entry
		entry := models.TimeEntry{
			ProjectID:   fmt.Sprintf("%d", a.selectedProject.ID),
			ActivityID:  fmt.Sprintf("%d", a.selectedActivity.ID),
			Description: a.descriptionInput.Value(),
			StartTime:   startTime,
			EndTime:     endTimePtr,
			Duration:    duration,
		}

		if a.selectedTask != nil {
			taskID := fmt.Sprintf("%d", a.selectedTask.ID)
			entry.TaskID = &taskID
		}

		// Create or update entry
		if a.editingEntryID == 0 {
			err = a.apiClient.CreateTimesheet(entry)
		} else {
			err = a.apiClient.UpdateTimesheet(fmt.Sprintf("%d", a.editingEntryID), entry)
		}

		if err != nil {
			return errorMsg(err)
		}

		// Reload history and exit form mode
		timelogs, err := a.apiClient.GetTimelogs()
		if err != nil {
			return errorMsg(err)
		}

		// Filter by selected task and activity
		var filtered []api.TimelogEntry
		activityName := a.selectedActivity.Label

		for _, entry := range timelogs {
			if entry.TaskName == a.selectedTask.Label && entry.Activity == activityName {
				filtered = append(filtered, entry)
			}
		}

		// Return submission message with running status
		return timesheetSubmittedMsg{
			isRunning: endTimePtr == nil,
			history:   filtered,
		}
	}
}

func (a *SimpleApp) Run() error {
	p := tea.NewProgram(a, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

// clearMessages clears both error and success messages
func (a *SimpleApp) clearMessages() {
	a.err = nil
	a.successMessage = ""
}
