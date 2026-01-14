package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/huh"

	"github.com/kartoza/go-timesheets-go/internal/api"
	"github.com/kartoza/go-timesheets-go/internal/models"
	"github.com/kartoza/go-timesheets-go/internal/service"
)

// Enhanced TUI with API integration and combo box widgets

var (
	titleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#DDA036")).
		Padding(1, 2)

	subtitleStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9A9EA0")).
		Padding(0, 2)

	errorStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FF6B6B")).
		Bold(true).
		Padding(0, 2)

	successStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#569FC6")).
		Bold(true).
		Padding(0, 2)

	boxStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#DDA036")).
		Padding(1, 2).
		Margin(1)

	activeStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#DDA036")).
		Bold(true)

	inactiveStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9A9EA0"))
)

// ViewState represents different views in the application
type ViewState int

const (
	MenuView ViewState = iota
	ProjectSelectView
	ActivitySelectView
	TaskSelectView
	TimesheetListView
	TimesheetEntryView
	SettingsView
)

// EnhancedModel represents the enhanced TUI model
type EnhancedModel struct {
	// Core components
	service    *service.TimesheetService
	apiClient  *api.Client
	help       help.Model
	keys       keyMap

	// State
	currentView ViewState
	error       string
	success     string
	loading     bool

	// Data from API
	projects   []api.ProjectListItem
	activities []api.ActivityListItem
	tasks      []api.TaskListItem
	timelogs   []api.TimelogEntry

	// Selection state
	selectedProject  *api.ProjectListItem
	selectedActivity *api.ActivityListItem
	selectedTask     *api.TaskListItem

	// Enhanced combo box widgets
	projectCombo     RefreshableComboBox
	activityCombo    RefreshableComboBox
	taskCombo        ComboBox
	timeEntryCombo   ComboBox

	// Forms
	form *huh.Form
	
	// Text inputs for description
	descriptionInput textinput.Model

	// Current entry being tracked
	currentEntry *models.TimeEntry

	// Configuration
	apiEnabled bool
	width      int
	height     int
	
	// Focus state for form fields
	focusedField int
	maxFields    int
}

// keyMap defines keyboard shortcuts
type keyMap struct {
	Up    key.Binding
	Down  key.Binding
	Left  key.Binding
	Right key.Binding
	Enter key.Binding
	Back  key.Binding
	Quit  key.Binding
	Help  key.Binding
}

// ShortHelp returns keybindings to be shown in the mini help view
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help, k.Quit}
}

// FullHelp returns keybindings for the expanded help view
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Left, k.Right},
		{k.Enter, k.Back, k.Help, k.Quit},
	}
}

var keys = keyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "move up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "move down"),
	),
	Left: key.NewBinding(
		key.WithKeys("left", "h"),
		key.WithHelp("←/h", "move left"),
	),
	Right: key.NewBinding(
		key.WithKeys("right", "l"),
		key.WithHelp("→/l", "move right"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "select"),
	),
	Back: key.NewBinding(
		key.WithKeys("esc", "backspace"),
		key.WithHelp("esc", "back"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "toggle help"),
	),
}

// listItem implements the list.Item interface for combo boxes
type listItem struct {
	title, desc string
	id          string
}

func (i listItem) FilterValue() string { return i.title }
func (i listItem) Title() string       { return i.title }
func (i listItem) Description() string { return i.desc }

