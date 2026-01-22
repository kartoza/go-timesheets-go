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
	crOrange   = ColorOrange
	crBlue     = ColorBlue
	crGray     = ColorGray
	crWhite    = ColorWhite
	crDarkGray = ColorDarkGray
	crRed      = ColorRed
	crGreen    = ColorGreen
)

// CodeRepoEditMode represents what's being edited
type CodeRepoEditMode int

const (
	CodeRepoEditModeNone CodeRepoEditMode = iota
	CodeRepoEditModeAdd
	CodeRepoEditModeEdit
)

// CodeRepoEditField represents which field in the edit popup is focused
type CodeRepoEditField int

const (
	CodeRepoFieldProject CodeRepoEditField = iota
	CodeRepoFieldRepoURL
)

// CodeReposModel represents the code repos associations screen
type CodeReposModel struct {
	apiClient   *api.Client
	storage     *storage.Storage
	width       int
	height      int
	headerState *HeaderState // Shared header state

	// Data
	associations *models.CodeRepoAssociations

	// UI State for main list
	selectedRow int
	scrollOffset int

	// Edit popup state
	editMode       CodeRepoEditMode
	editField      CodeRepoEditField
	editingIndex   int // -1 for new, otherwise index of association being edited

	// Edit popup - selection data
	projects   []api.ProjectListItem

	// Edit popup - selected/entered values
	editProject  *api.ProjectListItem
	repoURLInput textinput.Model

	// Edit popup - popovers
	showProjectPopover bool
	popoverCursor      int

	// Caching (same pattern as WorkspaceAssociations)
	projectCache          map[string][]api.ProjectListItem
	projectSearchInFlight map[string]bool

	// Search input for project popover
	projectSearchInput textinput.Model

	// Error/success messages
	err            error
	successMessage string
}

// NewCodeReposModel creates a new code repos model
func NewCodeReposModel(apiClient *api.Client) *CodeReposModel {
	// Initialize storage
	cfg, _ := config.LoadConfig()
	storageDir := cfg.GetStorageDir()
	store, _ := storage.New(storageDir)

	// Load existing associations
	associations, err := store.LoadCodeRepoAssociations()
	if err != nil {
		associations = models.NewCodeRepoAssociations()
	}

	// Project search input for popup
	projectSearch := textinput.New()
	projectSearch.Placeholder = "Type to search projects..."
	projectSearch.CharLimit = 100
	projectSearch.Width = 40

	// Repo URL input
	repoURLInput := textinput.New()
	repoURLInput.Placeholder = "e.g., github.com/owner/repo"
	repoURLInput.CharLimit = 200
	repoURLInput.Width = 50

	return &CodeReposModel{
		apiClient:             apiClient,
		storage:               store,
		associations:          associations,
		selectedRow:           0,
		editMode:              CodeRepoEditModeNone,
		editField:             CodeRepoFieldProject,
		editingIndex:          -1,
		projectSearchInput:    projectSearch,
		repoURLInput:          repoURLInput,
		projectCache:          make(map[string][]api.ProjectListItem),
		projectSearchInFlight: make(map[string]bool),
		width:                 80,
		height:                24,
	}
}

// Init initializes the code repos view
func (m *CodeReposModel) Init() tea.Cmd {
	return nil
}

