package tui

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/kartoza/go-timesheets-go/internal/api"
	"github.com/kartoza/go-timesheets-go/internal/config"
	"github.com/kartoza/go-timesheets-go/internal/models"
	"github.com/kartoza/go-timesheets-go/internal/storage"
)

// formatTaskLabel formats a task label for display by splitting the budget info onto a new line
// Input: "Testing Scripts, Admin and meetings        (1890.01/2400.0)"
// Output: "Testing Scripts, Admin and meetings\n  1890.01 of 2400.0 hrs used"
func formatTaskLabel(label string) string {
	// Match pattern: "Task Name (hours/budget)" or "Task Name        (hours/budget)"
	re := regexp.MustCompile(`^(.+?)\s*\(([0-9.]+)/([0-9.]+)\)$`)
	matches := re.FindStringSubmatch(label)
	if matches != nil && len(matches) == 4 {
		taskName := strings.TrimSpace(matches[1])
		hoursUsed := matches[2]
		totalHours := matches[3]
		return fmt.Sprintf("%s\n  %s of %s hrs used", taskName, hoursUsed, totalHours)
	}
	return label
}

// formatTaskLabelShort formats a task label for compact display (single line)
// Input: "Testing Scripts, Admin and meetings        (1890.01/2400.0)"
// Output: "Testing Scripts, Admin... (1890/2400h)"
func formatTaskLabelShort(label string, maxLen int) string {
	re := regexp.MustCompile(`^(.+?)\s*\(([0-9.]+)/([0-9.]+)\)$`)
	matches := re.FindStringSubmatch(label)
	if matches != nil && len(matches) == 4 {
		taskName := strings.TrimSpace(matches[1])
		hoursUsed := matches[2]
		totalHours := matches[3]

		// Truncate hours to integers for compact display
		hoursUsedShort := strings.Split(hoursUsed, ".")[0]
		totalHoursShort := strings.Split(totalHours, ".")[0]

		suffix := fmt.Sprintf(" (%s/%sh)", hoursUsedShort, totalHoursShort)
		availableLen := maxLen - len(suffix)

		if len(taskName) > availableLen && availableLen > 3 {
			taskName = taskName[:availableLen-3] + "..."
		}

		return taskName + suffix
	}
	return label
}

// tuiDebugLog is defined in debug_dev.go or debug_release.go

// TimesheetField represents which field is currently focused
type TimesheetField int

const (
	FieldProject TimesheetField = iota
	FieldTask
	FieldActivity
	FieldDescription
	FieldDate
	FieldStartTime
	FieldEndTime
	FieldSubmit
)

// TimesheetCreator is a unified screen for creating timesheet entries
type TimesheetCreator struct {
	apiClient   *api.Client
	username    string
	width       int
	height      int
	headerState *HeaderState // Shared header state

	// Current focused field
	focusedField TimesheetField

	// Selection data
	projects   []api.ProjectListItem
	tasks      []api.TaskListItem
	activities []api.ActivityListItem

	// Selected items
	selectedProject  *api.ProjectListItem
	selectedTask     *api.TaskListItem
	selectedActivity *api.ActivityListItem

	// Pop-over state
	showProjectPopover  bool
	showTaskPopover     bool
	showActivityPopover bool
	popoverCursor       int

	// Text inputs
	projectSearchInput textinput.Model
	descriptionInput   textinput.Model
	dateInput          textinput.Model
	startTimeInput     textinput.Model
	endTimeInput       textinput.Model

	// Caches
	projectCache          map[string][]api.ProjectListItem
	projectSearchInFlight map[string]bool
	taskCache             map[int][]api.TaskListItem
	taskCacheInFlight     map[int]bool
	activityCache         []api.ActivityListItem
	activityCacheValid    bool
	activityCacheInFlight bool

	// Last used activity for smart defaults
	lastUsedActivityID int

	// State
	err            error
	successMessage string
	isSubmitting   bool
}

// NewTimesheetCreator creates a new unified timesheet creator
func NewTimesheetCreator(apiClient *api.Client, username string) *TimesheetCreator {
	// Project search input
	projectSearch := textinput.New()
	projectSearch.Placeholder = "Search projects..."
	projectSearch.CharLimit = 100
	projectSearch.Width = 40
	projectSearch.Focus()

	// Description input
	description := textinput.New()
	description.Placeholder = "What are you working on?"
	description.CharLimit = 2000  // Allow longer descriptions for commit messages
	description.Width = 50

	// Date input
	date := textinput.New()
	date.Placeholder = "YYYY-MM-DD"
	date.CharLimit = 10
	date.Width = 12
	date.SetValue(time.Now().Format("2006-01-02"))

	// Start time input
	startTime := textinput.New()
	startTime.Placeholder = "HH:MM"
	startTime.CharLimit = 5
	startTime.Width = 8
	startTime.SetValue(time.Now().Format("15:04"))

	// End time input
	endTime := textinput.New()
	endTime.Placeholder = "HH:MM"
	endTime.CharLimit = 5
	endTime.Width = 8

	return &TimesheetCreator{
		apiClient:             apiClient,
		username:              username,
		focusedField:          FieldProject,
		projectSearchInput:    projectSearch,
		descriptionInput:      description,
		dateInput:             date,
		startTimeInput:        startTime,
		endTimeInput:          endTime,
		showProjectPopover:    true,
		projectCache:          make(map[string][]api.ProjectListItem),
		projectSearchInFlight: make(map[string]bool),
		taskCache:             make(map[int][]api.TaskListItem),
		taskCacheInFlight:     make(map[int]bool),
	}
}