// NewEnhancedModel creates a new enhanced TUI model
func NewEnhancedModel(service *service.TimesheetService, apiClient *api.Client) *EnhancedModel {
	// Initialize text input for description
	descInput := textinput.New()
	descInput.Placeholder = "Enter description..."
	descInput.CharLimit = 255
	descInput.Focus()

	// Default dimensions
	width := 80
	height := 25

	m := &EnhancedModel{
		service:          service,
		apiClient:        apiClient,
		help:             help.New(),
		keys:             keys,
		currentView:      MenuView,
		apiEnabled:       apiClient != nil,
		descriptionInput: descInput,
		width:            width,
		height:           height,
		focusedField:     0,
		maxFields:        4, // project, activity, task, description
	}

	// Initialize combo boxes
	if apiClient != nil {
		m.projectCombo = NewRefreshableProjectComboBox(apiClient, width-20, 8)
		m.activityCombo = NewRefreshableActivityComboBox(apiClient, width-20, 6)
		m.taskCombo = NewComboBox([]ComboBoxItem{}, "Select a task...", width-20, 8)
		m.timeEntryCombo = NewComboBox([]ComboBoxItem{}, "Select a time entry...", width-20, 10)
	}

	// Load current entry if any
	if entry, err := service.GetActiveTimeEntry(); err == nil && entry != nil {
		// Convert ActiveTimeEntry to TimeEntry for compatibility
		timeEntry := &models.TimeEntry{
			ID:           entry.ID,
			ProjectID:    entry.ProjectID,
			TaskID:       entry.TaskID,
			ActivityID:   entry.ActivityID,
			Description:  entry.Description,
			StartTime:    entry.StartTime,
			ProjectName:  entry.ProjectName,
			TaskName:     entry.TaskName,
			ActivityName: entry.ActivityName,
		}
		m.currentEntry = timeEntry
	}

	return m
}

// createList creates a new list model for combo boxes
func (m *EnhancedModel) createList(title string) list.Model {
	l := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 10)
	l.Title = title
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.Styles.Title = titleStyle
	return l
}

// Init initializes the model
func (m *EnhancedModel) Init() tea.Cmd {
	var cmds []tea.Cmd
	
	// Load data from API if available
	if m.apiEnabled {
		cmds = append(cmds, m.projectCombo.RefreshData())
		cmds = append(cmds, m.activityCombo.RefreshData())
	}

	return tea.Batch(cmds...)
}

