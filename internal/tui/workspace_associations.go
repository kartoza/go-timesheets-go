package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/kartoza/go-timesheets-go/internal/api"
	"github.com/kartoza/go-timesheets-go/internal/config"
	"github.com/kartoza/go-timesheets-go/internal/models"
	"github.com/kartoza/go-timesheets-go/internal/storage"
)

// Use shared brand colors from widgets.go
var (
	wsOrange   = ColorOrange
	wsBlue     = ColorBlue
	wsGray     = ColorGray
	wsWhite    = ColorWhite
	wsDarkGray = ColorDarkGray
	wsRed      = ColorRed
	wsGreen    = ColorGreen
)

// WorkspaceRow represents a single row state
type WorkspaceRow struct {
	nameInput   textinput.Model
	association models.WorkspaceAssociation
}

// EditMode represents what's being edited
type EditMode int

const (
	EditModeNone EditMode = iota
	EditModeName
	EditModeAssociation
)

// EditField represents which field in the edit popup is focused
type EditField int

const (
	EditFieldProject EditField = iota
	EditFieldTask
	EditFieldActivity
)

// WorkspaceAssociationsModel represents the workspace associations screen
type WorkspaceAssociationsModel struct {
	apiClient   *api.Client
	storage     *storage.Storage
	width       int
	height      int
	headerState *HeaderState // Shared header state

	// Data
	associations *models.WorkspaceAssociations
	rows         []WorkspaceRow

	// UI State for main list
	selectedRow    int
	selectedColumn int // 0=name, 1=edit, 2=clear
	editMode       EditMode

	// Edit popup state
	showEditPopup bool
	editingRow    int
	editField     EditField
	popoverCursor int

	// Edit popup - selection data
	projects   []api.ProjectListItem
	tasks      []api.TaskListItem
	activities []api.ActivityListItem

	// Edit popup - selected items (working copy)
	editProject  *api.ProjectListItem
	editTask     *api.TaskListItem
	editActivity *api.ActivityListItem

	// Edit popup - text inputs
	projectSearchInput textinput.Model

	// Edit popup - popovers
	showProjectPopover  bool
	showTaskPopover     bool
	showActivityPopover bool

	// Caching (same pattern as TimesheetCreator)
	projectCache          map[string][]api.ProjectListItem
	projectSearchInFlight map[string]bool
	taskCache             map[int][]api.TaskListItem
	taskCacheInFlight     map[int]bool
	activityCache         []api.ActivityListItem
	activityCacheValid    bool
	activityCacheInFlight bool

	// Error/success messages
	err            error
	successMessage string
}

// NewWorkspaceAssociationsModel creates a new workspace associations model
func NewWorkspaceAssociationsModel(apiClient *api.Client) *WorkspaceAssociationsModel {
	// Initialize storage
	cfg, _ := config.LoadConfig()
	storageDir := cfg.GetStorageDir()
	store, _ := storage.New(storageDir)

	// Load existing associations
	associations, err := store.LoadWorkspaceAssociations()
	if err != nil {
		associations = models.NewWorkspaceAssociations()
	}

	// Create rows with text inputs
	rows := make([]WorkspaceRow, 10)
	for i := 0; i < 10; i++ {
		ti := textinput.New()
		ti.Placeholder = fmt.Sprintf("Workspace %d", i+1)
		ti.CharLimit = 30
		ti.Width = 10

		if associations.Associations[i].WorkspaceName != "" {
			ti.SetValue(associations.Associations[i].WorkspaceName)
		}

		rows[i] = WorkspaceRow{
			nameInput:   ti,
			association: associations.Associations[i],
		}
	}

	// Project search input for popup
	projectSearch := textinput.New()
	projectSearch.Placeholder = "Type to search projects..."
	projectSearch.CharLimit = 100
	projectSearch.Width = 40

	return &WorkspaceAssociationsModel{
		apiClient:             apiClient,
		storage:               store,
		associations:          associations,
		rows:                  rows,
		selectedRow:           0,
		selectedColumn:        0,
		editMode:              EditModeNone,
		projectSearchInput:    projectSearch,
		projectCache:          make(map[string][]api.ProjectListItem),
		projectSearchInFlight: make(map[string]bool),
		taskCache:             make(map[int][]api.TaskListItem),
		taskCacheInFlight:     make(map[int]bool),
		width:                 80,
		height:                24,
	}
}

// Init initializes the workspace associations view
func (m *WorkspaceAssociationsModel) Init() tea.Cmd {
	return m.loadActivities()
}