// Init initializes the timesheet creator
func (t *TimesheetCreator) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		t.loadActivities(),
	)
}

// Update handles messages
func (t *TimesheetCreator) Update(msg tea.Msg) (*TimesheetCreator, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		t.width = msg.Width
		t.height = msg.Height

	case tea.KeyMsg:
		// Clear error message on any key press
		if t.err != nil {
			t.err = nil
		}

		// Handle global keys
		switch msg.String() {
		case "ctrl+c":
			return t, tea.Quit

		case "ctrl+g":
			// Add commits to description (only when project is selected and times are set)
			if t.selectedProject != nil && t.startTimeInput.Value() != "" {
				return t, t.fetchCommitsForDescription()
			}
			return t, nil

		case "esc":
			// Close popover or go back
			if t.showProjectPopover || t.showTaskPopover || t.showActivityPopover {
				t.showProjectPopover = false
				t.showTaskPopover = false
				t.showActivityPopover = false
				return t, nil
			}
			// Go back to main menu
			return t, func() tea.Msg { return backToMenuMsg{} }

		case "tab":
			// Move to next field
			t.moveToNextField()
			return t, nil

		case "shift+tab":
			// Move to previous field
			t.moveToPreviousField()
			return t, nil

		case "enter":
			// Handle enter based on context
			return t.handleEnter()

		case "up":
			// Navigate in popover
			if t.showProjectPopover || t.showTaskPopover || t.showActivityPopover {
				if t.popoverCursor > 0 {
					t.popoverCursor--
				}
				return t, nil
			}

		case "down":
			// Navigate in popover
			if t.showProjectPopover {
				if t.popoverCursor < len(t.projects)-1 {
					t.popoverCursor++
				}
				return t, nil
			}
			if t.showTaskPopover {
				if t.popoverCursor < len(t.tasks)-1 {
					t.popoverCursor++
				}
				return t, nil
			}
			if t.showActivityPopover {
				if t.popoverCursor < len(t.activities)-1 {
					t.popoverCursor++
				}
				return t, nil
			}

		default:
			// Handle text input for focused field
			cmd := t.handleTextInput(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}

	case timesheetProjectsLoadedMsg:
		if msg.CacheKey != "" {
			delete(t.projectSearchInFlight, msg.CacheKey)
			if len(msg.AllProjects) > 0 {
				t.projectCache[msg.CacheKey] = msg.AllProjects
			}
		}
		t.projects = msg.Projects
		t.popoverCursor = 0

	case timesheetTasksLoadedMsg:
		tasks := []api.TaskListItem(msg)
		if t.selectedProject != nil {
			t.taskCache[t.selectedProject.ID] = tasks
			delete(t.taskCacheInFlight, t.selectedProject.ID)
		}
		t.tasks = tasks
		t.popoverCursor = 0

	case timesheetActivitiesLoadedMsg:
		activities := []api.ActivityListItem(msg)
		t.activityCache = activities
		t.activityCacheValid = true
		t.activityCacheInFlight = false
		t.activities = activities
		// Set cursor to last used activity
		if t.lastUsedActivityID > 0 {
			for i, act := range t.activities {
				if act.ID == t.lastUsedActivityID {
					t.popoverCursor = i
					break
				}
			}
		}

	case timesheetSubmitSuccessMsg:
		t.isSubmitting = false
		t.successMessage = msg.message
		t.err = nil
		// Return to main menu after brief delay
		return t, tea.Tick(time.Second, func(tm time.Time) tea.Msg {
			return backToMenuMsg{}
		})

	case timesheetSubmitErrorMsg:
		t.isSubmitting = false
		t.err = msg.err

	case timesheetCommitsLoadedMsg:
		if msg.noCommitsFound {
			t.err = fmt.Errorf("no commits found in the specified time range")
		} else {
			// Append commits to description
			current := t.descriptionInput.Value()
			if current != "" {
				current += " | "
			}
			// Truncate if necessary (textinput has char limit of 2000)
			newDesc := current + msg.commits
			if len(newDesc) > 2000 {
				newDesc = newDesc[:1997] + "..."
			}
			t.descriptionInput.SetValue(newDesc)
			t.err = nil
		}

	case errorMsg:
		t.err = error(msg)
	}

	return t, tea.Batch(cmds...)
}

