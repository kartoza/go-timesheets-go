package tui

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/kartoza/go-timesheets-go/internal/api"
)

// ComboBoxItem represents an item in a combo box
type ComboBoxItem struct {
	ID          string
	Label       string
	Description string
	Value       interface{}
}

func (i ComboBoxItem) FilterValue() string { return i.Label }
func (i ComboBoxItem) String() string      { return i.Label }

// ComboBoxItemDelegate handles the rendering of combo box items
type ComboBoxItemDelegate struct {
	selectedStyle   lipgloss.Style
	unselectedStyle lipgloss.Style
	dimStyle        lipgloss.Style
}

func NewComboBoxItemDelegate() ComboBoxItemDelegate {
	return ComboBoxItemDelegate{
		selectedStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#DDA036")).
			Bold(true).
			PaddingLeft(2),
		unselectedStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#569FC6")).
			PaddingLeft(2),
		dimStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#8A8B8B")).
			PaddingLeft(4),
	}
}

func (d ComboBoxItemDelegate) Height() int                             { return 1 }
func (d ComboBoxItemDelegate) Spacing() int                            { return 0 }
func (d ComboBoxItemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d ComboBoxItemDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	comboItem, ok := item.(ComboBoxItem)
	if !ok {
		return
	}

	var str string
	if index == m.Index() {
		str = d.selectedStyle.Render("► " + comboItem.Label)
		if comboItem.Description != "" {
			str += "\n" + d.dimStyle.Render(comboItem.Description)
		}
	} else {
		str = d.unselectedStyle.Render("  " + comboItem.Label)
		if comboItem.Description != "" {
			str += "\n" + d.dimStyle.Render(comboItem.Description)
		}
	}

	fmt.Fprint(w, str)
}

// ComboBox represents a searchable combo box widget
type ComboBox struct {
	list          list.Model
	textInput     textinput.Model
	items         []ComboBoxItem
	filteredItems []ComboBoxItem
	showList      bool
	placeholder   string
	selected      *ComboBoxItem
	width         int
	height        int
}

// NewComboBox creates a new combo box widget
func NewComboBox(items []ComboBoxItem, placeholder string, width, height int) ComboBox {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = width - 4

	l := list.New([]list.Item{}, NewComboBoxItemDelegate(), width, height-3)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.SetShowTitle(false)

	cb := ComboBox{
		list:          l,
		textInput:     ti,
		items:         items,
		filteredItems: items,
		showList:      false,
		placeholder:   placeholder,
		width:         width,
		height:        height,
	}

	cb.updateList()
	return cb
}

// Update handles messages for the combo box
func (cb *ComboBox) Update(msg tea.Msg) (ComboBox, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			if cb.showList && len(cb.filteredItems) > 0 {
				selectedItem := cb.filteredItems[cb.list.Index()]
				cb.selected = &selectedItem
				cb.textInput.SetValue(selectedItem.Label)
				cb.showList = false
				return *cb, nil
			}
		case "esc":
			if cb.showList {
				cb.showList = false
				return *cb, nil
			}
		case "down", "up":
			if cb.showList {
				cb.list, cmd = cb.list.Update(msg)
				cmds = append(cmds, cmd)
			} else {
				cb.showList = true
				cb.filterItems()
				cb.updateList()
			}
		default:
			cb.textInput, cmd = cb.textInput.Update(msg)
			cmds = append(cmds, cmd)

			// Filter items as user types
			cb.filterItems()
			cb.updateList()

			// Show list when user starts typing
			if len(cb.textInput.Value()) > 0 && !cb.showList {
				cb.showList = true
			}
		}
	}

	if cb.showList {
		cb.list, cmd = cb.list.Update(msg)
		cmds = append(cmds, cmd)
	}

	return *cb, tea.Batch(cmds...)
}

// View renders the combo box
func (cb *ComboBox) View() string {
	var b strings.Builder

	// Text input
	inputStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#DDA036")).
		Padding(0, 1).
		Width(cb.width - 2)

	if cb.textInput.Focused() {
		inputStyle = inputStyle.BorderForeground(lipgloss.Color("#DDA036"))
	}

	b.WriteString(inputStyle.Render(cb.textInput.View()))

	// List (if shown)
	if cb.showList && len(cb.filteredItems) > 0 {
		b.WriteString("\n")
		listStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#DDA036")).
			Width(cb.width - 2).
			Height(cb.height - 3)

		b.WriteString(listStyle.Render(cb.list.View()))
	}

	return b.String()
}