// Update handles messages
func (m *WorkspaceAssociationsModel) Update(msg tea.Msg) (*WorkspaceAssociationsModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.MouseMsg:
		// Handle mouse clicks on workspace rows
		if msg.Action == tea.MouseActionRelease && msg.Button == tea.MouseButtonLeft {
			if !m.showEditPopup && m.editMode == EditModeNone {
				// Calculate which row was clicked
				row, col := m.getRowColFromMousePosition(msg.X, msg.Y)
				if row >= 0 && row < 10 {
					m.selectedRow = row
					if col >= 0 {
						m.selectedColumn = col
					}
					// Single click selects, handle action based on column
					return m.handleColumnAction()
				}
			}
		}
		return m, nil

	case tea.KeyMsg:
		// Clear messages on key press
		m.err = nil
		m.successMessage = ""

		// Handle edit popup mode
		if m.showEditPopup {
			return m.handleEditPopupKeys(msg)
		}

		// Handle name editing mode
		if m.editMode == EditModeName {
			return m.handleNameEditKeys(msg)
		}

		// Normal navigation
		switch msg.String() {
		case "esc", "q":
			return m, func() tea.Msg { return backToMenuMsg{} }

		case "ctrl+c":
			return m, tea.Quit

		case "up", "k":
			if m.selectedRow > 0 {
				m.selectedRow--
			}
			return m, nil

		case "down", "j":
			if m.selectedRow < 9 {
				m.selectedRow++
			}
			return m, nil

		case "left", "h":
			if m.selectedColumn > 0 {
				m.selectedColumn--
			}
			return m, nil

		case "right", "l":
			if m.selectedColumn < 2 {
				m.selectedColumn++
			}
			return m, nil

		case "tab":
			m.selectedColumn = (m.selectedColumn + 1) % 3
			return m, nil

		case "shift+tab":
			m.selectedColumn = (m.selectedColumn + 2) % 3
			return m, nil

		case "enter", " ":
			return m.handleSelect()
		}

	// Message handlers for API responses
	case wsProjectsLoadedMsg:
		if msg.CacheKey != "" {
			delete(m.projectSearchInFlight, msg.CacheKey)
			if len(msg.AllProjects) > 0 {
				m.projectCache[msg.CacheKey] = msg.AllProjects
			}
		}
		m.projects = msg.Projects
		m.popoverCursor = 0
		return m, nil

	case wsTasksLoadedMsg:
		tasks := msg.tasks
		if m.editProject != nil {
			m.taskCache[m.editProject.ID] = tasks
			delete(m.taskCacheInFlight, m.editProject.ID)
		}
		m.tasks = tasks
		m.popoverCursor = 0
		return m, nil

	case wsActivitiesLoadedMsg:
		m.activityCache = msg.activities
		m.activityCacheValid = true
		m.activityCacheInFlight = false
		m.activities = msg.activities
		return m, nil

	case wsSaveSuccessMsg:
		m.successMessage = "Workspace associations saved!"
		return m, nil

	case wsSaveErrorMsg:
		m.err = msg.err
		return m, nil
	}

	return m, tea.Batch(cmds...)
}

// handleNameEditKeys handles key events when editing a workspace name
func (m *WorkspaceAssociationsModel) handleNameEditKeys(msg tea.KeyMsg) (*WorkspaceAssociationsModel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.editMode = EditModeNone
		m.rows[m.selectedRow].nameInput.Blur()
		return m, nil

	case "enter":
		// Save the name
		m.editMode = EditModeNone
		m.rows[m.selectedRow].nameInput.Blur()
		m.associations.Associations[m.selectedRow].WorkspaceName = m.rows[m.selectedRow].nameInput.Value()
		return m, m.saveAssociations()

	default:
		var cmd tea.Cmd
		m.rows[m.selectedRow].nameInput, cmd = m.rows[m.selectedRow].nameInput.Update(msg)
		return m, cmd
	}
}