// View renders the timesheet creator
func (t *TimesheetCreator) View() string {
	if t.width == 0 {
		return "Loading..."
	}

	// Render header (same as main menu)
	header := t.renderHeader()

	// Main container
	containerStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#DDA036")).
		Padding(1, 2).
		Width(70)

	// Build the form
	formContent := t.renderForm()

	// Build help text
	helpText := t.renderHelp()

	// Build main content
	mainContent := containerStyle.Render(formContent)

	// Add error/success messages
	if t.err != nil {
		errorBoxStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#FF6B6B")).
			Padding(1, 2).
			Width(60)

		errorTitleStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF6B6B")).
			Bold(true)

		errorMsgStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF"))

		tipStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9A9EA0")).
			Italic(true)

		errorContent := lipgloss.JoinVertical(
			lipgloss.Left,
			errorTitleStyle.Render("⚠ Submission Failed"),
			"",
			errorMsgStyle.Render(t.err.Error()),
			"",
			tipStyle.Render("Troubleshooting tips:"),
			tipStyle.Render("  • Check your network connection"),
			tipStyle.Render("  • Verify all required fields are filled"),
			tipStyle.Render("  • Ensure start time is before end time"),
			tipStyle.Render("  • Check logs: /tmp/kartoza-timesheet-debug.log"),
		)
		mainContent = lipgloss.JoinVertical(lipgloss.Center, mainContent, "", errorBoxStyle.Render(errorContent))
	}

	if t.successMessage != "" {
		successBoxStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#569FC6")).
			Padding(1, 2).
			Width(50)

		successTitleStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#569FC6")).
			Bold(true)

		successMsgStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF"))

		successContent := lipgloss.JoinVertical(
			lipgloss.Center,
			successTitleStyle.Render("✓ Success"),
			"",
			successMsgStyle.Render(t.successMessage),
		)
		mainContent = lipgloss.JoinVertical(lipgloss.Center, mainContent, "", successBoxStyle.Render(successContent))
	}

	// Combine header and main content
	mainSection := lipgloss.JoinVertical(
		lipgloss.Center,
		header,
		"",
		mainContent,
	)

	// Center main content at top (leave room for help at bottom)
	centeredMain := lipgloss.Place(
		t.width,
		t.height-2,
		lipgloss.Center,
		lipgloss.Top,
		mainSection,
	)

	// Place help text at the bottom
	helpStyle := lipgloss.NewStyle().
		Width(t.width).
		Align(lipgloss.Center)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		centeredMain,
		helpStyle.Render(helpText),
	)
}

// renderHeader renders the consistent header (same as main menu)
func (t *TimesheetCreator) renderHeader() string {
	if t.headerState != nil {
		return RenderHeader("Create Timer", t.headerState)
	}
	// Fallback if header state not set
	return RenderHeader("Create Timer", &HeaderState{Username: t.username})
}

// SetHeaderState sets the shared header state
func (t *TimesheetCreator) SetHeaderState(state *HeaderState) {
	t.headerState = state
}