// Update handles messages
func (m *CodeReposModel) Update(msg tea.Msg) (*CodeReposModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.MouseMsg:
		// Handle mouse clicks on code repo rows
		if msg.Action == tea.MouseActionRelease && msg.Button == tea.MouseButtonLeft {
			if m.editMode == CodeRepoEditModeNone {
				// Calculate which row was clicked
				row := m.getRowFromMousePosition(msg.X, msg.Y)
				if row >= 0 && row < len(m.associations.Associations) {
					m.selectedRow = row
					// Click opens edit mode for that row
					m.editMode = CodeRepoEditModeEdit
					m.editField = CodeRepoFieldProject
					m.editingIndex = m.selectedRow
					assoc := m.associations.Associations[m.selectedRow]
					m.editProject = &api.ProjectListItem{ID: assoc.ProjectID, Label: assoc.ProjectName}
					m.repoURLInput.SetValue(assoc.RepoURL)
					m.repoURLInput.Focus()
					return m, nil
				}
			}
		}
		return m, nil

	case tea.KeyMsg:
		// Clear messages on key press
		m.err = nil
		m.successMessage = ""

		// Handle edit popup mode
		if m.editMode != CodeRepoEditModeNone {
			return m.handleEditPopupKeys(msg)
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
				// Adjust scroll if needed
				if m.selectedRow < m.scrollOffset {
					m.scrollOffset = m.selectedRow
				}
			}
			return m, nil

		case "down", "j":
			if m.selectedRow < len(m.associations.Associations)-1 {
				m.selectedRow++
				// Adjust scroll if needed (assuming 10 visible rows)
				maxVisible := 10
				if m.selectedRow >= m.scrollOffset+maxVisible {
					m.scrollOffset = m.selectedRow - maxVisible + 1
				}
			}
			return m, nil

		case "a":
			// Add new association
			m.editMode = CodeRepoEditModeAdd
			m.editField = CodeRepoFieldProject
			m.editingIndex = -1
			m.editProject = nil
			m.repoURLInput.SetValue("")
			m.showProjectPopover = true
			m.projectSearchInput.Focus()
			m.projectSearchInput.SetValue("")
			return m, textinput.Blink

		case "e", "enter":
			// Edit selected association
			if len(m.associations.Associations) > 0 && m.selectedRow < len(m.associations.Associations) {
				assoc := m.associations.Associations[m.selectedRow]
				m.editMode = CodeRepoEditModeEdit
				m.editField = CodeRepoFieldRepoURL
				m.editingIndex = m.selectedRow
				m.editProject = &api.ProjectListItem{
					ID:    assoc.ProjectID,
					Label: assoc.ProjectName,
				}
				m.repoURLInput.SetValue(assoc.RepoURL)
				m.repoURLInput.Focus()
				return m, textinput.Blink
			}
			return m, nil

		case "d", "delete", "backspace":
			// Delete selected association
			if len(m.associations.Associations) > 0 && m.selectedRow < len(m.associations.Associations) {
				assoc := m.associations.Associations[m.selectedRow]
				m.associations.RemoveAssociation(assoc.ProjectID)
				// Adjust selected row if needed
				if m.selectedRow >= len(m.associations.Associations) && m.selectedRow > 0 {
					m.selectedRow--
				}
				return m, m.saveAssociations()
			}
			return m, nil
		}

	// Message handlers for API responses
	case crProjectsLoadedMsg:
		if msg.CacheKey != "" {
			delete(m.projectSearchInFlight, msg.CacheKey)
			if len(msg.AllProjects) > 0 {
				m.projectCache[msg.CacheKey] = msg.AllProjects
			}
		}
		m.projects = msg.Projects
		m.popoverCursor = 0
		return m, nil

	case crSaveSuccessMsg:
		m.successMessage = "Code repo associations saved!"
		return m, nil

	case crSaveErrorMsg:
		m.err = msg.err
		return m, nil
	}

	return m, tea.Batch(cmds...)
}

// handleEditPopupKeys handles key events in the edit popup
func (m *CodeReposModel) handleEditPopupKeys(msg tea.KeyMsg) (*CodeReposModel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		// Close popovers first, then close popup
		if m.showProjectPopover {
			m.showProjectPopover = false
			return m, nil
		}
		m.editMode = CodeRepoEditModeNone
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
		if m.showProjectPopover {
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

	case "enter":
		return m.handleEditPopupEnter()

	default:
		// Handle text input
		if m.editField == CodeRepoFieldProject && m.showProjectPopover {
			var cmd tea.Cmd
			m.projectSearchInput, cmd = m.projectSearchInput.Update(msg)
			query := m.projectSearchInput.Value()
			if len(query) >= 2 {
				return m, tea.Batch(cmd, m.searchProjects(query))
			} else if len(query) == 0 {
				m.projects = nil
			}
			return m, cmd
		} else if m.editField == CodeRepoFieldRepoURL {
			var cmd tea.Cmd
			m.repoURLInput, cmd = m.repoURLInput.Update(msg)
			return m, cmd
		}
	}

	return m, nil
}