// Update handles messages and updates the model
func (m *EnhancedModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.help.Width = msg.Width

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.Help):
			m.help.ShowAll = !m.help.ShowAll
		case key.Matches(msg, m.keys.Back):
			return m, m.handleBack()
		case key.Matches(msg, m.keys.Enter):
			return m, m.handleEnter()
		default:
			// Handle view-specific key events
			switch m.currentView {
			case MenuView:
				return m, m.handleMenuKeys(msg)
			case TimesheetEntryView:
				return m, m.handleTimesheetEntryKeys(msg)
			}
		}

	case ComboBoxDataMsg:
		// Update combo boxes with loaded data
		if m.apiEnabled {
			m.projectCombo, cmd = m.projectCombo.Update(msg)
			cmds = append(cmds, cmd)
			
			m.activityCombo, cmd = m.activityCombo.Update(msg)
			cmds = append(cmds, cmd)
			
			// If it's task data, update the task combo
			m.taskCombo, cmd = m.taskCombo.Update(msg)
			cmds = append(cmds, cmd)
		}
		
	case ComboBoxErrorMsg:
		m.error = fmt.Sprintf("API Error: %v", msg.Err)
		
	case TimesheetSaveSuccessMsg:
		m.success = "Timesheet entry saved successfully!"
		m.currentEntry = msg.Entry
		m.currentView = MenuView
		
	case TimesheetSaveErrorMsg:
		m.error = fmt.Sprintf("Failed to save timesheet: %v", msg.Err)
	}

	// Update current view components
	switch m.currentView {
	case TimesheetEntryView:
		// Update combo boxes based on focus
		switch m.focusedField {
		case 0: // Project combo
			if m.apiEnabled {
				m.projectCombo, cmd = m.projectCombo.Update(msg)
				cmds = append(cmds, cmd)
				
				// If project selected, load tasks
				if selected := m.projectCombo.GetSelected(); selected != nil {
					if project, ok := selected.Value.(api.ProjectListItem); ok {
						cmds = append(cmds, m.loadTasksForProject(fmt.Sprintf("%d", project.ID)))
					}
				}
			}
		case 1: // Activity combo
			if m.apiEnabled {
				m.activityCombo, cmd = m.activityCombo.Update(msg)
				cmds = append(cmds, cmd)
			}
		case 2: // Task combo
			if m.apiEnabled {
				m.taskCombo, cmd = m.taskCombo.Update(msg)
				cmds = append(cmds, cmd)
			}
		case 3: // Description input
			m.descriptionInput, cmd = m.descriptionInput.Update(msg)
			cmds = append(cmds, cmd)
		}
	case TimesheetListView:
		if m.apiEnabled {
			m.timeEntryCombo, cmd = m.timeEntryCombo.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

// View renders the current view
func (m *EnhancedModel) View() string {
	if m.loading {
		return m.loadingView()
	}

	var content string
	
	switch m.currentView {
	case MenuView:
		content = m.menuView()
	case ProjectSelectView:
		content = m.projectSelectView()
	case ActivitySelectView:
		content = m.activitySelectView()
	case TaskSelectView:
		content = m.taskSelectView()
	case TimesheetListView:
		content = m.timesheetListView()
	case TimesheetEntryView:
		content = m.timesheetEntryView()
	case SettingsView:
		content = m.settingsView()
	}

	// Add error/success messages
	if m.error != "" {
		content = errorStyle.Render("Error: "+m.error) + "\n\n" + content
	}
	if m.success != "" {
		content = successStyle.Render("Success: "+m.success) + "\n\n" + content
	}

	// Add help
	helpView := m.help.View(m.keys)
	
	return lipgloss.JoinVertical(
		lipgloss.Left,
		content,
		"\n",
		helpView,
	)
}

// Menu view
func (m *EnhancedModel) menuView() string {
	title := titleStyle.Render("🕐 Kartoza Timesheet")
	subtitle := subtitleStyle.Render("Tempus Fugit - Time Flies")

	var status string
	if m.currentEntry != nil {
		duration := time.Since(m.currentEntry.StartTime)
		status = activeStyle.Render(fmt.Sprintf("⏱️  Currently tracking: %s (%s)",
			m.currentEntry.ProjectID, formatDuration(duration)))
	} else {
		status = inactiveStyle.Render("⏸️  No active time tracking")
	}

	var apiStatus string
	if m.apiEnabled {
		apiStatus = successStyle.Render("🌐 API Connected")
	} else {
		apiStatus = inactiveStyle.Render("📱 Local Mode")
	}

	options := []string{
		"1. Start Time Tracking",
		"2. View Timesheet History", 
		"3. Settings",
		"q. Quit",
	}

	menu := lipgloss.JoinVertical(
		lipgloss.Left,
		strings.Join(options, "\n"),
	)

	instructions := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9A9EA0")).
		Render("Press 1, 2, 3, or q")

	return lipgloss.JoinVertical(
		lipgloss.Center,
		title,
		subtitle,
		"",
		status,
		apiStatus,
		"",
		boxStyle.Render(menu),
		"",
		instructions,
	)
}

// Project selection view
func (m *EnhancedModel) projectSelectView() string {
	title := titleStyle.Render("Select Project")
	
	if len(m.projects) == 0 {
		return lipgloss.JoinVertical(
			lipgloss.Left,
			title,
			errorStyle.Render("No projects available"),
		)
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"Use the enhanced timesheet entry view instead",
	)
}

// Activity selection view
func (m *EnhancedModel) activitySelectView() string {
	title := titleStyle.Render("Select Activity")
	
	if len(m.activities) == 0 {
		return lipgloss.JoinVertical(
			lipgloss.Left,
			title,
			errorStyle.Render("No activities available"),
		)
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"Use the enhanced timesheet entry view instead",
	)
}

// Task selection view
func (m *EnhancedModel) taskSelectView() string {
	title := titleStyle.Render("Select Task (Optional)")
	subtitle := subtitleStyle.Render("Use the enhanced timesheet entry view instead")
	
	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		subtitle,
	)
}


// Timesheet list view
func (m *EnhancedModel) timesheetListView() string {
	title := titleStyle.Render("Timesheet History")
	
	if len(m.timelogs) == 0 {
		return lipgloss.JoinVertical(
			lipgloss.Left,
			title,
			inactiveStyle.Render("No timesheet entries found"),
		)
	}

	// Group timelogs by date
	groupedLogs := make(map[string][]api.TimelogEntry)
	for _, entry := range m.timelogs {
		fromTime, err := entry.GetFromTimeAsTime()
		if err == nil {
			date := fromTime.Format("2006-01-02")
			groupedLogs[date] = append(groupedLogs[date], entry)
		}
	}

	var content strings.Builder
	for date, entries := range groupedLogs {
		content.WriteString(activeStyle.Render(fmt.Sprintf("📅 %s\n", date)))

		for _, entry := range entries {
			content.WriteString(fmt.Sprintf("  ⏱️  %s - %s (%s)\n",
				entry.ProjectName, entry.ActivityType, entry.Hours))
			if entry.TaskName != "" {
				content.WriteString(fmt.Sprintf("     Task: %s\n", entry.TaskName))
			}
			desc := entry.GetDescriptionString()
			if desc != "" {
				content.WriteString(fmt.Sprintf("     %s\n", desc))
			}
		}
		content.WriteString("\n")
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		boxStyle.Render(content.String()),
	)
}

// Settings view
func (m *EnhancedModel) settingsView() string {
	title := titleStyle.Render("Settings")
	
	var settings strings.Builder
	settings.WriteString(fmt.Sprintf("API Mode: %v\n", m.apiEnabled))
	if m.apiEnabled && m.apiClient != nil {
		settings.WriteString("API Status: Connected\n")
	}
	settings.WriteString("\nPress Esc to go back")

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		boxStyle.Render(settings.String()),
	)
}

