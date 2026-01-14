package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/kartoza/go-timesheets-go/internal/api"
	"github.com/kartoza/go-timesheets-go/internal/models"
)

// HistoryViewMode represents the current mode of the history view
type HistoryViewMode int

const (
	HistoryListMode HistoryViewMode = iota
	HistoryDetailMode
	HistoryEditMode
)

// HistoryView displays timesheet history with infinite scroll
type HistoryView struct {
	apiClient *api.Client
	username  string
	width     int
	height    int

	// Data
	allHistory []api.TimelogEntry

	// Scrolling - cursor is absolute position in allHistory
	cursor int

	// Description cache to avoid expensive re-rendering
	descCache map[int]string

	// View mode
	mode HistoryViewMode

	// Detail/Edit view state
	selectedEntry *api.TimelogEntry
	editFields    struct {
		description textinput.Model
		date        textinput.Model
		startTime   textinput.Model
		endTime     textinput.Model
	}
	editFocusField int
	editError      string
	editSuccess    string
	isSaving       bool

	// State
	err     error
	loading bool
}

// NewHistoryView creates a new history view
func NewHistoryView(apiClient *api.Client, username string) *HistoryView {
	// Initialize edit fields
	descInput := textinput.New()
	descInput.Placeholder = "Description..."
	descInput.CharLimit = 500
	descInput.Width = 50

	dateInput := textinput.New()
	dateInput.Placeholder = "YYYY-MM-DD"
	dateInput.CharLimit = 10
	dateInput.Width = 12

	startInput := textinput.New()
	startInput.Placeholder = "HH:MM"
	startInput.CharLimit = 5
	startInput.Width = 8

	endInput := textinput.New()
	endInput.Placeholder = "HH:MM"
	endInput.CharLimit = 5
	endInput.Width = 8

	h := &HistoryView{
		apiClient: apiClient,
		username:  username,
		cursor:    0,
		loading:   true,
		descCache: make(map[int]string),
		mode:      HistoryListMode,
	}

	h.editFields.description = descInput
	h.editFields.date = dateInput
	h.editFields.startTime = startInput
	h.editFields.endTime = endInput

	return h
}

// Init initializes the history view
func (h *HistoryView) Init() tea.Cmd {
	return h.loadHistory()
}

// getVisibleCount returns how many entries can fit on screen
func (h *HistoryView) getVisibleCount() int {
	availableHeight := h.height - 10
	if availableHeight < 3 {
		return 1
	}
	count := availableHeight / 3
	if count < 1 {
		count = 1
	}
	if count > 15 {
		count = 15
	}
	return count
}

// Update handles messages
func (h *HistoryView) Update(msg tea.Msg) (*HistoryView, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		h.width = msg.Width
		h.height = msg.Height

	case tea.KeyMsg:
		switch h.mode {
		case HistoryListMode:
			return h.updateListMode(msg)
		case HistoryDetailMode:
			return h.updateDetailMode(msg)
		case HistoryEditMode:
			return h.updateEditMode(msg)
		}

	case historyLoadedMsg:
		h.loading = false
		h.allHistory = msg.entries
		h.descCache = make(map[int]string)
		sort.Slice(h.allHistory, func(i, j int) bool {
			return h.allHistory[i].StartTime().After(h.allHistory[j].StartTime())
		})

	case historyUpdateSuccessMsg:
		h.isSaving = false
		h.editSuccess = "Entry updated successfully!"
		h.editError = ""
		// Invalidate cache for this entry
		delete(h.descCache, h.cursor)
		// Reload history to get fresh data
		return h, h.loadHistory()

	case historyUpdateErrorMsg:
		h.isSaving = false
		h.editError = msg.err.Error()
		h.editSuccess = ""

	case errorMsg:
		h.loading = false
		h.err = error(msg)
	}

	return h, nil
}

