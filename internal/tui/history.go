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
	"github.com/kartoza/go-timesheets-go/internal/config"
	"github.com/kartoza/go-timesheets-go/internal/models"
	"github.com/kartoza/go-timesheets-go/internal/storage"
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
	apiClient   *api.Client
	username    string
	width       int
	height      int
	headerState *HeaderState // Shared header state
	store       *storage.Storage // For persistent cache

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

	// Submit all pending
	confirmSubmit    bool
	isSubmitting     bool
	submitSuccess    string
	pendingCount     int
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

	// Initialize storage for persistent cache
	var store *storage.Storage
	if cfg, err := config.LoadConfig(); err == nil {
		store, _ = storage.New(cfg.GetStorageDir())
	}

	h := &HistoryView{
		apiClient: apiClient,
		username:  username,
		cursor:    0,
		loading:   true,
		descCache: make(map[int]string),
		mode:      HistoryListMode,
		store:     store,
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
		h.isSubmitting = false
		sort.Slice(h.allHistory, func(i, j int) bool {
			return h.allHistory[i].StartTime().After(h.allHistory[j].StartTime())
		})
		// Count pending entries (stopped but not submitted)
		h.pendingCount = 0
		for _, entry := range h.allHistory {
			if !entry.IsSubmitted() && !entry.EndTime().IsZero() {
				h.pendingCount++
			}
		}
		// Save to persistent cache for app restart
		if h.store != nil {
			_ = h.store.SaveTimelogCache(h.allHistory)
		}

	case historyUpdateSuccessMsg:
		h.isSaving = false
		h.editSuccess = "Entry updated successfully!"
		h.editError = ""

		// Update the local cache instead of reloading from server
		for i := range h.allHistory {
			if h.allHistory[i].ID == msg.entryID {
				// Update the fields that were edited
				desc := msg.description
				h.allHistory[i].Description = &desc
				h.allHistory[i].FromTime = msg.fromTime
				h.allHistory[i].ToTime = msg.toTime
				h.allHistory[i].Hours = msg.hours
				// Also update the "all" fields which track aggregated time
				h.allHistory[i].AllFromTime = formatAllTimeField(msg.fromTime)
				h.allHistory[i].AllToTime = formatAllTimeField(msg.toTime)
				h.allHistory[i].AllHours = msg.hours

				// Also update selectedEntry if it's the same entry
				if h.selectedEntry != nil && h.selectedEntry.ID == msg.entryID {
					h.selectedEntry.Description = &desc
					h.selectedEntry.FromTime = msg.fromTime
					h.selectedEntry.ToTime = msg.toTime
					h.selectedEntry.Hours = msg.hours
					h.selectedEntry.AllFromTime = formatAllTimeField(msg.fromTime)
					h.selectedEntry.AllToTime = formatAllTimeField(msg.toTime)
					h.selectedEntry.AllHours = msg.hours
				}
				break
			}
		}

		// Persist the update to the cache file for app restart
		if h.store != nil {
			_ = h.store.UpdateTimelogCacheEntry(msg.entryID, msg.description, msg.fromTime, msg.toTime, msg.hours)
		}

		// Invalidate description cache for this entry
		delete(h.descCache, h.cursor)
		return h, nil

	case historyUpdateErrorMsg:
		h.isSaving = false
		h.editError = msg.err.Error()
		h.editSuccess = ""

	case historySubmitSuccessMsg:
		h.isSubmitting = false
		h.confirmSubmit = false
		h.submitSuccess = fmt.Sprintf("Successfully submitted %d timesheet(s)!", h.pendingCount)
		// Launch fireworks celebration for 10 seconds, then reload history
		return h, tea.Batch(
			FireworksCelebrationCmd(10*time.Second),
		)

	case fireworksCompleteMsg:
		// Fireworks finished, reload history to get fresh data
		return h, h.loadHistory()

	case historySubmitErrorMsg:
		h.isSubmitting = false
		h.confirmSubmit = false
		h.err = msg.err

	case errorMsg:
		h.loading = false
		h.err = error(msg)
	}

	return h, nil
}