// Loading view
func (m *EnhancedModel) loadingView() string {
	return titleStyle.Render("Loading... 🔄")
}

// Helper functions for handling navigation and actions

func (m *EnhancedModel) handleBack() tea.Cmd {
	// Clear any error/success messages
	m.error = ""
	m.success = ""
	
	switch m.currentView {
	case ProjectSelectView, TimesheetListView, SettingsView:
		m.currentView = MenuView
	case ActivitySelectView:
		m.currentView = ProjectSelectView
	case TaskSelectView:
		m.currentView = ActivitySelectView
	case TimesheetEntryView:
		m.currentView = MenuView
	}
	
	return nil
}

func (m *EnhancedModel) handleEnter() tea.Cmd {
	// Clear any error/success messages
	m.error = ""
	m.success = ""

	switch m.currentView {
	case MenuView:
		// Menu navigation will be handled by specific key presses (1, 2, 3)
		return nil
		
	case ProjectSelectView, ActivitySelectView, TaskSelectView:
		// These old views are no longer used, redirect to the enhanced entry view
		m.currentView = TimesheetEntryView
		m.focusedField = 0
		m.updateFieldFocus()
		
	case TimesheetEntryView:
		return m.startTimeTracking()
	}
	
	return nil
}

func (m *EnhancedModel) startTimeTracking() tea.Cmd {
	if m.selectedProject == nil || m.selectedActivity == nil {
		m.error = "Project and activity are required"
		return nil
	}

	var taskID *string
	if m.selectedTask != nil {
		id := strconv.Itoa(m.selectedTask.ID)
		taskID = &id
	}

	description := m.descriptionInput.Value()

	entry, err := m.service.StartTimeEntry(
		fmt.Sprintf("%d", m.selectedProject.ID),
		strconv.Itoa(m.selectedActivity.ID),
		taskID,
		description,
	)
	
	if err != nil {
		m.error = fmt.Sprintf("Failed to start time tracking: %v", err)
		return nil
	}

	m.currentEntry = entry
	m.success = "Time tracking started!"
	m.currentView = MenuView
	
	// Reset selections
	m.selectedProject = nil
	m.selectedActivity = nil
	m.selectedTask = nil
	m.descriptionInput.SetValue("")
	
	return nil
}

// Update list functions

// Command functions for loading data
func (m *EnhancedModel) loadProjectsCmd() tea.Cmd {
	if !m.apiEnabled {
		return nil
	}
	
	m.loading = true
	
	return func() tea.Msg {
		projects, err := m.apiClient.GetProjects("")
		return enhancedProjectsLoadedMsg{projects: projects, err: err}
	}
}