// renderForm renders the main form with all fields
func (t *TimesheetCreator) renderForm() string {
	labelWidth := 12
	fieldWidth := 50

	labelStyle := lipgloss.NewStyle().
		Width(labelWidth).
		Align(lipgloss.Right).
		Foreground(lipgloss.Color("#9A9EA0"))

	focusedLabelStyle := lipgloss.NewStyle().
		Width(labelWidth).
		Align(lipgloss.Right).
		Foreground(lipgloss.Color("#DDA036")).
		Bold(true)

	inputStyle := lipgloss.NewStyle().
		Width(fieldWidth)

	selectedValueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#569FC6")).
		Bold(true)

	unselectedValueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9A9EA0")).
		Italic(true)

	// Build rows
	var rows []string

	// Row 1: Project
	projectLabel := labelStyle.Render("Project:")
	if t.focusedField == FieldProject {
		projectLabel = focusedLabelStyle.Render("Project:")
	}
	var projectValue string
	if t.selectedProject != nil {
		projectValue = selectedValueStyle.Render("✓ " + t.selectedProject.Label)
	} else if t.showProjectPopover {
		projectValue = t.projectSearchInput.View()
	} else {
		projectValue = unselectedValueStyle.Render("Click to select...")
	}
	projectRow := lipgloss.JoinHorizontal(lipgloss.Top, projectLabel, "  ", inputStyle.Render(projectValue))
	rows = append(rows, projectRow)

	// Project popover
	if t.showProjectPopover && len(t.projects) > 0 {
		popover := t.renderPopover(t.projects, func(i int) string { return t.projects[i].Label })
		rows = append(rows, popover)
	}

	// Row 2: Task
	taskLabel := labelStyle.Render("Task:")
	if t.focusedField == FieldTask {
		taskLabel = focusedLabelStyle.Render("Task:")
	}
	var taskValue string
	if t.selectedTask != nil {
		// Format task label to show budget info nicely
		taskValue = selectedValueStyle.Render("✓ " + formatTaskLabel(t.selectedTask.Label))
	} else if t.selectedProject == nil {
		taskValue = unselectedValueStyle.Render("Select project first")
	} else if t.showTaskPopover {
		taskValue = unselectedValueStyle.Render("↓ Select from list...")
	} else {
		taskValue = unselectedValueStyle.Render("Click to select...")
	}
	taskRow := lipgloss.JoinHorizontal(lipgloss.Top, taskLabel, "  ", inputStyle.Render(taskValue))
	rows = append(rows, taskRow)

	// Task popover
	if t.showTaskPopover && len(t.tasks) > 0 {
		popover := t.renderPopover(t.tasks, func(i int) string { return formatTaskLabel(t.tasks[i].Label) })
		rows = append(rows, popover)
	}

	// Row 3: Activity
	activityLabel := labelStyle.Render("Activity:")
	if t.focusedField == FieldActivity {
		activityLabel = focusedLabelStyle.Render("Activity:")
	}
	var activityValue string
	if t.selectedActivity != nil {
		activityValue = selectedValueStyle.Render("✓ " + t.selectedActivity.Label)
	} else if t.showActivityPopover {
		activityValue = unselectedValueStyle.Render("↓ Select from list...")
	} else {
		activityValue = unselectedValueStyle.Render("Click to select...")
	}
	activityRow := lipgloss.JoinHorizontal(lipgloss.Top, activityLabel, "  ", inputStyle.Render(activityValue))
	rows = append(rows, activityRow)

	// Activity popover
	if t.showActivityPopover && len(t.activities) > 0 {
		popover := t.renderPopover(t.activities, func(i int) string { return t.activities[i].Label })
		rows = append(rows, popover)
	}

	// Divider
	divider := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9A9EA0")).
		Render(strings.Repeat("─", 62))
	rows = append(rows, "", divider, "")

	// Row 4: Description
	descLabel := labelStyle.Render("Description:")
	if t.focusedField == FieldDescription {
		descLabel = focusedLabelStyle.Render("Description:")
	}
	descRow := lipgloss.JoinHorizontal(lipgloss.Top, descLabel, "  ", t.descriptionInput.View())
	rows = append(rows, descRow)

	// Row 5: Date and Times (on same row for symmetry)
	dateLabel := labelStyle.Render("Date:")
	if t.focusedField == FieldDate {
		dateLabel = focusedLabelStyle.Render("Date:")
	}

	timeStyle := lipgloss.NewStyle().Width(12)
	timeLabelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9A9EA0")).
		Width(8)
	focusedTimeLabelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#DDA036")).
		Bold(true).
		Width(8)

	startLabel := timeLabelStyle.Render("Start:")
	if t.focusedField == FieldStartTime {
		startLabel = focusedTimeLabelStyle.Render("Start:")
	}

	endLabel := timeLabelStyle.Render("End:")
	if t.focusedField == FieldEndTime {
		endLabel = focusedTimeLabelStyle.Render("End:")
	}

	timeRow := lipgloss.JoinHorizontal(lipgloss.Top,
		dateLabel, "  ", t.dateInput.View(),
		"    ",
		startLabel, timeStyle.Render(t.startTimeInput.View()),
		"  ",
		endLabel, timeStyle.Render(t.endTimeInput.View()),
	)
	rows = append(rows, timeRow)

	// Row 6: Submit button
	rows = append(rows, "")
	buttonStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#DDA036")).
		Foreground(lipgloss.Color("#FFFFFF")).
		Padding(0, 3).
		Bold(true)

	if t.focusedField == FieldSubmit {
		buttonStyle = buttonStyle.Background(lipgloss.Color("#569FC6"))
	}

	buttonText := "START TIMER"
	if t.endTimeInput.Value() != "" {
		buttonText = "SUBMIT ENTRY"
	}
	if t.isSubmitting {
		buttonText = "SUBMITTING..."
	}

	// Check if ready to submit
	canSubmit := t.selectedProject != nil && t.selectedTask != nil && t.selectedActivity != nil
	if !canSubmit {
		buttonStyle = buttonStyle.Background(lipgloss.Color("#9A9EA0"))
	}

	button := buttonStyle.Render(buttonText)
	buttonRow := lipgloss.NewStyle().
		Width(62).
		Align(lipgloss.Center).
		Render(button)
	rows = append(rows, buttonRow)

	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

// renderPopover renders a pop-over selection list
func (t *TimesheetCreator) renderPopover(items interface{}, getLabel func(int) string) string {
	popoverStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#DDA036")).
		Padding(0, 1).
		MarginLeft(14).
		Width(50)

	selectedStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#DDA036")).
		Foreground(lipgloss.Color("#FFFFFF")).
		Bold(true).
		Width(46)

	normalStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Width(46)

	var itemCount int
	switch v := items.(type) {
	case []api.ProjectListItem:
		itemCount = len(v)
	case []api.TaskListItem:
		itemCount = len(v)
	case []api.ActivityListItem:
		itemCount = len(v)
	}

	// Limit visible items
	maxVisible := 8
	startIdx := 0
	endIdx := itemCount

	if itemCount > maxVisible {
		// Center the view around cursor
		startIdx = t.popoverCursor - maxVisible/2
		if startIdx < 0 {
			startIdx = 0
		}
		endIdx = startIdx + maxVisible
		if endIdx > itemCount {
			endIdx = itemCount
			startIdx = endIdx - maxVisible
		}
	}

	var rows []string
	for i := startIdx; i < endIdx; i++ {
		label := getLabel(i)
		if len(label) > 44 {
			label = label[:41] + "..."
		}

		if i == t.popoverCursor {
			rows = append(rows, selectedStyle.Render("▶ "+label))
		} else {
			rows = append(rows, normalStyle.Render("  "+label))
		}
	}

	// Add scroll indicator
	if itemCount > maxVisible {
		scrollInfo := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9A9EA0")).
			Italic(true).
			Render(fmt.Sprintf("  ↑↓ %d of %d", t.popoverCursor+1, itemCount))
		rows = append(rows, scrollInfo)
	}

	return popoverStyle.Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
}

// renderHelp renders the help text
func (t *TimesheetCreator) renderHelp() string {
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9A9EA0")).
		Italic(true).
		Align(lipgloss.Center)

	if t.showProjectPopover || t.showTaskPopover || t.showActivityPopover {
		return helpStyle.Render("↑/↓: Navigate • Enter: Select • Esc: Close • Tab: Next field")
	}

	return helpStyle.Render("Tab: Next • Shift+Tab: Prev • Enter: Select/Submit • Ctrl+G: Add Commits • Esc: Back")
}

// Field navigation
func (t *TimesheetCreator) moveToNextField() {
	t.closeAllPopovers()

	switch t.focusedField {
	case FieldProject:
		t.focusedField = FieldTask
		if t.selectedProject != nil && t.selectedTask == nil {
			t.showTaskPopover = true
			t.popoverCursor = 0
		}
	case FieldTask:
		t.focusedField = FieldActivity
		if t.selectedTask != nil && t.selectedActivity == nil {
			t.showActivityPopover = true
			t.popoverCursor = 0
		}
	case FieldActivity:
		t.focusedField = FieldDescription
		t.descriptionInput.Focus()
	case FieldDescription:
		t.focusedField = FieldDate
		t.descriptionInput.Blur()
		t.dateInput.Focus()
	case FieldDate:
		t.focusedField = FieldStartTime
		t.dateInput.Blur()
		t.startTimeInput.Focus()
	case FieldStartTime:
		t.focusedField = FieldEndTime
		t.startTimeInput.Blur()
		t.endTimeInput.Focus()
	case FieldEndTime:
		t.focusedField = FieldSubmit
		t.endTimeInput.Blur()
	case FieldSubmit:
		t.focusedField = FieldProject
		if t.selectedProject == nil {
			t.showProjectPopover = true
			t.projectSearchInput.Focus()
		}
	}
}

func (t *TimesheetCreator) moveToPreviousField() {
	t.closeAllPopovers()

	switch t.focusedField {
	case FieldProject:
		t.focusedField = FieldSubmit
	case FieldTask:
		t.focusedField = FieldProject
		if t.selectedProject == nil {
			t.showProjectPopover = true
			t.projectSearchInput.Focus()
		}
	case FieldActivity:
		t.focusedField = FieldTask
		if t.selectedTask == nil && t.selectedProject != nil {
			t.showTaskPopover = true
			t.popoverCursor = 0
		}
	case FieldDescription:
		t.focusedField = FieldActivity
		t.descriptionInput.Blur()
		if t.selectedActivity == nil {
			t.showActivityPopover = true
			t.popoverCursor = 0
		}
	case FieldDate:
		t.focusedField = FieldDescription
		t.dateInput.Blur()
		t.descriptionInput.Focus()
	case FieldStartTime:
		t.focusedField = FieldDate
		t.startTimeInput.Blur()
		t.dateInput.Focus()
	case FieldEndTime:
		t.focusedField = FieldStartTime
		t.endTimeInput.Blur()
		t.startTimeInput.Focus()
	case FieldSubmit:
		t.focusedField = FieldEndTime
		t.endTimeInput.Focus()
	}
}

func (t *TimesheetCreator) closeAllPopovers() {
	t.showProjectPopover = false
	t.showTaskPopover = false
	t.showActivityPopover = false
	t.blurAllInputs()
}

func (t *TimesheetCreator) blurAllInputs() {
	t.projectSearchInput.Blur()
	t.descriptionInput.Blur()
	t.dateInput.Blur()
	t.startTimeInput.Blur()
	t.endTimeInput.Blur()
}

// handleEnter handles the enter key based on current context
func (t *TimesheetCreator) handleEnter() (*TimesheetCreator, tea.Cmd) {
	// Handle popover selection
	if t.showProjectPopover && len(t.projects) > 0 {
		t.selectedProject = &t.projects[t.popoverCursor]
		t.showProjectPopover = false
		t.popoverCursor = 0
		// Auto-advance to task selection
		t.focusedField = FieldTask
		t.showTaskPopover = true
		return t, t.loadTasks(t.selectedProject.ID)
	}

	if t.showTaskPopover && len(t.tasks) > 0 {
		t.selectedTask = &t.tasks[t.popoverCursor]
		t.showTaskPopover = false
		t.popoverCursor = 0
		// Auto-advance to activity selection
		t.focusedField = FieldActivity
		t.showActivityPopover = true
		// Set cursor to last used activity
		if t.lastUsedActivityID > 0 {
			for i, act := range t.activities {
				if act.ID == t.lastUsedActivityID {
					t.popoverCursor = i
					break
				}
			}
		}
		return t, nil
	}

	if t.showActivityPopover && len(t.activities) > 0 {
		t.selectedActivity = &t.activities[t.popoverCursor]
		t.lastUsedActivityID = t.selectedActivity.ID
		t.showActivityPopover = false
		t.popoverCursor = 0
		// Auto-advance to description
		t.focusedField = FieldDescription
		t.descriptionInput.Focus()
		return t, nil
	}

	// Handle field-specific enter
	switch t.focusedField {
	case FieldProject:
		if t.selectedProject == nil {
			t.showProjectPopover = true
			t.projectSearchInput.Focus()
		}
	case FieldTask:
		if t.selectedProject != nil && t.selectedTask == nil {
			t.showTaskPopover = true
		}
	case FieldActivity:
		if t.selectedActivity == nil {
			t.showActivityPopover = true
		}
	case FieldDescription, FieldDate, FieldStartTime, FieldEndTime:
		t.moveToNextField()
	case FieldSubmit:
		return t.submitEntry()
	}

	return t, nil
}

// handleTextInput handles text input for the focused field
func (t *TimesheetCreator) handleTextInput(msg tea.KeyMsg) tea.Cmd {
	var cmd tea.Cmd

	switch t.focusedField {
	case FieldProject:
		if t.showProjectPopover {
			t.projectSearchInput, cmd = t.projectSearchInput.Update(msg)
			// Trigger search
			query := t.projectSearchInput.Value()
			if len(query) >= 2 {
				return t.searchProjects(query)
			} else if len(query) == 0 {
				t.projects = nil
			}
		}
	case FieldDescription:
		t.descriptionInput, cmd = t.descriptionInput.Update(msg)
	case FieldDate:
		t.dateInput, cmd = t.dateInput.Update(msg)
	case FieldStartTime:
		t.startTimeInput, cmd = t.startTimeInput.Update(msg)
	case FieldEndTime:
		t.endTimeInput, cmd = t.endTimeInput.Update(msg)
	}

	return cmd
}

// API loading commands
func (t *TimesheetCreator) searchProjects(query string) tea.Cmd {
	cacheKey := strings.ToLower(query)
	if len(cacheKey) > 2 {
		cacheKey = cacheKey[:2]
	}

	// Check cache
	if cached, ok := t.projectCache[cacheKey]; ok {
		filtered := t.filterProjectsLocally(cached, query)
		return func() tea.Msg {
			return timesheetProjectsLoadedMsg{
				Projects: filtered,
				CacheKey: cacheKey,
			}
		}
	}

	// Check in-flight
	if t.projectSearchInFlight[cacheKey] {
		return nil
	}

	t.projectSearchInFlight[cacheKey] = true

	return func() tea.Msg {
		if t.apiClient == nil {
			return errorMsg(fmt.Errorf("API client not available"))
		}

		projects, err := t.apiClient.GetProjects(cacheKey)
		if err != nil {
			return errorMsg(err)
		}

		queryLower := strings.ToLower(query)
		var filtered []api.ProjectListItem
		for _, p := range projects {
			if strings.Contains(strings.ToLower(p.Label), queryLower) {
				filtered = append(filtered, p)
			}
		}

		return timesheetProjectsLoadedMsg{
			Projects:    filtered,
			CacheKey:    cacheKey,
			AllProjects: projects,
		}
	}
}

func (t *TimesheetCreator) filterProjectsLocally(projects []api.ProjectListItem, query string) []api.ProjectListItem {
	if query == "" {
		return projects
	}
	queryLower := strings.ToLower(query)
	var filtered []api.ProjectListItem
	for _, p := range projects {
		if strings.Contains(strings.ToLower(p.Label), queryLower) {
			filtered = append(filtered, p)
		}
	}
	return filtered
}

func (t *TimesheetCreator) loadTasks(projectID int) tea.Cmd {
	// Check cache
	if cached, ok := t.taskCache[projectID]; ok {
		return func() tea.Msg {
			return timesheetTasksLoadedMsg(cached)
		}
	}

	// Check in-flight
	if t.taskCacheInFlight[projectID] {
		return nil
	}

	t.taskCacheInFlight[projectID] = true

	return func() tea.Msg {
		if t.apiClient == nil {
			return errorMsg(fmt.Errorf("API client not available"))
		}

		tasks, err := t.apiClient.GetTasks(fmt.Sprintf("%d", projectID))
		if err != nil {
			return errorMsg(err)
		}

		return timesheetTasksLoadedMsg(tasks)
	}
}

func (t *TimesheetCreator) loadActivities() tea.Cmd {
	if t.activityCacheValid {
		return func() tea.Msg {
			return timesheetActivitiesLoadedMsg(t.activityCache)
		}
	}

	if t.activityCacheInFlight {
		return nil
	}

	t.activityCacheInFlight = true

	return func() tea.Msg {
		if t.apiClient == nil {
			return errorMsg(fmt.Errorf("API client not available"))
		}

		activities, err := t.apiClient.GetActivities()
		if err != nil {
			return errorMsg(err)
		}

		return timesheetActivitiesLoadedMsg(activities)
	}
}

// submitEntry submits the timesheet entry
func (t *TimesheetCreator) submitEntry() (*TimesheetCreator, tea.Cmd) {
	tuiDebugLog.Printf("submitEntry called")
	tuiDebugLog.Printf("  selectedProject: %v", t.selectedProject)
	tuiDebugLog.Printf("  selectedTask: %v", t.selectedTask)
	tuiDebugLog.Printf("  selectedActivity: %v", t.selectedActivity)

	// Validate
	if t.selectedProject == nil || t.selectedTask == nil || t.selectedActivity == nil {
		tuiDebugLog.Printf("Validation failed: missing project, task, or activity")
		t.err = fmt.Errorf("please select project, task, and activity")
		return t, nil
	}

	t.isSubmitting = true
	t.err = nil

	return t, func() tea.Msg {
		tuiDebugLog.Printf("submitEntry command executing")
		// Parse times
		dateStr := t.dateInput.Value()
		startTimeStr := t.startTimeInput.Value()
		endTimeStr := t.endTimeInput.Value()

		tuiDebugLog.Printf("  dateStr: %s", dateStr)
		tuiDebugLog.Printf("  startTimeStr: %s", startTimeStr)
		tuiDebugLog.Printf("  endTimeStr: %s", endTimeStr)

		var startTime time.Time
		var err error

		if startTimeStr == "" {
			startTime = time.Now()
			tuiDebugLog.Printf("  Using current time as start time: %s", startTime.Format(time.RFC3339))
		} else {
			startStr := fmt.Sprintf("%s %s", dateStr, startTimeStr)
			startTime, err = time.Parse("2006-01-02 15:04", startStr)
			if err != nil {
				tuiDebugLog.Printf("  Invalid start time: %v", err)
				return timesheetSubmitErrorMsg{err: fmt.Errorf("invalid start time: %w", err)}
			}
			tuiDebugLog.Printf("  Parsed start time: %s", startTime.Format(time.RFC3339))
		}

		var endTimePtr *time.Time
		var duration float64

		if endTimeStr != "" {
			endStr := fmt.Sprintf("%s %s", dateStr, endTimeStr)
			endTime, err := time.Parse("2006-01-02 15:04", endStr)
			if err != nil {
				tuiDebugLog.Printf("  Invalid end time: %v", err)
				return timesheetSubmitErrorMsg{err: fmt.Errorf("invalid end time: %w", err)}
			}
			duration = endTime.Sub(startTime).Hours()
			if duration <= 0 {
				tuiDebugLog.Printf("  End time must be after start time")
				return timesheetSubmitErrorMsg{err: fmt.Errorf("end time must be after start time")}
			}
			endTimePtr = &endTime
			tuiDebugLog.Printf("  Parsed end time: %s, duration: %f", endTime.Format(time.RFC3339), duration)
		}

		// Create entry
		entry := models.TimeEntry{
			ProjectID:   fmt.Sprintf("%d", t.selectedProject.ID),
			ActivityID:  fmt.Sprintf("%d", t.selectedActivity.ID),
			Description: t.descriptionInput.Value(),
			StartTime:   startTime,
			EndTime:     endTimePtr,
			Duration:    duration,
		}

		if t.selectedTask != nil {
			taskID := fmt.Sprintf("%d", t.selectedTask.ID)
			entry.TaskID = &taskID
		}

		tuiDebugLog.Printf("  Calling CreateTimesheet...")
		// Submit
		err = t.apiClient.CreateTimesheet(entry)
		if err != nil {
			tuiDebugLog.Printf("  CreateTimesheet error: %v", err)
			return timesheetSubmitErrorMsg{err: err}
		}

		tuiDebugLog.Printf("  CreateTimesheet successful!")

		// Immediately update cache for waybar status command
		timelogs, err := t.apiClient.GetTimelogs()
		if err == nil {
			cfg, cfgErr := config.LoadConfig()
			if cfgErr == nil {
				store, storeErr := storage.New(cfg.GetStorageDir())
				if storeErr == nil {
					_ = store.SaveTimelogCache(timelogs)
				}
			}
		}

		msg := "Timer started successfully!"
		if endTimePtr != nil {
			msg = "Timesheet entry created successfully!"
		}

		return timesheetSubmitSuccessMsg{message: msg}
	}
}

// Message types for TimesheetCreator
type timesheetProjectsLoadedMsg struct {
	Projects    []api.ProjectListItem
	CacheKey    string
	AllProjects []api.ProjectListItem
}

type timesheetTasksLoadedMsg []api.TaskListItem
type timesheetActivitiesLoadedMsg []api.ActivityListItem

type timesheetSubmitSuccessMsg struct {
	message string
}

type timesheetSubmitErrorMsg struct {
	err error
}

type timesheetCommitsLoadedMsg struct {
	commits        string
	noCommitsFound bool
}

// fetchCommitsForDescription fetches git commits for the selected project and time range
func (t *TimesheetCreator) fetchCommitsForDescription() tea.Cmd {
	return func() tea.Msg {
		if t.selectedProject == nil {
			return timesheetCommitsLoadedMsg{noCommitsFound: true}
		}

		// Parse times
		dateStr := t.dateInput.Value()
		startTimeStr := t.startTimeInput.Value()
		endTimeStr := t.endTimeInput.Value()

		var startTime, endTime time.Time
		var err error

		if startTimeStr != "" {
			startStr := fmt.Sprintf("%s %s", dateStr, startTimeStr)
			startTime, err = time.Parse("2006-01-02 15:04", startStr)
			if err != nil {
				return timesheetCommitsLoadedMsg{noCommitsFound: true}
			}
		} else {
			return timesheetCommitsLoadedMsg{noCommitsFound: true}
		}

		if endTimeStr != "" {
			endStr := fmt.Sprintf("%s %s", dateStr, endTimeStr)
			endTime, err = time.Parse("2006-01-02 15:04", endStr)
			if err != nil {
				endTime = time.Now()
			}
		} else {
			endTime = time.Now()
		}

		// Load code repo associations
		cfg, err := config.LoadConfig()
		if err != nil {
			tuiDebugLog.Printf("fetchCommitsForDescription: failed to load config: %v", err)
			return timesheetCommitsLoadedMsg{noCommitsFound: true}
		}

		store, err := storage.New(cfg.GetStorageDir())
		if err != nil {
			tuiDebugLog.Printf("fetchCommitsForDescription: failed to create storage: %v", err)
			return timesheetCommitsLoadedMsg{noCommitsFound: true}
		}

		associations, err := store.LoadCodeRepoAssociations()
		if err != nil {
			tuiDebugLog.Printf("fetchCommitsForDescription: failed to load code repo associations: %v", err)
			return timesheetCommitsLoadedMsg{noCommitsFound: true}
		}

		// Find association for this project
		assoc := associations.GetAssociationByProject(t.selectedProject.ID)
		if assoc == nil || !assoc.HasAssociation() {
			tuiDebugLog.Printf("fetchCommitsForDescription: no repo linked to project %d", t.selectedProject.ID)
			return timesheetCommitsLoadedMsg{noCommitsFound: true}
		}

		// Check if this is a local path or a GitHub URL
		repoURL := assoc.RepoURL
		if api.IsLocalPath(repoURL) {
			// Local git repository
			repoPath := api.ExpandPath(repoURL)
			if !api.IsGitRepository(repoPath) {
				return timesheetCommitsLoadedMsg{noCommitsFound: true}
			}

			localGitClient := api.NewLocalGitClient()
			commits, err := localGitClient.GetCommitsInTimeRange(repoPath, startTime, endTime, "")
			if err != nil || len(commits) == 0 {
				return timesheetCommitsLoadedMsg{noCommitsFound: true}
			}

			return timesheetCommitsLoadedMsg{commits: api.FormatLocalCommitsAsDescription(commits)}
		}

		// GitHub repository
		githubClient := api.NewGitHubClient(cfg.GetGitHubToken())
		commits, err := githubClient.GetCommitsInTimeRange(assoc.RepoOwner, assoc.RepoName, startTime, endTime, "")
		if err != nil || len(commits) == 0 {
			return timesheetCommitsLoadedMsg{noCommitsFound: true}
		}

		return timesheetCommitsLoadedMsg{commits: api.FormatCommitsAsDescription(commits)}
	}
}