// handleEditPopupKeys handles key events in the edit popup
func (m *WorkspaceAssociationsModel) handleEditPopupKeys(msg tea.KeyMsg) (*WorkspaceAssociationsModel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		// Close popovers first, then close popup
		if m.showProjectPopover || m.showTaskPopover || m.showActivityPopover {
			m.closeAllPopovers()
			return m, nil
		}
		m.showEditPopup = false
		m.resetEditState()
		return m, nil

	case "ctrl+c":
		return m, tea.Quit

	case "tab":
		// Move to next field
		m.moveToNextEditField()
		return m, nil

	case "shift+tab":
		// Move to previous field
		m.moveToPreviousEditField()
		return m, nil

	case "up":
		if m.showProjectPopover || m.showTaskPopover || m.showActivityPopover {
			if m.popoverCursor > 0 {
				m.popoverCursor--
			}
			return m, nil
		}

	case "down":
		if m.showProjectPopover {
			if m.popoverCursor < len(m.projects)-1 {
				m.popoverCursor++
			}
			return m, nil
		}
		if m.showTaskPopover {
			if m.popoverCursor < len(m.tasks)-1 {
				m.popoverCursor++
			}
			return m, nil
		}
		if m.showActivityPopover {
			if m.popoverCursor < len(m.activities)-1 {
				m.popoverCursor++
			}
			return m, nil
		}

	case "enter":
		return m.handleEditPopupEnter()

	default:
		// Handle text input for project search
		if m.editField == EditFieldProject && m.showProjectPopover {
			var cmd tea.Cmd
			m.projectSearchInput, cmd = m.projectSearchInput.Update(msg)
			query := m.projectSearchInput.Value()
			if len(query) >= 2 {
				return m, tea.Batch(cmd, m.searchProjects(query))
			} else if len(query) == 0 {
				m.projects = nil
			}
			return m, cmd
		}
	}

	return m, nil
}

// handleEditPopupEnter handles enter key in the edit popup
func (m *WorkspaceAssociationsModel) handleEditPopupEnter() (*WorkspaceAssociationsModel, tea.Cmd) {
	// Handle popover selection
	if m.showProjectPopover && len(m.projects) > 0 {
		m.editProject = &m.projects[m.popoverCursor]
		m.editTask = nil // Clear task when project changes
		m.showProjectPopover = false
		m.popoverCursor = 0
		// Auto-advance to task selection
		m.editField = EditFieldTask
		m.showTaskPopover = true
		return m, m.loadTasks(m.editProject.ID)
	}

	if m.showTaskPopover && len(m.tasks) > 0 {
		m.editTask = &m.tasks[m.popoverCursor]
		m.showTaskPopover = false
		m.popoverCursor = 0
		// Auto-advance to activity selection
		m.editField = EditFieldActivity
		m.showActivityPopover = true
		return m, nil
	}

	if m.showActivityPopover && len(m.activities) > 0 {
		m.editActivity = &m.activities[m.popoverCursor]
		m.showActivityPopover = false
		m.popoverCursor = 0
		// Save and close
		return m.saveEditPopup()
	}

	// If no popover open, open the appropriate one
	switch m.editField {
	case EditFieldProject:
		if m.editProject == nil {
			m.showProjectPopover = true
			m.projectSearchInput.Focus()
			return m, textinput.Blink
		}
	case EditFieldTask:
		if m.editProject != nil && m.editTask == nil {
			m.showTaskPopover = true
		}
	case EditFieldActivity:
		if m.editActivity == nil {
			m.showActivityPopover = true
		}
	}

	return m, nil
}

// moveToNextEditField moves to the next field in the edit popup
func (m *WorkspaceAssociationsModel) moveToNextEditField() {
	m.closeAllPopovers()

	switch m.editField {
	case EditFieldProject:
		m.editField = EditFieldTask
		if m.editProject != nil && m.editTask == nil {
			m.showTaskPopover = true
			m.popoverCursor = 0
		}
	case EditFieldTask:
		m.editField = EditFieldActivity
		if m.editActivity == nil {
			m.showActivityPopover = true
			m.popoverCursor = 0
		}
	case EditFieldActivity:
		m.editField = EditFieldProject
		if m.editProject == nil {
			m.showProjectPopover = true
			m.projectSearchInput.Focus()
		}
	}
}

// moveToPreviousEditField moves to the previous field in the edit popup
func (m *WorkspaceAssociationsModel) moveToPreviousEditField() {
	m.closeAllPopovers()

	switch m.editField {
	case EditFieldProject:
		m.editField = EditFieldActivity
		if m.editActivity == nil {
			m.showActivityPopover = true
			m.popoverCursor = 0
		}
	case EditFieldTask:
		m.editField = EditFieldProject
		if m.editProject == nil {
			m.showProjectPopover = true
			m.projectSearchInput.Focus()
		}
	case EditFieldActivity:
		m.editField = EditFieldTask
		if m.editProject != nil && m.editTask == nil {
			m.showTaskPopover = true
			m.popoverCursor = 0
		}
	}
}

// closeAllPopovers closes all popovers in the edit popup
func (m *WorkspaceAssociationsModel) closeAllPopovers() {
	m.showProjectPopover = false
	m.showTaskPopover = false
	m.showActivityPopover = false
	m.projectSearchInput.Blur()
}

