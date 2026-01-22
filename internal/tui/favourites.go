package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/kartoza/go-timesheets-go/internal/api"
	"github.com/kartoza/go-timesheets-go/internal/config"
	"github.com/kartoza/go-timesheets-go/internal/models"
	"github.com/kartoza/go-timesheets-go/internal/pow"
	"github.com/kartoza/go-timesheets-go/internal/storage"
)

// Use shared brand colors from widgets.go
var (
	favOrange   = ColorOrange
	favBlue     = ColorBlue
	favGray     = ColorGray
	favWhite    = ColorWhite
	favDarkGray = ColorDarkGray
	favRed      = ColorRed
	favGreen    = ColorGreen
)

// FavEditMode represents what mode the favourites screen is in
type FavEditMode int

const (
	FavModeSelect FavEditMode = iota // Normal mode - click to start timer
	FavModeEdit                      // Edit mode - configure associations
)

// FavEditField represents which field in the edit popup is focused
type FavEditField int

const (
	FavEditFieldName FavEditField = iota
	FavEditFieldProject
	FavEditFieldTask
	FavEditFieldActivity
)

// FavouritesModel represents the favourites screen with 3x3 grid
type FavouritesModel struct {
	apiClient   *api.Client
	storage     *storage.Storage
	width       int
	height      int
	headerState *HeaderState // Shared header state

	// Data
	associations *models.FavouriteAssociations

	// UI State
	selectedSlot int // 0-8 for the 3x3 grid
	mode         FavEditMode

	// Edit popup state
	showEditPopup bool
	editingSlot   int
	editField     FavEditField
	popoverCursor int

	// Edit popup - selection data
	projects   []api.ProjectListItem
	tasks      []api.TaskListItem
	activities []api.ActivityListItem

	// Edit popup - selected items (working copy)
	editName     string
	editProject  *api.ProjectListItem
	editTask     *api.TaskListItem
	editActivity *api.ActivityListItem

	// Edit popup - text inputs
	nameInput          textinput.Model
	projectSearchInput textinput.Model

	// Edit popup - popovers
	showProjectPopover  bool
	showTaskPopover     bool
	showActivityPopover bool

	// Caching (same pattern as WorkspaceAssociations)
	projectCache          map[string][]api.ProjectListItem
	projectSearchInFlight map[string]bool
	taskCache             map[int][]api.TaskListItem
	taskCacheInFlight     map[int]bool
	activityCache         []api.ActivityListItem
	activityCacheValid    bool
	activityCacheInFlight bool

	// Timer state for quick-start
	activeTimer *api.TimelogEntry

	// POW (Proof of Work) capturer
	powCapturer *pow.Capturer

	// Error/success messages
	err            error
	successMessage string
	statusMessage  string
}

// NewFavouritesModel creates a new favourites model
// powCapturer can be nil if POW mode is not enabled
func NewFavouritesModel(apiClient *api.Client, powCapturer *pow.Capturer) *FavouritesModel {
	// Initialize storage
	cfg, _ := config.LoadConfig()
	storageDir := cfg.GetStorageDir()
	store, _ := storage.New(storageDir)

	// Load existing associations
	associations, err := store.LoadFavouriteAssociations()
	if err != nil {
		associations = models.NewFavouriteAssociations()
	}

	// Name input for popup
	nameInput := textinput.New()
	nameInput.Placeholder = "Enter name..."
	nameInput.CharLimit = 20
	nameInput.Width = 30

	// Project search input for popup
	projectSearch := textinput.New()
	projectSearch.Placeholder = "Type to search projects..."
	projectSearch.CharLimit = 100
	projectSearch.Width = 40

	return &FavouritesModel{
		apiClient:             apiClient,
		storage:               store,
		associations:          associations,
		selectedSlot:          0,
		mode:                  FavModeSelect,
		nameInput:             nameInput,
		projectSearchInput:    projectSearch,
		projectCache:          make(map[string][]api.ProjectListItem),
		projectSearchInFlight: make(map[string]bool),
		taskCache:             make(map[int][]api.TaskListItem),
		taskCacheInFlight:     make(map[int]bool),
		powCapturer:           powCapturer,
		width:                 80,
		height:                24,
	}
}