// updateListMode handles input in list mode
func (h *HistoryView) updateListMode(msg tea.KeyMsg) (*HistoryView, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return h, tea.Quit

	case "esc", "q":
		return h, func() tea.Msg { return backToMenuMsg{} }

	case "up", "k":
		if h.cursor > 0 {
			h.cursor--
		}

	case "down", "j":
		if h.cursor < len(h.allHistory)-1 {
			h.cursor++
		}

	case "home", "g":
		h.cursor = 0

	case "end", "G":
		if len(h.allHistory) > 0 {
			h.cursor = len(h.allHistory) - 1
		}

	case "pgup":
		h.cursor -= h.getVisibleCount()
		if h.cursor < 0 {
			h.cursor = 0
		}

	case "pgdown":
		h.cursor += h.getVisibleCount()
		if h.cursor >= len(h.allHistory) {
			h.cursor = len(h.allHistory) - 1
		}
		if h.cursor < 0 {
			h.cursor = 0
		}

	case "enter":
		// Open detail view
		if len(h.allHistory) > 0 && h.cursor < len(h.allHistory) {
			h.selectedEntry = &h.allHistory[h.cursor]
			h.mode = HistoryDetailMode
			h.editError = ""
			h.editSuccess = ""
		}

	case "r":
		h.loading = true
		h.cursor = 0
		return h, h.loadHistory()
	}

	return h, nil
}

// updateDetailMode handles input in detail view mode
func (h *HistoryView) updateDetailMode(msg tea.KeyMsg) (*HistoryView, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return h, tea.Quit

	case "esc", "q":
		// Go back to list
		h.mode = HistoryListMode
		h.selectedEntry = nil
		h.editError = ""
		h.editSuccess = ""

	case "e":
		// Enter edit mode if not submitted
		if h.selectedEntry != nil && !h.selectedEntry.IsSubmitted() {
			h.mode = HistoryEditMode
			h.initEditFields()
			h.editFocusField = 0
			h.editFields.description.Focus()
		}
	}

	return h, nil
}

// updateEditMode handles input in edit mode
func (h *HistoryView) updateEditMode(msg tea.KeyMsg) (*HistoryView, tea.Cmd) {
	var cmd tea.Cmd

	switch msg.String() {
	case "ctrl+c":
		return h, tea.Quit

	case "esc":
		// Go back to detail view
		h.mode = HistoryDetailMode
		h.editError = ""
		h.editSuccess = ""

	case "tab", "down":
		// Move to next field
		h.blurAllEditFields()
		h.editFocusField = (h.editFocusField + 1) % 4
		h.focusEditField()

	case "shift+tab", "up":
		// Move to previous field
		h.blurAllEditFields()
		h.editFocusField = (h.editFocusField + 3) % 4
		h.focusEditField()

	case "ctrl+s", "enter":
		// Save changes
		if !h.isSaving {
			return h, h.saveEntry()
		}

	default:
		// Update focused field
		switch h.editFocusField {
		case 0:
			h.editFields.description, cmd = h.editFields.description.Update(msg)
		case 1:
			h.editFields.date, cmd = h.editFields.date.Update(msg)
		case 2:
			h.editFields.startTime, cmd = h.editFields.startTime.Update(msg)
		case 3:
			h.editFields.endTime, cmd = h.editFields.endTime.Update(msg)
		}
		return h, cmd
	}

	return h, nil
}

// initEditFields populates edit fields from selected entry
func (h *HistoryView) initEditFields() {
	if h.selectedEntry == nil {
		return
	}

	h.editFields.description.SetValue(h.selectedEntry.GetDescriptionString())
	h.editFields.date.SetValue(h.selectedEntry.StartTime().Format("2006-01-02"))
	h.editFields.startTime.SetValue(h.selectedEntry.StartTime().Format("15:04"))

	endTime := h.selectedEntry.EndTime()
	if !endTime.IsZero() {
		h.editFields.endTime.SetValue(endTime.Format("15:04"))
	} else {
		h.editFields.endTime.SetValue("")
	}
}

// blurAllEditFields removes focus from all edit fields
func (h *HistoryView) blurAllEditFields() {
	h.editFields.description.Blur()
	h.editFields.date.Blur()
	h.editFields.startTime.Blur()
	h.editFields.endTime.Blur()
}

// focusEditField focuses the current edit field
func (h *HistoryView) focusEditField() {
	switch h.editFocusField {
	case 0:
		h.editFields.description.Focus()
	case 1:
		h.editFields.date.Focus()
	case 2:
		h.editFields.startTime.Focus()
	case 3:
		h.editFields.endTime.Focus()
	}
}