// filterItems filters the items based on the current input
func (cb *ComboBox) filterItems() {
	filter := strings.ToLower(cb.textInput.Value())
	cb.filteredItems = []ComboBoxItem{}

	for _, item := range cb.items {
		if strings.Contains(strings.ToLower(item.Label), filter) ||
			strings.Contains(strings.ToLower(item.Description), filter) {
			cb.filteredItems = append(cb.filteredItems, item)
		}
	}
}

// updateList updates the list with filtered items
func (cb *ComboBox) updateList() {
	items := make([]list.Item, len(cb.filteredItems))
	for i, item := range cb.filteredItems {
		items[i] = item
	}
	cb.list.SetItems(items)
}

// SetItems updates the items in the combo box
func (cb *ComboBox) SetItems(items []ComboBoxItem) {
	cb.items = items
	cb.filteredItems = items
	cb.updateList()
}

// GetSelected returns the currently selected item
func (cb *ComboBox) GetSelected() *ComboBoxItem {
	return cb.selected
}

// SetSelected sets the selected item
func (cb *ComboBox) SetSelected(item *ComboBoxItem) {
	cb.selected = item
	if item != nil {
		cb.textInput.SetValue(item.Label)
	} else {
		cb.textInput.SetValue("")
	}
}

// Focus gives focus to the combo box
func (cb *ComboBox) Focus() {
	cb.textInput.Focus()
}

// Blur removes focus from the combo box
func (cb *ComboBox) Blur() {
	cb.textInput.Blur()
	cb.showList = false
}

// ProjectComboBox creates a combo box for project selection
func ProjectComboBox(projects []api.ProjectListItem, width, height int) ComboBox {
	items := make([]ComboBoxItem, len(projects))
	for i, project := range projects {
		items[i] = ComboBoxItem{
			ID:          fmt.Sprintf("%d", project.ID),
			Label:       project.Label,
			Description: fmt.Sprintf("ID: %d", project.ID),
			Value:       project,
		}
	}
	return NewComboBox(items, "Select a project...", width, height)
}

// TaskComboBox creates a combo box for task selection
func TaskComboBox(tasks []api.TaskListItem, width, height int) ComboBox {
	items := make([]ComboBoxItem, len(tasks))
	for i, task := range tasks {
		items[i] = ComboBoxItem{
			ID:          fmt.Sprintf("%d", task.ID),
			Label:       task.Label,
			Description: task.Name,
			Value:       task,
		}
	}
	return NewComboBox(items, "Select a task...", width, height)
}

// ActivityComboBox creates a combo box for activity selection
func ActivityComboBox(activities []api.ActivityListItem, width, height int) ComboBox {
	items := make([]ComboBoxItem, len(activities))
	for i, activity := range activities {
		items[i] = ComboBoxItem{
			ID:          fmt.Sprintf("%d", activity.ID),
			Label:       activity.Label,
			Description: "Activity type",
			Value:       activity,
		}
	}
	return NewComboBox(items, "Select an activity...", width, height)
}

// TimeEntryComboBox creates a combo box for time entry selection
func TimesheetEntryComboBox(entries []api.TimelogEntry, width, height int) ComboBox {
	items := make([]ComboBoxItem, len(entries))
	for i, entry := range entries {
		description := fmt.Sprintf("%s - %s | %.1fh",
			entry.ProjectName,
			entry.Activity(),
			entry.Duration())

		if entry.IsSubmitted() {
			description += " [SUBMITTED]"
		}

		items[i] = ComboBoxItem{
			ID:          fmt.Sprintf("%d", entry.ID),
			Label:       fmt.Sprintf("%s: %s", entry.TaskName, entry.GetDescriptionString()),
			Description: description,
			Value:       entry,
		}
	}
	return NewComboBox(items, "Select a time entry...", width, height)
}

// RefreshableComboBox is a combo box that can refresh its data from the API
type RefreshableComboBox struct {
	ComboBox
	apiClient   *api.Client
	refreshFunc func(*api.Client) ([]ComboBoxItem, error)
}