// Init initializes the favourites view
func (m *FavouritesModel) Init() tea.Cmd {
	return tea.Batch(
		m.loadActivities(),
		m.loadActiveTimer(),
	)
}

// Update handles messages
func (m *FavouritesModel) Update(msg tea.Msg) (*FavouritesModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		// Clear messages on key press
		m.err = nil
		m.successMessage = ""

		// Handle edit popup mode
		if m.showEditPopup {
			return m.handleEditPopupKeys(msg)
		}

		// Normal navigation
		switch msg.String() {
		case "esc", "q":
			return m, func() tea.Msg { return backToMenuMsg{} }

		case "ctrl+c":
			return m, tea.Quit

		case "e":
			// Toggle edit mode
			if m.mode == FavModeSelect {
				m.mode = FavModeEdit
				m.statusMessage = "Edit mode: Press Enter to configure, or e to exit edit mode"
			} else {
				m.mode = FavModeSelect
				m.statusMessage = ""
			}
			return m, nil

		case "p":
			// Toggle POW mode
			return m, m.togglePowMode()

		case "up", "k":
			if m.selectedSlot >= 3 {
				m.selectedSlot -= 3
			}
			return m, nil

		case "down", "j":
			if m.selectedSlot < 6 {
				m.selectedSlot += 3
			}
			return m, nil

		case "left", "h":
			if m.selectedSlot%3 > 0 {
				m.selectedSlot--
			}
			return m, nil

		case "right", "l":
			if m.selectedSlot%3 < 2 {
				m.selectedSlot++
			}
			return m, nil

		case "1", "2", "3", "4", "5", "6", "7", "8", "9":
			// Quick select slot by number
			slotNum := int(msg.String()[0] - '0')
			m.selectedSlot = slotNum - 1
			return m.handleSelect()

		case "enter", " ":
			return m.handleSelect()
		}

	// Message handlers for API responses
	case favProjectsLoadedMsg:
		if msg.CacheKey != "" {
			delete(m.projectSearchInFlight, msg.CacheKey)
			if len(msg.AllProjects) > 0 {
				m.projectCache[msg.CacheKey] = msg.AllProjects
			}
		}
		m.projects = msg.Projects
		m.popoverCursor = 0
		return m, nil

	case favTasksLoadedMsg:
		tasks := msg.tasks
		if m.editProject != nil {
			m.taskCache[m.editProject.ID] = tasks
			delete(m.taskCacheInFlight, m.editProject.ID)
		}
		m.tasks = tasks
		m.popoverCursor = 0
		return m, nil

	case favActivitiesLoadedMsg:
		m.activityCache = msg.activities
		m.activityCacheValid = true
		m.activityCacheInFlight = false
		m.activities = msg.activities
		return m, nil

	case favActiveTimerLoadedMsg:
		m.activeTimer = msg.timer
		return m, nil

	case favSaveSuccessMsg:
		m.successMessage = "Favourite saved!"
		return m, nil

	case favSaveErrorMsg:
		m.err = msg.err
		return m, nil

	case favTimerStartedMsg:
		m.successMessage = fmt.Sprintf("Timer started: %s", msg.projectName)
		m.activeTimer = msg.timer
		return m, nil

	case favTimerStoppedMsg:
		if msg.description != "" {
			m.statusMessage = "Previous timer stopped with git commits"
		} else {
			m.statusMessage = "Previous timer stopped"
		}
		m.activeTimer = nil
		// Now start the new timer
		return m, m.startTimerForSlot(msg.nextSlot)

	case favTimerErrorMsg:
		m.err = msg.err
		return m, nil

	case favPowToggledMsg:
		m.err = nil
		m.statusMessage = ""
		if !msg.ok {
			m.err = fmt.Errorf("POW mode unavailable - missing screenshot tool or ffmpeg")
		} else if msg.enabled {
			m.statusMessage = "POW mode enabled - screenshots will be captured during timers"
		} else {
			m.statusMessage = "POW mode disabled"
		}
		return m, nil
	}

	return m, tea.Batch(cmds...)
}