// resetEditState resets the edit popup state
func (m *WorkspaceAssociationsModel) resetEditState() {
	m.editProject = nil
	m.editTask = nil
	m.editActivity = nil
	m.editField = EditFieldProject
	m.popoverCursor = 0
	m.projects = nil
	m.tasks = nil
	m.closeAllPopovers()
	m.projectSearchInput.SetValue("")
}

// saveEditPopup saves the current edit popup selections to the workspace
func (m *WorkspaceAssociationsModel) saveEditPopup() (*WorkspaceAssociationsModel, tea.Cmd) {
	if m.editProject == nil {
		m.err = fmt.Errorf("please select a project")
		return m, nil
	}

	assoc := &m.associations.Associations[m.editingRow]
	assoc.ProjectID = m.editProject.ID
	assoc.ProjectName = m.editProject.Label

	if m.editTask != nil {
		assoc.TaskID = m.editTask.ID
		assoc.TaskName = m.editTask.Name
	} else {
		assoc.TaskID = 0
		assoc.TaskName = ""
	}

	if m.editActivity != nil {
		assoc.ActivityID = m.editActivity.ID
		assoc.ActivityName = m.editActivity.Label
	} else {
		assoc.ActivityID = 0
		assoc.ActivityName = ""
	}

	m.rows[m.editingRow].association = *assoc
	m.showEditPopup = false
	m.resetEditState()

	return m, m.saveAssociations()
}

// handleSelect handles the enter/space key on the current selection
func (m *WorkspaceAssociationsModel) handleSelect() (*WorkspaceAssociationsModel, tea.Cmd) {
	switch m.selectedColumn {
	case 0: // Name column - start editing
		m.editMode = EditModeName
		m.rows[m.selectedRow].nameInput.Focus()
		return m, textinput.Blink

	case 1: // Edit button - open edit popup
		m.showEditPopup = true
		m.editingRow = m.selectedRow
		m.editField = EditFieldProject
		m.showProjectPopover = true
		m.projectSearchInput.Focus()

		// Pre-populate with existing values if any
		assoc := m.associations.Associations[m.selectedRow]
		if assoc.ProjectID > 0 {
			m.editProject = &api.ProjectListItem{ID: assoc.ProjectID, Label: assoc.ProjectName}
		}
		if assoc.TaskID > 0 {
			m.editTask = &api.TaskListItem{ID: assoc.TaskID, Name: assoc.TaskName, Label: assoc.TaskName}
		}
		if assoc.ActivityID > 0 {
			m.editActivity = &api.ActivityListItem{ID: assoc.ActivityID, Label: assoc.ActivityName}
		}

		return m, textinput.Blink

	case 2: // Clear button
		m.associations.ClearAssociation(m.selectedRow + 1)
		m.rows[m.selectedRow].association = m.associations.Associations[m.selectedRow]
		return m, m.saveAssociations()
	}

	return m, nil
}

// View renders the workspace associations screen
func (m *WorkspaceAssociationsModel) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	// Render the workspace list (background)
	listView := m.renderWorkspaceList()

	// If edit popup is showing, overlay it
	if m.showEditPopup {
		return m.renderWithModalOverlay(listView)
	}

	// Header
	header := m.renderHeader()

	// Help text
	help := m.renderHelp()

	// Status messages
	var statusMsg string
	if m.err != nil {
		statusStyle := lipgloss.NewStyle().Foreground(wsRed).Bold(true)
		statusMsg = statusStyle.Render(fmt.Sprintf("Error: %v", m.err))
	} else if m.successMessage != "" {
		statusStyle := lipgloss.NewStyle().Foreground(wsGreen).Bold(true)
		statusMsg = statusStyle.Render(m.successMessage)
	}

	// Combine all parts
	mainSection := lipgloss.JoinVertical(
		lipgloss.Center,
		header,
		"",
		listView,
	)

	if statusMsg != "" {
		mainSection = lipgloss.JoinVertical(
			lipgloss.Center,
			mainSection,
			"",
			statusMsg,
		)
	}

	// Center in viewport
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

// renderWithModalOverlay renders the view with a centered modal popup
func (m *WorkspaceAssociationsModel) renderWithModalOverlay(background string) string {
	// Dim the background
	dimmedBg := lipgloss.NewStyle().Foreground(lipgloss.Color("#555555")).Render(background)

	// Render the edit modal
	modal := m.renderEditModal()

	// Center the modal on screen
	modalPlaced := lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		modal,
	)

	// The modal takes precedence - we just show it centered
	_ = dimmedBg
	return modalPlaced
}