// saveEntry saves the edited entry
func (h *HistoryView) saveEntry() tea.Cmd {
	if h.selectedEntry == nil {
		return nil
	}

	h.isSaving = true
	h.editError = ""

	return func() tea.Msg {
		// Parse date and times
		dateStr := h.editFields.date.Value()
		startTimeStr := h.editFields.startTime.Value()
		endTimeStr := h.editFields.endTime.Value()

		startStr := fmt.Sprintf("%s %s", dateStr, startTimeStr)
		startTime, err := time.Parse("2006-01-02 15:04", startStr)
		if err != nil {
			return historyUpdateErrorMsg{err: fmt.Errorf("invalid start time: %w", err)}
		}

		var endTimePtr *time.Time
		var duration float64

		if endTimeStr != "" {
			endStr := fmt.Sprintf("%s %s", dateStr, endTimeStr)
			endTime, err := time.Parse("2006-01-02 15:04", endStr)
			if err != nil {
				return historyUpdateErrorMsg{err: fmt.Errorf("invalid end time: %w", err)}
			}
			if endTime.Before(startTime) {
				return historyUpdateErrorMsg{err: fmt.Errorf("end time must be after start time")}
			}
			endTimePtr = &endTime
			duration = endTime.Sub(startTime).Hours()
		}

		// Build entry for update
		entry := models.TimeEntry{
			ProjectID:   fmt.Sprintf("%d", h.selectedEntry.ProjectID),
			ActivityID:  fmt.Sprintf("%d", h.selectedEntry.ActivityID),
			Description: h.editFields.description.Value(),
			StartTime:   startTime,
			EndTime:     endTimePtr,
			Duration:    duration,
		}

		taskID := h.selectedEntry.TaskID.Value
		if taskID > 0 {
			taskIDStr := fmt.Sprintf("%d", taskID)
			entry.TaskID = &taskIDStr
		}

		// Update via API
		err = h.apiClient.UpdateTimesheet(fmt.Sprintf("%d", h.selectedEntry.ID), entry)
		if err != nil {
			return historyUpdateErrorMsg{err: err}
		}

		return historyUpdateSuccessMsg{}
	}
}

// View renders the history view
func (h *HistoryView) View() string {
	if h.width == 0 {
		return "Loading..."
	}

	switch h.mode {
	case HistoryDetailMode:
		return h.renderDetailView()
	case HistoryEditMode:
		return h.renderEditView()
	default:
		return h.renderListView()
	}
}

// renderListView renders the list mode view
func (h *HistoryView) renderListView() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#DDA036")).
		Align(lipgloss.Center)

	if h.loading {
		loadingStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9A9EA0")).
			Align(lipgloss.Center)

		content := lipgloss.JoinVertical(
			lipgloss.Center,
			titleStyle.Render("Timesheet History"),
			"",
			loadingStyle.Render("Loading history..."),
		)

		return lipgloss.Place(h.width, h.height, lipgloss.Center, lipgloss.Center, content)
	}

	if h.err != nil {
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF6B6B")).
			Align(lipgloss.Center)

		content := lipgloss.JoinVertical(
			lipgloss.Center,
			titleStyle.Render("Timesheet History"),
			"",
			errorStyle.Render("Error: "+h.err.Error()),
			"",
			lipgloss.NewStyle().Foreground(lipgloss.Color("#9A9EA0")).Render("Press 'r' to retry, Esc to go back"),
		)

		return lipgloss.Place(h.width, h.height, lipgloss.Center, lipgloss.Center, content)
	}

	if len(h.allHistory) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9A9EA0")).
			Align(lipgloss.Center)

		content := lipgloss.JoinVertical(
			lipgloss.Center,
			titleStyle.Render("Timesheet History"),
			"",
			emptyStyle.Render("No timesheet entries found"),
			"",
			lipgloss.NewStyle().Foreground(lipgloss.Color("#9A9EA0")).Render("Press Esc to go back"),
		)

		return lipgloss.Place(h.width, h.height, lipgloss.Center, lipgloss.Center, content)
	}

	positionInfo := fmt.Sprintf("Entry %d of %d", h.cursor+1, len(h.allHistory))
	title := titleStyle.Render("Timesheet History")
	posStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9A9EA0")).
		Align(lipgloss.Center)

	table := h.renderScrollableTable()
	scrollBar := h.renderScrollBar()

	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9A9EA0")).
		Italic(true).
		Align(lipgloss.Center)

	help := helpStyle.Render("↑/↓: Scroll • Enter: View Details • r: Refresh • Esc/q: Back")

	tableWithScroll := lipgloss.JoinHorizontal(lipgloss.Top, table, " ", scrollBar)

	content := lipgloss.JoinVertical(
		lipgloss.Center,
		title,
		posStyle.Render(positionInfo),
		"",
		tableWithScroll,
		"",
		help,
	)

	return lipgloss.Place(h.width, h.height, lipgloss.Center, lipgloss.Top, content)
}