// updateListMode handles input in list mode
func (h *HistoryView) updateListMode(msg tea.KeyMsg) (*HistoryView, tea.Cmd) {
	// Handle confirmation dialog
	if h.confirmSubmit {
		switch msg.String() {
		case "y", "Y":
			// Confirm submit
			h.isSubmitting = true
			return h, h.submitAllPending()
		case "n", "N", "esc":
			// Cancel
			h.confirmSubmit = false
		}
		return h, nil
	}

	switch msg.String() {
	case "ctrl+c":
		return h, tea.Quit

	case "esc", "q":
		h.submitSuccess = "" // Clear success message
		// Calculate hours from local cache and return with updated data
		return h, h.returnToMenuWithHours()

	case "s":
		// Submit all pending - show confirmation
		if h.pendingCount > 0 && !h.isSubmitting {
			h.confirmSubmit = true
			h.submitSuccess = "" // Clear previous success message
		}

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

		// Return success with the updated values so we can update the local cache
		return historyUpdateSuccessMsg{
			entryID:     h.selectedEntry.ID,
			description: h.editFields.description.Value(),
			fromTime:    startTime.Format(time.RFC3339),
			toTime:      func() string { if endTimePtr != nil { return endTimePtr.Format(time.RFC3339) }; return "" }(),
			hours:       duration,
		}
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

// renderHeader renders the consistent header using the shared component
func (h *HistoryView) renderHeader() string {
	if h.headerState != nil {
		return RenderHeader("History", h.headerState)
	}
	// Fallback if header state not set
	return RenderHeader("History", &HeaderState{Username: h.username})
}

// SetHeaderState sets the shared header state
func (h *HistoryView) SetHeaderState(state *HeaderState) {
	h.headerState = state
}

// renderListView renders the list mode view
func (h *HistoryView) renderListView() string {
	header := h.renderHeader()

	if h.loading {
		loadingStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9A9EA0")).
			Align(lipgloss.Center)

		mainContent := loadingStyle.Render("Loading history...")

		mainSection := lipgloss.JoinVertical(
			lipgloss.Center,
			header,
			"",
			mainContent,
		)

		centeredMain := lipgloss.Place(
			h.width,
			h.height-2,
			lipgloss.Center,
			lipgloss.Top,
			mainSection,
		)

		return centeredMain
	}

	if h.err != nil {
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF6B6B")).
			Align(lipgloss.Center)

		mainContent := lipgloss.JoinVertical(
			lipgloss.Center,
			errorStyle.Render("Error: "+h.err.Error()),
		)

		mainSection := lipgloss.JoinVertical(
			lipgloss.Center,
			header,
			"",
			mainContent,
		)

		helpStyle := lipgloss.NewStyle().
			Width(h.width).
			Align(lipgloss.Center).
			Foreground(lipgloss.Color("#9A9EA0")).
			Italic(true)

		centeredMain := lipgloss.Place(
			h.width,
			h.height-2,
			lipgloss.Center,
			lipgloss.Top,
			mainSection,
		)

		return lipgloss.JoinVertical(
			lipgloss.Left,
			centeredMain,
			helpStyle.Render("Press 'r' to retry, Esc to go back"),
		)
	}

	if len(h.allHistory) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9A9EA0")).
			Align(lipgloss.Center)

		mainContent := emptyStyle.Render("No timesheet entries found")

		mainSection := lipgloss.JoinVertical(
			lipgloss.Center,
			header,
			"",
			mainContent,
		)

		helpStyle := lipgloss.NewStyle().
			Width(h.width).
			Align(lipgloss.Center).
			Foreground(lipgloss.Color("#9A9EA0")).
			Italic(true)

		centeredMain := lipgloss.Place(
			h.width,
			h.height-2,
			lipgloss.Center,
			lipgloss.Top,
			mainSection,
		)

		return lipgloss.JoinVertical(
			lipgloss.Left,
			centeredMain,
			helpStyle.Render("Press Esc to go back"),
		)
	}

	// Position and pending info
	positionInfo := fmt.Sprintf("Entry %d of %d", h.cursor+1, len(h.allHistory))
	posStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9A9EA0")).
		Align(lipgloss.Center)

	// Pending count badge
	var pendingBadge string
	if h.pendingCount > 0 {
		pendingBadge = lipgloss.NewStyle().
			Background(lipgloss.Color("#DDA036")).
			Foreground(lipgloss.Color("#000000")).
			Padding(0, 1).
			Bold(true).
			Render(fmt.Sprintf("%d PENDING", h.pendingCount))
	}

	table := h.renderScrollableTable()
	scrollBar := h.renderScrollBar()

	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9A9EA0")).
		Italic(true)

	tableWithScroll := lipgloss.JoinHorizontal(lipgloss.Top, table, " ", scrollBar)

	// Build info line with position and pending count
	infoLine := posStyle.Render(positionInfo)
	if pendingBadge != "" {
		infoLine = lipgloss.JoinHorizontal(lipgloss.Center, infoLine, "   ", pendingBadge)
	}

	// Success message
	var successMsg string
	if h.submitSuccess != "" {
		successMsg = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#569FC6")).
			Bold(true).
			Align(lipgloss.Center).
			Render(h.submitSuccess)
	}

	// Confirmation dialog
	var confirmDialog string
	if h.confirmSubmit {
		dialogStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#DDA036")).
			Padding(1, 2).
			Width(50).
			Align(lipgloss.Center)

		titleStyle := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#DDA036"))

		confirmDialog = dialogStyle.Render(lipgloss.JoinVertical(
			lipgloss.Center,
			titleStyle.Render("Submit All Pending Timesheets?"),
			"",
			fmt.Sprintf("This will submit %d timesheet(s) to the ERP.", h.pendingCount),
			"",
			"Press Y to confirm, N to cancel",
		))
	}

	// Submitting indicator
	var submittingMsg string
	if h.isSubmitting {
		submittingMsg = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#DDA036")).
			Bold(true).
			Align(lipgloss.Center).
			Render("Submitting timesheets...")
	}

	// Build main section
	var mainElements []string
	mainElements = append(mainElements, header, "")
	if successMsg != "" {
		mainElements = append(mainElements, successMsg, "")
	}
	if confirmDialog != "" {
		mainElements = append(mainElements, confirmDialog, "")
	}
	if submittingMsg != "" {
		mainElements = append(mainElements, submittingMsg, "")
	}
	mainElements = append(mainElements, infoLine, "", tableWithScroll)

	mainSection := lipgloss.JoinVertical(lipgloss.Center, mainElements...)

	centeredMain := lipgloss.Place(
		h.width,
		h.height-2,
		lipgloss.Center,
		lipgloss.Top,
		mainSection,
	)

	helpFooter := lipgloss.NewStyle().
		Width(h.width).
		Align(lipgloss.Center)

	// Build help text based on state
	var helpText string
	if h.confirmSubmit {
		helpText = "Y: Confirm • N: Cancel"
	} else if h.pendingCount > 0 {
		helpText = "↑/↓: Scroll • Enter: View • s: Submit All Pending • r: Refresh • Esc: Back"
	} else {
		helpText = "↑/↓: Scroll • Enter: View Details • r: Refresh • Esc/q: Back"
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		centeredMain,
		helpFooter.Render(helpStyle.Render(helpText)),
	)
}

// renderDetailView renders the detail view for a selected entry
func (h *HistoryView) renderDetailView() string {
	if h.selectedEntry == nil {
		return "No entry selected"
	}

	entry := h.selectedEntry
	header := h.renderHeader()

	// Styles
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

	// Status badge row
	statusRow := lipgloss.NewStyle().Align(lipgloss.Center).Width(62).Render(statusBadge)
	rows = append(rows, statusRow)
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
		Italic(true)

	var helpText string
	if entry.IsSubmitted() {
		helpText = "Esc: Back to list"
	} else {
		helpText = "e: Edit Entry • Esc: Back to list"
	}

	mainSection := lipgloss.JoinVertical(
		lipgloss.Center,
		header,
		"",
		content,
	)

	centeredMain := lipgloss.Place(
		h.width,
		h.height-2,
		lipgloss.Center,
		lipgloss.Top,
		mainSection,
	)

	helpFooter := lipgloss.NewStyle().
		Width(h.width).
		Align(lipgloss.Center)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		centeredMain,
		helpFooter.Render(helpStyle.Render(helpText)),
	)
}