// renderEditModal renders the edit popup modal (same style as TimesheetCreator)
func (m *WorkspaceAssociationsModel) renderEditModal() string {
	assoc := m.associations.Associations[m.editingRow]

	// Styles matching TimesheetCreator
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(wsOrange).
		Align(lipgloss.Center)

	subtitleStyle := lipgloss.NewStyle().
		Foreground(wsGray).
		Italic(true).
		Align(lipgloss.Center)

	containerStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(wsOrange).
		Padding(1, 2).
		Width(70)

	labelWidth := 12
	fieldWidth := 50

	labelStyle := lipgloss.NewStyle().
		Width(labelWidth).
		Align(lipgloss.Right).
		Foreground(wsGray)

	focusedLabelStyle := lipgloss.NewStyle().
		Width(labelWidth).
		Align(lipgloss.Right).
		Foreground(wsOrange).
		Bold(true)

	inputStyle := lipgloss.NewStyle().
		Width(fieldWidth)

	selectedValueStyle := lipgloss.NewStyle().
		Foreground(wsBlue).
		Bold(true)

	unselectedValueStyle := lipgloss.NewStyle().
		Foreground(wsGray).
		Italic(true)

	// Title
	workspaceName := assoc.WorkspaceName
	if workspaceName == "" {
		workspaceName = fmt.Sprintf("Workspace %d", m.editingRow+1)
	}
	title := titleStyle.Render(fmt.Sprintf("Edit Association: %s", workspaceName))
	subtitle := subtitleStyle.Render("Select project, task, and activity")

	// Build form rows
	var rows []string

	// Row 1: Project
	projectLabel := labelStyle.Render("Project:")
	if m.editField == EditFieldProject {
		projectLabel = focusedLabelStyle.Render("Project:")
	}
	var projectValue string
	if m.editProject != nil {
		projectValue = selectedValueStyle.Render("✓ " + m.editProject.Label)
	} else if m.showProjectPopover {
		projectValue = m.projectSearchInput.View()
	} else {
		projectValue = unselectedValueStyle.Render("Click to select...")
	}
	projectRow := lipgloss.JoinHorizontal(lipgloss.Top, projectLabel, "  ", inputStyle.Render(projectValue))
	rows = append(rows, projectRow)

	// Project popover
	if m.showProjectPopover && len(m.projects) > 0 {
		popover := m.renderPopover(m.projects, func(i int) string { return m.projects[i].Label })
		rows = append(rows, popover)
	}

	// Row 2: Task
	taskLabel := labelStyle.Render("Task:")
	if m.editField == EditFieldTask {
		taskLabel = focusedLabelStyle.Render("Task:")
	}
	var taskValue string
	if m.editTask != nil {
		taskValue = selectedValueStyle.Render("✓ " + m.editTask.Label)
	} else if m.editProject == nil {
		taskValue = unselectedValueStyle.Render("Select project first")
	} else if m.showTaskPopover {
		taskValue = unselectedValueStyle.Render("↓ Select from list...")
	} else {
		taskValue = unselectedValueStyle.Render("Click to select (optional)...")
	}
	taskRow := lipgloss.JoinHorizontal(lipgloss.Top, taskLabel, "  ", inputStyle.Render(taskValue))
	rows = append(rows, taskRow)

	// Task popover
	if m.showTaskPopover && len(m.tasks) > 0 {
		popover := m.renderPopover(m.tasks, func(i int) string { return m.tasks[i].Label })
		rows = append(rows, popover)
	}

	// Row 3: Activity
	activityLabel := labelStyle.Render("Activity:")
	if m.editField == EditFieldActivity {
		activityLabel = focusedLabelStyle.Render("Activity:")
	}
	var activityValue string
	if m.editActivity != nil {
		activityValue = selectedValueStyle.Render("✓ " + m.editActivity.Label)
	} else if m.showActivityPopover {
		activityValue = unselectedValueStyle.Render("↓ Select from list...")
	} else {
		activityValue = unselectedValueStyle.Render("Click to select...")
	}
	activityRow := lipgloss.JoinHorizontal(lipgloss.Top, activityLabel, "  ", inputStyle.Render(activityValue))
	rows = append(rows, activityRow)

	// Activity popover
	if m.showActivityPopover && len(m.activities) > 0 {
		popover := m.renderPopover(m.activities, func(i int) string { return m.activities[i].Label })
		rows = append(rows, popover)
	}

	// Help text for popup
	helpStyle := lipgloss.NewStyle().
		Foreground(wsGray).
		Italic(true).
		Align(lipgloss.Center)

	var helpText string
	if m.showProjectPopover || m.showTaskPopover || m.showActivityPopover {
		helpText = "↑/↓: Navigate • Enter: Select • Esc: Close • Tab: Next field"
	} else {
		helpText = "Tab: Next field • Enter: Open selection • Esc: Cancel"
	}

	formContent := lipgloss.JoinVertical(lipgloss.Left, rows...)

	content := lipgloss.JoinVertical(
		lipgloss.Center,
		title,
		subtitle,
		"",
		containerStyle.Render(formContent),
		"",
		helpStyle.Render(helpText),
	)

	return content
}