// renderDetailView renders the detail view for a selected entry
func (h *HistoryView) renderDetailView() string {
	if h.selectedEntry == nil {
		return "No entry selected"
	}

	entry := h.selectedEntry

	// Styles
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#DDA036")).
		Align(lipgloss.Center)

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9A9EA0")).
		Width(14).
		Align(lipgloss.Right)

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Bold(true)

	highlightStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#569FC6")).
		Bold(true)

	containerStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#DDA036")).
		Padding(1, 3).
		Width(70)

	dividerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9A9EA0"))

	// Status badge
	var statusBadge string
	if entry.IsSubmitted() {
		statusBadge = lipgloss.NewStyle().
			Background(lipgloss.Color("#569FC6")).
			Foreground(lipgloss.Color("#FFFFFF")).
			Padding(0, 1).
			Bold(true).
			Render("✓ SUBMITTED")
	} else if entry.EndTime().IsZero() {
		statusBadge = lipgloss.NewStyle().
			Background(lipgloss.Color("#DDA036")).
			Foreground(lipgloss.Color("#000000")).
			Padding(0, 1).
			Bold(true).
			Render("● RUNNING")
	} else {
		statusBadge = lipgloss.NewStyle().
			Background(lipgloss.Color("#9A9EA0")).
			Foreground(lipgloss.Color("#000000")).
			Padding(0, 1).
			Bold(true).
			Render("○ PENDING")
	}

	// Build detail rows
	var rows []string

	// Header with status
	headerRow := lipgloss.JoinHorizontal(lipgloss.Center,
		titleStyle.Render("Timesheet Entry Details"),
		"  ",
		statusBadge,
	)
	rows = append(rows, headerRow)
	rows = append(rows, "")

	// Project
	rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top,
		labelStyle.Render("Project:"),
		"  ",
		highlightStyle.Render(entry.ProjectName),
	))

	// Task
	taskName := entry.TaskName
	if taskName == "" {
		taskName = "(no task)"
	}
	rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top,
		labelStyle.Render("Task:"),
		"  ",
		valueStyle.Render(taskName),
	))

	// Activity
	rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top,
		labelStyle.Render("Activity:"),
		"  ",
		valueStyle.Render(entry.Activity()),
	))

	// Divider
	rows = append(rows, "")
	rows = append(rows, dividerStyle.Render(strings.Repeat("─", 62)))
	rows = append(rows, "")

	// Date
	rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top,
		labelStyle.Render("Date:"),
		"  ",
		valueStyle.Render(entry.StartTime().Format("Monday, January 2, 2006")),
	))

	// Start Time
	rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top,
		labelStyle.Render("Start Time:"),
		"  ",
		valueStyle.Render(entry.StartTime().Format("15:04")),
	))

	// End Time
	endTimeStr := "(running)"
	if !entry.EndTime().IsZero() {
		endTimeStr = entry.EndTime().Format("15:04")
	}
	rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top,
		labelStyle.Render("End Time:"),
		"  ",
		valueStyle.Render(endTimeStr),
	))

	// Duration
	durationStr := fmt.Sprintf("%.2f hours", entry.Duration())
	rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top,
		labelStyle.Render("Duration:"),
		"  ",
		highlightStyle.Render(durationStr),
	))

	// Divider
	rows = append(rows, "")
	rows = append(rows, dividerStyle.Render(strings.Repeat("─", 62)))
	rows = append(rows, "")

	// Description
	rows = append(rows, labelStyle.Render("Description:"))
	desc := entry.GetDescriptionString()
	if desc == "" {
		desc = "(no description)"
	}
	descStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Width(60).
		MarginLeft(2)
	// Render description (strip HTML)
	cleanDesc := renderHistoryDescription(desc, 58)
	rows = append(rows, descStyle.Render(cleanDesc))

	// Success/Error messages
	if h.editSuccess != "" {
		successStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#569FC6")).
			Bold(true).
			Align(lipgloss.Center).
			Width(62)
		rows = append(rows, "")
		rows = append(rows, successStyle.Render(h.editSuccess))
	}

	content := containerStyle.Render(lipgloss.JoinVertical(lipgloss.Left, rows...))

	// Help text
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9A9EA0")).
		Italic(true).
		Align(lipgloss.Center)

	var helpText string
	if entry.IsSubmitted() {
		helpText = helpStyle.Render("Esc: Back to list")
	} else {
		helpText = helpStyle.Render("e: Edit Entry • Esc: Back to list")
	}

	fullContent := lipgloss.JoinVertical(
		lipgloss.Center,
		content,
		"",
		helpText,
	)

	return lipgloss.Place(h.width, h.height, lipgloss.Center, lipgloss.Center, fullContent)
}