func (m *EnhancedModel) loadActivitiesCmd() tea.Cmd {
	if !m.apiEnabled {
		return nil
	}
	
	m.loading = true
	
	return func() tea.Msg {
		activities, err := m.apiClient.GetActivities()
		return enhancedActivitiesLoadedMsg{activities: activities, err: err}
	}
}

func (m *EnhancedModel) loadTasksCmd(projectID string) tea.Cmd {
	if !m.apiEnabled {
		return nil
	}
	
	m.loading = true
	
	return func() tea.Msg {
		tasks, err := m.apiClient.GetTasks(projectID)
		return enhancedTasksLoadedMsg{tasks: tasks, err: err}
	}
}

func (m *EnhancedModel) loadTimelogsCmd() tea.Cmd {
	if !m.apiEnabled {
		return nil
	}
	
	m.loading = true
	
	return func() tea.Msg {
		timelogs, err := m.apiClient.GetTimelogs()
		return enhancedTimelogsLoadedMsg{timelogs: timelogs, err: err}
	}
}

// Message types for async operations
type enhancedProjectsLoadedMsg struct {
	projects []api.ProjectListItem
	err      error
}

type enhancedActivitiesLoadedMsg struct {
	activities []api.ActivityListItem
	err        error
}

type enhancedTasksLoadedMsg struct {
	tasks []api.TaskListItem
	err   error
}

type enhancedTimelogsLoadedMsg struct {
	timelogs []api.TimelogEntry
	err      error
}

// handleMenuKeys handles key events for the menu view
func (m *EnhancedModel) handleMenuKeys(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "1":
		// Start time tracking
		m.currentView = TimesheetEntryView
		m.focusedField = 0
		m.updateFieldFocus()
		return nil
	case "2":
		// View timesheet history
		m.currentView = TimesheetListView
		if m.apiEnabled {
			return m.loadTimelogsCmd()
		}
		return nil
	case "3":
		// Settings
		m.currentView = SettingsView
		return nil
	}
	return nil
}

// handleTimesheetEntryKeys handles key events for the timesheet entry view
func (m *EnhancedModel) handleTimesheetEntryKeys(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "tab", "shift+tab":
		// Navigate between form fields
		if msg.String() == "tab" {
			m.focusedField = (m.focusedField + 1) % m.maxFields
		} else {
			m.focusedField = (m.focusedField - 1 + m.maxFields) % m.maxFields
		}
		m.updateFieldFocus()
		return nil
	case "ctrl+s":
		// Save timesheet entry
		return m.saveTimesheetEntry()
	case "enter":
		// Handle enter key in combo boxes
		switch m.focusedField {
		case 0, 1, 2: // Combo boxes
			return nil // Combo boxes handle their own enter events
		case 3: // Description field - save entry
			return m.saveTimesheetEntry()
		}
	}
	return nil
}

// updateFieldFocus updates which field has focus
func (m *EnhancedModel) updateFieldFocus() {
	// Reset all focus
	m.projectCombo.Blur()
	m.activityCombo.Blur()
	m.taskCombo.Blur()
	m.descriptionInput.Blur()
	
	// Set focus on current field
	switch m.focusedField {
	case 0:
		m.projectCombo.Focus()
	case 1:
		m.activityCombo.Focus()
	case 2:
		m.taskCombo.Focus()
	case 3:
		m.descriptionInput.Focus()
	}
}

// loadTasksForProject loads tasks for a selected project
func (m *EnhancedModel) loadTasksForProject(projectID string) tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		if m.apiClient == nil {
			return ComboBoxErrorMsg{Err: fmt.Errorf("API client not available")}
		}
		
		tasks, err := m.apiClient.GetTasks(projectID)
		if err != nil {
			return ComboBoxErrorMsg{Err: err}
		}
		
		// Convert to combo box items
		items := make([]ComboBoxItem, len(tasks))
		for i, task := range tasks {
			items[i] = ComboBoxItem{
				ID:          fmt.Sprintf("%d", task.ID),
				Label:       task.Label,
				Description: task.Name,
				Value:       task,
			}
		}
		
		return ComboBoxDataMsg{Items: items}
	})
}