// renderPopover renders a pop-over selection list (same as TimesheetCreator)
func (m *WorkspaceAssociationsModel) renderPopover(items interface{}, getLabel func(int) string) string {
	popoverStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(wsOrange).
		Padding(0, 1).
		MarginLeft(14).
		Width(50)

	selectedStyle := lipgloss.NewStyle().
		Background(wsOrange).
		Foreground(wsWhite).
		Bold(true).
		Width(46)

	normalStyle := lipgloss.NewStyle().
		Foreground(wsWhite).
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
		startIdx = m.popoverCursor - maxVisible/2
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

		if i == m.popoverCursor {
			rows = append(rows, selectedStyle.Render("▶ "+label))
		} else {
			rows = append(rows, normalStyle.Render("  "+label))
		}
	}

	// Add scroll indicator
	if itemCount > maxVisible {
		scrollInfo := lipgloss.NewStyle().
			Foreground(wsGray).
			Italic(true).
			Render(fmt.Sprintf("  ↑↓ %d of %d", m.popoverCursor+1, itemCount))
		rows = append(rows, scrollInfo)
	}

	return popoverStyle.Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
}

// renderHeader renders the consistent header (same as main menu)
func (m *WorkspaceAssociationsModel) renderHeader() string {
	if m.headerState != nil {
		return RenderHeader("Workspace Associations", m.headerState)
	}
	// Fallback if header state not set
	return RenderHeader("Workspace Associations", &HeaderState{})
}

// SetHeaderState sets the shared header state
func (m *WorkspaceAssociationsModel) SetHeaderState(state *HeaderState) {
	m.headerState = state
}

// getRowColFromMousePosition calculates which row and column was clicked based on mouse coordinates
// Returns row (-1 if outside), col (-1 if outside columns)
func (m *WorkspaceAssociationsModel) getRowColFromMousePosition(x, y int) (int, int) {
	// Header takes about 5 lines, column header takes 1 line
	tableHeaderOffset := 6

	// Calculate clicked row
	clickedRow := y - tableHeaderOffset
	if clickedRow < 0 || clickedRow >= 10 {
		return -1, -1
	}

	// Calculate column based on X position
	// Column widths: # (3) + Name (14) + Association (28) + Clear (10)
	// Centered, so we need to find the start position
	tableWidth := 60
	tableStartX := (m.width - tableWidth) / 2

	relX := x - tableStartX
	if relX < 0 || relX > tableWidth {
		return clickedRow, -1
	}

	// Determine column
	col := -1
	if relX < 3 {
		col = -1 // Number column, not interactive
	} else if relX < 17 {
		col = 0 // Name column
	} else if relX < 45 {
		col = 1 // Association column
	} else {
		col = 2 // Clear column
	}

	return clickedRow, col
}

// handleColumnAction handles action when a column is selected (via keyboard or mouse)
func (m *WorkspaceAssociationsModel) handleColumnAction() (*WorkspaceAssociationsModel, tea.Cmd) {
	switch m.selectedColumn {
	case 0:
		// Edit name
		m.editMode = EditModeName
		m.rows[m.selectedRow].nameInput.Focus()
		return m, textinput.Blink
	case 1:
		// Edit association - open popup
		m.showEditPopup = true
		m.editingRow = m.selectedRow
		m.editField = EditFieldProject

		// Pre-populate with existing values
		assoc := m.associations.Associations[m.selectedRow]
		if assoc.ProjectID > 0 {
			m.editProject = &api.ProjectListItem{ID: assoc.ProjectID, Label: assoc.ProjectName}
		} else {
			m.editProject = nil
		}
		if assoc.TaskID > 0 {
			m.editTask = &api.TaskListItem{ID: assoc.TaskID, Name: assoc.TaskName, Label: assoc.TaskName}
		} else {
			m.editTask = nil
		}
		if assoc.ActivityID > 0 {
			m.editActivity = &api.ActivityListItem{ID: assoc.ActivityID, Label: assoc.ActivityName}
		} else {
			m.editActivity = nil
		}

		m.showProjectPopover = true
		m.projectSearchInput.Focus()
		return m, nil
	case 2:
		// Clear association
		m.associations.ClearAssociation(m.selectedRow + 1)
		return m, m.saveAssociations()
	}
	return m, nil
}