// NewRefreshableProjectComboBox creates a refreshable project combo box
func NewRefreshableProjectComboBox(apiClient *api.Client, width, height int) RefreshableComboBox {
	cb := NewComboBox([]ComboBoxItem{}, "Loading projects...", width, height)

	return RefreshableComboBox{
		ComboBox:  cb,
		apiClient: apiClient,
		refreshFunc: func(client *api.Client) ([]ComboBoxItem, error) {
			projects, err := client.GetProjects("")
			if err != nil {
				return nil, err
			}

			items := make([]ComboBoxItem, len(projects))
			for i, project := range projects {
				items[i] = ComboBoxItem{
					ID:          fmt.Sprintf("%d", project.ID),
					Label:       project.Label,
					Description: fmt.Sprintf("ID: %d", project.ID),
					Value:       project,
				}
			}
			return items, nil
		},
	}
}

// NewRefreshableActivityComboBox creates a refreshable activity combo box
func NewRefreshableActivityComboBox(apiClient *api.Client, width, height int) RefreshableComboBox {
	cb := NewComboBox([]ComboBoxItem{}, "Loading activities...", width, height)

	return RefreshableComboBox{
		ComboBox:  cb,
		apiClient: apiClient,
		refreshFunc: func(client *api.Client) ([]ComboBoxItem, error) {
			activities, err := client.GetActivities()
			if err != nil {
				return nil, err
			}

			items := make([]ComboBoxItem, len(activities))
			for i, activity := range activities {
				items[i] = ComboBoxItem{
					ID:          fmt.Sprintf("%d", activity.ID),
					Label:       activity.Label,
					Description: "Activity type",
					Value:       activity,
				}
			}
			return items, nil
		},
	}
}

// RefreshData refreshes the combo box data from the API
func (rcb *RefreshableComboBox) RefreshData() tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		items, err := rcb.refreshFunc(rcb.apiClient)
		if err != nil {
			return ComboBoxErrorMsg{Err: err}
		}
		return ComboBoxDataMsg{Items: items}
	})
}

// ComboBoxDataMsg is sent when combo box data is loaded
type ComboBoxDataMsg struct {
	Items []ComboBoxItem
}

// ComboBoxErrorMsg is sent when there's an error loading combo box data
type ComboBoxErrorMsg struct {
	Err error
}

// Update handles the refreshable combo box updates
func (rcb *RefreshableComboBox) Update(msg tea.Msg) (RefreshableComboBox, tea.Cmd) {
	switch msg := msg.(type) {
	case ComboBoxDataMsg:
		rcb.ComboBox.SetItems(msg.Items)
		rcb.ComboBox.textInput.Placeholder = rcb.ComboBox.placeholder
		return *rcb, nil
	case ComboBoxErrorMsg:
		rcb.ComboBox.textInput.Placeholder = fmt.Sprintf("Error: %v", msg.Err)
		return *rcb, nil
	}

	var cmd tea.Cmd
	rcb.ComboBox, cmd = rcb.ComboBox.Update(msg)
	return *rcb, cmd
}

// Common message types used across TUI components
type errorMsg error

// ========================================
// Shared Header Component
// ========================================

// Brand colors used throughout the application
var (
	ColorOrange   = lipgloss.Color("#DDA036")
	ColorBlue     = lipgloss.Color("#569FC6")
	ColorGray     = lipgloss.Color("#9A9EA0")
	ColorWhite    = lipgloss.Color("#FFFFFF")
	ColorDarkGray = lipgloss.Color("#3A3A3A")
	ColorRed      = lipgloss.Color("#E95420")
	ColorGreen    = lipgloss.Color("#4CAF50")
)

// HeaderWidth is the standard width for the header
const HeaderWidth = 60

// HeaderState contains the dynamic state for the header
type HeaderState struct {
	Username     string
	IsActive     bool
	MonthlyHours float64
	BlinkOn      bool // For blinking status indicator
}