// renderEditView renders the edit view for a selected entry
func (h *HistoryView) renderEditView() string {
	if h.selectedEntry == nil {
		return "No entry selected"
	}

	entry := h.selectedEntry

	// Styles
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#DDA036")).
		Align(lipgloss.Center)

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9A9EA0")).
		Width(14).
		Align(lipgloss.Right)

	focusedLabelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#DDA036")).
		Bold(true).
		Width(14).
		Align(lipgloss.Right)

	infoStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#569FC6")).
		Bold(true)

	containerStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#DDA036")).
		Padding(1, 3).
		Width(70)

	dividerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9A9EA0"))

	// Build edit form
	var rows []string

	// Header
	rows = append(rows, titleStyle.Render("Edit Timesheet Entry"))
	rows = append(rows, "")

	// Project (read-only)
	rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top,
		labelStyle.Render("Project:"),
		"  ",
		infoStyle.Render(entry.ProjectName),
	))

	// Task (read-only)
	taskName := entry.TaskName
	if taskName == "" {
		taskName = "(no task)"
	}
	rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top,
		labelStyle.Render("Task:"),
		"  ",
		infoStyle.Render(taskName),
	))

	// Activity (read-only)
	rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top,
		labelStyle.Render("Activity:"),
		"  ",
		infoStyle.Render(entry.Activity()),
	))

	// Divider
	rows = append(rows, "")
	rows = append(rows, dividerStyle.Render(strings.Repeat("─", 62)))
	rows = append(rows, "")

	// Editable fields
	// Description
	descLabel := labelStyle.Render("Description:")
	if h.editFocusField == 0 {
		descLabel = focusedLabelStyle.Render("Description:")
	}
	rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top,
		descLabel,
		"  ",
		h.editFields.description.View(),
	))

	// Date
	dateLabel := labelStyle.Render("Date:")
	if h.editFocusField == 1 {
		dateLabel = focusedLabelStyle.Render("Date:")
	}
	rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top,
		dateLabel,
		"  ",
		h.editFields.date.View(),
	))

	// Start Time
	startLabel := labelStyle.Render("Start Time:")
	if h.editFocusField == 2 {
		startLabel = focusedLabelStyle.Render("Start Time:")
	}
	rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top,
		startLabel,
		"  ",
		h.editFields.startTime.View(),
	))

	// End Time
	endLabel := labelStyle.Render("End Time:")
	if h.editFocusField == 3 {
		endLabel = focusedLabelStyle.Render("End Time:")
	}
	rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top,
		endLabel,
		"  ",
		h.editFields.endTime.View(),
	))

	// Error/Success messages
	if h.editError != "" {
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF6B6B")).
			Bold(true).
			Align(lipgloss.Center).
			Width(62)
		rows = append(rows, "")
		rows = append(rows, errorStyle.Render("Error: "+h.editError))
	}

	if h.isSaving {
		savingStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#DDA036")).
			Bold(true).
			Align(lipgloss.Center).
			Width(62)
		rows = append(rows, "")
		rows = append(rows, savingStyle.Render("Saving..."))
	}

	content := containerStyle.Render(lipgloss.JoinVertical(lipgloss.Left, rows...))

	// Help text
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9A9EA0")).
		Italic(true).
		Align(lipgloss.Center)

	helpText := helpStyle.Render("Tab: Next field • Ctrl+S/Enter: Save • Esc: Cancel")

	fullContent := lipgloss.JoinVertical(
		lipgloss.Center,
		content,
		"",
		helpText,
	)

	return lipgloss.Place(h.width, h.height, lipgloss.Center, lipgloss.Center, fullContent)
}