// renderWorkspaceList renders the list of workspace rows
func (m *WorkspaceAssociationsModel) renderWorkspaceList() string {
	// Column headers - total width ~60 to match header
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(wsOrange).
		Width(60)

	numStyle := lipgloss.NewStyle().Width(3).Align(lipgloss.Center)
	nameHeaderStyle := lipgloss.NewStyle().Width(14).Align(lipgloss.Left)
	assocHeaderStyle := lipgloss.NewStyle().Width(28).Align(lipgloss.Left)
	actionHeaderStyle := lipgloss.NewStyle().Width(15).Align(lipgloss.Center)

	headerRow := lipgloss.JoinHorizontal(
		lipgloss.Top,
		numStyle.Render("#"),
		nameHeaderStyle.Render("Name"),
		assocHeaderStyle.Render("Project / Task"),
		actionHeaderStyle.Render("Actions"),
	)
	header := headerStyle.Render(headerRow)

	// Render each row
	var rows []string
	rows = append(rows, header)
	rows = append(rows, lipgloss.NewStyle().Foreground(wsGray).Render(strings.Repeat("─", 60)))

	for i := 0; i < 10; i++ {
		rows = append(rows, m.renderRow(i))
	}

	return lipgloss.JoinVertical(lipgloss.Center, rows...)
}

// renderRow renders a single workspace row
func (m *WorkspaceAssociationsModel) renderRow(index int) string {
	isSelected := index == m.selectedRow && !m.showEditPopup
	assoc := m.associations.Associations[index]

	// Base styles - widths match header (3+14+28+15 = 60)
	numStyle := lipgloss.NewStyle().Width(3).Align(lipgloss.Center)
	if isSelected {
		numStyle = numStyle.Foreground(wsOrange).Bold(true)
	} else {
		numStyle = numStyle.Foreground(wsGray)
	}

	// Workspace number
	num := numStyle.Render(fmt.Sprintf("%d", index+1))

	// Name field (editable box)
	nameBoxStyle := lipgloss.NewStyle().
		Width(12).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(wsGray).
		Padding(0, 0)

	if isSelected && m.selectedColumn == 0 {
		nameBoxStyle = nameBoxStyle.BorderForeground(wsOrange)
	}
	if m.editMode == EditModeName && index == m.selectedRow {
		nameBoxStyle = nameBoxStyle.BorderForeground(wsBlue)
	}

	var nameContent string
	if m.editMode == EditModeName && index == m.selectedRow {
		nameContent = m.rows[index].nameInput.View()
	} else {
		name := assoc.WorkspaceName
		if name == "" {
			name = fmt.Sprintf("WS %d", index+1)
			nameContent = lipgloss.NewStyle().Foreground(wsGray).Italic(true).Render(name)
		} else {
			if len(name) > 10 {
				name = name[:9] + "…"
			}
			nameContent = lipgloss.NewStyle().Foreground(wsWhite).Render(name)
		}
	}
	nameBox := nameBoxStyle.Render(nameContent)

	// Association display
	assocStyle := lipgloss.NewStyle().Width(28).Align(lipgloss.Left)
	var assocContent string
	if assoc.HasAssociation() {
		assocText := assoc.GetDisplayString()
		if len(assocText) > 26 {
			assocText = assocText[:24] + "…"
		}
		assocContent = assocStyle.Foreground(wsBlue).Render(assocText)
	} else {
		assocContent = assocStyle.Foreground(wsGray).Italic(true).Render("Not set")
	}

	// Edit button
	editBtnStyle := lipgloss.NewStyle().
		Padding(0, 1).
		Margin(0, 1)

	if isSelected && m.selectedColumn == 1 {
		editBtnStyle = editBtnStyle.
			Background(wsOrange).
			Foreground(wsWhite).
			Bold(true)
	} else {
		editBtnStyle = editBtnStyle.
			Background(wsDarkGray).
			Foreground(wsWhite)
	}
	editBtn := editBtnStyle.Render("Edit")

	// Clear button
	clearBtnStyle := lipgloss.NewStyle().
		Padding(0, 1).
		Margin(0, 1)

	if isSelected && m.selectedColumn == 2 {
		clearBtnStyle = clearBtnStyle.
			Background(wsRed).
			Foreground(wsWhite).
			Bold(true)
	} else {
		clearBtnStyle = clearBtnStyle.
			Background(wsDarkGray).
			Foreground(wsWhite)
	}
	clearBtn := clearBtnStyle.Render("Clear")

	// Combine row elements
	return lipgloss.JoinHorizontal(
		lipgloss.Center,
		num,
		nameBox,
		assocContent,
		editBtn,
		clearBtn,
	)
}