// saveTimesheetEntry saves the current timesheet entry
func (m *EnhancedModel) saveTimesheetEntry() tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		// Get selected values
		projectSelected := m.projectCombo.GetSelected()
		activitySelected := m.activityCombo.GetSelected()
		description := m.descriptionInput.Value()
		
		if projectSelected == nil {
			return TimesheetSaveErrorMsg{Err: fmt.Errorf("please select a project")}
		}
		if activitySelected == nil {
			return TimesheetSaveErrorMsg{Err: fmt.Errorf("please select an activity")}
		}
		if description == "" {
			return TimesheetSaveErrorMsg{Err: fmt.Errorf("please enter a description")}
		}
		
		// Create time entry
		entry := models.NewTimeEntry(
			"default-user", // TODO: Get from config
			projectSelected.ID,
			activitySelected.ID,
			nil, // Task is optional
			description,
		)
		
		// Set task if selected
		if taskSelected := m.taskCombo.GetSelected(); taskSelected != nil {
			taskID := taskSelected.ID
			entry.TaskID = &taskID
		}
		
		// Save via API if available
		if m.apiClient != nil {
			err := m.apiClient.CreateTimesheet(*entry)
			if err != nil {
				return TimesheetSaveErrorMsg{Err: err}
			}
		}
		
		// Also save locally via service
		savedEntry, err := m.service.StartTimeEntry(
			entry.ProjectID,
			entry.ActivityID,
			entry.TaskID,
			entry.Description,
		)
		if err != nil {
			return TimesheetSaveErrorMsg{Err: err}
		}
		
		// Update the entry with the saved data
		entry = savedEntry
		
		return TimesheetSaveSuccessMsg{Entry: entry}
	})
}

// TimesheetSaveSuccessMsg is sent when a timesheet entry is saved successfully
type TimesheetSaveSuccessMsg struct {
	Entry *models.TimeEntry
}

// TimesheetSaveErrorMsg is sent when there's an error saving a timesheet entry
type TimesheetSaveErrorMsg struct {
	Err error
}

// timesheetEntryView renders the enhanced timesheet entry form
func (m *EnhancedModel) timesheetEntryView() string {
	title := titleStyle.Render("📝 New Time Entry")
	
	var content strings.Builder
	
	// Project selection
	projectLabel := "Project:"
	if m.focusedField == 0 {
		projectLabel = activeStyle.Render("► Project:")
	}
	content.WriteString(projectLabel + "\n")
	content.WriteString(m.projectCombo.View() + "\n\n")
	
	// Activity selection
	activityLabel := "Activity:"
	if m.focusedField == 1 {
		activityLabel = activeStyle.Render("► Activity:")
	}
	content.WriteString(activityLabel + "\n")
	content.WriteString(m.activityCombo.View() + "\n\n")
	
	// Task selection (optional)
	taskLabel := "Task (optional):"
	if m.focusedField == 2 {
		taskLabel = activeStyle.Render("► Task (optional):")
	}
	content.WriteString(taskLabel + "\n")
	content.WriteString(m.taskCombo.View() + "\n\n")
	
	// Description
	descLabel := "Description:"
	if m.focusedField == 3 {
		descLabel = activeStyle.Render("► Description:")
	}
	content.WriteString(descLabel + "\n")
	
	descStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#DDA036")).
		Padding(0, 1).
		Width(m.width - 20)
	
	if m.focusedField == 3 {
		descStyle = descStyle.BorderForeground(lipgloss.Color("#FF8C42"))
	}
	
	content.WriteString(descStyle.Render(m.descriptionInput.View()) + "\n\n")
	
	// Instructions
	instructions := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9A9EA0")).
		Render("Tab: Next field | Shift+Tab: Previous field | Ctrl+S: Save | Esc: Back")
	
	content.WriteString(instructions)
	
	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		content.String(),
	)
}

// Helper function to format duration
func formatDuration(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60
	
	if hours > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
	}
	return fmt.Sprintf("%02d:%02d", minutes, seconds)
}