// renderEditView renders the edit view for a selected entry
func (h *HistoryView) renderEditView() string {
	if h.selectedEntry == nil {
		return "No entry selected"
	}

	entry := h.selectedEntry
	header := h.renderHeader()

	// Styles
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
		Italic(true)

	mainSection := lipgloss.JoinVertical(
		lipgloss.Center,
		header,
		"",
		content,
	)

	centeredMain := lipgloss.Place(
		h.width,
		h.height-2,
		lipgloss.Center,
		lipgloss.Top,
		mainSection,
	)

	helpFooter := lipgloss.NewStyle().
		Width(h.width).
		Align(lipgloss.Center)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		centeredMain,
		helpFooter.Render(helpStyle.Render("Tab: Next field • Ctrl+S/Enter: Save • Esc: Cancel")),
	)
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

	// Column widths: 12+12+10+10+6+10 = 60 (matches header width)
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#DDA036")).
		Width(12).
		Align(lipgloss.Left)

	cellStyle := lipgloss.NewStyle().
		Width(12).
		Align(lipgloss.Left)

	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#000000")).
		Background(lipgloss.Color("#DDA036")).
		Width(12).
		Align(lipgloss.Left)

	descStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9A9EA0")).
		Width(56).
		Align(lipgloss.Left)

	selectedDescStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#000000")).
		Background(lipgloss.Color("#DDA036")).
		Width(56).
		Align(lipgloss.Left)

	header := lipgloss.JoinHorizontal(lipgloss.Top,
		headerStyle.Render("Project"),
		headerStyle.Render("Task"),
		headerStyle.Width(10).Render("Activity"),
		headerStyle.Width(10).Render("Date"),
		headerStyle.Width(6).Render("Hrs"),
		headerStyle.Width(10).Render("Status"),
	)

	var rows []string
	for i, entry := range visibleEntries {
		absoluteIdx := startIdx + i
		isSelected := absoluteIdx == h.cursor

		project := truncate(entry.ProjectName, 10)
		task := truncate(entry.TaskName, 10)
		activity := truncate(entry.Activity(), 8)
		dateStr := entry.StartTime().Format("01-02") // Shorter date format
		hours := fmt.Sprintf("%.1fh", entry.Duration())

		status := "Pend"
		if entry.IsSubmitted() {
			status = "✓ Sub"
		} else if entry.EndTime().IsZero() {
			status = "● Run"
		}

		var row1 string
		if isSelected {
			row1 = lipgloss.JoinHorizontal(lipgloss.Top,
				selectedStyle.Render(project),
				selectedStyle.Render(task),
				selectedStyle.Width(10).Render(activity),
				selectedStyle.Width(10).Render(dateStr),
				selectedStyle.Width(6).Render(hours),
				selectedStyle.Width(10).Render(status),
			)
		} else {
			row1 = lipgloss.JoinHorizontal(lipgloss.Top,
				cellStyle.Render(project),
				cellStyle.Render(task),
				cellStyle.Width(10).Render(activity),
				cellStyle.Width(10).Render(dateStr),
				cellStyle.Width(6).Render(hours),
				cellStyle.Width(10).Render(status),
			)
		}

		renderedDesc, ok := h.descCache[absoluteIdx]
		if !ok {
			desc := entry.GetDescriptionString()
			if desc == "" {
				renderedDesc = "(no description)"
			} else {
				renderedDesc = renderHistoryDescription(desc, 52)
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
				Render(strings.Repeat("─", 56))
			rows = append(rows, sep)
		}
	}

	var topIndicator, bottomIndicator string
	indicatorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#DDA036")).
		Bold(true).
		Align(lipgloss.Center).
		Width(56)

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

// returnToMenuWithHours calculates current hours from local cache and returns to menu
func (h *HistoryView) returnToMenuWithHours() tea.Cmd {
	return func() tea.Msg {
		now := time.Now()
		monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		monthEnd := monthStart.AddDate(0, 1, 0)
		todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		todayEnd := todayStart.AddDate(0, 0, 1)

		var monthlyHours, todayHours float64
		for _, entry := range h.allHistory {
			startTime := entry.StartTime()

			// Calculate monthly hours
			if (startTime.After(monthStart) || startTime.Equal(monthStart)) && startTime.Before(monthEnd) {
				monthlyHours += entry.Hours
			}

			// Calculate today hours (only completed entries)
			if (startTime.After(todayStart) || startTime.Equal(todayStart)) && startTime.Before(todayEnd) {
				if entry.ToTime != "" {
					todayHours += entry.Hours
				}
			}
		}

		return backToMenuWithHoursMsg{
			monthlyHours: monthlyHours,
			todayHours:   todayHours,
		}
	}
}

// Helper functions
func (h *HistoryView) loadHistory() tea.Cmd {
	return func() tea.Msg {
		if h.apiClient == nil {
			return errorMsg(fmt.Errorf("API client not available"))
		}

		entries, err := h.apiClient.GetTimelogs()
		if err != nil {
			// On API error, try to load from persistent cache
			if h.store != nil {
				if cache, cacheErr := h.store.LoadTimelogCache(); cacheErr == nil && cache != nil {
					return historyLoadedMsg{entries: cache.Entries}
				}
			}
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

// formatAllTimeField converts RFC3339 time to "YYYY-MM-DD HH:MM:SS" format for all_* fields
func formatAllTimeField(rfc3339Time string) string {
	t, err := time.Parse(time.RFC3339, rfc3339Time)
	if err != nil {
		return rfc3339Time // Return as-is if parsing fails
	}
	return t.Format("2006-01-02 15:04:05")
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

// submitAllPending submits all pending timesheet entries
func (h *HistoryView) submitAllPending() tea.Cmd {
	return func() tea.Msg {
		if h.apiClient == nil {
			return historySubmitErrorMsg{err: fmt.Errorf("API client not available")}
		}

		err := h.apiClient.SubmitAllPending()
		if err != nil {
			return historySubmitErrorMsg{err: err}
		}

		return historySubmitSuccessMsg{}
	}
}

// Message types
type historyLoadedMsg struct {
	entries []api.TimelogEntry
}

type historyUpdateSuccessMsg struct {
	entryID     int
	description string
	fromTime    string
	toTime      string
	hours       float64
}

type historyUpdateErrorMsg struct {
	err error
}

type historySubmitSuccessMsg struct{}

type historySubmitErrorMsg struct {
	err error
}

// backToMenuWithHoursMsg is sent when returning to menu with updated hours data
type backToMenuWithHoursMsg struct {
	monthlyHours float64
	todayHours   float64
}