// renderHelp renders the help text
func (m *WorkspaceAssociationsModel) renderHelp() string {
	helpStyle := lipgloss.NewStyle().
		Foreground(wsGray).
		Italic(true)

	if m.editMode == EditModeName {
		return helpStyle.Render("Type to edit name | Enter: Save | Esc: Cancel")
	}

	return helpStyle.Render("↑/↓: Navigate rows | ←/→: Navigate columns | Enter/Space: Select | q/Esc: Back to menu")
}

// API loading commands (same pattern as TimesheetCreator)
func (m *WorkspaceAssociationsModel) searchProjects(query string) tea.Cmd {
	cacheKey := strings.ToLower(query)
	if len(cacheKey) > 2 {
		cacheKey = cacheKey[:2]
	}

	// Check cache
	if cached, ok := m.projectCache[cacheKey]; ok {
		filtered := m.filterProjectsLocally(cached, query)
		return func() tea.Msg {
			return wsProjectsLoadedMsg{
				Projects: filtered,
				CacheKey: cacheKey,
			}
		}
	}

	// Check in-flight
	if m.projectSearchInFlight[cacheKey] {
		return nil
	}

	m.projectSearchInFlight[cacheKey] = true

	return func() tea.Msg {
		if m.apiClient == nil {
			return wsSaveErrorMsg{err: fmt.Errorf("API client not available")}
		}

		projects, err := m.apiClient.GetProjects(cacheKey)
		if err != nil {
			return wsSaveErrorMsg{err: err}
		}

		queryLower := strings.ToLower(query)
		var filtered []api.ProjectListItem
		for _, p := range projects {
			if strings.Contains(strings.ToLower(p.Label), queryLower) {
				filtered = append(filtered, p)
			}
		}

		return wsProjectsLoadedMsg{
			Projects:    filtered,
			CacheKey:    cacheKey,
			AllProjects: projects,
		}
	}
}

func (m *WorkspaceAssociationsModel) filterProjectsLocally(projects []api.ProjectListItem, query string) []api.ProjectListItem {
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

func (m *WorkspaceAssociationsModel) loadTasks(projectID int) tea.Cmd {
	// Check cache
	if cached, ok := m.taskCache[projectID]; ok {
		return func() tea.Msg {
			return wsTasksLoadedMsg{tasks: cached}
		}
	}

	// Check in-flight
	if m.taskCacheInFlight[projectID] {
		return nil
	}

	m.taskCacheInFlight[projectID] = true

	return func() tea.Msg {
		if m.apiClient == nil {
			return wsSaveErrorMsg{err: fmt.Errorf("API client not available")}
		}

		tasks, err := m.apiClient.GetTasks(fmt.Sprintf("%d", projectID))
		if err != nil {
			return wsSaveErrorMsg{err: err}
		}

		return wsTasksLoadedMsg{tasks: tasks}
	}
}

func (m *WorkspaceAssociationsModel) loadActivities() tea.Cmd {
	if m.activityCacheValid {
		return func() tea.Msg {
			return wsActivitiesLoadedMsg{activities: m.activityCache}
		}
	}

	if m.activityCacheInFlight {
		return nil
	}

	m.activityCacheInFlight = true

	return func() tea.Msg {
		if m.apiClient == nil {
			return wsSaveErrorMsg{err: fmt.Errorf("API client not available")}
		}

		activities, err := m.apiClient.GetActivities()
		if err != nil {
			return wsSaveErrorMsg{err: err}
		}

		return wsActivitiesLoadedMsg{activities: activities}
	}
}

func (m *WorkspaceAssociationsModel) saveAssociations() tea.Cmd {
	return func() tea.Msg {
		if m.storage == nil {
			return wsSaveErrorMsg{err: fmt.Errorf("storage not initialized")}
		}
		err := m.storage.SaveWorkspaceAssociations(m.associations)
		if err != nil {
			return wsSaveErrorMsg{err: err}
		}
		return wsSaveSuccessMsg{}
	}
}

// Message types
type wsProjectsLoadedMsg struct {
	Projects    []api.ProjectListItem
	CacheKey    string
	AllProjects []api.ProjectListItem
}

type wsTasksLoadedMsg struct {
	tasks []api.TaskListItem
}

type wsActivitiesLoadedMsg struct {
	activities []api.ActivityListItem
}

type wsSaveSuccessMsg struct{}

type wsSaveErrorMsg struct {
	err error
}