// handleEditPopupEnter handles enter key in the edit popup
func (m *CodeReposModel) handleEditPopupEnter() (*CodeReposModel, tea.Cmd) {
	// Handle popover selection
	if m.showProjectPopover && len(m.projects) > 0 {
		m.editProject = &m.projects[m.popoverCursor]
		m.showProjectPopover = false
		m.popoverCursor = 0
		// Auto-advance to repo URL
		m.editField = CodeRepoFieldRepoURL
		m.repoURLInput.Focus()
		return m, textinput.Blink
	}

	// If on repo URL field, save
	if m.editField == CodeRepoFieldRepoURL {
		return m.saveEditPopup()
	}

	// If no popover open on project field, open it
	if m.editField == CodeRepoFieldProject && !m.showProjectPopover {
		m.showProjectPopover = true
		m.projectSearchInput.Focus()
		return m, textinput.Blink
	}

	return m, nil
}

// moveToNextEditField moves to the next field in the edit popup
func (m *CodeReposModel) moveToNextEditField() {
	m.showProjectPopover = false
	m.projectSearchInput.Blur()

	switch m.editField {
	case CodeRepoFieldProject:
		m.editField = CodeRepoFieldRepoURL
		m.repoURLInput.Focus()
	case CodeRepoFieldRepoURL:
		m.editField = CodeRepoFieldProject
		if m.editProject == nil {
			m.showProjectPopover = true
			m.projectSearchInput.Focus()
		}
		m.repoURLInput.Blur()
	}
}

// moveToPreviousEditField moves to the previous field in the edit popup
func (m *CodeReposModel) moveToPreviousEditField() {
	m.showProjectPopover = false
	m.projectSearchInput.Blur()

	switch m.editField {
	case CodeRepoFieldProject:
		m.editField = CodeRepoFieldRepoURL
		m.repoURLInput.Focus()
	case CodeRepoFieldRepoURL:
		m.editField = CodeRepoFieldProject
		if m.editProject == nil {
			m.showProjectPopover = true
			m.projectSearchInput.Focus()
		}
		m.repoURLInput.Blur()
	}
}

// resetEditState resets the edit popup state
func (m *CodeReposModel) resetEditState() {
	m.editProject = nil
	m.editField = CodeRepoFieldProject
	m.editingIndex = -1
	m.popoverCursor = 0
	m.projects = nil
	m.showProjectPopover = false
	m.projectSearchInput.Blur()
	m.projectSearchInput.SetValue("")
	m.repoURLInput.Blur()
	m.repoURLInput.SetValue("")
}

// saveEditPopup saves the current edit popup selections
func (m *CodeReposModel) saveEditPopup() (*CodeReposModel, tea.Cmd) {
	if m.editProject == nil {
		m.err = fmt.Errorf("please select a project")
		return m, nil
	}

	repoURL := strings.TrimSpace(m.repoURLInput.Value())
	if repoURL == "" {
		m.err = fmt.Errorf("please enter a repository URL")
		return m, nil
	}

	// Parse repo URL to extract owner and name
	owner, name := parseRepoURL(repoURL)

	assoc := models.CodeRepoAssociation{
		ProjectID:   m.editProject.ID,
		ProjectName: m.editProject.Label,
		RepoURL:     repoURL,
		RepoOwner:   owner,
		RepoName:    name,
	}

	m.associations.SetAssociation(assoc)
	m.editMode = CodeRepoEditModeNone
	m.resetEditState()

	return m, m.saveAssociations()
}