// renderScrollBar renders a visual scroll indicator
func (h *HistoryView) renderScrollBar() string {
	if len(h.allHistory) == 0 {
		return ""
	}

	visibleCount := h.getVisibleCount()
	totalEntries := len(h.allHistory)

	barHeight := h.height - 14
	if barHeight < 5 {
		barHeight = 5
	}

	if totalEntries <= visibleCount {
		return ""
	}

	thumbSize := (visibleCount * barHeight) / totalEntries
	if thumbSize < 1 {
		thumbSize = 1
	}
	if thumbSize > barHeight {
		thumbSize = barHeight
	}

	thumbPos := (h.cursor * (barHeight - thumbSize)) / (totalEntries - 1)
	if thumbPos < 0 {
		thumbPos = 0
	}
	if thumbPos > barHeight-thumbSize {
		thumbPos = barHeight - thumbSize
	}

	var sb strings.Builder
	trackStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9A9EA0"))
	thumbStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#DDA036"))

	for i := 0; i < barHeight; i++ {
		if i >= thumbPos && i < thumbPos+thumbSize {
			sb.WriteString(thumbStyle.Render("┃"))
		} else {
			sb.WriteString(trackStyle.Render("│"))
		}
		if i < barHeight-1 {
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// renderScrollableTable renders the visible portion of the history table
func (h *HistoryView) renderScrollableTable() string {
	visibleCount := h.getVisibleCount()
	totalEntries := len(h.allHistory)

	startIdx := h.cursor - visibleCount/2
	if startIdx < 0 {
		startIdx = 0
	}
	endIdx := startIdx + visibleCount
	if endIdx > totalEntries {
		endIdx = totalEntries
		startIdx = endIdx - visibleCount
		if startIdx < 0 {
			startIdx = 0
		}
	}

	visibleEntries := h.allHistory[startIdx:endIdx]

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#DDA036")).
		Width(16).
		Align(lipgloss.Left)

	cellStyle := lipgloss.NewStyle().
		Width(16).
		Align(lipgloss.Left)

	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#000000")).
		Background(lipgloss.Color("#DDA036")).
		Width(16).
		Align(lipgloss.Left)

	descStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9A9EA0")).
		Width(90).
		Align(lipgloss.Left)

	selectedDescStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#000000")).
		Background(lipgloss.Color("#DDA036")).
		Width(90).
		Align(lipgloss.Left)

	header := lipgloss.JoinHorizontal(lipgloss.Top,
		headerStyle.Render("Project"),
		headerStyle.Render("Task"),
		headerStyle.Render("Activity"),
		headerStyle.Render("Date"),
		headerStyle.Width(10).Render("Hours"),
		headerStyle.Width(12).Render("Status"),
	)

	var rows []string
	for i, entry := range visibleEntries {
		absoluteIdx := startIdx + i
		isSelected := absoluteIdx == h.cursor

		project := truncate(entry.ProjectName, 14)
		task := truncate(entry.TaskName, 14)
		activity := truncate(entry.Activity(), 14)
		dateStr := entry.StartTime().Format("2006-01-02")
		hours := fmt.Sprintf("%.2fh", entry.Duration())

		status := "Pending"
		if entry.IsSubmitted() {
			status = "✓ Submitted"
		} else if entry.EndTime().IsZero() {
			status = "● Running"
		}

		var row1 string
		if isSelected {
			row1 = lipgloss.JoinHorizontal(lipgloss.Top,
				selectedStyle.Render(project),
				selectedStyle.Render(task),
				selectedStyle.Render(activity),
				selectedStyle.Render(dateStr),
				selectedStyle.Width(10).Render(hours),
				selectedStyle.Width(12).Render(status),
			)
		} else {
			row1 = lipgloss.JoinHorizontal(lipgloss.Top,
				cellStyle.Render(project),
				cellStyle.Render(task),
				cellStyle.Render(activity),
				cellStyle.Render(dateStr),
				cellStyle.Width(10).Render(hours),
				cellStyle.Width(12).Render(status),
			)
		}

		renderedDesc, ok := h.descCache[absoluteIdx]
		if !ok {
			desc := entry.GetDescriptionString()
			if desc == "" {
				renderedDesc = "(no description)"
			} else {
				renderedDesc = renderHistoryDescription(desc, 86)
			}
			h.descCache[absoluteIdx] = renderedDesc
		}

		var row2 string
		if isSelected {
			row2 = selectedDescStyle.Render("  " + renderedDesc)
		} else {
			row2 = descStyle.Render("  " + renderedDesc)
		}

		rows = append(rows, row1, row2)

		if i < len(visibleEntries)-1 {
			sep := lipgloss.NewStyle().
				Foreground(lipgloss.Color("#9A9EA0")).
				Render(strings.Repeat("─", 90))
			rows = append(rows, sep)
		}
	}

	var topIndicator, bottomIndicator string
	indicatorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#DDA036")).
		Bold(true).
		Align(lipgloss.Center).
		Width(90)

	if startIdx > 0 {
		topIndicator = indicatorStyle.Render(fmt.Sprintf("↑ %d more entries above", startIdx))
	}
	if endIdx < totalEntries {
		bottomIndicator = indicatorStyle.Render(fmt.Sprintf("↓ %d more entries below", totalEntries-endIdx))
	}

	tableContent := lipgloss.JoinVertical(lipgloss.Left, append([]string{header, ""}, rows...)...)

	if topIndicator != "" {
		tableContent = lipgloss.JoinVertical(lipgloss.Left, topIndicator, "", tableContent)
	}
	if bottomIndicator != "" {
		tableContent = lipgloss.JoinVertical(lipgloss.Left, tableContent, "", bottomIndicator)
	}

	tableStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#DDA036")).
		Padding(1, 2)

	return tableStyle.Render(tableContent)
}

// Helper functions
func (h *HistoryView) loadHistory() tea.Cmd {
	return func() tea.Msg {
		if h.apiClient == nil {
			return errorMsg(fmt.Errorf("API client not available"))
		}

		entries, err := h.apiClient.GetTimelogs()
		if err != nil {
			return errorMsg(err)
		}

		return historyLoadedMsg{entries: entries}
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func renderHistoryDescription(html string, width int) string {
	if html == "" {
		return "(no description)"
	}

	text := html

	text = strings.ReplaceAll(text, "<br>", " ")
	text = strings.ReplaceAll(text, "<br/>", " ")
	text = strings.ReplaceAll(text, "<br />", " ")
	text = strings.ReplaceAll(text, "</p>", " ")
	text = strings.ReplaceAll(text, "<p>", "")
	text = strings.ReplaceAll(text, "</div>", " ")
	text = strings.ReplaceAll(text, "<div>", "")
	text = strings.ReplaceAll(text, "&nbsp;", " ")
	text = strings.ReplaceAll(text, "&amp;", "&")
	text = strings.ReplaceAll(text, "&lt;", "<")
	text = strings.ReplaceAll(text, "&gt;", ">")

	for {
		start := strings.Index(text, "<")
		if start == -1 {
			break
		}
		end := strings.Index(text[start:], ">")
		if end == -1 {
			break
		}
		text = text[:start] + text[start+end+1:]
	}

	text = strings.TrimSpace(text)
	for strings.Contains(text, "  ") {
		text = strings.ReplaceAll(text, "  ", " ")
	}

	if idx := strings.Index(text, "\n"); idx != -1 {
		text = text[:idx]
	}

	if len(text) > width {
		text = text[:width-3] + "..."
	}

	return text
}

// Message types
type historyLoadedMsg struct {
	entries []api.TimelogEntry
}

type historyUpdateSuccessMsg struct{}

type historyUpdateErrorMsg struct {
	err error
}