// RenderHeader renders the standard application header
// screenTitle should be the name of the current screen (e.g., "Main Menu", "Create Timer")
func RenderHeader(screenTitle string, state *HeaderState) string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorOrange).
		Align(lipgloss.Center).
		Width(HeaderWidth)

	mottoStyle := lipgloss.NewStyle().
		Italic(true).
		Foreground(ColorGray).
		Align(lipgloss.Center).
		Width(HeaderWidth)

	dividerStyle := lipgloss.NewStyle().
		Foreground(ColorGray).
		Width(HeaderWidth)

	statusStyle := lipgloss.NewStyle().
		Foreground(ColorWhite).
		Align(lipgloss.Center).
		Width(HeaderWidth)

	title := titleStyle.Render("Kartoza Timesheets - " + screenTitle)
	motto := mottoStyle.Render("tempus fugit")
	divider := dividerStyle.Render("────────────────────────────────────────────────────────────")

	// Build status line if state is provided
	var status string
	if state != nil {
		trackerState := "Inactive"
		stateColor := ColorGray
		if state.IsActive {
			// Blink the dot when timer is active
			if state.BlinkOn {
				trackerState = "● Active"
			} else {
				trackerState = "○ Active"
			}
			stateColor = ColorOrange
		}

		trackerStateStyled := lipgloss.NewStyle().
			Foreground(stateColor).
			Bold(true).
			Render(trackerState)

		statusLine := fmt.Sprintf("User: %s  |  Tracker: %s  |  Monthly Hours: %.1fh",
			state.Username,
			trackerStateStyled,
			state.MonthlyHours,
		)
		status = statusStyle.Render(statusLine)
	}

	if state != nil {
		return lipgloss.JoinVertical(
			lipgloss.Center,
			title,
			motto,
			divider,
			status,
			divider,
		)
	}

	return lipgloss.JoinVertical(
		lipgloss.Center,
		title,
		motto,
		divider,
	)
}

// RenderSimpleHeader renders a header without the full status bar
// Useful for screens that don't have access to the full state
func RenderSimpleHeader(screenTitle string, username string) string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorOrange).
		Align(lipgloss.Center).
		Width(HeaderWidth)

	mottoStyle := lipgloss.NewStyle().
		Italic(true).
		Foreground(ColorGray).
		Align(lipgloss.Center).
		Width(HeaderWidth)

	dividerStyle := lipgloss.NewStyle().
		Foreground(ColorGray).
		Width(HeaderWidth)

	statusStyle := lipgloss.NewStyle().
		Foreground(ColorWhite).
		Align(lipgloss.Center).
		Width(HeaderWidth)

	title := titleStyle.Render("Kartoza Timesheets - " + screenTitle)
	motto := mottoStyle.Render("tempus fugit")
	divider := dividerStyle.Render("────────────────────────────────────────────────────────────")

	if username != "" {
		status := statusStyle.Render("User: " + username)
		return lipgloss.JoinVertical(
			lipgloss.Center,
			title,
			motto,
			divider,
			status,
			divider,
		)
	}

	return lipgloss.JoinVertical(
		lipgloss.Center,
		title,
		motto,
		divider,
	)
}

// RenderHelpFooter renders the standard help footer at the bottom of the screen
func RenderHelpFooter(helpText string, width int) string {
	helpStyle := lipgloss.NewStyle().
		Foreground(ColorGray).
		Italic(true)

	footerStyle := lipgloss.NewStyle().
		Width(width).
		Align(lipgloss.Center)

	return footerStyle.Render(helpStyle.Render(helpText))
}

// LayoutWithHeaderFooter creates a standard layout with header at top and footer at bottom
func LayoutWithHeaderFooter(header, content, footer string, width, height int) string {
	// Main section with header and content
	mainSection := lipgloss.JoinVertical(
		lipgloss.Center,
		header,
		"",
		content,
	)

	// Center main content at top (leave room for footer)
	centeredMain := lipgloss.Place(
		width,
		height-2,
		lipgloss.Center,
		lipgloss.Top,
		mainSection,
	)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		centeredMain,
		footer,
	)
}

// ParseTaskLabelParts extracts task name and budget info from a task label
// Input: "Testing Scripts, Admin and meetings        (1890.01/2400.0)"
// Returns: taskName, hoursUsed, totalHours, hasBudget
func ParseTaskLabelParts(label string) (taskName string, hoursUsed string, totalHours string, hasBudget bool) {
	// Match pattern: "Task Name (hours/budget)" or "Task Name        (hours/budget)"
	idx := strings.LastIndex(label, "(")
	if idx == -1 {
		return label, "", "", false
	}

	// Check if it ends with closing paren and contains a slash
	suffix := label[idx:]
	if !strings.HasSuffix(suffix, ")") || !strings.Contains(suffix, "/") {
		return label, "", "", false
	}

	// Extract task name (trim trailing spaces)
	taskName = strings.TrimSpace(label[:idx])

	// Extract hours info - remove parens
	hoursInfo := suffix[1 : len(suffix)-1] // Remove ( and )
	parts := strings.Split(hoursInfo, "/")
	if len(parts) != 2 {
		return label, "", "", false
	}

	return taskName, parts[0], parts[1], true
}