// parseRepoURL extracts owner and repo name from a URL
func parseRepoURL(url string) (owner, name string) {
	// Remove protocol prefix if present
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "http://")
	url = strings.TrimPrefix(url, "git@")

	// Remove .git suffix if present
	url = strings.TrimSuffix(url, ".git")

	// Handle github.com/owner/repo format
	if strings.HasPrefix(url, "github.com/") {
		parts := strings.Split(strings.TrimPrefix(url, "github.com/"), "/")
		if len(parts) >= 2 {
			return parts[0], parts[1]
		}
	}

	// Handle github.com:owner/repo format (SSH)
	if strings.HasPrefix(url, "github.com:") {
		parts := strings.Split(strings.TrimPrefix(url, "github.com:"), "/")
		if len(parts) >= 2 {
			return parts[0], parts[1]
		}
	}

	// Just return the last two path components as a fallback
	parts := strings.Split(url, "/")
	if len(parts) >= 2 {
		return parts[len(parts)-2], parts[len(parts)-1]
	}

	return "", url
}

// View renders the code repos screen
func (m *CodeReposModel) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	// Render the list (background)
	listView := m.renderList()

	// If edit popup is showing, overlay it
	if m.editMode != CodeRepoEditModeNone {
		return m.renderWithModalOverlay(listView)
	}

	// Header
	header := m.renderHeader()

	// Help text
	help := m.renderHelp()

	// Status messages
	var statusMsg string
	if m.err != nil {
		statusStyle := lipgloss.NewStyle().Foreground(crRed).Bold(true)
		statusMsg = statusStyle.Render(fmt.Sprintf("Error: %v", m.err))
	} else if m.successMessage != "" {
		statusStyle := lipgloss.NewStyle().Foreground(crGreen).Bold(true)
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
func (m *CodeReposModel) renderWithModalOverlay(background string) string {
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
func (m *CodeReposModel) renderEditModal() string {
	// Styles
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(crOrange).
		Align(lipgloss.Center)

	subtitleStyle := lipgloss.NewStyle().
		Foreground(crGray).
		Italic(true).
		Align(lipgloss.Center)

	containerStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(crOrange).
		Padding(1, 2).
		Width(70)

	labelWidth := 12
	fieldWidth := 50

	labelStyle := lipgloss.NewStyle().
		Width(labelWidth).
		Align(lipgloss.Right).
		Foreground(crGray)

	focusedLabelStyle := lipgloss.NewStyle().
		Width(labelWidth).
		Align(lipgloss.Right).
		Foreground(crOrange).
		Bold(true)

	inputStyle := lipgloss.NewStyle().
		Width(fieldWidth)

	selectedValueStyle := lipgloss.NewStyle().
		Foreground(crBlue).
		Bold(true)

	unselectedValueStyle := lipgloss.NewStyle().
		Foreground(crGray).
		Italic(true)

	// Title
	var title string
	if m.editMode == CodeRepoEditModeAdd {
		title = titleStyle.Render("Add Code Repository Association")
	} else {
		title = titleStyle.Render("Edit Code Repository Association")
	}
	subtitle := subtitleStyle.Render("Link a project to a GitHub repository")

	// Build form rows
	var rows []string

	// Row 1: Project
	projectLabel := labelStyle.Render("Project:")
	if m.editField == CodeRepoFieldProject {
		projectLabel = focusedLabelStyle.Render("Project:")
	}
	var projectValue string
	if m.editProject != nil {
		projectValue = selectedValueStyle.Render("  " + m.editProject.Label)
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

	// Row 2: Repo URL
	repoLabel := labelStyle.Render("Repo URL:")
	if m.editField == CodeRepoFieldRepoURL {
		repoLabel = focusedLabelStyle.Render("Repo URL:")
	}

	repoInputStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(crGray).
		Width(fieldWidth - 4)

	if m.editField == CodeRepoFieldRepoURL {
		repoInputStyle = repoInputStyle.BorderForeground(crOrange)
	}

	repoRow := lipgloss.JoinHorizontal(lipgloss.Top, repoLabel, "  ", repoInputStyle.Render(m.repoURLInput.View()))
	rows = append(rows, repoRow)

	// Help text for popup
	helpStyle := lipgloss.NewStyle().
		Foreground(crGray).
		Italic(true).
		Align(lipgloss.Center)

	var helpText string
	if m.showProjectPopover {
		helpText = "  /  : Navigate  Enter: Select  Esc: Close  Tab: Next field"
	} else {
		helpText = "Tab: Next field  Enter: Save  Esc: Cancel"
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
func (m *CodeReposModel) renderPopover(items []api.ProjectListItem, getLabel func(int) string) string {
	popoverStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(crOrange).
		Padding(0, 1).
		MarginLeft(14).
		Width(50)

	selectedStyle := lipgloss.NewStyle().
		Background(crOrange).
		Foreground(crWhite).
		Bold(true).
		Width(46)

	normalStyle := lipgloss.NewStyle().
		Foreground(crWhite).
		Width(46)

	itemCount := len(items)

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
			rows = append(rows, selectedStyle.Render("  "+label))
		} else {
			rows = append(rows, normalStyle.Render("  "+label))
		}
	}

	// Add scroll indicator
	if itemCount > maxVisible {
		scrollInfo := lipgloss.NewStyle().
			Foreground(crGray).
			Italic(true).
			Render(fmt.Sprintf("     %d of %d", m.popoverCursor+1, itemCount))
		rows = append(rows, scrollInfo)
	}

	return popoverStyle.Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
}

// renderHeader renders the consistent header
func (m *CodeReposModel) renderHeader() string {
	if m.headerState != nil {
		return RenderHeader("Code Repositories", m.headerState)
	}
	// Fallback if header state not set
	return RenderHeader("Code Repositories", &HeaderState{})
}

// SetHeaderState sets the shared header state
func (m *CodeReposModel) SetHeaderState(state *HeaderState) {
	m.headerState = state
}

// renderList renders the list of code repo associations
func (m *CodeReposModel) renderList() string {
	// Column headers
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(crOrange).
		Width(70)

	projectHeaderStyle := lipgloss.NewStyle().Width(30).Align(lipgloss.Left)
	repoHeaderStyle := lipgloss.NewStyle().Width(40).Align(lipgloss.Left)

	headerRow := lipgloss.JoinHorizontal(
		lipgloss.Top,
		projectHeaderStyle.Render("Project"),
		repoHeaderStyle.Render("Repository"),
	)
	header := headerStyle.Render(headerRow)

	// Render rows
	var rows []string
	rows = append(rows, header)
	rows = append(rows, lipgloss.NewStyle().Foreground(crGray).Render(strings.Repeat("-", 70)))

	if len(m.associations.Associations) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Foreground(crGray).
			Italic(true).
			Width(70).
			Align(lipgloss.Center)
		rows = append(rows, "")
		rows = append(rows, emptyStyle.Render("No code repository associations yet."))
		rows = append(rows, emptyStyle.Render("Press 'a' to add a new association."))
	} else {
		// Show visible rows based on scroll offset
		maxVisible := 10
		endIdx := m.scrollOffset + maxVisible
		if endIdx > len(m.associations.Associations) {
			endIdx = len(m.associations.Associations)
		}

		for i := m.scrollOffset; i < endIdx; i++ {
			rows = append(rows, m.renderRow(i))
		}

		// Show scroll indicator if needed
		if len(m.associations.Associations) > maxVisible {
			scrollInfo := lipgloss.NewStyle().
				Foreground(crGray).
				Italic(true).
				Width(70).
				Align(lipgloss.Center).
				Render(fmt.Sprintf("Showing %d-%d of %d", m.scrollOffset+1, endIdx, len(m.associations.Associations)))
			rows = append(rows, "")
			rows = append(rows, scrollInfo)
		}
	}

	return lipgloss.JoinVertical(lipgloss.Center, rows...)
}

// renderRow renders a single association row
func (m *CodeReposModel) renderRow(index int) string {
	isSelected := index == m.selectedRow && m.editMode == CodeRepoEditModeNone
	assoc := m.associations.Associations[index]

	// Base styles
	projectStyle := lipgloss.NewStyle().Width(30).Align(lipgloss.Left)
	repoStyle := lipgloss.NewStyle().Width(40).Align(lipgloss.Left)

	if isSelected {
		projectStyle = projectStyle.Foreground(crOrange).Bold(true)
		repoStyle = repoStyle.Foreground(crOrange).Bold(true)
	} else {
		projectStyle = projectStyle.Foreground(crWhite)
		repoStyle = repoStyle.Foreground(crBlue)
	}

	// Project name (truncate if needed)
	projectName := assoc.ProjectName
	if len(projectName) > 28 {
		projectName = projectName[:25] + "..."
	}

	// Repo display
	repoDisplay := assoc.GetDisplayString()
	if len(repoDisplay) > 38 {
		repoDisplay = repoDisplay[:35] + "..."
	}

	// Selection indicator
	prefix := "  "
	if isSelected {
		prefix = "  "
	}

	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		prefix,
		projectStyle.Render(projectName),
		repoStyle.Render(repoDisplay),
	)
}

// renderHelp renders the help text
func (m *CodeReposModel) renderHelp() string {
	helpStyle := lipgloss.NewStyle().
		Foreground(crGray).
		Italic(true)

	return helpStyle.Render("  /  : Navigate | a: Add | e/Enter: Edit | d: Delete | q/Esc: Back to menu")
}

// API loading commands
func (m *CodeReposModel) searchProjects(query string) tea.Cmd {
	cacheKey := strings.ToLower(query)
	if len(cacheKey) > 2 {
		cacheKey = cacheKey[:2]
	}

	// Check cache
	if cached, ok := m.projectCache[cacheKey]; ok {
		filtered := m.filterProjectsLocally(cached, query)
		return func() tea.Msg {
			return crProjectsLoadedMsg{
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
			return crSaveErrorMsg{err: fmt.Errorf("API client not available")}
		}

		projects, err := m.apiClient.GetProjects(cacheKey)
		if err != nil {
			return crSaveErrorMsg{err: err}
		}

		queryLower := strings.ToLower(query)
		var filtered []api.ProjectListItem
		for _, p := range projects {
			if strings.Contains(strings.ToLower(p.Label), queryLower) {
				filtered = append(filtered, p)
			}
		}

		return crProjectsLoadedMsg{
			Projects:    filtered,
			CacheKey:    cacheKey,
			AllProjects: projects,
		}
	}
}

func (m *CodeReposModel) filterProjectsLocally(projects []api.ProjectListItem, query string) []api.ProjectListItem {
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

func (m *CodeReposModel) saveAssociations() tea.Cmd {
	return func() tea.Msg {
		if m.storage == nil {
			return crSaveErrorMsg{err: fmt.Errorf("storage not initialized")}
		}
		err := m.storage.SaveCodeRepoAssociations(m.associations)
		if err != nil {
			return crSaveErrorMsg{err: err}
		}
		return crSaveSuccessMsg{}
	}
}

// Message types
type crProjectsLoadedMsg struct {
	Projects    []api.ProjectListItem
	CacheKey    string
	AllProjects []api.ProjectListItem
}

type crSaveSuccessMsg struct{}

type crSaveErrorMsg struct {
	err error
}

// getRowFromMousePosition returns the row index from mouse coordinates
// Returns -1 if the click is not on a valid row
func (m *CodeReposModel) getRowFromMousePosition(x, y int) int {
	// The list starts after header, subtitle, and a couple blank lines
	// Header is centered, list is below it
	// List structure: header row, separator, then data rows

	// Calculate vertical center offset (same as in renderList/View)
	contentHeight := 4 + len(m.associations.Associations) + 2 // header + rows + some padding
	startY := (m.height - contentHeight) / 2
	if startY < 0 {
		startY = 0
	}

	// Account for: screen title (~3 lines), blank line, column header, separator
	headerOffset := startY + 5

	// Check if click is within the row area
	rowY := y - headerOffset
	if rowY < 0 {
		return -1
	}

	// Calculate which row was clicked (accounting for scroll offset)
	row := rowY + m.scrollOffset

	return row
}