// handleSelect handles the enter/space key on the current selection
func (m *FavouritesModel) handleSelect() (*FavouritesModel, tea.Cmd) {
	if m.mode == FavModeEdit {
		// Open edit popup
		m.showEditPopup = true
		m.editingSlot = m.selectedSlot
		m.editField = FavEditFieldName

		// Pre-populate with existing values
		assoc := m.associations.Associations[m.selectedSlot]
		m.editName = assoc.Name
		m.nameInput.SetValue(assoc.Name)
		m.nameInput.Focus()

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

		return m, textinput.Blink
	}

	// Select mode - start timer if slot has an association
	assoc := m.associations.Associations[m.selectedSlot]
	if !assoc.HasAssociation() {
		m.err = fmt.Errorf("slot %d is not configured - press 'e' to edit", m.selectedSlot+1)
		return m, nil
	}

	// Check if there's an active timer that needs to be stopped first
	if m.activeTimer != nil {
		// Stop the current timer with git commit auto-fill
		return m, m.stopCurrentTimerAndStartNew(m.selectedSlot)
	}

	// No active timer, just start new one
	return m, m.startTimerForSlot(m.selectedSlot)
}

// handleEditPopupKeys handles key events in the edit popup
func (m *FavouritesModel) handleEditPopupKeys(msg tea.KeyMsg) (*FavouritesModel, tea.Cmd) {
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

	case "ctrl+s":
		// Quick save
		return m.saveEditPopup()

	default:
		// Handle text input
		if m.editField == FavEditFieldName {
			var cmd tea.Cmd
			m.nameInput, cmd = m.nameInput.Update(msg)
			m.editName = m.nameInput.Value()
			return m, cmd
		}
		if m.editField == FavEditFieldProject && m.showProjectPopover {
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
func (m *FavouritesModel) handleEditPopupEnter() (*FavouritesModel, tea.Cmd) {
	// Handle popover selection
	if m.showProjectPopover && len(m.projects) > 0 {
		m.editProject = &m.projects[m.popoverCursor]
		m.editTask = nil // Clear task when project changes
		m.showProjectPopover = false
		m.popoverCursor = 0
		// Auto-advance to task selection
		m.editField = FavEditFieldTask
		m.showTaskPopover = true
		return m, m.loadTasks(m.editProject.ID)
	}

	if m.showTaskPopover && len(m.tasks) > 0 {
		m.editTask = &m.tasks[m.popoverCursor]
		m.showTaskPopover = false
		m.popoverCursor = 0
		// Auto-advance to activity selection
		m.editField = FavEditFieldActivity
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

	// If no popover open, open the appropriate one or advance
	switch m.editField {
	case FavEditFieldName:
		// Advance to project
		m.editField = FavEditFieldProject
		m.showProjectPopover = true
		m.projectSearchInput.Focus()
		m.nameInput.Blur()
		return m, textinput.Blink
	case FavEditFieldProject:
		if m.editProject == nil {
			m.showProjectPopover = true
			m.projectSearchInput.Focus()
			return m, textinput.Blink
		}
	case FavEditFieldTask:
		if m.editProject != nil && m.editTask == nil {
			m.showTaskPopover = true
		}
	case FavEditFieldActivity:
		if m.editActivity == nil {
			m.showActivityPopover = true
		}
	}

	return m, nil
}

// moveToNextEditField moves to the next field in the edit popup
func (m *FavouritesModel) moveToNextEditField() {
	m.closeAllPopovers()
	m.nameInput.Blur()

	switch m.editField {
	case FavEditFieldName:
		m.editField = FavEditFieldProject
		if m.editProject == nil {
			m.showProjectPopover = true
			m.projectSearchInput.Focus()
		}
	case FavEditFieldProject:
		m.editField = FavEditFieldTask
		if m.editProject != nil && m.editTask == nil {
			m.showTaskPopover = true
			m.popoverCursor = 0
		}
	case FavEditFieldTask:
		m.editField = FavEditFieldActivity
		if m.editActivity == nil {
			m.showActivityPopover = true
			m.popoverCursor = 0
		}
	case FavEditFieldActivity:
		m.editField = FavEditFieldName
		m.nameInput.Focus()
	}
}

// moveToPreviousEditField moves to the previous field in the edit popup
func (m *FavouritesModel) moveToPreviousEditField() {
	m.closeAllPopovers()
	m.nameInput.Blur()

	switch m.editField {
	case FavEditFieldName:
		m.editField = FavEditFieldActivity
		if m.editActivity == nil {
			m.showActivityPopover = true
			m.popoverCursor = 0
		}
	case FavEditFieldProject:
		m.editField = FavEditFieldName
		m.nameInput.Focus()
	case FavEditFieldTask:
		m.editField = FavEditFieldProject
		if m.editProject == nil {
			m.showProjectPopover = true
			m.projectSearchInput.Focus()
		}
	case FavEditFieldActivity:
		m.editField = FavEditFieldTask
		if m.editProject != nil && m.editTask == nil {
			m.showTaskPopover = true
			m.popoverCursor = 0
		}
	}
}

// closeAllPopovers closes all popovers in the edit popup
func (m *FavouritesModel) closeAllPopovers() {
	m.showProjectPopover = false
	m.showTaskPopover = false
	m.showActivityPopover = false
	m.projectSearchInput.Blur()
}

// resetEditState resets the edit popup state
func (m *FavouritesModel) resetEditState() {
	m.editName = ""
	m.editProject = nil
	m.editTask = nil
	m.editActivity = nil
	m.editField = FavEditFieldName
	m.popoverCursor = 0
	m.projects = nil
	m.tasks = nil
	m.closeAllPopovers()
	m.projectSearchInput.SetValue("")
	m.nameInput.SetValue("")
}

// saveEditPopup saves the current edit popup selections to the favourite
func (m *FavouritesModel) saveEditPopup() (*FavouritesModel, tea.Cmd) {
	if m.editProject == nil {
		m.err = fmt.Errorf("please select a project")
		return m, nil
	}

	assoc := &m.associations.Associations[m.editingSlot]
	assoc.Name = m.editName
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

	m.showEditPopup = false
	m.resetEditState()

	return m, m.saveAssociations()
}

// View renders the favourites screen
func (m *FavouritesModel) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	// Render the 3x3 grid (background)
	gridView := m.renderGrid()

	// If edit popup is showing, overlay it
	if m.showEditPopup {
		return m.renderWithModalOverlay(gridView)
	}

	// Header
	header := m.renderHeader()

	// Help text
	help := m.renderHelp()

	// Status messages
	var statusMsg string
	if m.err != nil {
		statusStyle := lipgloss.NewStyle().Foreground(favRed).Bold(true)
		statusMsg = statusStyle.Render(fmt.Sprintf("Error: %v", m.err))
	} else if m.successMessage != "" {
		statusStyle := lipgloss.NewStyle().Foreground(favGreen).Bold(true)
		statusMsg = statusStyle.Render(m.successMessage)
	} else if m.statusMessage != "" {
		statusStyle := lipgloss.NewStyle().Foreground(favBlue).Italic(true)
		statusMsg = statusStyle.Render(m.statusMessage)
	}

	// Mode indicator
	modeIndicator := ""
	if m.mode == FavModeEdit {
		modeStyle := lipgloss.NewStyle().
			Background(favOrange).
			Foreground(favWhite).
			Bold(true).
			Padding(0, 2)
		modeIndicator = modeStyle.Render("EDIT MODE")
	}

	// Combine all parts
	mainSection := lipgloss.JoinVertical(
		lipgloss.Center,
		header,
		"",
		modeIndicator,
		"",
		gridView,
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

// renderGrid renders the 3x3 grid of favourites
func (m *FavouritesModel) renderGrid() string {
	buttonWidth := 18
	buttonHeight := 5

	var rows []string

	for row := 0; row < 3; row++ {
		var cols []string
		for col := 0; col < 3; col++ {
			slotIndex := row*3 + col
			cols = append(cols, m.renderButton(slotIndex, buttonWidth, buttonHeight))
		}
		rowContent := lipgloss.JoinHorizontal(lipgloss.Center, cols...)
		rows = append(rows, rowContent)
	}

	return lipgloss.JoinVertical(lipgloss.Center, rows...)
}

// renderButton renders a single favourite button
func (m *FavouritesModel) renderButton(slotIndex, width, height int) string {
	assoc := m.associations.Associations[slotIndex]
	isSelected := slotIndex == m.selectedSlot

	// Base style
	style := lipgloss.NewStyle().
		Width(width).
		Height(height).
		Align(lipgloss.Center, lipgloss.Center).
		Border(lipgloss.RoundedBorder()).
		Margin(0, 1)

	// Style based on state
	if isSelected {
		if m.mode == FavModeEdit {
			style = style.
				BorderForeground(favOrange).
				Foreground(favOrange).
				Bold(true)
		} else {
			style = style.
				BorderForeground(favBlue).
				Foreground(favBlue).
				Bold(true)
		}
	} else if assoc.HasAssociation() {
		style = style.
			BorderForeground(favGray).
			Foreground(favWhite)
	} else {
		style = style.
			BorderForeground(favDarkGray).
			Foreground(favGray)
	}

	// Button content
	slotNum := fmt.Sprintf("[%d]", slotIndex+1)
	name := assoc.Name
	if name == "" {
		name = assoc.GetShortDisplayString(width - 4)
	} else if len(name) > width-4 {
		name = name[:width-5] + "…"
	}

	var content string
	if assoc.HasAssociation() {
		content = lipgloss.JoinVertical(
			lipgloss.Center,
			slotNum,
			name,
		)
	} else {
		content = lipgloss.JoinVertical(
			lipgloss.Center,
			slotNum,
			"Empty",
		)
	}

	return style.Render(content)
}

// renderWithModalOverlay renders the view with a centered modal popup
func (m *FavouritesModel) renderWithModalOverlay(background string) string {
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

	return modalPlaced
}

// renderEditModal renders the edit popup modal
func (m *FavouritesModel) renderEditModal() string {
	assoc := m.associations.Associations[m.editingSlot]

	// Styles matching TimesheetCreator
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(favOrange).
		Align(lipgloss.Center)

	subtitleStyle := lipgloss.NewStyle().
		Foreground(favGray).
		Italic(true).
		Align(lipgloss.Center)

	containerStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(favOrange).
		Padding(1, 2).
		Width(70)

	labelWidth := 12
	fieldWidth := 50

	labelStyle := lipgloss.NewStyle().
		Width(labelWidth).
		Align(lipgloss.Right).
		Foreground(favGray)

	focusedLabelStyle := lipgloss.NewStyle().
		Width(labelWidth).
		Align(lipgloss.Right).
		Foreground(favOrange).
		Bold(true)

	inputStyle := lipgloss.NewStyle().
		Width(fieldWidth)

	selectedValueStyle := lipgloss.NewStyle().
		Foreground(favBlue).
		Bold(true)

	unselectedValueStyle := lipgloss.NewStyle().
		Foreground(favGray).
		Italic(true)

	// Title
	slotName := assoc.Name
	if slotName == "" {
		slotName = fmt.Sprintf("Slot %d", m.editingSlot+1)
	}
	title := titleStyle.Render(fmt.Sprintf("Edit Favourite: %s", slotName))
	subtitle := subtitleStyle.Render("Configure project, task, and activity")

	// Build form rows
	var rows []string

	// Row 0: Name
	nameLabel := labelStyle.Render("Name:")
	if m.editField == FavEditFieldName {
		nameLabel = focusedLabelStyle.Render("Name:")
	}
	nameRow := lipgloss.JoinHorizontal(lipgloss.Top, nameLabel, "  ", inputStyle.Render(m.nameInput.View()))
	rows = append(rows, nameRow)

	// Row 1: Project
	projectLabel := labelStyle.Render("Project:")
	if m.editField == FavEditFieldProject {
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
	if m.editField == FavEditFieldTask {
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
	if m.editField == FavEditFieldActivity {
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
		Foreground(favGray).
		Italic(true).
		Align(lipgloss.Center)

	var helpText string
	if m.showProjectPopover || m.showTaskPopover || m.showActivityPopover {
		helpText = "↑/↓: Navigate • Enter: Select • Esc: Close • Tab: Next field"
	} else {
		helpText = "Tab: Next field • Enter: Open selection • Ctrl+S: Save • Esc: Cancel"
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

// renderPopover renders a pop-over selection list
func (m *FavouritesModel) renderPopover(items interface{}, getLabel func(int) string) string {
	popoverStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(favOrange).
		Padding(0, 1).
		MarginLeft(14).
		Width(50)

	selectedStyle := lipgloss.NewStyle().
		Background(favOrange).
		Foreground(favWhite).
		Bold(true).
		Width(46)

	normalStyle := lipgloss.NewStyle().
		Foreground(favWhite).
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
			Foreground(favGray).
			Italic(true).
			Render(fmt.Sprintf("  ↑↓ %d of %d", m.popoverCursor+1, itemCount))
		rows = append(rows, scrollInfo)
	}

	return popoverStyle.Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
}

// renderHeader renders the consistent header
func (m *FavouritesModel) renderHeader() string {
	if m.headerState != nil {
		return RenderHeader("Favourites", m.headerState)
	}
	return RenderHeader("Favourites", &HeaderState{})
}

// SetHeaderState sets the shared header state
func (m *FavouritesModel) SetHeaderState(state *HeaderState) {
	m.headerState = state
}

// renderHelp renders the help text
func (m *FavouritesModel) renderHelp() string {
	helpStyle := lipgloss.NewStyle().
		Foreground(favGray).
		Italic(true)

	powStatus := "off"
	if m.powCapturer != nil && m.powCapturer.IsEnabled() {
		powStatus = "on"
	}

	if m.mode == FavModeEdit {
		return helpStyle.Render(fmt.Sprintf("↑/↓/←/→ or 1-9: Navigate • Enter: Edit slot • e: Exit edit mode • p: POW (%s) • Esc/q: Back", powStatus))
	}

	return helpStyle.Render(fmt.Sprintf("↑/↓/←/→ or 1-9: Select • Enter: Start timer • e: Edit mode • p: POW (%s) • Esc/q: Back", powStatus))
}

// API loading commands
func (m *FavouritesModel) searchProjects(query string) tea.Cmd {
	cacheKey := strings.ToLower(query)
	if len(cacheKey) > 2 {
		cacheKey = cacheKey[:2]
	}

	// Check cache
	if cached, ok := m.projectCache[cacheKey]; ok {
		filtered := m.filterProjectsLocally(cached, query)
		return func() tea.Msg {
			return favProjectsLoadedMsg{
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
			return favSaveErrorMsg{err: fmt.Errorf("API client not available")}
		}

		projects, err := m.apiClient.GetProjects(cacheKey)
		if err != nil {
			return favSaveErrorMsg{err: err}
		}

		queryLower := strings.ToLower(query)
		var filtered []api.ProjectListItem
		for _, p := range projects {
			if strings.Contains(strings.ToLower(p.Label), queryLower) {
				filtered = append(filtered, p)
			}
		}

		return favProjectsLoadedMsg{
			Projects:    filtered,
			CacheKey:    cacheKey,
			AllProjects: projects,
		}
	}
}

func (m *FavouritesModel) filterProjectsLocally(projects []api.ProjectListItem, query string) []api.ProjectListItem {
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

func (m *FavouritesModel) loadTasks(projectID int) tea.Cmd {
	// Check cache
	if cached, ok := m.taskCache[projectID]; ok {
		return func() tea.Msg {
			return favTasksLoadedMsg{tasks: cached}
		}
	}

	// Check in-flight
	if m.taskCacheInFlight[projectID] {
		return nil
	}

	m.taskCacheInFlight[projectID] = true

	return func() tea.Msg {
		if m.apiClient == nil {
			return favSaveErrorMsg{err: fmt.Errorf("API client not available")}
		}

		tasks, err := m.apiClient.GetTasks(fmt.Sprintf("%d", projectID))
		if err != nil {
			return favSaveErrorMsg{err: err}
		}

		return favTasksLoadedMsg{tasks: tasks}
	}
}

func (m *FavouritesModel) loadActivities() tea.Cmd {
	if m.activityCacheValid {
		return func() tea.Msg {
			return favActivitiesLoadedMsg{activities: m.activityCache}
		}
	}

	if m.activityCacheInFlight {
		return nil
	}

	m.activityCacheInFlight = true

	return func() tea.Msg {
		if m.apiClient == nil {
			return favSaveErrorMsg{err: fmt.Errorf("API client not available")}
		}

		activities, err := m.apiClient.GetActivities()
		if err != nil {
			return favSaveErrorMsg{err: err}
		}

		return favActivitiesLoadedMsg{activities: activities}
	}
}

func (m *FavouritesModel) loadActiveTimer() tea.Cmd {
	return func() tea.Msg {
		if m.apiClient == nil {
			return favActiveTimerLoadedMsg{timer: nil}
		}

		timer, err := m.apiClient.GetActiveTimesheet()
		if err != nil {
			return favActiveTimerLoadedMsg{timer: nil}
		}

		return favActiveTimerLoadedMsg{timer: timer}
	}
}

func (m *FavouritesModel) saveAssociations() tea.Cmd {
	return func() tea.Msg {
		if m.storage == nil {
			return favSaveErrorMsg{err: fmt.Errorf("storage not initialized")}
		}
		err := m.storage.SaveFavouriteAssociations(m.associations)
		if err != nil {
			return favSaveErrorMsg{err: err}
		}
		return favSaveSuccessMsg{}
	}
}

// Timer control commands
func (m *FavouritesModel) stopCurrentTimerAndStartNew(nextSlot int) tea.Cmd {
	return func() tea.Msg {
		if m.activeTimer == nil {
			// No timer to stop, just return message to start new one
			return favTimerStoppedMsg{nextSlot: nextSlot}
		}

		// Try to get git commits for description
		description := m.fetchGitCommitsForTimer()

		// Stop the timer
		var err error
		if description != "" {
			err = m.apiClient.StopTimesheetWithDescription(m.activeTimer, description)
		} else {
			err = m.apiClient.StopTimesheet(m.activeTimer)
		}

		if err != nil {
			return favTimerErrorMsg{err: fmt.Errorf("failed to stop timer: %w", err)}
		}

		return favTimerStoppedMsg{
			nextSlot:    nextSlot,
			description: description,
		}
	}
}

func (m *FavouritesModel) startTimerForSlot(slotIndex int) tea.Cmd {
	return func() tea.Msg {
		assoc := m.associations.Associations[slotIndex]
		if !assoc.HasAssociation() {
			return favTimerErrorMsg{err: fmt.Errorf("slot %d is not configured", slotIndex+1)}
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
			StartTime:   time.Now(),
		}

		// Set TaskID if configured
		if assoc.TaskID > 0 {
			taskID := fmt.Sprintf("%d", assoc.TaskID)
			entry.TaskID = &taskID
		}

		err := m.apiClient.CreateTimesheet(entry)
		if err != nil {
			return favTimerErrorMsg{err: fmt.Errorf("failed to start timer: %w", err)}
		}

		// Fetch the newly created timer to return it
		timer, err := m.apiClient.GetActiveTimesheet()
		if err != nil {
			// Timer was created but we couldn't fetch it - still a success
			return favTimerStartedMsg{
				timer:       nil,
				projectName: assoc.ProjectName,
			}
		}

		return favTimerStartedMsg{
			timer:       timer,
			projectName: assoc.ProjectName,
		}
	}
}

// fetchGitCommitsForTimer fetches commit messages from a linked repo for the active timer
func (m *FavouritesModel) fetchGitCommitsForTimer() string {
	if m.activeTimer == nil {
		return ""
	}

	// Load code repo associations from storage
	cfg, err := config.LoadConfig()
	if err != nil {
		return ""
	}

	store, err := storage.New(cfg.GetStorageDir())
	if err != nil {
		return ""
	}

	associations, err := store.LoadCodeRepoAssociations()
	if err != nil {
		return ""
	}

	// Find association for this project
	assoc := associations.GetAssociationByProject(m.activeTimer.ProjectID)
	if assoc == nil || !assoc.HasAssociation() {
		return ""
	}

	// Parse start time
	startTime, err := m.activeTimer.GetFromTimeAsTime()
	if err != nil {
		return ""
	}

	// End time is now
	endTime := time.Now()

	// Check if this is a local path or a GitHub URL
	repoURL := assoc.RepoURL
	if api.IsLocalPath(repoURL) {
		return m.fetchLocalGitCommits(repoURL, startTime, endTime)
	}

	// GitHub repository
	return m.fetchGitHubCommits(assoc.RepoOwner, assoc.RepoName, startTime, endTime, cfg.GetGitHubToken())
}

func (m *FavouritesModel) fetchLocalGitCommits(repoPath string, startTime, endTime time.Time) string {
	repoPath = api.ExpandPath(repoPath)

	if !api.IsGitRepository(repoPath) {
		return ""
	}

	localGitClient := api.NewLocalGitClient()
	commits, err := localGitClient.GetCommitsInTimeRange(repoPath, startTime, endTime, "")
	if err != nil || len(commits) == 0 {
		return ""
	}

	return api.FormatLocalCommitsAsDescription(commits)
}

func (m *FavouritesModel) fetchGitHubCommits(owner, repo string, startTime, endTime time.Time, githubToken string) string {
	githubClient := api.NewGitHubClient(githubToken)
	commits, err := githubClient.GetCommitsInTimeRange(owner, repo, startTime, endTime, "")
	if err != nil || len(commits) == 0 {
		return ""
	}

	return api.FormatCommitsAsDescription(commits)
}

// togglePowMode toggles the POW (Proof of Work) screenshot capture mode
func (m *FavouritesModel) togglePowMode() tea.Cmd {
	return func() tea.Msg {
		if m.powCapturer == nil {
			// Create a new capturer if none exists
			cfg := pow.DefaultConfig()
			cfg.Enabled = true
			capturer, err := pow.New(cfg)
			if err != nil {
				return favPowToggledMsg{enabled: false, ok: false}
			}
			m.powCapturer = capturer
			return favPowToggledMsg{enabled: capturer.IsEnabled(), ok: capturer.IsEnabled()}
		}

		enabled, ok := m.powCapturer.Toggle()
		return favPowToggledMsg{enabled: enabled, ok: ok}
	}
}

// Message types
type favProjectsLoadedMsg struct {
	Projects    []api.ProjectListItem
	CacheKey    string
	AllProjects []api.ProjectListItem
}

type favTasksLoadedMsg struct {
	tasks []api.TaskListItem
}

type favActivitiesLoadedMsg struct {
	activities []api.ActivityListItem
}

type favActiveTimerLoadedMsg struct {
	timer *api.TimelogEntry
}

type favSaveSuccessMsg struct{}

type favSaveErrorMsg struct {
	err error
}

type favTimerStartedMsg struct {
	timer       *api.TimelogEntry
	projectName string
}

type favTimerStoppedMsg struct {
	nextSlot    int
	description string
}

type favTimerErrorMsg struct {
	err error
}

type favPowToggledMsg struct {
	enabled bool
	ok      bool
}
